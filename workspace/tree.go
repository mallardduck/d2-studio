// Package workspace scans a folder for D2 files and watches it for changes.
// It has no UI dependencies so it can be tested on its own.
package workspace

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Limits bound a scan so a huge folder cannot hang or flood the UI.
type Limits struct {
	MaxDepth int
	// MaxEntries caps the .d2 files and folders listed.
	MaxEntries int
	// MaxVisited caps every directory entry examined, listed or not. Without
	// it a huge folder of unrelated files would be walked in full, and again
	// on every poll. Zero or negative means no limit.
	MaxVisited int
}

// DefaultLimits suit a "lightweight" project tree.
var DefaultLimits = Limits{MaxDepth: 8, MaxEntries: 2000, MaxVisited: 20_000}

// Node is a directory or a .d2 file.
type Node struct {
	Name string `json:"name"`
	// Path is absolute.
	Path     string  `json:"path"`
	Dir      bool    `json:"dir"`
	Children []*Node `json:"children"`
}

// Tree is the scanned contents of a workspace folder.
type Tree struct {
	Root string `json:"root"`
	Name string `json:"name"`
	// Nodes are the root's children.
	Nodes []*Node `json:"nodes"`
	// FileCount is the number of .d2 files found.
	FileCount int `json:"fileCount"`
	// Truncated is true when a limit stopped the scan early, so the tree may
	// be missing files (a folder that is too large or too deeply nested).
	Truncated bool `json:"truncated"`
	// Signature changes whenever a listed file is added, removed or modified.
	Signature string `json:"signature"`
}

// skipDir reports whether a directory is never descended into.
func skipDir(name string) bool {
	return strings.HasPrefix(name, ".") || name == "node_modules"
}

func isD2File(name string) bool {
	return !strings.HasPrefix(name, ".") && strings.EqualFold(filepath.Ext(name), ".d2")
}

type scanner struct {
	limits    Limits
	entries   int
	visited   int
	fileCount int
	truncated bool
	sig       []string
}

// Scan lists the .d2 files under root. Folders without any .d2 file below
// them are omitted; hidden folders and node_modules are skipped; symlinked
// folders are not followed (so cycles cannot occur) though symlinked files are
// listed. Unreadable subfolders are skipped rather than failing the scan.
func Scan(root string, limits Limits) (*Tree, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%s does not exist", abs)
	}
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a folder", abs)
	}

	sc := &scanner{limits: limits}
	nodes, err := sc.dir(abs, 0)
	if err != nil {
		return nil, err // the root itself could not be read
	}

	sort.Strings(sc.sig)
	sum := sha256.Sum256([]byte(strings.Join(sc.sig, "\n")))
	return &Tree{
		Root:      abs,
		Name:      filepath.Base(abs),
		Nodes:     nodes,
		FileCount: sc.fileCount,
		Truncated: sc.truncated,
		Signature: hex.EncodeToString(sum[:]),
	}, nil
}

func (sc *scanner) dir(path string, depth int) ([]*Node, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	nodes := []*Node{}
	for _, e := range entries {
		if sc.entries >= sc.limits.MaxEntries {
			sc.truncated = true
			break
		}
		sc.visited++
		if sc.limits.MaxVisited > 0 && sc.visited > sc.limits.MaxVisited {
			sc.truncated = true
			break
		}
		name := e.Name()
		full := filepath.Join(path, name)

		isDir, isFile := e.IsDir(), e.Type().IsRegular()
		if e.Type()&fs.ModeSymlink != 0 {
			target, err := os.Stat(full)
			if err != nil {
				continue // broken link
			}
			if target.IsDir() {
				continue // never follow linked folders
			}
			isFile = target.Mode().IsRegular()
		}

		switch {
		case isDir:
			if skipDir(name) {
				continue
			}
			if depth+1 > sc.limits.MaxDepth {
				sc.truncated = true
				continue
			}
			children, err := sc.dir(full, depth+1)
			if err != nil || len(children) == 0 {
				continue // unreadable, or nothing D2 inside
			}
			sc.entries++
			nodes = append(nodes, &Node{Name: name, Path: full, Dir: true, Children: children})
		case isFile && isD2File(name):
			info, err := os.Stat(full)
			if err != nil {
				continue
			}
			sc.entries++
			sc.fileCount++
			sc.sig = append(sc.sig, fmt.Sprintf("%s|%d|%d", full, info.Size(), info.ModTime().UnixNano()))
			nodes = append(nodes, &Node{Name: name, Path: full, Children: []*Node{}})
		}
	}
	sortNodes(nodes)
	return nodes, nil
}

// sortNodes puts folders first, then files, each alphabetical ignoring case.
func sortNodes(nodes []*Node) {
	sort.SliceStable(nodes, func(i, j int) bool {
		a, b := nodes[i], nodes[j]
		if a.Dir != b.Dir {
			return a.Dir
		}
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})
}
