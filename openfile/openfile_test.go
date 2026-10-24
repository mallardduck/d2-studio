package openfile

import (
	"path/filepath"
	"reflect"
	"sync"
	"testing"
)

func TestPathsFromArgs(t *testing.T) {
	wd := filepath.FromSlash("/work/proj")
	abs := filepath.FromSlash("/elsewhere/b.d2")
	cases := []struct {
		name string
		args []string
		want []string
	}{
		{"none", nil, []string{}},
		{"relative resolves against the launch directory", []string{"a.d2"}, []string{filepath.FromSlash("/work/proj/a.d2")}},
		{"absolute is kept", []string{abs}, []string{abs}},
		{"nested relative", []string{"sub/../a.d2"}, []string{filepath.FromSlash("/work/proj/a.d2")}},
		{"flags are skipped", []string{"--verbose", "-x", "a.d2"}, []string{filepath.FromSlash("/work/proj/a.d2")}},
		{"URLs are skipped", []string{"wails://open?x=1", "https://example.com/a.d2", "a.d2"}, []string{filepath.FromSlash("/work/proj/a.d2")}},
		{"blanks are skipped", []string{"", "  ", "a.d2"}, []string{filepath.FromSlash("/work/proj/a.d2")}},
		{"several, in order", []string{"a.d2", "b.d2"}, []string{filepath.FromSlash("/work/proj/a.d2"), filepath.FromSlash("/work/proj/b.d2")}},
		{"spaces are preserved", []string{"my diagram.d2"}, []string{filepath.FromSlash("/work/proj/my diagram.d2")}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := PathsFromArgs(c.args, wd); !reflect.DeepEqual(got, c.want) {
				t.Fatalf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestRelativePathWithNoWorkingDirIsLeftAlone(t *testing.T) {
	if got := PathsFromArgs([]string{"a.d2"}, ""); !reflect.DeepEqual(got, []string{"a.d2"}) {
		t.Fatalf("got %q", got)
	}
}

func TestQueueTakeEmptiesIt(t *testing.T) {
	r := New(nil)
	if got := r.Take(); got == nil || len(got) != 0 {
		t.Fatalf("an empty queue should give an empty, non-nil slice (it serializes as [] not null): %#v", got)
	}
	r.Add("/a.d2")
	r.Add("/b.d2")
	if got := r.Take(); !reflect.DeepEqual(got, []string{"/a.d2", "/b.d2"}) {
		t.Fatalf("got %q", got)
	}
	if got := r.Take(); len(got) != 0 {
		t.Fatalf("Take must empty the queue, still has %q", got)
	}
}

func TestQueueIgnoresBlanksAndImmediateRepeats(t *testing.T) {
	r := New(nil)
	r.Add("")
	r.Add("   ")
	r.Add("/a.d2")
	r.Add("/a.d2") // the same file reported twice at launch
	r.Add("/b.d2")
	r.Add("/a.d2") // not an immediate repeat: a real second request
	if got := r.Take(); !reflect.DeepEqual(got, []string{"/a.d2", "/b.d2", "/a.d2"}) {
		t.Fatalf("got %q", got)
	}
}

func TestQueueIsBoundedAndKeepsTheNewest(t *testing.T) {
	r := New(nil)
	for i := 0; i < maxPending+10; i++ {
		r.Add(filepath.FromSlash("/f" + string(rune('a'+i%26)) + string(rune('a'+i/26)) + ".d2"))
	}
	got := r.Take()
	if len(got) != maxPending {
		t.Fatalf("len = %d, want %d", len(got), maxPending)
	}
	if got[len(got)-1] != filepath.FromSlash("/f"+string(rune('a'+(maxPending+9)%26))+string(rune('a'+(maxPending+9)/26))+".d2") {
		t.Fatalf("the newest request must be kept, last = %q", got[len(got)-1])
	}
}

func TestNotifyIsCalledOnlyForRealAdds(t *testing.T) {
	calls := 0
	r := New(func() { calls++ })
	r.Add("")
	r.Add("/a.d2")
	r.Add("/a.d2")
	r.AddArgs([]string{"--flag", "b.d2"}, "/w")
	if calls != 2 {
		t.Fatalf("notify called %d times, want 2 (one per accepted path)", calls)
	}
}

func TestAddArgsQueuesTheFilesOnACommandLine(t *testing.T) {
	r := New(nil)
	r.AddArgs([]string{"-v", "a.d2", "https://x/y.d2"}, filepath.FromSlash("/w"))
	if got := r.Take(); !reflect.DeepEqual(got, []string{filepath.FromSlash("/w/a.d2")}) {
		t.Fatalf("got %q", got)
	}
}

func TestQueueIsSafeForConcurrentUse(t *testing.T) {
	r := New(func() {})
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() { defer wg.Done(); r.Add(filepath.FromSlash("/x.d2")) }()
		go func() { defer wg.Done(); _ = r.Take() }()
	}
	wg.Wait()
}
