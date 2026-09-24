package files

import (
	"os"
	"runtime"
	"testing"
	"time"
)

type recorder struct{ events []Event }

func (r *recorder) on(e Event) { r.events = append(r.events, e) }

func (r *recorder) kinds() []EventKind {
	out := make([]EventKind, len(r.events))
	for i, e := range r.events {
		out[i] = e.Kind
	}
	return out
}

func newWatched(t *testing.T, content string) (*Watcher, *recorder, string, Version) {
	t.Helper()
	p := tmpFile(t, "a.d2", content)
	doc, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	rec := &recorder{}
	w := NewWatcher(time.Hour, rec.on) // polled by hand in tests
	w.path, w.base = p, doc.Version
	return w, rec, p, doc.Version
}

func wantKinds(t *testing.T, rec *recorder, want ...EventKind) {
	t.Helper()
	got := rec.kinds()
	if len(got) != len(want) {
		t.Fatalf("events = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("events = %v, want %v", got, want)
		}
	}
}

func TestWatcherQuietWhenNothingChanges(t *testing.T) {
	w, rec, _, _ := newWatched(t, "x")
	w.poll()
	w.poll()
	wantKinds(t, rec)
}

func TestWatcherReportsModificationOnce(t *testing.T) {
	w, rec, p, _ := newWatched(t, "x")
	write(t, p, "changed")
	bump(t, p)
	w.poll()
	w.poll() // same content again: no repeat
	wantKinds(t, rec, EventModified)
	if rec.events[0].Version.Hash == "" {
		t.Fatal("modified event should carry the new version")
	}

	write(t, p, "changed again")
	bump(t, p)
	w.poll()
	wantKinds(t, rec, EventModified, EventModified) // new content is a new event
}

func TestWatcherIgnoresTouchOnly(t *testing.T) {
	w, rec, p, _ := newWatched(t, "same")
	bump(t, p)
	w.poll()
	wantKinds(t, rec)
}

func TestWatcherReportsRevertAfterModification(t *testing.T) {
	w, rec, p, _ := newWatched(t, "original")
	write(t, p, "other")
	bump(t, p)
	w.poll()
	write(t, p, "original") // changed back
	w.poll()
	wantKinds(t, rec, EventModified, EventRestored)
}

func TestWatcherDeletedAndRestored(t *testing.T) {
	w, rec, p, _ := newWatched(t, "keep")
	os.Remove(p)
	w.poll()
	w.poll() // still gone: no repeat
	write(t, p, "keep")
	w.poll()
	wantKinds(t, rec, EventDeleted, EventRestored)
}

func TestWatcherDeletedThenRecreatedDifferently(t *testing.T) {
	w, rec, p, _ := newWatched(t, "keep")
	os.Remove(p)
	w.poll()
	write(t, p, "something else")
	w.poll()
	wantKinds(t, rec, EventDeleted, EventModified)
}

// Atomic-rename saves (what Save itself and many tools do) replace the inode.
func TestWatcherSeesRenameOverFile(t *testing.T) {
	w, rec, p, _ := newWatched(t, "old")
	tmp := p + ".new"
	write(t, tmp, "new contents")
	bump(t, tmp)
	if err := os.Rename(tmp, p); err != nil {
		t.Fatal(err)
	}
	w.poll()
	wantKinds(t, rec, EventModified)
}

func TestWatcherUnavailableThenRestored(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs POSIX permissions and a non-root user")
	}
	w, rec, p, _ := newWatched(t, "secret")
	if err := os.Chmod(p, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(p, 0o644) })
	bump(t, p) // so the poll reads the file instead of trusting metadata
	w.poll()
	w.poll()
	if err := os.Chmod(p, 0o644); err != nil {
		t.Fatal(err)
	}
	w.poll()
	wantKinds(t, rec, EventUnavailable, EventRestored)
	if rec.events[0].Message == "" {
		t.Fatal("unavailable event should carry the reason")
	}
}

func TestRebaseAfterOwnSave(t *testing.T) {
	w, rec, p, base := newWatched(t, "v1")
	res, err := Save(p, "v2", SaveOptions{Base: &base})
	if err != nil || !res.Saved {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	w.Rebase(res.Version)
	w.poll()
	wantKinds(t, rec) // our own save is not an external change
}

func TestWatcherRunsAndStops(t *testing.T) {
	p := tmpFile(t, "a.d2", "v1")
	doc, _ := Open(p)
	got := make(chan Event, 4)
	w := NewWatcher(5*time.Millisecond, func(e Event) { got <- e })
	w.Watch(p, doc.Version)

	write(t, p, "v2")
	bump(t, p)
	select {
	case e := <-got:
		if e.Kind != EventModified {
			t.Fatalf("event = %+v", e)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no event from the running watcher")
	}

	w.Stop()
	w.Stop() // idempotent
	write(t, p, "v3")
	bump(t, p)
	select {
	case e := <-got:
		t.Fatalf("event after Stop: %+v", e)
	case <-time.After(60 * time.Millisecond):
	}
}

func TestWatchReplacesPreviousWatch(t *testing.T) {
	a := tmpFile(t, "a.d2", "a")
	b := tmpFile(t, "b.d2", "b")
	da, _ := Open(a)
	db, _ := Open(b)
	got := make(chan Event, 4)
	w := NewWatcher(5*time.Millisecond, func(e Event) { got <- e })
	w.Watch(a, da.Version)
	w.Watch(b, db.Version) // switching files must stop watching a
	defer w.Stop()

	write(t, a, "a changed")
	bump(t, a)
	select {
	case e := <-got:
		t.Fatalf("event for the old file: %+v", e)
	case <-time.After(60 * time.Millisecond):
	}
}
