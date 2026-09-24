package files

import (
	"errors"
	"io/fs"
	"os"
	"sync"
	"time"
)

// EventKind says what happened to the watched file.
type EventKind string

const (
	// EventModified: the content on disk differs from the last known version.
	EventModified EventKind = "modified"
	// EventDeleted: the file no longer exists (deleted, or renamed away).
	EventDeleted EventKind = "deleted"
	// EventUnavailable: the file exists but cannot be read right now
	// (permissions, a network drive going away, ...).
	EventUnavailable EventKind = "unavailable"
	// EventRestored: a deleted or unavailable file is back, with the content
	// we last knew about.
	EventRestored EventKind = "restored"
)

// Event describes a change to the watched file. Each change is reported once.
type Event struct {
	Path string    `json:"path"`
	Kind EventKind `json:"kind"`
	// Version is the current on-disk version for modified events.
	Version Version `json:"version"`
	// Message carries the underlying error for unavailable events.
	Message string `json:"message"`
}

type fileState int

const (
	stateOK fileState = iota
	stateDeleted
	stateUnavailable
)

// Watcher polls a single file for external changes. Polling is used instead
// of OS notifications because it behaves the same on every platform, copes
// with editors and tools that save by renaming a new file over the old one,
// and for one file the cost is a stat call per interval; the file is read
// and hashed only when its size or modification time changed.
type Watcher struct {
	interval time.Duration
	onEvent  func(Event)

	mu       sync.Mutex
	path     string
	base     Version // what the editor currently considers on disk
	state    fileState
	notified string // hash already reported as modified, to avoid repeats
	stop     chan struct{}
	done     chan struct{}
}

// NewWatcher creates a Watcher that calls onEvent from its own goroutine.
func NewWatcher(interval time.Duration, onEvent func(Event)) *Watcher {
	return &Watcher{interval: interval, onEvent: onEvent}
}

// Watch starts watching path, which is currently at version base. Any
// previous watch is replaced.
func (w *Watcher) Watch(path string, base Version) {
	w.Stop()
	w.mu.Lock()
	w.path, w.base, w.state, w.notified = path, base, stateOK, ""
	stop, done := make(chan struct{}), make(chan struct{})
	w.stop, w.done = stop, done
	w.mu.Unlock()

	go func() {
		defer close(done)
		t := time.NewTicker(w.interval)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				w.poll()
			}
		}
	}()
}

// Rebase records that the editor now matches version (after the user saved
// or reloaded), so earlier changes are no longer reported.
func (w *Watcher) Rebase(version Version) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.base, w.notified, w.state = version, "", stateOK
}

// Stop ends the current watch and waits for its goroutine to exit.
func (w *Watcher) Stop() {
	w.mu.Lock()
	stop, done := w.stop, w.done
	w.stop, w.done = nil, nil
	w.mu.Unlock()
	if stop != nil {
		close(stop)
		<-done
	}
}

// poll checks the file once and emits at most one event.
func (w *Watcher) poll() {
	w.mu.Lock()
	ev := w.check()
	w.mu.Unlock()
	if ev != nil && w.onEvent != nil {
		w.onEvent(*ev)
	}
}

// check inspects the file and updates state. Callers hold w.mu.
func (w *Watcher) check() *Event {
	if w.path == "" {
		return nil
	}
	info, err := os.Stat(w.path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		if w.state == stateDeleted {
			return nil
		}
		w.state = stateDeleted
		return &Event{Path: w.path, Kind: EventDeleted}
	case err != nil:
		if w.state == stateUnavailable {
			return nil
		}
		w.state = stateUnavailable
		return &Event{Path: w.path, Kind: EventUnavailable, Message: err.Error()}
	}

	wasDown := w.state != stateOK
	// Unchanged metadata means unchanged content, unless the file just came back.
	if !wasDown && info.ModTime().UnixNano() == w.base.ModTimeNs && info.Size() == w.base.Size {
		return nil
	}

	data, err := os.ReadFile(w.path)
	if err != nil {
		if w.state == stateUnavailable {
			return nil
		}
		w.state = stateUnavailable
		return &Event{Path: w.path, Kind: EventUnavailable, Message: err.Error()}
	}
	cur := versionOf(info, data)
	w.state = stateOK

	if cur.Hash == w.base.Hash {
		// Back to (or still at) the known content. If we had told the caller
		// the file was gone or changed, tell them it is fine again.
		reported := wasDown || w.notified != ""
		w.base = cur // remember the new timestamp so the next poll skips hashing
		w.notified = ""
		if reported {
			return &Event{Path: w.path, Kind: EventRestored, Version: cur}
		}
		return nil
	}
	if cur.Hash == w.notified {
		return nil // already reported this exact content
	}
	w.notified = cur.Hash
	return &Event{Path: w.path, Kind: EventModified, Version: cur}
}
