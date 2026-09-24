// Package files reads and writes .d2 files safely: atomic saves, conflict
// detection against external changes, and preservation of line endings and
// the UTF-8 BOM. It has no UI dependencies so it can be tested on its own.
package files

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// MaxFileSize is the largest file Open will read. A diagram source larger
// than this is almost certainly not something to edit in a live preview.
const MaxFileSize = 10 << 20

const (
	LineEndingLF   = "lf"
	LineEndingCRLF = "crlf"
)

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// Version identifies the on-disk state of a file. Hash is authoritative;
// ModTimeNs and Size let polling skip hashing when nothing changed.
type Version struct {
	ModTimeNs int64  `json:"modTimeNs"`
	Size      int64  `json:"size"`
	Hash      string `json:"hash"`
}

// Document is a file loaded for editing. Content always uses "\n" line
// endings and has no BOM; LineEnding and BOM record how to write it back.
type Document struct {
	Path             string  `json:"path"`
	Content          string  `json:"content"`
	Version          Version `json:"version"`
	LineEnding       string  `json:"lineEnding"`
	BOM              bool    `json:"bom"`
	MixedLineEndings bool    `json:"mixedLineEndings"`
}

// Open reads a file as UTF-8 text. It refuses (rather than guesses at) files
// that are too large, are not regular files, or are not valid UTF-8, so a
// later Save can never corrupt them.
func Open(path string) (*Document, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, friendly(path, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", path)
	}
	if info.Size() > MaxFileSize {
		return nil, fmt.Errorf("%s is %d MB, larger than the %d MB limit", path, info.Size()>>20, MaxFileSize>>20)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, friendly(path, err)
	}
	// Re-stat after reading so the version describes what we actually read.
	after, err := os.Stat(path)
	if err != nil {
		return nil, friendly(path, err)
	}

	body := data
	bom := bytes.HasPrefix(body, utf8BOM)
	if bom {
		body = body[len(utf8BOM):]
	}
	if !utf8.Valid(body) {
		return nil, fmt.Errorf("%s is not valid UTF-8; D2 Studio only edits UTF-8 text", path)
	}

	text := string(body)
	ending, mixed := detectLineEnding(text)
	if strings.Contains(text, "\r") {
		text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	}

	return &Document{
		Path:             path,
		Content:          text,
		Version:          versionOf(after, data),
		LineEnding:       ending,
		BOM:              bom,
		MixedLineEndings: mixed,
	}, nil
}

// detectLineEnding returns the dominant ending and whether both kinds occur.
func detectLineEnding(s string) (ending string, mixed bool) {
	crlf := strings.Count(s, "\r\n")
	lf := strings.Count(s, "\n") - crlf
	switch {
	case crlf == 0:
		return LineEndingLF, false
	case crlf >= lf:
		return LineEndingCRLF, lf > 0
	default:
		return LineEndingLF, true
	}
}

// Conflict kinds reported by Save.
const (
	ConflictModified = "modified" // file changed on disk since it was opened
	ConflictDeleted  = "deleted"  // file no longer exists
	ConflictExists   = "exists"   // saving a new document over a file that appeared
)

// SaveOptions control how Save writes.
type SaveOptions struct {
	// Base is the version the document was opened or last saved at. When
	// set, Save refuses if the file on disk no longer matches it. Leave nil
	// for Save As, where the user has already confirmed the destination.
	Base *Version `json:"base"`
	// Force skips the conflict check (the user chose to overwrite).
	Force      bool   `json:"force"`
	LineEnding string `json:"lineEnding"`
	BOM        bool   `json:"bom"`
}

// SaveResult reports the outcome. A conflict is a normal outcome, not an
// error: the file was left untouched and the caller decides what to do.
type SaveResult struct {
	Saved bool `json:"saved"`
	// Conflict is one of the Conflict* constants when Saved is false.
	Conflict string `json:"conflict"`
	// Version is the new version after a save, or the current on-disk
	// version when there is a "modified" conflict.
	Version Version `json:"version"`
}

