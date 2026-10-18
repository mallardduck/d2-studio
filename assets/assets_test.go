package assets

import (
	"context"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/d2lang/d2/d2target"
)

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func diagramWith(t *testing.T, icons ...string) *d2target.Diagram {
	t.Helper()
	d := &d2target.Diagram{}
	for _, raw := range icons {
		d.Shapes = append(d.Shapes, d2target.Shape{Icon: mustURL(t, raw)})
	}
	return d
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func project(t *testing.T) string {
	t.Helper()
	d, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// fakeFetcher serves canned images and records every request.
type fakeFetcher struct {
	mu     sync.Mutex
	images map[string]*Image
	errs   map[string]error
	calls  []string
}

func (f *fakeFetcher) Fetch(ctx context.Context, raw string) (*Image, error) {
	f.mu.Lock()
	f.calls = append(f.calls, raw)
	f.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err, ok := f.errs[raw]; ok {
		return nil, err
	}
	if img, ok := f.images[raw]; ok {
		return img, nil
	}
	return nil, errors.New("not found")
}

func TestLocalImagesAreInlinedAndTheOriginalIsLeftAlone(t *testing.T) {
	dir := project(t)
	writeFile(t, filepath.Join(dir, "img", "logo.png"), pngBytes(t))
	d := diagramWith(t, "./img/logo.png")
	d.Root.Icon = mustURL(t, "img/logo.png")
	d.Connections = []d2target.Connection{{Icon: mustURL(t, "./img/logo.png")}}

	p, err := Prepare(context.Background(), d, dir, dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	for name, u := range map[string]*url.URL{"shape": p.Diagram.Shapes[0].Icon, "root": p.Diagram.Root.Icon, "connection": p.Diagram.Connections[0].Icon} {
		if u == nil || u.Scheme != "data" || !strings.HasPrefix(u.Opaque, "image/png;base64,") {
			t.Errorf("%s icon = %v", name, u)
		}
	}
	if d.Shapes[0].Icon.String() != "./img/logo.png" || d.Root.Icon.String() != "img/logo.png" {
		t.Fatal("the caller's diagram must not be modified")
	}
}

func TestStrictModeFailsOnABadLocalImage(t *testing.T) {
	dir := project(t)
	for name, icon := range map[string]string{"missing": "./nope.png", "outside": "../elsewhere.png", "unsupported scheme": "ftp://x/y.png"} {
		_, err := Prepare(context.Background(), diagramWith(t, icon), dir, dir, Options{})
		if err == nil {
			t.Errorf("%s: strict mode should fail", name)
		}
	}
}

func TestLenientModeReportsProblemsAndKeepsGoing(t *testing.T) {
	dir := project(t)
	writeFile(t, filepath.Join(dir, "good.png"), pngBytes(t))
	d := diagramWith(t, "./good.png", "./nope.png", "../outside.png", "data:image/png;base64,AAAA")

	p, err := Prepare(context.Background(), d, dir, dir, Options{Lenient: true})
	if err != nil {
		t.Fatalf("lenient mode must not fail: %v", err)
	}
	if p.Diagram.Shapes[0].Icon.Scheme != "data" {
		t.Error("the good image should still be inlined")
	}
	if p.Diagram.Shapes[1].Icon.String() != "./nope.png" || p.Diagram.Shapes[2].Icon.String() != "../outside.png" {
		t.Error("references that could not be loaded should be left as they were")
	}
	if len(p.Problems) != 2 {
		t.Fatalf("problems = %q", p.Problems)
	}
	joined := strings.Join(p.Problems, "\n")
	if !strings.Contains(joined, "not found") || !strings.Contains(joined, "outside") {
		t.Errorf("problems should say what is wrong: %s", joined)
	}
}

func TestLenientModeWithAnUnsavedDocument(t *testing.T) {
	p, err := Prepare(context.Background(), diagramWith(t, "./logo.png"), "", "", Options{Lenient: true})
	if err != nil || len(p.Problems) != 1 || !strings.Contains(p.Problems[0], "not saved") {
		t.Fatalf("p=%+v err=%v", p, err)
	}
}

func TestWorkspaceParentResolvesRelativePathsAgainstTheFileFolder(t *testing.T) {
	base := project(t)
	writeFile(t, filepath.Join(base, "shared", "logo.png"), pngBytes(t))
	fileDir := filepath.Join(base, "proj")
	if err := os.MkdirAll(fileDir, 0o755); err != nil {
		t.Fatal(err)
	}
	p, err := Prepare(context.Background(), diagramWith(t, "../shared/logo.png"), base, fileDir, Options{})
	if err != nil || p.Diagram.Shapes[0].Icon.Scheme != "data" {
		t.Fatalf("p=%+v err=%v", p, err)
	}
}

func TestSymlinkEscapeIsReportedNotFollowed(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on Windows")
	}
	base := project(t)
	writeFile(t, filepath.Join(base, "elsewhere", "secret.png"), pngBytes(t))
	proj := filepath.Join(base, "proj")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(base, "elsewhere"), filepath.Join(proj, "link")); err != nil {
		t.Fatal(err)
	}
	p, err := Prepare(context.Background(), diagramWith(t, "./link/secret.png"), proj, proj, Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	if p.Diagram.Shapes[0].Icon.Scheme == "data" || len(p.Problems) != 1 {
		t.Fatalf("a symlink let an image escape: icon=%v problems=%v", p.Diagram.Shapes[0].Icon, p.Problems)
	}
}

func TestRemoteImagesAreOnlyReportedWithoutAFetcher(t *testing.T) {
	d := diagramWith(t, "https://b.example/2.png", "https://a.example/1.png", "https://b.example/2.png", "data:image/png;base64,AAAA")
	p, err := Prepare(context.Background(), d, "", "", Options{})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"https://a.example/1.png", "https://b.example/2.png"}
	if strings.Join(p.Remote, ",") != strings.Join(want, ",") {
		t.Fatalf("Remote = %v, want the distinct URLs sorted: %v", p.Remote, want)
	}
	if p.RemoteLoaded != 0 || p.Diagram.Shapes[0].Icon.String() != "https://b.example/2.png" {
		t.Fatal("without a fetcher nothing is fetched and the references stay as they were")
	}
}

func TestRemoteImagesAreFetchedAndInlinedWhenAllowed(t *testing.T) {
	f := &fakeFetcher{
		images: map[string]*Image{"https://a.example/1.png": {MIME: "image/png", Data: pngBytes(t)}},
		errs:   map[string]error{"https://b.example/2.png": errors.New("the server answered 503")},
	}
	// The same URL on several shapes is fetched once.
	d := diagramWith(t, "https://a.example/1.png", "https://a.example/1.png", "https://b.example/2.png")
	p, err := Prepare(context.Background(), d, "", "", Options{Fetcher: f})
	if err != nil {
		t.Fatal(err)
	}
	if p.RemoteLoaded != 1 {
		t.Fatalf("RemoteLoaded = %d", p.RemoteLoaded)
	}
	for _, i := range []int{0, 1} {
		if p.Diagram.Shapes[i].Icon.Scheme != "data" {
			t.Errorf("shape %d should be inlined: %v", i, p.Diagram.Shapes[i].Icon)
		}
	}
	if p.Diagram.Shapes[2].Icon.String() != "https://b.example/2.png" {
		t.Error("an image that failed to load keeps its reference")
	}
	if len(p.Problems) != 1 || !strings.Contains(p.Problems[0], "b.example/2.png") || !strings.Contains(p.Problems[0], "503") {
		t.Fatalf("problems = %q", p.Problems)
	}
	if len(f.calls) != 2 {
		t.Fatalf("fetched %v; each distinct URL should be requested once", f.calls)
	}
	if d.Shapes[0].Icon.String() != "https://a.example/1.png" {
		t.Fatal("the caller's diagram must not be modified")
	}
}

func TestProblemsNeverContainQueryStrings(t *testing.T) {
	f := &fakeFetcher{errs: map[string]error{"https://a.example/i.png?token=SECRET": errors.New("denied")}}
	p, err := Prepare(context.Background(), diagramWith(t, "https://a.example/i.png?token=SECRET"), "", "", Options{Fetcher: f})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Problems) != 1 || strings.Contains(p.Problems[0], "SECRET") {
		t.Fatalf("problems = %q", p.Problems)
	}
}

func TestCancellationReachesTheFetcher(t *testing.T) {
	f := &fakeFetcher{images: map[string]*Image{"https://a.example/1.png": {MIME: "image/png", Data: pngBytes(t)}}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	p, err := Prepare(ctx, diagramWith(t, "https://a.example/1.png"), "", "", Options{Fetcher: f})
	if err != nil {
		t.Fatal(err)
	}
	if p.RemoteLoaded != 0 {
		t.Fatal("nothing should load once the context is cancelled")
	}
}

func TestImagelessDiagramIsReturnedUnchanged(t *testing.T) {
	d := &d2target.Diagram{Shapes: []d2target.Shape{{}, {}}}
	p, err := Prepare(context.Background(), d, "", "", Options{})
	if err != nil || len(p.Diagram.Shapes) != 2 || len(p.Remote) != 0 || len(p.Problems) != 0 {
		t.Fatalf("p=%+v err=%v", p, err)
	}
}
