package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
)

// MaxRecent is how many recent files are remembered.
const MaxRecent = 15

// RecentStore remembers recently opened files, most recent first. It is kept
// apart from Settings because the frontend saves the whole Settings object;
// recents living there could be overwritten by a stale copy.
type RecentStore struct {
	mu   sync.Mutex
	path string
}

// NewRecentStore returns a RecentStore that persists to path.
func NewRecentStore(path string) *RecentStore { return &RecentStore{path: path} }

// RecentPath is <user config dir>/D2 Studio/recent.json, beside settings.json.
func RecentPath() (string, error) {
	p, err := DefaultPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(p), "recent.json"), nil
}

// List returns the remembered paths, most recent first. A missing file is an
// empty list; a corrupt one yields an empty list plus an error, and is left
// untouched until the next change.
func (r *RecentStore) List() ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.load()
}

func (r *RecentStore) load() ([]string, error) {
	b, err := os.ReadFile(r.path)
	if errors.Is(err, os.ErrNotExist) {
		return []string{}, nil
	}
	if err != nil {
		return []string{}, fmt.Errorf("read recent files: %w", err)
	}
	var paths []string
	if err := json.Unmarshal(b, &paths); err != nil {
		return []string{}, fmt.Errorf("recent files list is corrupt, ignoring it: %w", err)
	}
	return cleanRecent(paths), nil
}

// Add records path as the most recently used.
func (r *RecentStore) Add(path string) error {
	if path == "" {
		return errors.New("no path to remember")
	}
	return r.change(func(list []string) []string {
		return append([]string{path}, list...)
	})
}

// Remove forgets path (for example because the file no longer exists).
func (r *RecentStore) Remove(path string) error {
	return r.change(func(list []string) []string {
		return slices.DeleteFunc(list, func(p string) bool { return p == path })
	})
}

// Clear forgets every recent file.
func (r *RecentStore) Clear() error {
	return r.change(func([]string) []string { return nil })
}

func (r *RecentStore) change(fn func([]string) []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	// A corrupt list is replaced by the new change rather than blocking it.
	list, _ := r.load()
	next := cleanRecent(fn(list))
	b, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(r.path, b)
}

// cleanRecent drops empty entries and duplicates (keeping the first, i.e.
// most recent) and caps the length.
func cleanRecent(in []string) []string {
	out := make([]string, 0, len(in))
	seen := make(map[string]bool, len(in))
	for _, p := range in {
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
		if len(out) == MaxRecent {
			break
		}
	}
	return out
}
