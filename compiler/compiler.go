package compiler

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"d2studio/logging"

	"github.com/d2lang/d2/d2compiler"
	"github.com/d2lang/d2/d2lib"
	"github.com/d2lang/d2/d2plugin"
	"github.com/d2lang/d2/d2renderers/d2svg"
	"github.com/d2lang/d2/d2target"
	"github.com/d2lang/d2/d2themes/d2themescatalog"
	d2log "github.com/d2lang/d2/lib/log"
	"github.com/d2lang/d2/lib/textmeasure"
	"github.com/d2lang/util-go/go2"
)

var logger = logging.For("compiler")

// Service compiles D2 source to SVG. It is safe for concurrent use. Requests
// are numbered; starting a new compilation cancels the one in flight, and a
// superseded request always returns StatusStale so callers can never apply an
// outdated result over a newer one.
type Service struct {
	// seq is the number of the most recently started request.
	seq atomic.Uint64

	cancelMu   sync.Mutex
	cancelPrev context.CancelFunc

	// run serializes compilations: the text ruler is not safe for
	// concurrent use and layout engines are CPU heavy.
	run   sync.Mutex
	ruler *textmeasure.Ruler

	fsys fs.FS
}

// New creates a Service. fsys, when non-nil, is used to resolve D2 imports.
func New(fsys fs.FS) (*Service, error) {
	ruler, err := textmeasure.NewRuler()
	if err != nil {
		return nil, fmt.Errorf("initialize text measurement: %w", err)
	}
	return &Service{ruler: ruler, fsys: fsys}, nil
}

// Compile compiles req. The returned error is reserved for failures of the
// service itself; D2 compilation errors are reported in Result.Diagnostics.
func (s *Service) Compile(ctx context.Context, req Request) (*Result, error) {
	seq := s.seq.Add(1)
	// Only sizes and settings are logged, never the source text.
	logger.Trace("compile requested", "seq", seq, "path", req.Path, "source_bytes", len(req.Source),
		"layout", req.Options.Layout, "theme_set", req.Options.ThemeID != nil, "workspace", req.Workspace)

	ctx = quietContext(ctx)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	s.cancelMu.Lock()
	if s.cancelPrev != nil {
		s.cancelPrev()
	}
	s.cancelPrev = cancel
	s.cancelMu.Unlock()

	if msg := validateOptions(req.Options); msg != "" {
		return &Result{Seq: seq, Status: StatusError, Diagnostics: []Diagnostic{{Severity: SeverityError, Message: msg}}}, nil
	}

	s.run.Lock()
	defer s.run.Unlock()

	// Superseded while waiting for the previous compilation to finish.
	if s.superseded(seq) {
		logger.Trace("compile skipped: superseded before it started", "seq", seq)
		return stale(seq), nil
	}

	start := time.Now()
	res := s.compileLocked(ctx, seq, req)
	res.Meta.DurationMs = time.Since(start).Milliseconds()

	if s.superseded(seq) || res.Status == StatusStale {
		logger.Trace("compile result discarded: superseded", "seq", seq, "took_ms", res.Meta.DurationMs)
		return stale(seq), nil
	}
	logger.Debug("compiled", "seq", seq, "status", res.Status, "layout", res.Meta.Layout,
		"theme_id", res.Meta.ThemeID, "took_ms", res.Meta.DurationMs, "svg_bytes", len(res.SVG),
		"diagnostics", len(res.Diagnostics), "path", req.Path)
	return res, nil
}

func (s *Service) superseded(seq uint64) bool { return s.seq.Load() != seq }

func stale(seq uint64) *Result {
	return &Result{Seq: seq, Status: StatusStale, Diagnostics: []Diagnostic{}}
}

func validateOptions(o Options) string {
	if o.Layout != "" {
		if _, ok := enginePlugins()[lower(o.Layout)]; !ok {
			return fmt.Sprintf("layout engine %q is not available", o.Layout)
		}
	}
	if o.ThemeID != nil && !validTheme(*o.ThemeID) {
		return fmt.Sprintf("theme %d is not a known D2 theme", *o.ThemeID)
	}
	return ""
}

