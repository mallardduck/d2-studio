package settings

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

func newStore(t *testing.T) (*Store, string) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "nested", "settings.json")
	return NewStore(p), p
}

func TestLoadMissingFileReturnsDefaults(t *testing.T) {
	st, _ := newStore(t)
	got, err := st.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got != Defaults() {
		t.Fatalf("got %+v, want defaults", got)
	}
	if got.ViewMode != ViewSplit {
		t.Fatalf("default view mode = %q, want split", got.ViewMode)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	st, _ := newStore(t)
	theme := int64(200)
	want := Settings{ViewMode: ViewPreview, SplitRatio: 0.3, Layout: "elk", ThemeID: &theme}
	if err := st.Save(want); err != nil {
		t.Fatal(err)
	}
	got, err := st.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.ViewMode != want.ViewMode || got.SplitRatio != want.SplitRatio || got.Layout != "elk" ||
		got.ThemeID == nil || *got.ThemeID != 200 {
		t.Fatalf("round trip mismatch: %+v", got)
	}
}

func TestCorruptFileFallsBackToDefaultsWithError(t *testing.T) {
	st, p := newStore(t)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := st.Load()
	if err == nil {
		t.Fatal("expected an error for a corrupt file")
	}
	if got != Defaults() {
		t.Fatalf("got %+v, want defaults", got)
	}
	// The user's file must not be destroyed by a failed load.
	if b, _ := os.ReadFile(p); string(b) != "{not json" {
		t.Fatalf("corrupt file was modified: %q", b)
	}
}

func TestNormalize(t *testing.T) {
	cases := []struct {
		name string
		in   Settings
		view string
		w    float64
	}{
		{"unknown mode", Settings{ViewMode: "zen", SplitRatio: 0.5}, ViewSplit, 0.5},
		{"zero ratio", Settings{ViewMode: ViewSource}, ViewSource, 0.5},
		{"too small", Settings{ViewMode: ViewSplit, SplitRatio: 0.01}, ViewSplit, MinSplitRatio},
		{"too large", Settings{ViewMode: ViewSplit, SplitRatio: 0.99}, ViewSplit, MaxSplitRatio},
		{"NaN", Settings{ViewMode: ViewSplit, SplitRatio: math.NaN()}, ViewSplit, 0.5},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := c.in.Normalize()
			if got.ViewMode != c.view || got.SplitRatio != c.w {
				t.Fatalf("got %q %v, want %q %v", got.ViewMode, got.SplitRatio, c.view, c.w)
			}
		})
	}
}

func TestSaveLeavesNoTempFiles(t *testing.T) {
	st, p := newStore(t)
	for i := 0; i < 3; i++ {
		if err := st.Save(Defaults()); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(filepath.Dir(p))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "settings.json" {
		t.Fatalf("directory contents: %v", entries)
	}
}

// A saved file with an invalid ratio is repaired on load, not trusted.
func TestLoadNormalizesHandEditedFile(t *testing.T) {
	st, p := newStore(t)
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	_ = os.WriteFile(p, []byte(`{"viewMode":"preview","splitRatio":7}`), 0o644)
	got, err := st.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.ViewMode != ViewPreview || got.SplitRatio != MaxSplitRatio {
		t.Fatalf("got %+v", got)
	}
}
