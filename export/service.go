package export

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"d2studio/compiler"
	"d2studio/files"
	"d2studio/logging"
)

var logger = logging.For("export")

// Platform is what the Service needs from the desktop shell.
type Platform interface {
	// PickExportFile shows a native save dialog for the format. It returns
	// "" if the user cancelled.
	PickExportFile(dir, filename string, format Format) (string, error)
}

// Compiler produces the diagram to export. *compiler.Service implements it.
type Compiler interface {
	CompileForExport(ctx context.Context, req compiler.Request) (*compiler.ExportInput, []compiler.Diagnostic, error)
}

// Service is the frontend-facing export API.
type Service struct {
	comp Compiler
	plat Platform
}

// NewService creates a Service.
func NewService(comp Compiler, plat Platform) *Service {
	return &Service{comp: comp, plat: plat}
}

// Request asks for an export of the document being edited.
type Request struct {
	Format  Format           `json:"format"`
	Compile compiler.Request `json:"compile"`
	// SuggestedName is the open file's name, used to propose a file name.
	SuggestedName string `json:"suggestedName"`
	// Dir is where the save dialog starts.
	Dir string `json:"dir"`
	// AllowRemoteImages lets PNG and PDF download remote images.
	AllowRemoteImages bool `json:"allowRemoteImages"`
}

// Result describes what happened. Success is only reported (Path set) after
// the file has been completely written.
type Result struct {
	// Cancelled is true when the user dismissed the save dialog.
	Cancelled bool `json:"cancelled"`
	// Diagnostics is set when the diagram has errors and so nothing was
	// exported.
	Diagnostics []compiler.Diagnostic `json:"diagnostics"`
	// Path and Bytes describe the file that was written.
	Path  string `json:"path"`
	Bytes int    `json:"bytes"`
	// RemoteImages and RemoteImagesBlocked: see Output.
	RemoteImages        int  `json:"remoteImages"`
	RemoteImagesBlocked bool `json:"remoteImagesBlocked"`
}

// Export renders the document and, after the user chooses a destination,
// writes it. The diagram is rendered before the save dialog opens, so
// problems are reported before the user is asked where to put a file that
// could not be made. Failures are returned as errors; they are never reported
// as a success.
func (s *Service) Export(ctx context.Context, req Request) (*Result, error) {
	switch req.Format {
	case SVG, PNG, PDF:
	default:
		return nil, fmt.Errorf("unknown export format %q", req.Format)
	}

	start := time.Now()
	logger.Debug("export requested", "format", req.Format, "allow_remote_images", req.AllowRemoteImages,
		"path", req.Compile.Path)
	in, diags, err := s.comp.CompileForExport(ctx, req.Compile)
	if err != nil {
		logger.Debug("export failed: compile", "err", err)
		return nil, err
	}
	if in == nil {
		logger.Debug("export not started: the diagram has errors", "diagnostics", len(diags))
		return &Result{Diagnostics: diags}, nil
	}

	out, err := Render(ctx, req.Format, in, Options{AllowRemoteImages: req.AllowRemoteImages})
	if err != nil {
		logger.Debug("export failed: render", "format", req.Format, "err", err)
		return nil, err
	}
	logger.Trace("rendered", "format", req.Format, "bytes", len(out.Data), "took_ms", time.Since(start).Milliseconds())

	path, err := s.plat.PickExportFile(req.Dir, ExportName(req.SuggestedName, req.Format), req.Format)
	if err != nil {
		return nil, err
	}
	if path == "" {
		logger.Debug("export cancelled at the save dialog", "format", req.Format)
		return &Result{Cancelled: true}, nil
	}
	path = withExtension(path, req.Format)

	if err := files.WriteBytes(path, out.Data); err != nil {
		logger.Debug("export failed: write", "path", path, "err", err)
		return nil, fmt.Errorf("could not write %s: %w", filepath.Base(path), err)
	}
	logger.Debug("exported", "format", req.Format, "path", path, "bytes", len(out.Data),
		"remote_images", out.RemoteImages, "remote_blocked", out.RemoteImagesBlocked,
		"took_ms", time.Since(start).Milliseconds())
	return &Result{
		Path:                path,
		Bytes:               len(out.Data),
		RemoteImages:        out.RemoteImages,
		RemoteImagesBlocked: out.RemoteImagesBlocked,
	}, nil
}

// ExportName proposes a file name: the document's name with its extension
// replaced by the export format's.
func ExportName(documentName string, f Format) string {
	base := filepath.Base(documentName)
	if base == "." || base == string(filepath.Separator) || base == "" {
		base = "diagram"
	}
	if ext := filepath.Ext(base); ext != "" {
		base = strings.TrimSuffix(base, ext)
	}
	if base == "" {
		base = "diagram"
	}
	return base + "." + f.Extension()
}

// withExtension adds the format's extension when the user typed none.
func withExtension(path string, f Format) string {
	if filepath.Ext(path) == "" {
		return path + "." + f.Extension()
	}
	return path
}