// compiled is the result of the D2 pipeline up to, but not including, SVG
// rendering. It is what the live preview and the exporters both build on.
type compiled struct {
	diagram  *d2target.Diagram
	copts    *d2lib.CompileOptions
	ropts    *d2svg.RenderOpts
	importFS fs.FS
}

// compileDiagram compiles req into a laid-out diagram. The caller must hold
// s.run and call cleanup when done with the result. On failure it returns the
// diagnostics to report; stale is true when ctx was canceled, in which case
// there is nothing to report.
func (s *Service) compileDiagram(ctx context.Context, req Request, ropts *d2svg.RenderOpts) (c *compiled, cleanup func(), diags []Diagnostic, stale bool) {
	noop := func() {}
	layoutResolver, routerResolver := resolvers()

	importFS, closeFS, err := s.importFS(req)
	if err != nil {
		return nil, noop, []Diagnostic{{Severity: SeverityError, Message: fmt.Sprintf("cannot prepare imports: %v", err)}}, false
	}

	copts := &d2lib.CompileOptions{
		UTF16Pos:       true,
		FS:             importFS,
		Ruler:          s.ruler,
		InputPath:      req.Path,
		LayoutResolver: layoutResolver,
		RouterResolver: routerResolver,
		LayoutReuse:    true,
	}
	if req.Options.Layout != "" {
		copts.Layout = go2.Pointer(lower(req.Options.Layout))
	}
	if req.Options.ThemeID != nil {
		ropts.ThemeID = req.Options.ThemeID
	}
	ropts.Sketch = req.Options.Sketch

	if ctx.Err() != nil {
		closeFS()
		return nil, noop, nil, true
	}
	diagram, g, err := d2lib.Compile(ctx, req.Source, copts, ropts)
	if err != nil || ctx.Err() != nil {
		closeFS()
		// D2 does not always notice cancellation on small graphs, so a
		// canceled context is treated as stale even if compilation finished.
		if ctx.Err() != nil || isCancellation(err) {
			return nil, noop, nil, true
		}
		return nil, noop, diagnosticsFromError(err, req.Path), false
	}

	// Mirror the D2 CLI: reject diagrams that use features the chosen
	// engine cannot lay out, instead of rendering something wrong.
	if plugin, ok := enginePlugins()[*copts.Layout]; ok {
		info, ierr := plugin.Info(ctx)
		if ierr == nil {
			if ferr := d2plugin.FeatureSupportCheck(info, g); ferr != nil {
				closeFS()
				return nil, noop, []Diagnostic{{Severity: SeverityError, Message: ferr.Error()}}, false
			}
		}
	}
	return &compiled{diagram: diagram, copts: copts, ropts: ropts, importFS: importFS}, closeFS, nil, false
}

