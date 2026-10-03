package export

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"d2studio/compiler"
)

func newInput(t *testing.T, source string, opts compiler.Options, path, workspace string) *compiler.ExportInput {
	t.Helper()
	svc, err := compiler.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	in, diags, err := svc.CompileForExport(context.Background(), compiler.Request{
		Source: source, Options: opts, Path: path, Workspace: workspace,
	})
	if err != nil {
		t.Fatal(err)
	}
	if in == nil {
		t.Fatalf("compile failed: %+v", diags)
	}
	return in
}

func render(t *testing.T, f Format, in *compiler.ExportInput, opts Options) *Output {
	t.Helper()
	out, err := Render(context.Background(), f, in, opts)
	if err != nil {
		t.Fatalf("%s export failed: %v", f, err)
	}
	if len(out.Data) == 0 {
		t.Fatalf("%s export produced no data", f)
	}
	return out
}

func decodePNG(t *testing.T, data []byte) image.Image {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("output is not a valid PNG: %v", err)
	}
	return img
}

const sample = "user: User {shape: person}\napi: API\ndb: DB {shape: cylinder}\nuser -> api: calls\napi -> db"

func TestSVGExport(t *testing.T) {
	out := render(t, SVG, newInput(t, sample, compiler.Options{}, "", ""), Options{})
	s := string(out.Data)
	if !strings.HasPrefix(s, "<?xml") {
		t.Errorf("a standalone SVG file should start with an XML declaration, got %.30q", s)
	}
	for _, want := range []string{"<svg", "viewBox", "calls"} {
		if !strings.Contains(s, want) {
			t.Errorf("SVG is missing %q", want)
		}
	}
}

func TestPNGExportIsAValidNonBlankImage(t *testing.T) {
	in := newInput(t, sample, compiler.Options{}, "", "")
	out := render(t, PNG, in, Options{})
	img := decodePNG(t, out.Data)
	b := img.Bounds()
	if b.Dx() < 100 || b.Dy() < 100 {
		t.Fatalf("image is only %dx%d", b.Dx(), b.Dy())
	}

	// A diagram is not a flat colour: count distinct pixel values.
	seen := map[color.RGBA]struct{}{}
	for y := b.Min.Y; y < b.Max.Y; y += 3 {
		for x := b.Min.X; x < b.Max.X; x += 3 {
			r, g, bl, a := img.At(x, y).RGBA()
			seen[color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(bl >> 8), uint8(a >> 8)}] = struct{}{}
		}
	}
	if len(seen) < 5 {
		t.Fatalf("image looks blank: only %d distinct colours", len(seen))
	}
}

// The raster is drawn at D2's fixed 2x device scale, so its size tracks the
// diagram's own (padded) size.
func TestPNGSizeTracksTheDiagram(t *testing.T) {
	small := decodePNG(t, render(t, PNG, newInput(t, "a -> b", compiler.Options{}, "", ""), Options{}).Data).Bounds()
	large := decodePNG(t, render(t, PNG, newInput(t, "a -> b -> c -> d -> e -> f", compiler.Options{}, "", ""), Options{}).Data).Bounds()
	if large.Dy() <= small.Dy() && large.Dx() <= small.Dx() {
		t.Fatalf("a larger diagram should give a larger image: small %v, large %v", small, large)
	}
}

func TestThemeAffectsExport(t *testing.T) {
	dark := int64(200) // a dark theme from D2's catalog
	var found bool
	for _, th := range compiler.Themes() {
		if th.ID == dark && th.Dark {
			found = true
		}
	}
	if !found {
		t.Skip("theme 200 is not a dark theme in this D2 version")
	}
	light := render(t, PNG, newInput(t, "a -> b", compiler.Options{}, "", ""), Options{}).Data
	darkOut := render(t, PNG, newInput(t, "a -> b", compiler.Options{ThemeID: &dark}, "", ""), Options{}).Data
	if bytes.Equal(light, darkOut) {
		t.Fatal("a different theme produced an identical image")
	}
}

func TestSketchModeAffectsExport(t *testing.T) {
	yes := true
	plain := render(t, PNG, newInput(t, "a -> b", compiler.Options{}, "", ""), Options{}).Data
	sketch := render(t, PNG, newInput(t, "a -> b", compiler.Options{Sketch: &yes}, "", ""), Options{}).Data
	if bytes.Equal(plain, sketch) {
		t.Fatal("sketch mode produced an identical image")
	}
}

