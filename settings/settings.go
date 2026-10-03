// Package settings persists user preferences as JSON in the OS config
// directory. It is independent of the UI so it can be tested on its own.
package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

const (
	ViewSource  = "source"
	ViewSplit   = "split"
	ViewPreview = "preview"

	MinSplitRatio = 0.15
	MaxSplitRatio = 0.85
)

// Settings are the persisted preferences. Zero values mean "not chosen".
type Settings struct {
	ViewMode string `json:"viewMode"`
	// SplitRatio is the fraction of the width given to the source pane.
	SplitRatio float64 `json:"splitRatio"`
	// Layout is a layout engine ID, or "" to follow the D2 source / default.
	Layout string `json:"layout"`
	// ThemeID is a D2 theme ID, or nil to follow the D2 source / default.
	ThemeID *int64 `json:"themeId"`
	// Workspace is the folder open in the sidebar, or "" for none.
	Workspace string `json:"workspace"`
	// SidebarOpen is whether the workspace sidebar is shown.
	SidebarOpen bool `json:"sidebarOpen"`
	// ExportRemoteImages lets PNG and PDF export download remote images such
	// as icons. Off by default so exporting works offline.
	ExportRemoteImages bool `json:"exportRemoteImages"`
}

// Defaults returns the settings used on first launch.
func Defaults() Settings {
	return Settings{ViewMode: ViewSplit, SplitRatio: 0.5, SidebarOpen: true}
}

// Normalize replaces invalid values with defaults and clamps the split ratio,
// so a hand-edited or corrupt file can never produce an unusable layout.
func (s Settings) Normalize() Settings {
	d := Defaults()
	switch s.ViewMode {
	case ViewSource, ViewSplit, ViewPreview:
	default:
		s.ViewMode = d.ViewMode
	}
	switch {
	case !(s.SplitRatio > 0): // unset, negative or NaN
		s.SplitRatio = d.SplitRatio
	case s.SplitRatio < MinSplitRatio:
		s.SplitRatio = MinSplitRatio
	case s.SplitRatio > MaxSplitRatio:
		s.SplitRatio = MaxSplitRatio
	}
	return s
}

// Store reads and writes Settings at a fixed path. Safe for concurrent use.
type Store struct {
	mu   sync.Mutex
	path string
}

// NewStore returns a Store using path.
func NewStore(path string) *Store { return &Store{path: path} }

// DefaultPath is <user config dir>/D2 Studio/settings.json.
func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locate user config directory: %w", err)
	}
	return filepath.Join(dir, "D2 Studio", "settings.json"), nil
}

// Load returns the saved settings. A missing file yields defaults and no
// error. An unreadable or corrupt file yields defaults together with an error
// describing the problem; the bad file is left untouched until the next Save.
func (s *Store) Load() (Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return Defaults(), nil
	}
	if err != nil {
		return Defaults(), fmt.Errorf("read settings: %w", err)
	}
	got := Defaults()
	if err := json.Unmarshal(b, &got); err != nil {
		return Defaults(), fmt.Errorf("settings file is corrupt, using defaults: %w", err)
	}
	return got.Normalize(), nil
}

// Save writes settings atomically (temp file + rename) so a crash mid-write
// cannot leave a truncated file.
func (s *Store) Save(in Settings) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, err := json.MarshalIndent(in.Normalize(), "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(s.path, b)
}

// writeFileAtomic writes data to path via a temp file in the same directory
// and a rename, creating the directory if needed.
func writeFileAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create settings directory: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".settings-*.tmp")
	if err != nil {
		return fmt.Errorf("write settings: %w", err)
	}
	defer os.Remove(tmp.Name()) // no-op after a successful rename
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write settings: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("write settings: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("write settings: %w", err)
	}
	return nil
}
