package export

import (
	"bytes"
	"context"
	"errors"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"d2studio/compiler"
)

type fakePlatform struct {
	path    string
	err     error
	picks   int
	gotDir  string
	gotName string
	gotFmt  Format
}

func (f *fakePlatform) PickExportFile(dir, name string, format Format) (string, error) {
	f.picks++
	f.gotDir, f.gotName, f.gotFmt = dir, name, format
	return f.path, f.err
}

func newService(t *testing.T) (*Service, *fakePlatform) {
	t.Helper()
	c, err := compiler.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	p := &fakePlatform{}
	return NewService(c, p), p
}

func req(f Format, source string) Request {
	return Request{Format: f, Compile: compiler.Request{Source: source}, SuggestedName: "my diagram.d2", Dir: "/start"}
}

func TestExportWritesAValidPNG(t *testing.T) {
	s, p := newService(t)
	p.path = filepath.Join(t.TempDir(), "out.png")

	res, err := s.Export(context.Background(), req(PNG, "a -> b"))
	if err != nil {
		t.Fatal(err)
	}
	if res.Cancelled || res.Path != p.path || res.Bytes == 0 {
		t.Fatalf("res = %+v", res)
	}
	data, err := os.ReadFile(p.path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != res.Bytes {
		t.Fatalf("reported %d bytes, file has %d", res.Bytes, len(data))
	}
	if _, err := png.Decode(bytes.NewReader(data)); err != nil {
		t.Fatalf("the written file is not a valid PNG: %v", err)
	}
	if p.gotName != "my diagram.png" || p.gotDir != "/start" || p.gotFmt != PNG {
		t.Fatalf("dialog got dir=%q name=%q format=%q", p.gotDir, p.gotName, p.gotFmt)
	}
}

func TestEveryFormatWritesItsFile(t *testing.T) {
	s, p := newService(t)
	wants := map[Format]string{SVG: "<svg", PNG: "\x89PNG", PDF: "%PDF-"}
	for f, magic := range wants {
		p.path = filepath.Join(t.TempDir(), "out."+f.Extension())
		if _, err := s.Export(context.Background(), req(f, "a -> b")); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		data, _ := os.ReadFile(p.path)
		if !strings.Contains(string(data[:min(len(data), 200)]), magic) {
			t.Errorf("%s: file does not look like %s: %.20q", f, f, data)
		}
	}
}

func TestCancelledDialogWritesNothing(t *testing.T) {
	s, p := newService(t)
	dir := t.TempDir()
	p.path = ""
	res, err := s.Export(context.Background(), req(PNG, "a -> b"))
	if err != nil || !res.Cancelled || res.Path != "" {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatalf("files created: %v", entries)
	}
}

// A diagram with errors must be reported before the user is asked where to
// save a file that cannot be produced.
func TestDiagramWithErrorsNeverOpensTheDialogOrWritesAnything(t *testing.T) {
	s, p := newService(t)
	p.path = filepath.Join(t.TempDir(), "out.png")
	res, err := s.Export(context.Background(), req(PNG, "a -> b\nc: {"))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Diagnostics) == 0 || res.Path != "" || res.Cancelled {
		t.Fatalf("res = %+v", res)
	}
	if p.picks != 0 {
		t.Fatal("the save dialog opened for a diagram that cannot be exported")
	}
	if _, err := os.Stat(p.path); err == nil {
		t.Fatal("a file was written")
	}
}

func TestWriteFailureIsAnErrorNotASuccess(t *testing.T) {
	s, p := newService(t)
	dir := t.TempDir()
	p.path = filepath.Join(dir, "no-such-folder", "out.png")
	res, err := s.Export(context.Background(), req(PNG, "a -> b"))
	if err == nil {
		t.Fatalf("a failed write was reported as success: %+v", res)
	}
	if !strings.Contains(err.Error(), "out.png") {
		t.Errorf("the error should name the file: %v", err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatalf("left behind: %v", entries)
	}
}

func TestPickerErrorIsReturned(t *testing.T) {
	s, p := newService(t)
	p.err = errors.New("dialog exploded")
	if _, err := s.Export(context.Background(), req(PNG, "a -> b")); err == nil {
		t.Fatal("expected the error")
	}
}

func TestExtensionIsAddedWhenOmitted(t *testing.T) {
	s, p := newService(t)
	dir := t.TempDir()
	p.path = filepath.Join(dir, "diagram") // the user typed no extension
	res, err := s.Export(context.Background(), req(PDF, "a -> b"))
	if err != nil || res.Path != filepath.Join(dir, "diagram.pdf") {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}

func TestUnknownFormatIsRejectedBeforeAnyWork(t *testing.T) {
	s, p := newService(t)
	if _, err := s.Export(context.Background(), req("bmp", "a -> b")); err == nil {
		t.Fatal("expected an error")
	}
	if p.picks != 0 {
		t.Fatal("the dialog opened for an unknown format")
	}
}

func TestBlockedRemoteImagesAreReported(t *testing.T) {
	s, p := newService(t)
	p.path = filepath.Join(t.TempDir(), "out.png")
	res, err := s.Export(context.Background(), req(PNG, "x: {shape: image; icon: https://icons.example.test/a.png}"))
	if err != nil {
		t.Fatal(err)
	}
	if res.RemoteImages != 1 || !res.RemoteImagesBlocked {
		t.Fatalf("res = %+v; the user must be told images were replaced", res)
	}
}

func TestExportName(t *testing.T) {
	cases := []struct {
		in   string
		f    Format
		want string
	}{
		{"architecture.d2", PNG, "architecture.png"},
		{"my diagram.d2", PDF, "my diagram.pdf"},
		{"/some/dir/a.b.d2", SVG, "a.b.svg"},
		{"Untitled.d2", PNG, "Untitled.png"},
		{"noext", PNG, "noext.png"},
		{"", PNG, "diagram.png"},
		{".d2", PNG, "diagram.png"},
	}
	for _, c := range cases {
		if got := ExportName(c.in, c.f); got != c.want {
			t.Errorf("ExportName(%q, %s) = %q, want %q", c.in, c.f, got, c.want)
		}
	}
}