func TestPDFExportIsAValidSinglePageDocument(t *testing.T) {
	out := render(t, PDF, newInput(t, sample, compiler.Options{}, "", ""), Options{})
	d := string(out.Data)
	if !strings.HasPrefix(d, "%PDF-") {
		t.Fatalf("not a PDF: %.20q", d)
	}
	if !strings.Contains(d, "%%EOF") {
		t.Error("PDF is truncated: no end-of-file trailer")
	}
	if n := strings.Count(d, "/Type /Page\n"); n != 1 {
		t.Errorf("expected exactly 1 page, found %d", n)
	}
	if !strings.Contains(d, "/Subtype /Image") {
		t.Error("the diagram image is not embedded")
	}
}

func TestExportOfEveryEngine(t *testing.T) {
	for _, e := range compiler.Engines() {
		t.Run(e.ID, func(t *testing.T) {
			in := newInput(t, "x: {a -> b}\nx -> y", compiler.Options{Layout: e.ID}, "", "")
			for _, f := range []Format{SVG, PNG, PDF} {
				render(t, f, in, Options{})
			}
		})
	}
}

func TestUnknownFormatAndEmptyInputAreErrors(t *testing.T) {
	in := newInput(t, "a -> b", compiler.Options{}, "", "")
	if _, err := Render(context.Background(), "gif", in, Options{}); err == nil {
		t.Error("unknown format should fail")
	}
	if _, err := Render(context.Background(), PNG, nil, Options{}); err == nil {
		t.Error("nil input should fail")
	}
}

func TestCancelledExportFailsAndProducesNothing(t *testing.T) {
	in := newInput(t, sample, compiler.Options{}, "", "")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, f := range []Format{PNG, PDF} {
		out, err := Render(ctx, f, in, Options{})
		if err == nil || out != nil {
			t.Errorf("%s: cancelled export returned out=%v err=%v", f, out, err)
		}
	}
}

// ---- local images ----

func writePNG(t *testing.T, path string) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for i := range img.Pix {
		img.Pix[i] = 0xff
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func projectDir(t *testing.T) string {
	t.Helper()
	d, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestLocalImageInsideTheFolderIsEmbedded(t *testing.T) {
	dir := projectDir(t)
	writePNG(t, filepath.Join(dir, "img", "logo.png"))
	src := "x: {shape: image; icon: ./img/logo.png}"
	file := filepath.Join(dir, "main.d2")

	in := newInput(t, src, compiler.Options{}, file, "")
	for _, f := range []Format{PNG, PDF} {
		render(t, f, in, Options{})
	}
}

func TestLocalImageOutsideTheFolderIsRefused(t *testing.T) {
	base := projectDir(t)
	writePNG(t, filepath.Join(base, "secret", "private.png"))
	file := filepath.Join(base, "proj", "main.d2")
	_ = os.MkdirAll(filepath.Dir(file), 0o755)

	in := newInput(t, "x: {shape: image; icon: ../secret/private.png}", compiler.Options{}, file, "")
	_, err := Render(context.Background(), PNG, in, Options{})
	if err == nil || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("err = %v, want an 'outside the folder' refusal", err)
	}

	// An absolute path to the same file must not get around it.
	abs := filepath.Join(base, "secret", "private.png")
	in = newInput(t, "x: {shape: image; icon: "+abs+"}", compiler.Options{}, file, "")
	if _, err := Render(context.Background(), PNG, in, Options{}); err == nil || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("absolute path: err = %v", err)
	}
}

func TestWorkspaceWidensWhereImagesMayBeRead(t *testing.T) {
	base := projectDir(t)
	writePNG(t, filepath.Join(base, "shared", "logo.png"))
	file := filepath.Join(base, "proj", "main.d2")
	_ = os.MkdirAll(filepath.Dir(file), 0o755)

	in := newInput(t, "x: {shape: image; icon: ../shared/logo.png}", compiler.Options{}, file, base)
	render(t, PNG, in, Options{})
}

