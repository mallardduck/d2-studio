package compiler

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"d2studio/assets"
)

func tinyPNG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// stubFetcher answers remote image requests without any network.
type stubFetcher struct {
	mu    sync.Mutex
	calls []string
	img   *assets.Image
	err   error
}

func (f *stubFetcher) Fetch(_ context.Context, raw string) (*assets.Image, error) {
	f.mu.Lock()
	f.calls = append(f.calls, raw)
	f.mu.Unlock()
	return f.img, f.err
}

func (f *stubFetcher) count() int { f.mu.Lock(); defer f.mu.Unlock(); return len(f.calls) }

func compileImages(t *testing.T, svc *Service, src, path string, opts Options) *Result {
	t.Helper()
	res, err := svc.Compile(context.Background(), Request{Source: src, Path: path, Options: opts})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestPreviewShowsLocalImages(t *testing.T) {
	dir := tree(t, nil)
	if err := os.WriteFile(filepath.Join(dir, "logo.png"), tinyPNG(t), 0o644); err != nil {
		t.Fatal(err)
	}
	res := compileImages(t, newService(t), "x: {shape: image; icon: ./logo.png}", filepath.Join(dir, "main.d2"), Options{})
	if res.Status != StatusSuccess {
		t.Fatalf("diagnostics = %+v", res.Diagnostics)
	}
	if !strings.Contains(res.SVG, "data:image/png;base64,") {
		t.Fatal("the image should be inlined into the preview so it can display")
	}
	if len(res.Warnings) != 0 {
		t.Fatalf("warnings = %+v", res.Warnings)
	}
	if strings.Contains(res.SVG, "./logo.png") {
		t.Error("the SVG should not keep a file reference the <img> cannot load")
	}
}

// One bad image must not blank the diagram.
func TestAnImageProblemIsAWarningNotAnError(t *testing.T) {
	dir := tree(t, map[string]string{"proj/main.d2": ""})
	main := filepath.Join(dir, "proj", "main.d2")
	if err := os.WriteFile(filepath.Join(dir, "secret.png"), tinyPNG(t), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{"missing": "./nope.png", "outside the folder": "../secret.png"}
	for name, icon := range cases {
		t.Run(name, func(t *testing.T) {
			res := compileImages(t, newService(t), "a -> b\nx: {shape: image; icon: "+icon+"}", main, Options{})
			if res.Status != StatusSuccess || res.SVG == "" {
				t.Fatalf("the diagram must still render: status=%s diagnostics=%+v", res.Status, res.Diagnostics)
			}
			if len(res.Diagnostics) != 0 {
				t.Errorf("an image problem is not an error: %+v", res.Diagnostics)
			}
			if len(res.Warnings) != 1 || res.Warnings[0].Severity != SeverityWarning || res.Warnings[0].HasLocation {
				t.Fatalf("warnings = %+v", res.Warnings)
			}
		})
	}
	// An image outside the allowed folder must never be embedded.
	res := compileImages(t, newService(t), "x: {shape: image; icon: ../secret.png}", main, Options{})
	if strings.Contains(res.SVG, "data:image/png") {
		t.Fatal("an image outside the folder was embedded")
	}
}

func TestLocalImageInAnUnsavedDocumentGivesAWarning(t *testing.T) {
	res := compileImages(t, newService(t), "x: {shape: image; icon: ./logo.png}", "", Options{})
	if res.Status != StatusSuccess || len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0].Message, "not saved") {
		t.Fatalf("status=%s warnings=%+v", res.Status, res.Warnings)
	}
}

