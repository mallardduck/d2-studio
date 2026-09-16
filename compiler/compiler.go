package compiler

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/d2lang/d2/d2lib"
	"github.com/d2lang/d2/d2plugin"
	"github.com/d2lang/d2/d2renderers/d2svg"
	"github.com/d2lang/d2/d2themes/d2themescatalog"
	d2log "github.com/d2lang/d2/lib/log"
	"github.com/d2lang/d2/lib/textmeasure"
	"github.com/d2lang/util-go/go2"
)

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

	// D2 expects a logger on the context; compiles are silent.
	ctx = d2log.With(ctx, slog.New(slog.NewTextHandler(io.Discard, nil)))
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
		return stale(seq), nil
	}

	start := time.Now()
	res := s.compileLocked(ctx, seq, req)
	res.Meta.DurationMs = time.Since(start).Milliseconds()

	if s.superseded(seq) || res.Status == StatusStale {
		return stale(seq), nil
	}
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

func (s *Service) compileLocked(ctx context.Context, seq uint64, req Request) (res *Result) {
	// A panic inside a layout engine must not take down the desktop app.
	defer func() {
		if r := recover(); r != nil {
			res = &Result{Seq: seq, Status: StatusError, Diagnostics: []Diagnostic{{
				Severity: SeverityError,
				Message:  fmt.Sprintf("internal compiler error: %v", r),
			}}}
		}
	}()
	layoutResolver, routerResolver := resolvers()

	copts := &d2lib.CompileOptions{
		UTF16Pos:       true,
		FS:             s.fsys,
		Ruler:          s.ruler,
		InputPath:      req.Path,
		LayoutResolver: layoutResolver,
		RouterResolver: routerResolver,
		LayoutReuse:    true,
	}
	ropts := &d2svg.RenderOpts{
		NoXMLTag:    go2.Pointer(true),
		OmitVersion: go2.Pointer(true),
	}

	layoutSource, themeSource := "default", "d2"
	if req.Options.Layout != "" {
		copts.Layout = go2.Pointer(lower(req.Options.Layout))
		layoutSource = "ui"
	}
	if req.Options.ThemeID != nil {
		ropts.ThemeID = req.Options.ThemeID
		themeSource = "ui"
	}
	ropts.Sketch = req.Options.Sketch

	if ctx.Err() != nil {
		return &Result{Seq: seq, Status: StatusStale}
	}
	diagram, g, err := d2lib.Compile(ctx, req.Source, copts, ropts)
	if err != nil || ctx.Err() != nil {
		// D2 does not always notice cancellation on small graphs, so a
		// canceled context is treated as stale even if compilation finished.
		if ctx.Err() != nil || isCancellation(err) {
			return &Result{Seq: seq, Status: StatusStale}
		}
		return &Result{Seq: seq, Status: StatusError, Diagnostics: diagnosticsFromError(err, req.Path)}
	}

	if layoutSource == "default" && diagram.Config != nil && diagram.Config.LayoutEngine != nil {
		layoutSource = "source"
	}

	// Mirror the D2 CLI: reject diagrams that use features the chosen
	// engine cannot lay out, instead of rendering something wrong.
	if plugin, ok := enginePlugins()[*copts.Layout]; ok {
		info, ierr := plugin.Info(ctx)
		if ierr == nil {
			if ferr := d2plugin.FeatureSupportCheck(info, g); ferr != nil {
				return &Result{Seq: seq, Status: StatusError, Diagnostics: []Diagnostic{{Severity: SeverityError, Message: ferr.Error()}}}
			}
		}
	}

	svg, err := d2svg.Render(diagram, ropts)
	if err != nil {
		return &Result{Seq: seq, Status: StatusError, Diagnostics: []Diagnostic{{Severity: SeverityError, Message: "render: " + err.Error()}}}
	}

	themeID := d2themescatalog.NeutralDefault.ID
	if ropts.ThemeID != nil {
		themeID = *ropts.ThemeID
	}
	tl, br := diagram.BoundingBox()
	return &Result{
		Seq:         seq,
		Status:      StatusSuccess,
		SVG:         string(svg),
		Diagnostics: []Diagnostic{},
		Meta: Meta{
			Layout:       *copts.Layout,
			ThemeID:      themeID,
			LayoutSource: layoutSource,
			ThemeSource:  themeSource,
			Width:        br.X - tl.X,
			Height:       br.Y - tl.Y,
		},
	}
}
