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
// messages. Symlinks are resolved before the check, so a link inside root
// cannot be used to escape it.
type rootFS struct {
	root     string // as given, for messages
	realRoot string // symlinks resolved, for the containment check
}

func newRootFS(root string) (*rootFS, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return nil, err
	}
	return &rootFS{root: abs, realRoot: real}, nil
}

// Open implements fs.FS. D2 passes absolute paths for imports (built from the
// importing file's path); relative names are taken as relative to root.
func (r *rootFS) Open(name string) (fs.File, error) {
	p := filepath.FromSlash(name)
	if !filepath.IsAbs(p) {
		p = filepath.Join(r.root, p)
	}
	p = filepath.Clean(p)

	real, err := filepath.EvalSymlinks(p)
	if err != nil {
		// Report a missing file as missing, even if it would also be outside.
		return nil, &fs.PathError{Op: "open", Path: name, Err: unwrapNotExist(err)}
	}
	if !within(r.realRoot, real) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: &outsideError{root: r.root}}
	}
	info, err := os.Stat(real)
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: unwrapNotExist(err)}
	}
	if !info.Mode().IsRegular() {
		return nil, &fs.PathError{Op: "open", Path: name, Err: errors.New("not a regular file")}
	}
	return os.Open(real)
}

type outsideError struct{ root string }

func (e *outsideError) Error() string {
	return fmt.Sprintf("it is outside the folder imports may be read from (%s); open that folder as the workspace to allow it", e.root)
}

// unwrapNotExist returns the underlying error of an *fs.PathError so messages
// do not repeat the path.
func unwrapNotExist(err error) error {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return pe.Err
	}
	return err
}

// within reports whether path is root or inside it. Both must be absolute
// and symlink-free.
func within(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
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