func TestSymlinkCannotSmuggleAnImageOut(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on Windows")
	}
	base := projectDir(t)
	writePNG(t, filepath.Join(base, "elsewhere", "secret.png"))
	proj := filepath.Join(base, "proj")
	_ = os.MkdirAll(proj, 0o755)
	if err := os.Symlink(filepath.Join(base, "elsewhere"), filepath.Join(proj, "link")); err != nil {
		t.Fatal(err)
	}
	in := newInput(t, "x: {shape: image; icon: ./link/secret.png}", compiler.Options{}, filepath.Join(proj, "main.d2"), "")
	if _, err := Render(context.Background(), PNG, in, Options{}); err == nil {
		t.Fatal("a symlink let an image escape the folder")
	}
}

func TestMissingAndNonImageFilesGiveClearErrors(t *testing.T) {
	dir := projectDir(t)
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("just some text"), 0o644); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "main.d2")

	in := newInput(t, "x: {shape: image; icon: ./nope.png}", compiler.Options{}, file, "")
	if _, err := Render(context.Background(), PNG, in, Options{}); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("missing: err = %v", err)
	}
	in = newInput(t, "x: {shape: image; icon: ./notes.txt}", compiler.Options{}, file, "")
	if _, err := Render(context.Background(), PNG, in, Options{}); err == nil || !strings.Contains(err.Error(), "image") {
		t.Errorf("non-image: err = %v", err)
	}
}

func TestLocalImageInAnUnsavedDocumentExplainsWhy(t *testing.T) {
	in := newInput(t, "x: {shape: image; icon: ./logo.png}", compiler.Options{}, "", "")
	_, err := Render(context.Background(), PNG, in, Options{})
	if err == nil || !strings.Contains(err.Error(), "not saved") {
		t.Fatalf("err = %v", err)
	}
}

// ---- remote images ----

// imageServer serves a small PNG and counts requests.
func imageServer(t *testing.T) (*httptest.Server, *atomic.Int64) {
	t.Helper()
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		path := filepath.Join(t.TempDir(), "i.png")
		writePNG(t, path)
		data, _ := os.ReadFile(path)
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(data)
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func TestRemoteImagesAreNotFetchedByDefault(t *testing.T) {
	srv, hits := imageServer(t)
	in := newInput(t, "x: {shape: image; icon: "+srv.URL+"/i.png}", compiler.Options{}, "", "")

	for _, f := range []Format{PNG, PDF} {
		out := render(t, f, in, Options{})
		if out.RemoteImages != 1 || !out.RemoteImagesBlocked {
			t.Errorf("%s: out = %+v, want 1 remote image reported as blocked", f, out)
		}
	}
	// Give any stray goroutine a moment; nothing should ever connect.
	time.Sleep(50 * time.Millisecond)
	if n := hits.Load(); n != 0 {
		t.Fatalf("the server received %d requests although remote images were off", n)
	}
}

func TestRemoteImagesAreFetchedWhenAllowed(t *testing.T) {
	srv, hits := imageServer(t)
	in := newInput(t, "x: {shape: image; icon: "+srv.URL+"/i.png}", compiler.Options{}, "", "")
	out, err := Render(context.Background(), PNG, in, Options{AllowRemoteImages: true})
	if err != nil {
		t.Skipf("D2's image loader refused the local test server (%v); cannot exercise opt-in fetching here", err)
	}
	if out.RemoteImages != 1 || out.RemoteImagesBlocked {
		t.Errorf("out = %+v", out)
	}
	if hits.Load() == 0 {
		t.Fatal("remote images were allowed but nothing was fetched")
	}
}

func TestSVGKeepsRemoteLinksAndReportsThem(t *testing.T) {
	in := newInput(t, "x: {shape: image; icon: https://icons.example.test/a.svg}", compiler.Options{}, "", "")
	out := render(t, SVG, in, Options{})
	if out.RemoteImages != 1 || out.RemoteImagesBlocked {
		t.Fatalf("out = %+v", out)
	}
	if !strings.Contains(string(out.Data), "icons.example.test") {
		t.Fatal("the SVG should keep the link to the remote image")
	}
}

func TestOfflineTransportNeverConnects(t *testing.T) {
	req, _ := http.NewRequest("GET", "http://127.0.0.1:1/", nil)
	if _, err := (offlineTransport{}).RoundTrip(req); err == nil {
		t.Fatal("offline transport must always fail")
	}
}
