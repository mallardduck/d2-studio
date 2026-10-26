package settings

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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
	if !reflect.DeepEqual(got, Defaults()) {
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
	if !reflect.DeepEqual(got, Defaults()) {
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

func TestSidebarDefaultsOpenAndWorkspaceIsRemembered(t *testing.T) {
	if !Defaults().SidebarOpen {
		t.Fatal("the sidebar should be open by default")
	}
	st, p := newStore(t)
	// A settings file written before these fields existed keeps the defaults.
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	_ = os.WriteFile(p, []byte(`{"viewMode":"split","splitRatio":0.4}`), 0o644)
	got, err := st.Load()
	if err != nil || !got.SidebarOpen || got.Workspace != "" {
		t.Fatalf("got %+v err=%v", got, err)
	}

	got.Workspace, got.SidebarOpen = "/work/diagrams", false
	if err := st.Save(got); err != nil {
		t.Fatal(err)
	}
	again, _ := st.Load()
	if again.Workspace != "/work/diagrams" || again.SidebarOpen {
		t.Fatalf("round trip: %+v", again)
	}
}

func TestAutosaveIsOffByDefault(t *testing.T) {
	d := Defaults()
	if d.Autosave != AutosaveOff || d.AutosaveDelaySec != DefaultAutosaveDelaySec {
		t.Fatalf("defaults = %+v", d)
	}
	// A settings file from before autosave existed must not turn it on.
	st, p := newStore(t)
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	_ = os.WriteFile(p, []byte(`{"viewMode":"split","splitRatio":0.5}`), 0o644)
	got, err := st.Load()
	if err != nil || got.Autosave != AutosaveOff {
		t.Fatalf("got %+v err=%v", got, err)
	}
}

func TestAutosaveNormalization(t *testing.T) {
	cases := []struct {
		name     string
		in       Settings
		mode     string
		delaySec int
	}{
		{"valid delay", Settings{Autosave: AutosaveDelay, AutosaveDelaySec: 10}, AutosaveDelay, 10},
		{"valid blur", Settings{Autosave: AutosaveBlur, AutosaveDelaySec: 2}, AutosaveBlur, 2},
		{"unknown mode is off", Settings{Autosave: "always", AutosaveDelaySec: 2}, AutosaveOff, 2},
		{"empty mode is off", Settings{AutosaveDelaySec: 2}, AutosaveOff, 2},
		{"zero delay uses the default", Settings{Autosave: AutosaveDelay}, AutosaveDelay, DefaultAutosaveDelaySec},
		{"negative delay uses the default", Settings{Autosave: AutosaveDelay, AutosaveDelaySec: -5}, AutosaveDelay, DefaultAutosaveDelaySec},
		{"huge delay is capped", Settings{Autosave: AutosaveDelay, AutosaveDelaySec: 100000}, AutosaveDelay, MaxAutosaveDelaySec},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := c.in.Normalize()
			if got.Autosave != c.mode || got.AutosaveDelaySec != c.delaySec {
				t.Fatalf("got %q %d, want %q %d", got.Autosave, got.AutosaveDelaySec, c.mode, c.delaySec)
			}
		})
	}
}

// Logging options are kept exactly as written, so the logging package can tell
// the user about a typo instead of it being silently replaced here.
func TestLoggingSettingsRoundTripUnchanged(t *testing.T) {
	st, _ := newStore(t)
	in := Defaults()
	in.LogLevel, in.LogFormat, in.LogFileFormat = "trace", "json", "text"
	if err := st.Save(in); err != nil {
		t.Fatal(err)
	}
	got, err := st.Load()
	if err != nil || got.LogLevel != "trace" || got.LogFormat != "json" || got.LogFileFormat != "text" {
		t.Fatalf("got %+v err=%v", got, err)
	}
	in.LogLevel = "chatty"
	_ = st.Save(in)
	if got, _ := st.Load(); got.LogLevel != "chatty" {
		t.Fatalf("an unknown level must be preserved for the logger to report, got %q", got.LogLevel)
	}
	if Defaults().LogLevel != "" {
		t.Fatal("logging is at the default level unless the user asks otherwise")
	}
}

func TestRemoteImageFilesAreEmptyByDefaultAndCleaned(t *testing.T) {
	if got := Defaults().Normalize().RemoteImageFiles; len(got) != 0 {
		t.Fatalf("nothing is trusted by default: %v", got)
	}
	in := Defaults()
	in.RemoteImageFiles = []string{"/a.d2", "", "/b.d2", "/a.d2", "/c.d2"}
	got := in.Normalize().RemoteImageFiles
	if strings.Join(got, ",") != "/b.d2,/a.d2,/c.d2" {
		t.Fatalf("duplicates and blanks should go, keeping the latest occurrence: %v", got)
	}

	many := make([]string, 0, MaxRemoteImageFiles+20)
	for i := 0; i < MaxRemoteImageFiles+20; i++ {
		many = append(many, fmt.Sprintf("/f%d.d2", i))
	}
	in.RemoteImageFiles = many
	got = in.Normalize().RemoteImageFiles
	if len(got) != MaxRemoteImageFiles || got[0] != "/f20.d2" || got[len(got)-1] != fmt.Sprintf("/f%d.d2", MaxRemoteImageFiles+19) {
		t.Fatalf("only the newest %d should be kept, oldest first: len=%d first=%s last=%s", MaxRemoteImageFiles, len(got), got[0], got[len(got)-1])
	}
}

func TestRemoteImageFilesRoundTrip(t *testing.T) {
	st, _ := newStore(t)
	in := Defaults()
	in.RemoteImageFiles = []string{"/work/a.d2", "/work/b.d2"}
	if err := st.Save(in); err != nil {
		t.Fatal(err)
	}
	got, _ := st.Load()
	if strings.Join(got.RemoteImageFiles, ",") != "/work/a.d2,/work/b.d2" {
		t.Fatalf("got %v", got.RemoteImageFiles)
	}
}

func TestSidebarSplitIsClampedAndDefaulted(t *testing.T) {
	cases := []struct {
		name string
		in   float64
		want float64
	}{
		{"unset", 0, DefaultSidebarSplit},
		{"negative", -1, DefaultSidebarSplit},
		{"NaN", math.NaN(), DefaultSidebarSplit},
		{"too small", 0.01, MinSplitRatio},
		{"too large", 5, MaxSplitRatio},
		{"in range", 0.3, 0.3},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := (Settings{SidebarSplit: c.in}).Normalize().SidebarSplit; got != c.want {
				t.Fatalf("got %v, want %v", got, c.want)
			}
		})
	}
	if got := Defaults().SidebarSplit; got != DefaultSidebarSplit {
		t.Fatalf("default = %v", got)
	}
}

func TestOldSettingsFileGetsTheDefaultSidebarSplit(t *testing.T) {
	st, p := newStore(t)
	_ = os.WriteFile(p, []byte(`{"viewMode":"split","splitRatio":0.4}`), 0o644)
	got, err := st.Load()
	if err != nil || got.SidebarSplit != DefaultSidebarSplit {
		t.Fatalf("got %v, %v", got.SidebarSplit, err)
	}
}
