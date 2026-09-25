package settings

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func newRecent(t *testing.T) (*RecentStore, string) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "sub", "recent.json")
	return NewRecentStore(p), p
}

func list(t *testing.T, r *RecentStore) []string {
	t.Helper()
	got, err := r.List()
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestRecentEmptyWhenMissing(t *testing.T) {
	r, _ := newRecent(t)
	got := list(t, r)
	if got == nil || len(got) != 0 {
		t.Fatalf("got %#v, want empty non-nil slice", got)
	}
}

func TestRecentMostRecentFirstAndDeduplicated(t *testing.T) {
	r, _ := newRecent(t)
	for _, p := range []string{"/a.d2", "/b.d2", "/c.d2", "/a.d2"} {
		if err := r.Add(p); err != nil {
			t.Fatal(err)
		}
	}
	if got := list(t, r); !slices.Equal(got, []string{"/a.d2", "/c.d2", "/b.d2"}) {
		t.Fatalf("got %v", got)
	}
}

func TestRecentIsCapped(t *testing.T) {
	r, _ := newRecent(t)
	for i := 0; i < MaxRecent+5; i++ {
		if err := r.Add(fmt.Sprintf("/f%d.d2", i)); err != nil {
			t.Fatal(err)
		}
	}
	got := list(t, r)
	if len(got) != MaxRecent || got[0] != fmt.Sprintf("/f%d.d2", MaxRecent+4) {
		t.Fatalf("len=%d first=%s", len(got), got[0])
	}
}

func TestRecentRemoveAndClear(t *testing.T) {
	r, _ := newRecent(t)
	_ = r.Add("/a.d2")
	_ = r.Add("/b.d2")
	if err := r.Remove("/a.d2"); err != nil {
		t.Fatal(err)
	}
	if got := list(t, r); !slices.Equal(got, []string{"/b.d2"}) {
		t.Fatalf("after remove: %v", got)
	}
	if err := r.Remove("/never-added.d2"); err != nil {
		t.Fatalf("removing an unknown path should be a no-op: %v", err)
	}
	if err := r.Clear(); err != nil {
		t.Fatal(err)
	}
	if got := list(t, r); len(got) != 0 {
		t.Fatalf("after clear: %v", got)
	}
}

func TestRecentRejectsEmptyPath(t *testing.T) {
	r, _ := newRecent(t)
	if err := r.Add(""); err == nil {
		t.Fatal("expected an error")
	}
}

func TestRecentCorruptFileIgnoredThenRepairedByNextChange(t *testing.T) {
	r, p := newRecent(t)
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	_ = os.WriteFile(p, []byte("not json"), 0o644)

	got, err := r.List()
	if err == nil || len(got) != 0 {
		t.Fatalf("got %v, err %v; want empty list and an error", got, err)
	}
	if b, _ := os.ReadFile(p); string(b) != "not json" {
		t.Fatal("listing must not modify a corrupt file")
	}
	if err := r.Add("/a.d2"); err != nil {
		t.Fatalf("a corrupt file must not block adding: %v", err)
	}
	if got := list(t, r); !slices.Equal(got, []string{"/a.d2"}) {
		t.Fatalf("got %v", got)
	}
}

func TestRecentPersistsAcrossInstances(t *testing.T) {
	r, p := newRecent(t)
	_ = r.Add("/a.d2")
	if got := list(t, NewRecentStore(p)); !slices.Equal(got, []string{"/a.d2"}) {
		t.Fatalf("got %v", got)
	}
}