// Save writes content atomically. The data goes to a temporary file in the
// same directory which is then renamed over the target, so a crash or full
// disk can never leave a truncated file behind. Symlinks are written through
// to their target and existing permissions are kept.
//
// The conflict check and the rename are not one atomic step, so a write by
// another program in that tiny window can still be overwritten; this is the
// best that plain filesystems allow.
func Save(path, content string, opts SaveOptions) (*SaveResult, error) {
	target, err := resolveTarget(path)
	if err != nil {
		return nil, friendly(path, err)
	}

	if !opts.Force && opts.Base != nil {
		if res, err := checkConflict(target, *opts.Base); err != nil || res != nil {
			return res, err
		}
	}

	data := encode(content, opts)
	mode := fs.FileMode(0o644)
	if info, err := os.Stat(target); err == nil {
		mode = info.Mode().Perm()
	}

	if err := writeAtomic(target, data, mode); err != nil {
		return nil, friendly(path, err)
	}
	info, err := os.Stat(target)
	if err != nil {
		return nil, friendly(path, err)
	}
	return &SaveResult{Saved: true, Version: versionOf(info, data)}, nil
}

// checkConflict returns a non-nil result when the on-disk file no longer
// matches base.
func checkConflict(target string, base Version) (*SaveResult, error) {
	info, err := os.Stat(target)
	if errors.Is(err, fs.ErrNotExist) {
		return &SaveResult{Conflict: ConflictDeleted}, nil
	}
	if err != nil {
		return nil, friendly(target, err)
	}
	if info.ModTime().UnixNano() == base.ModTimeNs && info.Size() == base.Size {
		return nil, nil // metadata unchanged: the common, cheap case
	}
	data, err := os.ReadFile(target)
	if err != nil {
		return nil, friendly(target, err)
	}
	cur := versionOf(info, data)
	if cur.Hash == base.Hash {
		return nil, nil // touched, but the content is what we opened
	}
	return &SaveResult{Conflict: ConflictModified, Version: cur}, nil
}

// resolveTarget follows symlinks so the link itself is not replaced by a
// regular file. A path that does not exist yet is used as is.
func resolveTarget(path string) (string, error) {
	if path == "" {
		return "", errors.New("no file path given")
	}
	resolved, err := filepath.EvalSymlinks(path)
	if errors.Is(err, fs.ErrNotExist) {
		return path, nil
	}
	return resolved, err
}

func encode(content string, opts SaveOptions) []byte {
	if opts.LineEnding == LineEndingCRLF {
		content = strings.ReplaceAll(content, "\n", "\r\n")
	}
	var buf bytes.Buffer
	if opts.BOM {
		buf.Write(utf8BOM)
	}
	buf.WriteString(content)
	return buf.Bytes()
}

func writeAtomic(target string, data []byte, mode fs.FileMode) (err error) {
	dir := filepath.Dir(target)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(target)+".*.tmp")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tmp.Close()
			os.Remove(tmp.Name())
		}
	}()
	if _, err = tmp.Write(data); err != nil {
		return err
	}
	if err = tmp.Chmod(mode); err != nil && !errors.Is(err, errors.ErrUnsupported) {
		return err
	}
	if err = tmp.Sync(); err != nil {
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), target)
}

func versionOf(info fs.FileInfo, data []byte) Version {
	sum := sha256.Sum256(data)
	return Version{ModTimeNs: info.ModTime().UnixNano(), Size: info.Size(), Hash: hex.EncodeToString(sum[:])}
}

// Stat returns the current version of a file, reading it only when needed.
func Stat(path string) (Version, error) {
	info, err := os.Stat(path)
	if err != nil {
		return Version{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Version{}, err
	}
	return versionOf(info, data), nil
}

// friendly turns low-level errors into messages a user can act on, keeping
// the original error available to errors.Is.
func friendly(path string, err error) error {
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("%s does not exist: %w", path, err)
	case errors.Is(err, fs.ErrPermission):
		return fmt.Errorf("permission denied for %s: %w", path, err)
	default:
		return fmt.Errorf("%s: %w", path, err)
	}
}
