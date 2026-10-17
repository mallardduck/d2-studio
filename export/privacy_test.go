package export

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"d2studio/compiler"
	"d2studio/files"
	"d2studio/logging"
	"d2studio/settings"
	"d2studio/workspace"
)

// The logs may be attached to bug reports, so they must describe what the app
// did (sizes, paths, timings) and never what is in the user's documents.
// This runs the instrumented operations at the most verbose level, with a
// marker planted in the diagram text, an imported file and a saved file, and
// checks that the marker never reaches the log.
func TestTraceLoggingNeverContainsDocumentText(t *testing.T) {
	const marker = "ZZ_PRIVATE_DIAGRAM_TEXT_93f1"

	var logs bytes.Buffer
	logging.Init(logging.Options{Level: "trace", Format: "text", FileFormat: "text", Stderr: &logs, Getenv: func(string) string { return "" }})
	t.Cleanup(func() {
		logging.Init(logging.Options{Stderr: &bytes.Buffer{}, Getenv: func(string) string { return "" }})
	})

	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	mainPath := filepath.Join(dir, "main.d2")
	source := "...@part\nsecret: " + marker + "\nsecret -> other: " + marker
	if err := os.WriteFile(mainPath, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "part.d2"), []byte("imported: "+marker), 0o644); err != nil {
		t.Fatal(err)
	}

	svc, err := compiler.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	// Preview compiles, including one that fails.
	req := compiler.Request{Source: source, Path: mainPath}
	if _, err := svc.Compile(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Compile(context.Background(), compiler.Request{Source: "a: {\n" + marker, Path: mainPath}); err != nil {
		t.Fatal(err)
	}

	// Exports in every format through the service.
	plat := &fakePlatform{}
	exp := NewService(svc, plat)
	for _, f := range []Format{SVG, PNG, PDF} {
		plat.path = filepath.Join(dir, "out."+f.Extension())
		if _, err := exp.Export(context.Background(), Request{Format: f, Compile: req}); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
	}

	// File operations and the workspace.
	fsvc := files.NewService(&filesPlatform{path: mainPath}, settings.NewRecentStore(filepath.Join(dir, "recent.json")), time.Hour)
	res, err := fsvc.OpenPath(mainPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fsvc.Save(mainPath, source+"\nmore: "+marker, files.SaveOptions{Base: &res.Document.Version}); err != nil {
		t.Fatal(err)
	}
	fsvc.Shutdown()
	wsvc := workspace.NewService(&workspacePlatform{}, workspace.DefaultLimits, time.Hour)
	if _, err := wsvc.OpenFolder(dir); err != nil {
		t.Fatal(err)
	}
	wsvc.Close()

	out := logs.String()
	if !strings.Contains(out, "level=TRACE") || !strings.Contains(out, "component=compiler") ||
		!strings.Contains(out, "component=export") || !strings.Contains(out, "component=files") {
		t.Fatalf("the test did not exercise the instrumented code; log was:\n%s", out)
	}
	if strings.Contains(out, marker) {
		for _, line := range strings.Split(out, "\n") {
			if strings.Contains(line, marker) {
				t.Errorf("document text reached the log: %s", line)
			}
		}
	}
}

type filesPlatform struct{ path string }

func (p *filesPlatform) PickOpenFile(string) (string, error)         { return p.path, nil }
func (p *filesPlatform) PickSaveFile(string, string) (string, error) { return p.path, nil }
func (p *filesPlatform) Emit(string, any)                            {}

type workspacePlatform struct{}

func (*workspacePlatform) PickFolder(string) (string, error) { return "", nil }
func (*workspacePlatform) Emit(string, any)                  {}
