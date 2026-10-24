// Package openfile collects requests from the operating system to open a file
// (a double-click, "Open With", a path on the command line, or a second launch
// of the app) until the frontend is ready to act on them.
//
// A request can arrive before the window has loaded, so requests are queued
// and the frontend collects them, rather than being pushed and possibly lost.
package openfile

import (
	"path/filepath"
	"strings"
	"sync"
)

// maxPending bounds the queue; a flood of launches cannot grow it without limit.
const maxPending = 16

// Requests is a thread-safe queue of file paths waiting to be opened.
type Requests struct {
	mu      sync.Mutex
	pending []string
	notify  func()
}

// New creates a queue. notify, if set, is called after a path is added, to
// tell the frontend there is something to collect.
func New(notify func()) *Requests { return &Requests{notify: notify} }

// Add queues one path. Empty paths and repeats of the most recent one are
// ignored (some platforms report the same file twice at launch).
func (r *Requests) Add(path string) {
	if strings.TrimSpace(path) == "" {
		return
	}
	r.mu.Lock()
	if n := len(r.pending); n > 0 && r.pending[n-1] == path {
		r.mu.Unlock()
		return
	}
	r.pending = append(r.pending, path)
	if len(r.pending) > maxPending {
		r.pending = r.pending[len(r.pending)-maxPending:] // keep the newest
	}
	r.mu.Unlock()
	if r.notify != nil {
		r.notify()
	}
}

// AddArgs queues the files named on a command line. args must not include the
// program name; see PathsFromArgs.
func (r *Requests) AddArgs(args []string, workingDir string) {
	for _, p := range PathsFromArgs(args, workingDir) {
		r.Add(p)
	}
}

// Take returns the queued paths, oldest first, and empties the queue.
func (r *Requests) Take() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := r.pending
	r.pending = nil
	if out == nil {
		out = []string{}
	}
	return out
}

// PathsFromArgs returns the file paths in command-line arguments (without the
// program name). Flags and URLs are skipped, and relative paths are resolved
// against workingDir, which is where the command was run, not where this
// process happens to be.
func PathsFromArgs(args []string, workingDir string) []string {
	var out []string
	for _, a := range args {
		a = strings.TrimSpace(a)
		switch {
		case a == "", strings.HasPrefix(a, "-"), strings.Contains(a, "://"):
			continue
		}
		if !filepath.IsAbs(a) && workingDir != "" {
			a = filepath.Join(workingDir, a)
		}
		out = append(out, filepath.Clean(a))
	}
	if out == nil {
		out = []string{}
	}
	return out
}
