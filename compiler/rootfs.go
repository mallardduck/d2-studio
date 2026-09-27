package compiler

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// rootFS is the filesystem D2 reads imports from. It only lets a compile
// read files inside root, so a diagram opened from an untrusted source cannot
// pull arbitrary files from elsewhere on disk into its output or error
// messages.
//
// Containment is enforced by os.Root (Go 1.24+), which checks every path
// component as it opens the file. That closes the gap a "resolve symlinks,
// check the prefix, then open" approach leaves, where a symlink can be
// swapped between the check and the open. Symlinks that stay inside root
// work if their target is relative; ones that lead outside, ones with an
// absolute target (even if it points inside), and ".." escapes are refused.
type rootFS struct {
	root string // as given, for messages
	dir  *os.Root
}

func newRootFS(root string) (*rootFS, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	dir, err := os.OpenRoot(abs)
	if err != nil {
		return nil, err
	}
	return &rootFS{root: abs, dir: dir}, nil
}

// Close releases the directory handle.
func (r *rootFS) Close() error { return r.dir.Close() }

// Open implements fs.FS. D2 passes absolute paths for imports (built from the
// importing file's path); relative names are taken as relative to root.
func (r *rootFS) Open(name string) (fs.File, error) {
	p := filepath.Clean(filepath.FromSlash(name))
	rel := p
	if filepath.IsAbs(p) {
		var err error
		if rel, err = filepath.Rel(r.root, p); err != nil || escapesLexically(rel) {
			return nil, &fs.PathError{Op: "open", Path: name, Err: &outsideError{root: r.root}}
		}
	} else if escapesLexically(rel) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: &outsideError{root: r.root}}
	}

	f, err := r.dir.Open(rel)
	if err != nil {
		if isEscape(err) {
			return nil, &fs.PathError{Op: "open", Path: name, Err: &outsideError{root: r.root}}
		}
		return nil, &fs.PathError{Op: "open", Path: name, Err: unwrapPathError(err)}
	}
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		f.Close()
		return nil, &fs.PathError{Op: "open", Path: name, Err: errors.New("not a regular file")}
	}
	return f, nil
}

func escapesLexically(rel string) bool {
	return rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// isEscape reports whether os.Root refused a path for leaving the root. The
// standard library does not export that error, so it is recognized by text.
func isEscape(err error) bool {
	return err != nil && strings.Contains(err.Error(), "path escapes from parent")
}

type outsideError struct{ root string }

func (e *outsideError) Error() string {
	return fmt.Sprintf("it is outside the folder imports may be read from (%s), or reached through a symlink that leaves it or uses an absolute path; open the containing folder as the workspace to allow it", e.root)
}

// unwrapPathError returns the underlying error of an *fs.PathError so
// messages do not repeat the path.
func unwrapPathError(err error) error {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return pe.Err
	}
	return err
}

// within reports whether path is root or inside it, comparing whole path
// components (so "/a/proj-evil" is not inside "/a/proj"). Both must be absolute.
func within(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || !escapesLexically(rel)
}

// ImportRoot chooses the directory imports may be read from for a file: the
// workspace folder when the file is inside it, otherwise the file's own
// directory. It returns "" for an unsaved document, which has no location to
// resolve imports against.
func ImportRoot(filePath, workspace string) string {
	if filePath == "" {
		return ""
	}
	abs, err := filepath.Abs(filePath)
	if err != nil {
		return ""
	}
	if workspace != "" {
		if ws, err := filepath.Abs(workspace); err == nil && within(ws, abs) {
			return ws
		}
	}
	return filepath.Dir(abs)
}