func TestRemoteImagesAreNotFetchedUnlessAllowed(t *testing.T) {
	svc := newService(t)
	stub := &stubFetcher{img: &assets.Image{MIME: "image/png", Data: tinyPNG(t)}}
	svc.SetRemoteFetcher(stub)

	res := compileImages(t, svc, "x: {shape: image; icon: https://icons.example.test/a.png}", "", Options{})
	if res.Status != StatusSuccess {
		t.Fatalf("status = %s", res.Status)
	}
	if stub.count() != 0 {
		t.Fatalf("the fetcher was called %d times although remote images are off", stub.count())
	}
	// The user is told what is being hidden so they can choose to show it.
	if res.Meta.RemoteImages != 1 || res.Meta.RemoteImagesLoaded != 0 {
		t.Fatalf("meta = %+v", res.Meta)
	}
	if len(res.Warnings) != 0 {
		t.Errorf("hiding remote images is a choice, not a problem: %+v", res.Warnings)
	}
}

func TestRemoteImagesAreShownWhenAllowed(t *testing.T) {
	svc := newService(t)
	stub := &stubFetcher{img: &assets.Image{MIME: "image/png", Data: tinyPNG(t)}}
	svc.SetRemoteFetcher(stub)

	res := compileImages(t, svc, "a: {shape: image; icon: https://icons.example.test/a.png}\nb: {shape: image; icon: https://icons.example.test/a.png}",
		"", Options{RemoteImages: true})
	if res.Status != StatusSuccess || len(res.Warnings) != 0 {
		t.Fatalf("status=%s warnings=%+v", res.Status, res.Warnings)
	}
	if res.Meta.RemoteImages != 1 || res.Meta.RemoteImagesLoaded != 1 {
		t.Fatalf("meta = %+v", res.Meta)
	}
	if stub.count() != 1 {
		t.Fatalf("one distinct URL should be requested once, got %d", stub.count())
	}
	if !strings.Contains(res.SVG, "data:image/png;base64,") || strings.Contains(res.SVG, "icons.example.test") {
		t.Fatal("the fetched image should be inlined and the URL gone from the SVG")
	}
}

func TestAFailedRemoteImageIsAWarningWithoutTheQueryString(t *testing.T) {
	svc := newService(t)
	svc.SetRemoteFetcher(&stubFetcher{err: errors.New("the server answered 503")})
	res := compileImages(t, svc, "x: {shape: image; icon: https://icons.example.test/a.png?token=SECRET}", "", Options{RemoteImages: true})
	if res.Status != StatusSuccess {
		t.Fatalf("status = %s", res.Status)
	}
	if len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0].Message, "503") {
		t.Fatalf("warnings = %+v", res.Warnings)
	}
	if strings.Contains(res.Warnings[0].Message, "SECRET") {
		t.Fatalf("the query string leaked into a message: %s", res.Warnings[0].Message)
	}
	if res.Meta.RemoteImages != 1 || res.Meta.RemoteImagesLoaded != 0 {
		t.Fatalf("meta = %+v", res.Meta)
	}
}

func TestDiagramsWithoutImagesAreUnaffected(t *testing.T) {
	res := compileImages(t, newService(t), "a -> b", "", Options{RemoteImages: true})
	if res.Status != StatusSuccess || len(res.Warnings) != 0 || res.Meta.RemoteImages != 0 {
		t.Fatalf("status=%s warnings=%+v meta=%+v", res.Status, res.Warnings, res.Meta)
	}
}

// A bad image must not take the good ones down with it.
func TestGoodImagesStillShowBesideABadOne(t *testing.T) {
	dir := tree(t, nil)
	if err := os.WriteFile(filepath.Join(dir, "good.png"), tinyPNG(t), 0o644); err != nil {
		t.Fatal(err)
	}
	src := "good: {shape: image; icon: ./good.png}\nbad: {shape: image; icon: ./missing.png}"
	res := compileImages(t, newService(t), src, filepath.Join(dir, "main.d2"), Options{})
	if res.Status != StatusSuccess {
		t.Fatalf("status = %s", res.Status)
	}
	if !strings.Contains(res.SVG, "data:image/png;base64,") {
		t.Fatal("the good image should still be shown")
	}
	if len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0].Message, "missing.png") ||
		!strings.Contains(res.Warnings[0].Message, "not found") {
		t.Fatalf("the warning should name the bad image and say why: %+v", res.Warnings)
	}
}
