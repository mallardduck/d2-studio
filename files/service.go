package files

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"d2studio/settings"
)

// EventFileChanged is emitted when the open file changes on disk. Its data
// is an Event.
const EventFileChanged = "file:changed"

// DefaultPollInterval is how often the open file is checked for changes.
const DefaultPollInterval = time.Second

// Platform is what the Service needs from the desktop shell. It is an
// interface so the service can be tested without a window system.
type Platform interface {
	// PickOpenFile shows a native open dialog. It returns "" if cancelled.
	PickOpenFile(dir string) (string, error)
	// PickSaveFile shows a native save dialog. It returns "" if cancelled.
	PickSaveFile(dir, filename string) (string, error)
	// Emit sends an event to the frontend.
	Emit(event string, data any)
}

// Service is the frontend-facing file API: open, save, save as, recent
// files, and change notifications for the open file.
type Service struct {
	plat    Platform
	recent  *settings.RecentStore
	watcher *Watcher
}

// NewService creates a Service. recent may be nil, in which case no recent
// files are kept.
func NewService(plat Platform, recent *settings.RecentStore, pollInterval time.Duration) *Service {
	s := &Service{plat: plat, recent: recent}
	s.watcher = NewWatcher(pollInterval, func(e Event) { plat.Emit(EventFileChanged, e) })
	return s
}

// OpenResult is the outcome of an open operation.
type OpenResult struct {
	// Cancelled is true when the user dismissed the dialog.
	Cancelled bool      `json:"cancelled"`
	Document  *Document `json:"document"`
}

// OpenDialog asks the user to choose a .d2 file and opens it. dir is the
// directory the dialog starts in (may be empty).
func (s *Service) OpenDialog(dir string) (*OpenResult, error) {
	path, err := s.plat.PickOpenFile(dir)
	if err != nil {
		return nil, err
	}
	if path == "" {
		return &OpenResult{Cancelled: true}, nil
	}
	return s.OpenPath(path)
}

// OpenPath opens a file by path (from recent files or the workspace tree) and
// starts watching it. It is also how a file is reloaded after an external
// change. If the file cannot be opened, whatever was being watched keeps
// being watched.
func (s *Service) OpenPath(path string) (*OpenResult, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	doc, err := Open(abs)
	if err != nil {
		return nil, err
	}
	s.remember(abs)
	s.watcher.Watch(abs, doc.Version)
	return &OpenResult{Document: doc}, nil
}

// Save writes the open document to path. If the file changed on disk since it
// was opened, nothing is written and the result carries the conflict.
func (s *Service) Save(path, content string, opts SaveOptions) (*SaveResult, error) {
	res, err := Save(path, content, opts)
	if err != nil {
		return nil, err
	}
	if res.Saved {
		s.watcher.Rebase(res.Version) // our own write is not an external change
	}
	return res, nil
}

// SaveAsResult is the outcome of Save As.
type SaveAsResult struct {
	Cancelled bool    `json:"cancelled"`
	Path      string  `json:"path"`
	Name      string  `json:"name"`
	Version   Version `json:"version"`
}

// SaveAs asks for a destination and writes content there. A ".d2" extension
// is added when the user gave none. The new file becomes the watched file.
func (s *Service) SaveAs(dir, suggestedName, content string, opts SaveOptions) (*SaveAsResult, error) {
	path, err := s.plat.PickSaveFile(dir, suggestedName)
	if err != nil {
		return nil, err
	}
	if path == "" {
		return &SaveAsResult{Cancelled: true}, nil
	}
	if filepath.Ext(path) == "" {
		path += ".d2"
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	// The save dialog already asked the user to confirm an overwrite, so
	// there is no base version to compare against.
	opts.Base, opts.Force = nil, true
	res, err := Save(abs, content, opts)
	if err != nil {
		return nil, err
	}
	s.remember(abs)
	s.watcher.Watch(abs, res.Version)
	return &SaveAsResult{Path: abs, Name: filepath.Base(abs), Version: res.Version}, nil
}

// Unwatch stops watching the open file (for a new, unsaved document).
func (s *Service) Unwatch() { s.watcher.Stop() }

// Shutdown stops background work. Call when the app exits. It is for the Go
// side only, so it is left out of the frontend bindings.
//
//wails:ignore
func (s *Service) Shutdown() { s.watcher.Stop() }

// RecentFile is an entry in the recent files list.
type RecentFile struct {
	Path string `json:"path"`
	Name string `json:"name"`
	// Dir is the containing directory, to tell same-named files apart.
	Dir string `json:"dir"`
	// Exists is false when the file has been deleted or moved.
	Exists bool `json:"exists"`
}

// Recent lists recently opened files, most recent first.
func (s *Service) Recent() []RecentFile {
	out := []RecentFile{}
	if s.recent == nil {
		return out
	}
	paths, err := s.recent.List()
	if err != nil {
		log.Printf("files: %v", err)
	}
	for _, p := range paths {
		_, statErr := os.Stat(p)
		out = append(out, RecentFile{
			Path:   p,
			Name:   filepath.Base(p),
			Dir:    shortDir(filepath.Dir(p)),
			Exists: statErr == nil,
		})
	}
	return out
}

// RemoveRecent forgets one recent file.
func (s *Service) RemoveRecent(path string) error {
	if s.recent == nil {
		return nil
	}
	return s.recent.Remove(path)
}

// ClearRecent forgets all recent files.
func (s *Service) ClearRecent() error {
	if s.recent == nil {
		return nil
	}
	return s.recent.Clear()
}

// remember records a recent file. Failing to do so never fails the open or
// save the user asked for.
func (s *Service) remember(path string) {
	if s.recent == nil {
		return
	}
	if err := s.recent.Add(path); err != nil {
		log.Printf("files: remember recent file: %v", err)
	}
}

// shortDir abbreviates the home directory to "~" for display.
func shortDir(dir string) string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		if dir == home {
			return "~"
		}
		if strings.HasPrefix(dir, home+string(filepath.Separator)) {
			return "~" + dir[len(home):]
		}
	}
	return dir
}
