package workspace

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func mk(t *testing.T, files ...string) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		p := filepath.Join(dir, filepath.FromSlash(f))
		if strings.HasSuffix(f, "/") { // an empty directory
			if err := os.MkdirAll(p, 0o755); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// names flattens a tree into "dir/" and "file" entries in display order.
func names(nodes []*Node, prefix string) []string {
	var out []string
	for _, n := range nodes {
		if n.Dir {
			out = append(out, prefix+n.Name+"/")
			out = append(out, names(n.Children, prefix+n.Name+"/")...)
		} else {
			out = append(out, prefix+n.Name)
		}
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestScanListsD2FilesFoldersFirstCaseInsensitive(t *testing.T) {
	dir := mk(t, "b.d2", "A.d2", "zeta/one.d2", "alpha/two.d2", "notes.txt", "README.md")
	tree, err := Scan(dir, DefaultLimits)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"alpha/", "alpha/two.d2", "zeta/", "zeta/one.d2", "A.d2", "b.d2"}
	if got := names(tree.Nodes, ""); !equal(got, want) {
		t.Fatalf("got %v\nwant %v", got, want)
	}
	if tree.FileCount != 4 || tree.Truncated || tree.Name != filepath.Base(dir) || tree.Root != dir {
		t.Fatalf("tree = %+v", tree)
	}
}

func TestScanPrunesFoldersWithoutD2Files(t *testing.T) {
	dir := mk(t, "docs/readme.md", "empty/", "a/b/c/deep.d2", "images/logo.png")
	if info, err := os.Stat(filepath.Join(dir, "empty")); err != nil || !info.IsDir() {
		t.Fatal("test setup: the empty folder should exist so pruning is really exercised")
	}
	tree, _ := Scan(dir, DefaultLimits)
	want := []string{"a/", "a/b/", "a/b/c/", "a/b/c/deep.d2"}
	if got := names(tree.Nodes, ""); !equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestScanSkipsHiddenAndNodeModules(t *testing.T) {
	dir := mk(t, ".git/hooks/x.d2", ".hidden.d2", "node_modules/pkg/diagram.d2", "src/ok.d2")
	tree, _ := Scan(dir, DefaultLimits)
	if got := names(tree.Nodes, ""); !equal(got, []string{"src/", "src/ok.d2"}) {
		t.Fatalf("got %v", got)
	}
}

func TestScanMatchesExtensionCaseInsensitively(t *testing.T) {
	dir := mk(t, "UPPER.D2", "mixed.D2", "not-d2.d2x")
	tree, _ := Scan(dir, DefaultLimits)
	if got := names(tree.Nodes, ""); !equal(got, []string{"mixed.D2", "UPPER.D2"}) {
		t.Fatalf("got %v", got)
	}
}

func TestScanDoesNotFollowLinkedFolders(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on Windows")
	}
	dir := mk(t, "real/a.d2", "loop/")
	// A link back to the root would recurse forever if followed.
	if err := os.Symlink(dir, filepath.Join(dir, "loop", "back")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "real"), filepath.Join(dir, "alias")); err != nil {
		t.Fatal(err)
	}
	tree, err := Scan(dir, DefaultLimits)
	if err != nil {
		t.Fatal(err)
	}
	if got := names(tree.Nodes, ""); !equal(got, []string{"real/", "real/a.d2"}) {
		t.Fatalf("got %v", got)
	}
}

func TestScanListsSymlinkedFilesAndIgnoresBrokenLinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on Windows")
	}
	dir := mk(t, "real.d2")
	_ = os.Symlink(filepath.Join(dir, "real.d2"), filepath.Join(dir, "link.d2"))
	_ = os.Symlink(filepath.Join(dir, "gone.d2"), filepath.Join(dir, "broken.d2"))
	tree, _ := Scan(dir, DefaultLimits)
	if got := names(tree.Nodes, ""); !equal(got, []string{"link.d2", "real.d2"}) {
		t.Fatalf("got %v", got)
	}
}

func TestScanEntryLimitTruncates(t *testing.T) {
	dir := mk(t, "a.d2", "b.d2", "c.d2", "d.d2", "e.d2")
	tree, _ := Scan(dir, Limits{MaxDepth: 8, MaxEntries: 3})
	if !tree.Truncated || tree.FileCount != 3 {
		t.Fatalf("tree = %+v", tree)
	}
}

func TestScanDepthLimitTruncates(t *testing.T) {
	dir := mk(t, "top.d2", "a/b/c/deep.d2")
	tree, _ := Scan(dir, Limits{MaxDepth: 2, MaxEntries: 100})
	if !tree.Truncated {
		t.Fatal("a folder beyond the depth limit should mark the scan truncated")
	}
	if got := names(tree.Nodes, ""); !equal(got, []string{"top.d2"}) {
		t.Fatalf("got %v", got)
	}
}

