package compiler

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

func newService(t *testing.T) *Service {
	t.Helper()
	s, err := New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func compile(t *testing.T, s *Service, src string, opts Options) *Result {
	t.Helper()
	res, err := s.Compile(context.Background(), Request{Source: src, Options: opts, Path: "test.d2"})
	if err != nil {
		t.Fatalf("Compile returned service error: %v", err)
	}
	return res
}

func TestCompileSuccess(t *testing.T) {
	res := compile(t, newService(t), "a -> b: hello\nb -> c", Options{})
	if res.Status != StatusSuccess {
		t.Fatalf("status = %s, diagnostics = %+v", res.Status, res.Diagnostics)
	}
	if !strings.HasPrefix(res.SVG, "<svg") {
		t.Fatalf("SVG should start with <svg (no XML prolog), got %.40q", res.SVG)
	}
	if res.Meta.Layout != "dagre" || res.Meta.LayoutSource != "default" {
		t.Fatalf("meta = %+v, want default dagre", res.Meta)
	}
	if res.Meta.Width <= 0 || res.Meta.Height <= 0 {
		t.Fatalf("expected positive dimensions, got %dx%d", res.Meta.Width, res.Meta.Height)
	}
}

func TestAllBundledEnginesRender(t *testing.T) {
	s := newService(t)
	for _, e := range Engines() {
		t.Run(e.ID, func(t *testing.T) {
			res := compile(t, s, "x: {a -> b}\nx -> y", Options{Layout: e.ID})
			if res.Status != StatusSuccess {
				t.Fatalf("status = %s, diagnostics = %+v", res.Status, res.Diagnostics)
			}
			if res.Meta.Layout != e.ID || res.Meta.LayoutSource != "ui" {
				t.Fatalf("meta = %+v", res.Meta)
			}
		})
	}
}

func TestDiagnosticsHaveLocation(t *testing.T) {
	res := compile(t, newService(t), "a -> b\nc: {", Options{})
	if res.Status != StatusError || len(res.Diagnostics) == 0 {
		t.Fatalf("expected error, got %s %+v", res.Status, res.Diagnostics)
	}
	d := res.Diagnostics[0]
	if !d.HasLocation || d.Line != 2 {
		t.Fatalf("diagnostic = %+v, want location on line 2", d)
	}
	if d.Message != "maps must be terminated with }" {
		t.Fatalf("message = %q; location prefix should be stripped", d.Message)
	}
	if res.SVG != "" {
		t.Fatal("failed compile must not return SVG")
	}
}

// Offsets must be UTF-16 based so they index JavaScript strings correctly
// even when earlier text contains multi-byte characters.
func TestDiagnosticOffsetsAreUTF16(t *testing.T) {
	src := "é😀: ok\na -> "
	res := compile(t, newService(t), src, Options{})
	if res.Status != StatusError {
		t.Fatalf("status = %s", res.Status)
	}
	d := res.Diagnostics[0]
	if d.Line != 2 {
		t.Fatalf("line = %d, want 2", d.Line)
	}
	// "é😀: ok\n" is 1 + 2 (surrogate pair) + 5 chars = 8 UTF-16 units.
	if d.From < 8 {
		t.Fatalf("from = %d, want >= 8 (UTF-16 offset past first line)", d.From)
	}
}

func TestSourceLayoutConfigAndUIPrecedence(t *testing.T) {
	s := newService(t)
	src := "vars: {d2-config: {layout-engine: elk}}\na -> b"

	res := compile(t, s, src, Options{})
	if res.Status != StatusSuccess {
		t.Fatalf("diagnostics = %+v", res.Diagnostics)
	}
	if res.Meta.Layout != "elk" || res.Meta.LayoutSource != "source" {
		t.Fatalf("meta = %+v, want elk from source", res.Meta)
	}

	res = compile(t, s, src, Options{Layout: "dagre"})
	if res.Meta.Layout != "dagre" || res.Meta.LayoutSource != "ui" {
		t.Fatalf("meta = %+v, UI selection should override source", res.Meta)
	}
}

func TestUnknownEngineInSourceIsExplicitError(t *testing.T) {
	res := compile(t, newService(t), "vars: {d2-config: {layout-engine: bogus}}\na -> b", Options{})
	if res.Status != StatusError {
		t.Fatalf("status = %s, want error (never silently substitute an engine)", res.Status)
	}
	if !strings.Contains(res.Diagnostics[0].Message, "bogus") {
		t.Fatalf("message should name the engine: %q", res.Diagnostics[0].Message)
	}
}

func TestInvalidOptionsReportedExplicitly(t *testing.T) {
	s := newService(t)
	res := compile(t, s, "a -> b", Options{Layout: "nope"})
	if res.Status != StatusError || !strings.Contains(res.Diagnostics[0].Message, "nope") {
		t.Fatalf("unknown layout: %s %+v", res.Status, res.Diagnostics)
	}
	bad := int64(987654)
	res = compile(t, s, "a -> b", Options{ThemeID: &bad})
	if res.Status != StatusError {
		t.Fatalf("unknown theme: %s", res.Status)
	}
}

func TestThemeSelection(t *testing.T) {
	themes := Themes()
	if len(themes) < 2 {
		t.Fatalf("expected themes from D2 catalog, got %d", len(themes))
	}
	pick := themes[1].ID
	res := compile(t, newService(t), "a -> b", Options{ThemeID: &pick})
	if res.Status != StatusSuccess || res.Meta.ThemeID != pick || res.Meta.ThemeSource != "ui" {
		t.Fatalf("status = %s meta = %+v", res.Status, res.Meta)
	}
}

func TestEnginesAndThemesFromD2(t *testing.T) {
	ids := map[string]bool{}
	for _, e := range Engines() {
		ids[e.ID] = e.Available
	}
	for _, want := range []string{"dagre", "elk", "tala"} {
		if !ids[want] {
			t.Errorf("engine %q missing or unavailable", want)
		}
	}
	var dark int
	for _, th := range Themes() {
		if th.Dark {
			dark++
		}
	}
	if dark == 0 {
		t.Error("expected dark themes from D2 DarkCatalog")
	}
}

// A request that is superseded before it runs must report StatusStale so the
// caller never applies an outdated result over a newer one.
func TestSupersededRequestIsStale(t *testing.T) {
	s := newService(t)
	s.run.Lock() // hold the service busy so requests queue up

	var wg sync.WaitGroup
	results := make([]*Result, 2)
	start := func(i int, src string, wantSeq uint64) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := s.Compile(context.Background(), Request{Source: src})
			if err != nil {
				t.Error(err)
			}
			results[i] = res
		}()
		deadline := time.Now().Add(5 * time.Second)
		for s.seq.Load() != wantSeq {
			if time.Now().After(deadline) {
				t.Fatalf("request %d never started", i)
			}
			time.Sleep(time.Millisecond)
		}
	}
	start(0, "old -> diagram", 1)
	start(1, "new -> diagram", 2)
	s.run.Unlock()
	wg.Wait()

	if results[0].Status != StatusStale || results[0].SVG != "" {
		t.Fatalf("older request: status = %s, want stale with no SVG", results[0].Status)
	}
	if results[1].Status != StatusSuccess || !strings.Contains(results[1].SVG, "new") {
		t.Fatalf("newer request: status = %s", results[1].Status)
	}
}