// quietContext attaches the logger D2 expects on its context. Compiles are
// silent; without it D2 prints a warning with a stack trace on every call.
func quietContext(ctx context.Context) context.Context {
	return d2log.With(ctx, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func internalError(r any) []Diagnostic {
	return []Diagnostic{{Severity: SeverityError, Message: fmt.Sprintf("internal compiler error: %v", r)}}
}

func (s *Service) compileLocked(ctx context.Context, seq uint64, req Request) (res *Result) {
	// A panic inside a layout engine must not take down the desktop app.
	defer func() {
		if r := recover(); r != nil {
			res = &Result{Seq: seq, Status: StatusError, Diagnostics: internalError(r)}
		}
	}()

	ropts := &d2svg.RenderOpts{
		NoXMLTag:    go2.Pointer(true),
		OmitVersion: go2.Pointer(true),
	}
	c, cleanup, diags, isStale := s.compileDiagram(ctx, req, ropts)
	if isStale {
		return &Result{Seq: seq, Status: StatusStale}
	}
	if diags != nil {
		return &Result{Seq: seq, Status: StatusError, Diagnostics: diags}
	}
	defer cleanup()

	srcLayout, srcTheme := s.sourceConfig(req, c.importFS)
	layoutSource := pickSource(req.Options.Layout != "", srcLayout != "")
	themeSource := pickSource(req.Options.ThemeID != nil, srcTheme != nil)

	svg, err := d2svg.Render(c.diagram, ropts)
	if err != nil {
		return &Result{Seq: seq, Status: StatusError, Diagnostics: []Diagnostic{{Severity: SeverityError, Message: "render: " + err.Error()}}}
	}

	themeID := d2themescatalog.NeutralDefault.ID
	if ropts.ThemeID != nil {
		themeID = *ropts.ThemeID
	}
	tl, br := c.diagram.BoundingBox()
	return &Result{
		Seq:         seq,
		Status:      StatusSuccess,
		SVG:         string(svg),
		Diagnostics: []Diagnostic{},
		Meta: Meta{
			Layout:        *c.copts.Layout,
			ThemeID:       themeID,
			LayoutSource:  layoutSource,
			ThemeSource:   themeSource,
			SourceLayout:  srcLayout,
			SourceThemeID: srcTheme,
			Width:         br.X - tl.X,
			Height:        br.Y - tl.Y,
		},
	}
}

// ExportInput is a compiled diagram ready to be rendered to a file format.
type ExportInput struct {
	Diagram *d2target.Diagram
	// Render holds the resolved theme, padding, sketch mode and so on.
	Render d2svg.RenderOpts
	// ImportRoot is the folder local images may be read from, or "" for an
	// unsaved document.
	ImportRoot string
	// FileDir is the folder of the file being exported. Relative image paths
	// in the diagram are relative to it. "" for an unsaved document.
	FileDir string
}

// CompileForExport compiles req for export. Unlike Compile it does not take
// part in stale-result tracking, so exporting never cancels or is cancelled
// by the live preview. D2 errors come back as diagnostics, and the diagram is
// nil in that case.
func (s *Service) CompileForExport(ctx context.Context, req Request) (in *ExportInput, diags []Diagnostic, err error) {
	if msg := validateOptions(req.Options); msg != "" {
		return nil, []Diagnostic{{Severity: SeverityError, Message: msg}}, nil
	}
	ctx = quietContext(ctx)
	s.run.Lock()
	defer s.run.Unlock()
	defer func() {
		if r := recover(); r != nil {
			in, diags, err = nil, internalError(r), nil
		}
	}()

	ropts := &d2svg.RenderOpts{}
	c, cleanup, diags, isStale := s.compileDiagram(ctx, req, ropts)
	if isStale {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		return nil, nil, context.Canceled
	}
	if diags != nil {
		return nil, diags, nil
	}
	defer cleanup()
	logger.Debug("compiled for export", "path", req.Path, "source_bytes", len(req.Source))
	in = &ExportInput{Diagram: c.diagram, Render: *ropts, ImportRoot: ImportRoot(req.Path, req.Workspace)}
	if req.Path != "" {
		if abs, aerr := filepath.Abs(req.Path); aerr == nil {
			in.FileDir = filepath.Dir(abs)
		}
	}
	return in, nil, nil
}

// sourceConfig reports the layout engine and theme the .d2 source itself
// declares in d2-config. D2 overwrites its own config with the resolved
// values, so this reads the config directly; it parses and builds the IR but
// runs no layout, so it is cheap next to a full compile.
func (s *Service) sourceConfig(req Request, importFS fs.FS) (layout string, themeID *int64) {
	_, cfg, err := d2compiler.Compile(req.Path, strings.NewReader(req.Source), &d2compiler.CompileOptions{
		UTF16Pos: true,
		FS:       importFS,
	})
	if err != nil || cfg == nil {
		return "", nil
	}
	if cfg.LayoutEngine != nil {
		layout = strings.ToLower(*cfg.LayoutEngine)
	}
	return layout, cfg.ThemeID
}

func pickSource(ui, source bool) string {
	switch {
	case ui:
		return "ui"
	case source:
		return "source"
	default:
		return "default"
	}
}

// importFS returns the filesystem imports are read from for req, and a
// function that releases it. A fixed Service.fsys (used by tests) wins;
// otherwise reads are confined to the import root. A nil filesystem means
// there is nothing to resolve imports against.
func (s *Service) importFS(req Request) (fs.FS, func(), error) {
	noop := func() {}
	if s.fsys != nil {
		return s.fsys, noop, nil
	}
	root := ImportRoot(req.Path, req.Workspace)
	if root == "" {
		return nil, noop, nil
	}
	r, err := newRootFS(root)
	if err != nil {
		return nil, noop, err // not r: a nil *rootFS would be a non-nil fs.FS
	}
	return r, func() { r.Close() }, nil
}