func TestScanErrors(t *testing.T) {
	dir := mk(t, "file.d2")
	for name, path := range map[string]string{
		"missing":      filepath.Join(dir, "nope"),
		"not a folder": filepath.Join(dir, "file.d2"),
	} {
		if _, err := Scan(path, DefaultLimits); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
	if _, err := Scan(filepath.Join(dir, "nope"), DefaultLimits); err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("message should be friendly: %v", err)
	}
}

func TestScanUnreadableSubfolderIsSkippedNotFatal(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs POSIX permissions and a non-root user")
	}
	dir := mk(t, "ok.d2", "locked/hidden.d2")
	if err := os.Chmod(filepath.Join(dir, "locked"), 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(filepath.Join(dir, "locked"), 0o755) })
	tree, err := Scan(dir, DefaultLimits)
	if err != nil {
		t.Fatal(err)
	}
	if got := names(tree.Nodes, ""); !equal(got, []string{"ok.d2"}) {
		t.Fatalf("got %v", got)
	}
}

func TestSignatureTracksAddRemoveAndModify(t *testing.T) {
	dir := mk(t, "a.d2")
	sig := func() string {
		tree, err := Scan(dir, DefaultLimits)
		if err != nil {
			t.Fatal(err)
		}
		return tree.Signature
	}
	base := sig()
	if sig() != base {
		t.Fatal("signature must be stable when nothing changed")
	}

	if err := os.WriteFile(filepath.Join(dir, "b.d2"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	added := sig()
	if added == base {
		t.Fatal("adding a file did not change the signature")
	}

	p := filepath.Join(dir, "a.d2")
	if err := os.WriteFile(p, []byte("longer content"), 0o644); err != nil {
		t.Fatal(err)
	}
	modified := sig()
	if modified == added {
		t.Fatal("modifying a file did not change the signature")
	}

	// Same size, new mtime: still a change (an edit that keeps the length).
	future := time.Now().Add(time.Hour)
	_ = os.Chtimes(p, future, future)
	if sig() == modified {
		t.Fatal("a new mtime did not change the signature")
	}

	os.Remove(filepath.Join(dir, "b.d2"))
	if sig() == added {
		t.Fatal("removing a file did not restore a different signature")
	}
}

func TestSignatureIgnoresNonD2Files(t *testing.T) {
	dir := mk(t, "a.d2")
	before, _ := Scan(dir, DefaultLimits)
	_ = os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("hi"), 0o644)
	after, _ := Scan(dir, DefaultLimits)
	if before.Signature != after.Signature {
		t.Fatal("unrelated files must not trigger updates")
	}
}

// ---- Service ----

type fakePlatform struct {
	pick    string
	pickErr error
	updates []Update
	pickDir string
}

func (f *fakePlatform) PickFolder(dir string) (string, error) {
	f.pickDir = dir
	return f.pick, f.pickErr
}
func (f *fakePlatform) Emit(event string, data any) {
	if u, ok := data.(Update); ok && event == EventChanged {
		f.updates = append(f.updates, u)
	}
}

func newService(t *testing.T) (*Service, *fakePlatform) {
	t.Helper()
	p := &fakePlatform{}
	s := NewService(p, DefaultLimits, time.Hour) // polled by hand
	t.Cleanup(s.Close)
	return s, p
}

func TestOpenFolderDialogCancelled(t *testing.T) {
	s, p := newService(t)
	p.pick = ""
	res, err := s.OpenFolderDialog("/start")
	if err != nil || !res.Cancelled || res.Tree != nil {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if p.pickDir != "/start" {
		t.Fatalf("dialog directory = %q", p.pickDir)
	}
}

func TestOpenFolderDialogReturnsTree(t *testing.T) {
	s, p := newService(t)
	dir := mk(t, "a.d2")
	p.pick = dir
	res, err := s.OpenFolderDialog("")
	if err != nil || res.Cancelled || res.Tree.FileCount != 1 {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}

func TestFailedOpenKeepsTheCurrentWorkspace(t *testing.T) {
	s, p := newService(t)
	dir := mk(t, "a.d2")
	if _, err := s.OpenFolder(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := s.OpenFolder(filepath.Join(dir, "does-not-exist")); err == nil {
		t.Fatal("expected an error")
	}
	// Still watching the first one.
	_ = os.WriteFile(filepath.Join(dir, "new.d2"), []byte("x"), 0o644)
	s.poll()
	if len(p.updates) != 1 || p.updates[0].Root != dir {
		t.Fatalf("updates = %+v", p.updates)
	}
}

func TestPollEmitsOnlyWhenSomethingChanged(t *testing.T) {
	s, p := newService(t)
	dir := mk(t, "a.d2")
	if _, err := s.OpenFolder(dir); err != nil {
		t.Fatal(err)
	}
	s.poll()
	s.poll()
	if len(p.updates) != 0 {
		t.Fatalf("no change, but got %+v", p.updates)
	}

	_ = os.WriteFile(filepath.Join(dir, "b.d2"), []byte("x"), 0o644)
	s.poll()
	s.poll() // the same change must not be reported twice
	if len(p.updates) != 1 || p.updates[0].Tree == nil || p.updates[0].Tree.FileCount != 2 {
		t.Fatalf("updates = %+v", p.updates)
	}
}

func TestEditingAnExistingFileIsReported(t *testing.T) {
	// This is what keeps previews of diagrams that import it fresh.
	s, p := newService(t)
	dir := mk(t, "a.d2", "lib.d2")
	_, _ = s.OpenFolder(dir)
	lib := filepath.Join(dir, "lib.d2")
	_ = os.WriteFile(lib, []byte("changed and longer"), 0o644)
	future := time.Now().Add(time.Hour)
	_ = os.Chtimes(lib, future, future)
	s.poll()
	if len(p.updates) != 1 {
		t.Fatalf("updates = %+v", p.updates)
	}
}

func TestPollReportsFolderLossOnceThenRecovery(t *testing.T) {
	s, p := newService(t)
	dir := mk(t, "a.d2")
	_, _ = s.OpenFolder(dir)

	moved := dir + "-moved"
	if err := os.Rename(dir, moved); err != nil {
		t.Fatal(err)
	}
	s.poll()
	s.poll()
	if len(p.updates) != 1 || p.updates[0].Error == "" || p.updates[0].Tree != nil {
		t.Fatalf("updates = %+v", p.updates)
	}

	if err := os.Rename(moved, dir); err != nil {
		t.Fatal(err)
	}
	s.poll()
	if len(p.updates) != 2 || p.updates[1].Tree == nil || p.updates[1].Error != "" {
		t.Fatalf("updates = %+v", p.updates)
	}
}

func TestRefreshAbsorbsChangesSoTheyAreNotReportedAgain(t *testing.T) {
	s, p := newService(t)
	dir := mk(t, "a.d2")
	_, _ = s.OpenFolder(dir)
	_ = os.WriteFile(filepath.Join(dir, "b.d2"), []byte("x"), 0o644)

	tree, err := s.Refresh()
	if err != nil || tree.FileCount != 2 {
		t.Fatalf("tree=%+v err=%v", tree, err)
	}
	s.poll()
	if len(p.updates) != 0 {
		t.Fatalf("a refreshed change was reported again: %+v", p.updates)
	}
}

func TestRefreshWithoutWorkspace(t *testing.T) {
	s, _ := newService(t)
	tree, err := s.Refresh()
	if tree != nil || err != nil {
		t.Fatalf("tree=%v err=%v", tree, err)
	}
}

func TestCloseStopsEverything(t *testing.T) {
	s, p := newService(t)
	dir := mk(t, "a.d2")
	_, _ = s.OpenFolder(dir)
	s.Close()
	s.Close() // idempotent
	_ = os.WriteFile(filepath.Join(dir, "b.d2"), []byte("x"), 0o644)
	s.poll()
	if len(p.updates) != 0 {
		t.Fatalf("closed workspace still reported: %+v", p.updates)
	}
}

func TestRunningServicePollsAndStops(t *testing.T) {
	got := make(chan Update, 4)
	plat := &chanPlatform{ch: got}
	s := NewService(plat, DefaultLimits, 5*time.Millisecond)
	dir := mk(t, "a.d2")
	if _, err := s.OpenFolder(dir); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(dir, "b.d2"), []byte("x"), 0o644)
	select {
	case u := <-got:
		if u.Tree == nil || u.Tree.FileCount != 2 {
			t.Fatalf("update = %+v", u)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no update from the running service")
	}
	s.Close()
	_ = os.WriteFile(filepath.Join(dir, "c.d2"), []byte("x"), 0o644)
	select {
	case u := <-got:
		t.Fatalf("update after Close: %+v", u)
	case <-time.After(60 * time.Millisecond):
	}
}

type chanPlatform struct{ ch chan Update }

func (c *chanPlatform) PickFolder(string) (string, error) { return "", nil }
func (c *chanPlatform) Emit(event string, data any) {
	if u, ok := data.(Update); ok {
		c.ch <- u
	}
}
