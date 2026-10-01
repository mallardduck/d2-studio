package workspace

import (
	"sync"
	"time"
)

// EventChanged is emitted when the workspace's .d2 files change on disk, or
// when the folder becomes unreadable or readable again. Its data is an Update.
const EventChanged = "workspace:changed"

// DefaultPollInterval is how often the workspace is rescanned.
const DefaultPollInterval = 2 * time.Second

// Update is sent to the frontend when the workspace changes.
type Update struct {
	Root string `json:"root"`
	// Tree is the fresh scan, or nil when Error is set.
	Tree *Tree `json:"tree"`
	// Error is set when the folder can no longer be read (deleted, moved,
	// permissions). The workspace keeps being polled and an update with a
	// Tree follows if it comes back.
	Error string `json:"error"`
}

// Platform is what the Service needs from the desktop shell.
type Platform interface {
	// PickFolder shows a native folder dialog. It returns "" if cancelled.
	PickFolder(dir string) (string, error)
	// Emit sends an event to the frontend.
	Emit(event string, data any)
}

// OpenResult is the outcome of opening a folder.
type OpenResult struct {
	Cancelled bool  `json:"cancelled"`
	Tree      *Tree `json:"tree"`
}

// Service is the frontend-facing workspace API.
type Service struct {
	plat     Platform
	limits   Limits
	interval time.Duration

	mu      sync.Mutex
	root    string
	sig     string
	failing bool // the last poll could not read the folder
	stop    chan struct{}
	done    chan struct{}
}

// NewService creates a Service.
func NewService(plat Platform, limits Limits, interval time.Duration) *Service {
	return &Service{plat: plat, limits: limits, interval: interval}
}

// OpenFolderDialog asks the user for a folder and opens it as the workspace.
func (s *Service) OpenFolderDialog(dir string) (*OpenResult, error) {
	path, err := s.plat.PickFolder(dir)
	if err != nil {
		return nil, err
	}
	if path == "" {
		return &OpenResult{Cancelled: true}, nil
	}
	tree, err := s.OpenFolder(path)
	if err != nil {
		return nil, err
	}
	return &OpenResult{Tree: tree}, nil
}

// OpenFolder scans path, makes it the workspace and starts watching it. If the
// folder cannot be scanned, the current workspace is left as it was.
func (s *Service) OpenFolder(path string) (*Tree, error) {
	tree, err := Scan(path, s.limits)
	if err != nil {
		return nil, err
	}
	s.Close()

	s.mu.Lock()
	s.root, s.sig, s.failing = tree.Root, tree.Signature, false
	stop, done := make(chan struct{}), make(chan struct{})
	s.stop, s.done = stop, done
	s.mu.Unlock()

	go func() {
		defer close(done)
		t := time.NewTicker(s.interval)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				s.poll()
			}
		}
	}()
	return tree, nil
}

// Refresh rescans the workspace now and returns the result.
func (s *Service) Refresh() (*Tree, error) {
	s.mu.Lock()
	root := s.root
	s.mu.Unlock()
	if root == "" {
		return nil, nil
	}
	tree, err := Scan(root, s.limits)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.sig, s.failing = tree.Signature, false
	s.mu.Unlock()
	return tree, nil
}

// Close stops watching and forgets the workspace.
func (s *Service) Close() {
	s.mu.Lock()
	stop, done := s.stop, s.done
	s.stop, s.done = nil, nil
	s.root, s.sig, s.failing = "", "", false
	s.mu.Unlock()
	if stop != nil {
		close(stop)
		<-done
	}
}

// poll rescans once and emits an update if something changed.
func (s *Service) poll() {
	s.mu.Lock()
	root, prev, wasFailing := s.root, s.sig, s.failing
	s.mu.Unlock()
	if root == "" {
		return
	}

	tree, err := Scan(root, s.limits)

	s.mu.Lock()
	if s.root != root { // closed or replaced while scanning
		s.mu.Unlock()
		return
	}
	var update *Update
	switch {
	case err != nil:
		if !wasFailing {
			s.failing = true
			update = &Update{Root: root, Error: err.Error()}
		}
	case wasFailing || tree.Signature != prev:
		s.failing, s.sig = false, tree.Signature
		update = &Update{Root: root, Tree: tree}
	}
	s.mu.Unlock()

	if update != nil {
		s.plat.Emit(EventChanged, *update)
	}
}
