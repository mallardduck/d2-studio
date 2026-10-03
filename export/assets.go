package export

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/d2lang/d2/d2target"
)

// errOffline is returned for every network request made while remote images
// are not allowed.
var errOffline = errors.New("remote images are disabled for this export")

// offlineTransport refuses all requests, so no connection is ever attempted.
type offlineTransport struct{}

func (offlineTransport) RoundTrip(*http.Request) (*http.Response, error) { return nil, errOffline }

// prepared is a diagram whose local images have been read and inlined.
type prepared struct {
	diagram *d2target.Diagram
	// remote lists the distinct http(s) image URLs the diagram refers to.
	remote []string
}

// prepareAssets returns a copy of d in which every local image has been read
// from disk and replaced by a data URI, and reports the remote images the
// diagram uses.
//
// Local images are read through os.Root confined to root, exactly like D2
// imports, so a diagram from an untrusted file cannot embed arbitrary files
// from elsewhere on disk into an export. Reading them here (and not leaving
// it to D2's image resolver, which accepts any path) also leaves no gap in
// which a symlink could be swapped between a check and the read.
func prepareAssets(d *d2target.Diagram, root, fileDir string) (*prepared, error) {
	p := &prepared{}
	var dir *os.Root
	if root != "" {
		r, err := os.OpenRoot(root)
		if err != nil {
			return nil, fmt.Errorf("cannot read images from %s: %w", root, err)
		}
		defer r.Close()
		dir = r
	}
	in := &inliner{root: root, fileDir: fileDir, dir: dir, cache: map[string]*url.URL{}}

	c := *d
	c.Root = d.Root
	var err error
	if c.Root.Icon, err = in.resolve(c.Root.Icon); err != nil {
		return nil, err
	}
	c.Shapes = slices.Clone(d.Shapes)
	for i := range c.Shapes {
		if c.Shapes[i].Icon, err = in.resolve(c.Shapes[i].Icon); err != nil {
			return nil, err
		}
	}
	c.Connections = slices.Clone(d.Connections)
	for i := range c.Connections {
		if c.Connections[i].Icon, err = in.resolve(c.Connections[i].Icon); err != nil {
			return nil, err
		}
	}
	p.diagram = &c
	p.remote = slices.Sorted(func(yield func(string) bool) {
		for u := range in.remote {
			if !yield(u) {
				return
			}
		}
	})
	return p, nil
}

type inliner struct {
	root string
	// fileDir is what relative image paths are relative to: the folder of
	// the file being exported (not the root, which may be a parent workspace).
	fileDir string
	dir     *os.Root
	cache   map[string]*url.URL
	remote  map[string]struct{}
}

// resolve returns u unchanged for data and remote images, and an inlined data
// URI for local ones.
func (in *inliner) resolve(u *url.URL) (*url.URL, error) {
	if u == nil {
		return nil, nil
	}
	switch strings.ToLower(u.Scheme) {
	case "data":
		return u, nil
	case "http", "https":
		if in.remote == nil {
			in.remote = map[string]struct{}{}
		}
		in.remote[u.String()] = struct{}{}
		return u, nil
	case "", "file":
		return in.local(u)
	default:
		return nil, fmt.Errorf("image %q uses the unsupported %q scheme", u.String(), u.Scheme)
	}
}

func (in *inliner) local(u *url.URL) (*url.URL, error) {
	name := u.Path
	if name == "" {
		name = u.Opaque
	}
	if name == "" {
		return nil, fmt.Errorf("image %q has no path", u.String())
	}
	if in.dir == nil {
		return nil, fmt.Errorf("image %q is a local file, but this document is not saved yet; save it first so the image can be found", name)
	}
	if cached, ok := in.cache[name]; ok {
		return cached, nil
	}

	// Resolve against the file's folder, then express the result relative to
	// the root so os.Root can enforce containment.
	abs := filepath.FromSlash(name)
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(in.fileDir, abs)
	}
	rel, err := filepath.Rel(in.root, filepath.Clean(abs))
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, outside(name, in.root)
	}

	f, err := in.dir.Open(rel)
	if err != nil {
		switch {
		case errors.Is(err, fs.ErrNotExist):
			return nil, fmt.Errorf("image %q was not found", name)
		case strings.Contains(err.Error(), "path escapes from parent"):
			return nil, outside(name, in.root)
		}
		return nil, fmt.Errorf("cannot read image %q: %w", name, unwrap(err))
	}
	defer f.Close()

	data, err := io.ReadAll(io.LimitReader(f, imageAssetMaxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("cannot read image %q: %w", name, err)
	}
	if int64(len(data)) > imageAssetMaxBytes {
		return nil, fmt.Errorf("image %q is larger than the %d MB limit", name, imageAssetMaxBytes>>20)
	}
	mime := imageMIME(name, data)
	if mime == "" {
		return nil, fmt.Errorf("%q does not look like an image", name)
	}
	out, err := url.Parse("data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data))
	if err != nil {
		return nil, err
	}
	in.cache[name] = out
	return out, nil
}

func outside(name, root string) error {
	return fmt.Errorf("image %q is outside the folder images may be read from (%s), or reached through a symlink that leaves it; open the containing folder as the workspace to allow it", name, root)
}

func unwrap(err error) error {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return pe.Err
	}
	return err
}

// imageMIME identifies an image from its content, with SVG recognized by
// extension (it is text and cannot be sniffed reliably). It returns "" for
// anything that is not an image.
func imageMIME(name string, data []byte) string {
	if strings.EqualFold(filepath.Ext(name), ".svg") {
		return "image/svg+xml"
	}
	mime := http.DetectContentType(data)
	if strings.HasPrefix(mime, "image/") {
		return mime
	}
	return ""
}