func TestCanceledContextIsStale(t *testing.T) {
	s := newService(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	res, err := s.Compile(ctx, Request{Source: "a -> b"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != StatusStale {
		t.Fatalf("status = %s, want stale for canceled context", res.Status)
	}
}

func TestEmptySourceCompiles(t *testing.T) {
	res := compile(t, newService(t), "", Options{})
	if res.Status == StatusStale {
		t.Fatal("empty source must not be stale")
	}
}

func TestThemeSourceDistinguishesSourceFromDefault(t *testing.T) {
	s := newService(t)

	res := compile(t, s, "a -> b", Options{})
	if res.Meta.ThemeSource != "default" || res.Meta.SourceThemeID != nil {
		t.Fatalf("no d2-config: %+v", res.Meta)
	}

	res = compile(t, s, "vars: {d2-config: {theme-id: 300}}\na -> b", Options{})
	if res.Status != StatusSuccess {
		t.Fatalf("diagnostics = %+v", res.Diagnostics)
	}
	if res.Meta.ThemeID != 300 || res.Meta.ThemeSource != "source" ||
		res.Meta.SourceThemeID == nil || *res.Meta.SourceThemeID != 300 {
		t.Fatalf("theme from source: %+v", res.Meta)
	}
}

// A UI selection wins, but the result still reports what the source declares
// so the UI can say it is overriding d2-config.
func TestOverrideStillReportsSourceConfig(t *testing.T) {
	s := newService(t)
	src := "vars: {d2-config: {layout-engine: elk; theme-id: 300}}\na -> b"
	other := int64(1)

	res := compile(t, s, src, Options{Layout: "dagre", ThemeID: &other})
	if res.Status != StatusSuccess {
		t.Fatalf("diagnostics = %+v", res.Diagnostics)
	}
	m := res.Meta
	if m.Layout != "dagre" || m.LayoutSource != "ui" || m.SourceLayout != "elk" {
		t.Fatalf("layout meta: %+v", m)
	}
	if m.ThemeID != 1 || m.ThemeSource != "ui" || m.SourceThemeID == nil || *m.SourceThemeID != 300 {
		t.Fatalf("theme meta: %+v", m)
	}
}
