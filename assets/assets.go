// Package assets prepares the images a diagram refers to: local files are read
// from disk and inlined as data URIs (confined to a folder, with no gap for a
// symlink to be swapped in), and remote images are reported, or fetched when
// the caller allows it. Both the live preview and the exporters use it.
package assets

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/d2lang/d2/d2target"

	"d2studio/logging"
)

var logger = logging.For("assets")

// maxLocalImageBytes is the largest local image that will be read. It matches
// the limit D2's own image loader applies.
const maxLocalImageBytes int64 = 32 * 1024 * 1024

// Options control Prepare.
type Options struct {
	// Lenient makes a local image that cannot be loaded (missing, outside the
	// folder, not an image) a reported Problem instead of an error, and leaves
	// its reference alone. The live preview uses it so one bad image does not
	// blank the diagram; exports are strict.
	Lenient bool
	// Fetcher loads remote (http/https) images. With none, remote images are
	// only reported and their references are left as they are.
	Fetcher Fetcher
}

// Prepared is a diagram whose images have been read and inlined.
type Prepared struct {
	Diagram *d2target.Diagram
	// Remote lists the distinct http(s) image URLs the diagram refers to.
	Remote []string
	// RemoteLoaded counts the remote images that were fetched and inlined.
	RemoteLoaded int
	// Problems describes images that could not be loaded (lenient mode, and
	// failed remote fetches in any mode). They never contain image data.
	Problems []string
}

// Prepare returns a copy of d in which every local image has been read from
// disk and replaced by a data URI, and, when a Fetcher is given, every remote
// image has been fetched and inlined too.
//
// Local images are read through os.Root confined to root, exactly like D2
// imports, so a diagram from an untrusted file cannot embed arbitrary files
// from elsewhere on disk. Reading them here (and not leaving it to D2's image
// resolver, which accepts any path) also leaves no gap in which a symlink
// could be swapped between a check and the read.
func Prepare(ctx context.Context, d *d2target.Diagram, root, fileDir string, opts Options) (*Prepared, error) {
	var dir *os.Root
	if root != "" {
		r, err := os.OpenRoot(root)
		if err != nil {
			if !opts.Lenient {
				return nil, fmt.Errorf("cannot read images from %s: %w", root, err)
			}
		} else {
			defer r.Close()
			dir = r
		}
	}
	in := &inliner{root: root, fileDir: fileDir, dir: dir, cache: map[string]*url.URL{}, lenient: opts.Lenient}

	c := *d
	p := &Prepared{}

	// Pass 1: inline local images and collect the remote URLs.
	if err := mapIcons(&c, d, in.resolve); err != nil {
		return nil, err
	}
	p.Problems = append(p.Problems, in.problems...)
	p.Remote = slices.Sorted(maps.Keys(in.remote))

	// Pass 2: fetch the remote images, if allowed.
	if opts.Fetcher != nil && len(p.Remote) > 0 {
		fetched, failed := fetchAll(ctx, opts.Fetcher, p.Remote)
		p.Problems = append(p.Problems, failed...)
		p.RemoteLoaded = len(fetched)
		if len(fetched) > 0 {
			err := mapIcons(&c, &c, func(u *url.URL) (*url.URL, error) {
				if u != nil {
					if data, ok := fetched[u.String()]; ok {
						return data, nil
					}
				}
				return u, nil
			})
			if err != nil {
				return nil, err
			}
		}
	}
	p.Diagram = &c
	return p, nil
}

// mapIcons applies fn to every icon in src and stores the results in dst,
// cloning the shape and connection slices so src is never modified. dst and
// src may be the same diagram.
func mapIcons(dst, src *d2target.Diagram, fn func(*url.URL) (*url.URL, error)) error {
	var err error
	if dst.Root.Icon, err = fn(src.Root.Icon); err != nil {
		return err
	}
	shapes := slices.Clone(src.Shapes)
	for i := range shapes {
		if shapes[i].Icon, err = fn(shapes[i].Icon); err != nil {
			return err
		}
	}
	dst.Shapes = shapes
	conns := slices.Clone(src.Connections)
	for i := range conns {
		if conns[i].Icon, err = fn(conns[i].Icon); err != nil {
			return err
		}
	}
	dst.Connections = conns
	return nil
}

// fetchAll fetches each URL (a few at a time) and returns the ones that
// succeeded as data URIs, and a message for each that failed.
func fetchAll(ctx context.Context, f Fetcher, urls []string) (map[string]*url.URL, []string) {
	type result struct {
		url string
		img *url.URL
		err error
	}
	results := make(chan result, len(urls))
	sem := make(chan struct{}, 4)
	var wg sync.WaitGroup
	for _, u := range urls {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			img, err := f.Fetch(ctx, u)
			if err != nil {
				results <- result{url: u, err: err}
				return
			}
			data, err := url.Parse("data:" + img.MIME + ";base64," + base64.StdEncoding.EncodeToString(img.Data))
			results <- result{url: u, img: data, err: err}
		}()
	}
	wg.Wait()
	close(results)

	ok := map[string]*url.URL{}
	var failed []string
	for r := range results {
		if r.err != nil {
			failed = append(failed, fmt.Sprintf("remote image %s could not be loaded: %v", RedactURL(r.url), r.err))
			continue
		}
		ok[r.url] = r.img
	}
	slices.Sort(failed)
	return ok, failed
}

// RedactURL drops the query and fragment, which often carry tokens, so a URL
// can be shown or logged.
func RedactURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "(unparseable URL)"
	}
	u.RawQuery, u.Fragment, u.User = "", "", nil
	return u.String()
}

type inliner struct {
	root string
	// fileDir is what relative image paths are relative to: the folder of
	// the file being exported (not the root, which may be a parent workspace).
	fileDir  string
	dir      *os.Root
	cache    map[string]*url.URL
	remote   map[string]struct{}
	lenient  bool
	problems []string
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
		out, err := in.local(u)
		return in.settle(u, out, err)
	default:
		return in.settle(u, nil, fmt.Errorf("image %q uses the unsupported %q scheme", u.String(), u.Scheme))
	}
}

// settle applies the strict or lenient policy to a local image result.
func (in *inliner) settle(orig, out *url.URL, err error) (*url.URL, error) {
	if err == nil {
		return out, nil
	}
	if !in.lenient {
		return nil, err
	}
	in.problems = append(in.problems, err.Error())
	return orig, nil // leave the reference as it was; it simply will not show
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

	data, err := io.ReadAll(io.LimitReader(f, maxLocalImageBytes+1))
	if err != nil {
		return nil, fmt.Errorf("cannot read image %q: %w", name, err)
	}
	if int64(len(data)) > maxLocalImageBytes {
		return nil, fmt.Errorf("image %q is larger than the %d MB limit", name, maxLocalImageBytes>>20)
	}
	mime := imageMIME(name, data)
	if mime == "" {
		return nil, fmt.Errorf("%q does not look like an image", name)
	}
	logger.Trace("inlined local image", "image", name, "bytes", len(data), "mime", mime)
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
