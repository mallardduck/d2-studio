package files

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"d2studio/settings"
)

type fakePlatform struct {
	openPath, savePath string
	openErr, saveErr   error
	openDir, saveDir   string
	saveName           string
	events             []Event
}

func (f *fakePlatform) PickOpenFile(dir string) (string, error) {
	f.openDir = dir
	return f.openPath, f.openErr
}
func (f *fakePlatform) PickSaveFile(dir, name string) (string, error) {
	f.saveDir, f.saveName = dir, name
	return f.savePath, f.saveErr
}
func (f *fakePlatform) Emit(event string, data any) {
	if e, ok := data.(Event); ok && event == EventFileChanged {
		f.events = append(f.events, e)
	}
}

func newService(t *testing.T) (*Service, *fakePlatform, string) {
	t.Helper()
	dir := t.TempDir()
	plat := &fakePlatform{}
	recent := settings.NewRecentStore(filepath.Join(dir, "cfg", "recent.json"))
	return NewService(plat, recent, time.Hour), plat, dir
}

func TestOpenDialogCancelled(t *testing.T) {
	s, plat, _ := newService(t)
	plat.openPath = ""
	res, err := s.OpenDialog("/start/here")
	if err != nil || !res.Cancelled || res.Document != nil {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if plat.openDir != "/start/here" {
		t.Fatalf("dialog directory = %q", plat.openDir)
	}
	if got := s.Recent(); len(got) != 0 {
		t.Fatalf("a cancelled open must not add a recent file: %v", got)
	}
}

func TestOpenDialogOpensWatchesAndRemembers(t *testing.T) {
	s, plat, dir := newService(t)
	p := filepath.Join(dir, "a.d2")
	write(t, p, "a -> b")
	plat.openPath = p

	res, err := s.OpenDialog("")
	if err != nil || res.Cancelled || res.Document.Content != "a -> b" {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if got := s.Recent(); len(got) != 1 || got[0].Path != p || got[0].Name != "a.d2" || !got[0].Exists {
		t.Fatalf("recent = %+v", got)
	}

	// External change is reported through the platform.
	write(t, p, "changed outside")
	bump(t, p)
	s.watcher.poll()
	if len(plat.events) != 1 || plat.events[0].Kind != EventModified {
		t.Fatalf("events = %+v", plat.events)
	}
}

func TestOpenDialogPropagatesPickerError(t *testing.T) {
	s, plat, _ := newService(t)
	plat.openErr = errors.New("dialog exploded")
	if _, err := s.OpenDialog(""); err == nil {
		t.Fatal("expected the error")
	}
}

func TestFailedOpenKeepsWatchingPreviousFile(t *testing.T) {
	s, plat, dir := newService(t)
	good := filepath.Join(dir, "good.d2")
	write(t, good, "v1")
	if _, err := s.OpenPath(good); err != nil {
		t.Fatal(err)
	}

	if _, err := s.OpenPath(filepath.Join(dir, "missing.d2")); err == nil {
		t.Fatal("expected an error opening a missing file")
	}
	if got := s.Recent(); len(got) != 1 || got[0].Path != good {
		t.Fatalf("a failed open must not be remembered: %+v", got)
	}

	write(t, good, "v2")
	bump(t, good)
	s.watcher.poll()
	if len(plat.events) != 1 {
		t.Fatalf("the previous file should still be watched; events = %+v", plat.events)
	}
}

func TestSaveDoesNotReportOwnWriteAsExternalChange(t *testing.T) {
	s, plat, dir := newService(t)
	p := filepath.Join(dir, "a.d2")
	write(t, p, "v1")
	res, _ := s.OpenPath(p)

	saved, err := s.Save(p, "v2", SaveOptions{Base: &res.Document.Version})
	if err != nil || !saved.Saved {
		t.Fatalf("saved=%+v err=%v", saved, err)
	}
	s.watcher.poll()
	if len(plat.events) != 0 {
		t.Fatalf("own save reported as external: %+v", plat.events)
	}
}

func TestSaveConflictLeavesWatcherAlone(t *testing.T) {
	s, plat, dir := newService(t)
	p := filepath.Join(dir, "a.d2")
	write(t, p, "v1")
	res, _ := s.OpenPath(p)

	write(t, p, "theirs")
	bump(t, p)
	saved, err := s.Save(p, "mine", SaveOptions{Base: &res.Document.Version})
	if err != nil || saved.Saved || saved.Conflict != ConflictModified {
		t.Fatalf("saved=%+v err=%v", saved, err)
	}
	s.watcher.poll() // the conflict has not been resolved, so the change is still news
	if len(plat.events) != 1 || plat.events[0].Kind != EventModified {
		t.Fatalf("events = %+v", plat.events)
	}
}

func TestSaveAsAddsExtensionAndSwitchesWatch(t *testing.T) {
	s, plat, dir := newService(t)
	plat.savePath = filepath.Join(dir, "diagram") // no extension
	res, err := s.SaveAs(dir, "Untitled.d2", "a -> b\n", SaveOptions{})
	if err != nil || res.Cancelled {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	want := filepath.Join(dir, "diagram.d2")
	if res.Path != want || res.Name != "diagram.d2" {
		t.Fatalf("res = %+v, want path %s", res, want)
	}
	if b, _ := os.ReadFile(want); string(b) != "a -> b\n" {
		t.Fatalf("file = %q", b)
	}
	if plat.saveName != "Untitled.d2" || plat.saveDir != dir {
		t.Fatalf("dialog got dir=%q name=%q", plat.saveDir, plat.saveName)
	}

	write(t, want, "edited elsewhere")
	bump(t, want)
	s.watcher.poll()
	if len(plat.events) != 1 {
		t.Fatalf("the saved-as file should now be watched: %+v", plat.events)
	}
}

func TestSaveAsKeepsAnExplicitExtension(t *testing.T) {
	s, plat, dir := newService(t)
	plat.savePath = filepath.Join(dir, "notes.txt")
	res, err := s.SaveAs(dir, "", "x", SaveOptions{})
	if err != nil || filepath.Base(res.Path) != "notes.txt" {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}

func TestSaveAsCancelledWritesNothing(t *testing.T) {
	s, plat, dir := newService(t)
	plat.savePath = ""
	res, err := s.SaveAs(dir, "x.d2", "content", SaveOptions{})
	if err != nil || !res.Cancelled {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatalf("files created: %v", entries)
	}
}

func TestSaveAsPreservesLineEndings(t *testing.T) {
	s, plat, dir := newService(t)
	plat.savePath = filepath.Join(dir, "a.d2")
	if _, err := s.SaveAs(dir, "", "a\nb\n", SaveOptions{LineEnding: LineEndingCRLF, BOM: true}); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(plat.savePath)
	if string(b) != "\xEF\xBB\xBFa\r\nb\r\n" {
		t.Fatalf("bytes = %q", b)
	}
}

func TestRecentFlagsMissingFilesAndSupportsRemoval(t *testing.T) {
	s, _, dir := newService(t)
	a := filepath.Join(dir, "a.d2")
	b := filepath.Join(dir, "b.d2")
	write(t, a, "a")
	write(t, b, "b")
	_, _ = s.OpenPath(a)
	_, _ = s.OpenPath(b)
	os.Remove(a)

	got := s.Recent()
	if len(got) != 2 || got[0].Path != b || !got[0].Exists || got[1].Path != a || got[1].Exists {
		t.Fatalf("recent = %+v", got)
	}
	if err := s.RemoveRecent(a); err != nil {
		t.Fatal(err)
	}
	if got := s.Recent(); len(got) != 1 || got[0].Path != b {
		t.Fatalf("after remove: %+v", got)
	}
	if err := s.ClearRecent(); err != nil {
		t.Fatal(err)
	}
	if got := s.Recent(); len(got) != 0 {
		t.Fatalf("after clear: %+v", got)
	}
}

func TestServiceWithoutRecentStore(t *testing.T) {
	plat := &fakePlatform{}
	s := NewService(plat, nil, time.Hour)
	p := tmpFile(t, "a.d2", "x")
	if _, err := s.OpenPath(p); err != nil {
		t.Fatal(err)
	}
	if got := s.Recent(); got == nil || len(got) != 0 {
		t.Fatalf("recent = %#v, want empty non-nil", got)
	}
	if s.RemoveRecent("x") != nil || s.ClearRecent() != nil {
		t.Fatal("recent operations should be no-ops without a store")
	}
}

func TestShortDir(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skip("no home directory")
	}
	if got := shortDir(home); got != "~" {
		t.Errorf("home = %q", got)
	}
	if got := shortDir(filepath.Join(home, "work", "x")); got != filepath.Join("~", "work", "x") {
		t.Errorf("under home = %q", got)
	}
	if got := shortDir(home + "-other"); got != home+"-other" {
		t.Errorf("sibling with shared prefix must not be abbreviated: %q", got)
	}
}
