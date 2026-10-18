package assets

import (
	"bytes"
	"container/list"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Image is a fetched image.
type Image struct {
	MIME string
	Data []byte
}

// Fetcher loads a remote image.
type Fetcher interface {
	Fetch(ctx context.Context, rawURL string) (*Image, error)
}

// HTTPOptions configure an HTTPFetcher. Zero values mean the defaults.
type HTTPOptions struct {
	// MaxImageBytes is the largest image accepted. Default 2 MiB: plenty for
	// an icon, and a small bound on what a diagram can make the app download.
	MaxImageBytes int64
	// CacheBytes bounds the images kept in memory. Default 32 MiB.
	CacheBytes int64
	// Timeout is the limit for one fetch, redirects included. Default 10 s.
	Timeout time.Duration
	// SuccessTTL and FailureTTL say how long results are reused. Defaults are
	// 10 minutes and 60 seconds: a failing URL is not retried on every
	// keystroke, and a fixed one is picked up soon.
	SuccessTTL, FailureTTL time.Duration
	// AllowPrivateNetworks lets fetches reach loopback, private and link-local
	// addresses. Off by default so a diagram cannot make the app probe the
	// user's own network; tests turn it on to use a local server.
	AllowPrivateNetworks bool
	// Now is the clock; defaults to time.Now.
	Now func() time.Time
}

const maxRedirects = 3

var (
	errPrivateAddress = errors.New("the address is on a private or local network, which images may not be fetched from")
	errNotImage       = errors.New("the response is not an image")
)

// HTTPFetcher fetches images over HTTP(S) with safeguards: only http and
// https, no private networks (checked when connecting, so redirects and DNS
// tricks cannot get around it), a size limit, a timeout, image content only,
// and a bounded cache.
type HTTPFetcher struct {
	opts   HTTPOptions
	client *http.Client

	mu       sync.Mutex
	entries  map[string]*list.Element
	order    *list.List // front = most recently used
	bytes    int64
	inflight map[string]*call
}

type cacheEntry struct {
	key     string
	img     *Image
	err     error
	expires time.Time
}

type call struct {
	done chan struct{}
	img  *Image
	err  error
}

// NewHTTPFetcher creates a fetcher.
func NewHTTPFetcher(o HTTPOptions) *HTTPFetcher {
	if o.MaxImageBytes <= 0 {
		o.MaxImageBytes = 2 << 20
	}
	if o.CacheBytes <= 0 {
		o.CacheBytes = 32 << 20
	}
	if o.Timeout <= 0 {
		o.Timeout = 10 * time.Second
	}
	if o.SuccessTTL <= 0 {
		o.SuccessTTL = 10 * time.Minute
	}
	if o.FailureTTL <= 0 {
		o.FailureTTL = time.Minute
	}
	if o.Now == nil {
		o.Now = time.Now
	}

	dialer := &net.Dialer{Timeout: 5 * time.Second}
	if !o.AllowPrivateNetworks {
		// Control runs on the resolved address just before connecting, for
		// every connection including redirects.
		dialer.Control = func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			if ip := net.ParseIP(host); ip == nil || blockedIP(ip) {
				return errPrivateAddress
			}
			return nil
		}
	}
	transport := &http.Transport{
		Proxy:                  nil, // direct connections only, so the address check above applies
		DialContext:            dialer.DialContext,
		TLSHandshakeTimeout:    5 * time.Second,
		ResponseHeaderTimeout:  o.Timeout,
		MaxResponseHeaderBytes: 32 << 10,
		DisableKeepAlives:      true,
	}
	return &HTTPFetcher{
		opts: o,
		client: &http.Client{
			Transport: transport,
			Timeout:   o.Timeout,
			CheckRedirect: func(_ *http.Request, via []*http.Request) error {
				if len(via) > maxRedirects {
					return fmt.Errorf("stopped after %d redirects", maxRedirects)
				}
				return nil
			},
		},
		entries:  map[string]*list.Element{},
		order:    list.New(),
		inflight: map[string]*call{},
	}
}

// cgnat is the carrier-grade NAT range, which IsPrivate does not cover.
var cgnat = &net.IPNet{IP: net.IPv4(100, 64, 0, 0), Mask: net.CIDRMask(10, 32)}

// blockedIP reports whether ip is an address a diagram must not make the app
// connect to.
func blockedIP(ip net.IP) bool {
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() || cgnat.Contains(ip)
}

// Fetch returns the image at rawURL, from the cache when it can.
func (f *HTTPFetcher) Fetch(ctx context.Context, rawURL string) (*Image, error) {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("%q is not an http(s) address", RedactURL(rawURL))
	}
	key := u.String()

	f.mu.Lock()
	if img, err, ok := f.cached(key); ok {
		f.mu.Unlock()
		logger.Trace("remote image served from cache", "url", RedactURL(key), "failure", err != nil)
		return img, err
	}
	// Several compiles asking for the same URL share one request.
	if c, ok := f.inflight[key]; ok {
		f.mu.Unlock()
		select {
		case <-c.done:
			return c.img, c.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	c := &call{done: make(chan struct{})}
	f.inflight[key] = c
	f.mu.Unlock()

	c.img, c.err = f.fetch(ctx, u)

	f.mu.Lock()
	delete(f.inflight, key)
	// A cancelled fetch says nothing about the URL, so it is not remembered.
	if !errors.Is(c.err, context.Canceled) {
		f.store(key, c.img, c.err)
	}
	f.mu.Unlock()
	close(c.done)
	return c.img, c.err
}

func (f *HTTPFetcher) fetch(ctx context.Context, u *url.URL) (*Image, error) {
	start := f.opts.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "image/*")
	req.Header.Set("User-Agent", "D2-Studio")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, friendlyNetErr(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("the server answered %s", resp.Status)
	}
	if resp.ContentLength > f.opts.MaxImageBytes {
		return nil, fmt.Errorf("the image is larger than the %s limit", humanBytes(f.opts.MaxImageBytes))
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, f.opts.MaxImageBytes+1))
	if err != nil {
		return nil, friendlyNetErr(err)
	}
	if int64(len(data)) > f.opts.MaxImageBytes {
		return nil, fmt.Errorf("the image is larger than the %s limit", humanBytes(f.opts.MaxImageBytes))
	}
	mimeType := remoteMIME(resp.Header.Get("Content-Type"), data)
	if mimeType == "" {
		return nil, errNotImage
	}
	logger.Debug("fetched remote image", "url", RedactURL(u.String()), "bytes", len(data), "mime", mimeType,
		"took_ms", f.opts.Now().Sub(start).Milliseconds())
	return &Image{MIME: mimeType, Data: data}, nil
}

// remoteMIME decides what a response is, trusting the content over the
// Content-Type header, and returns "" for anything that is not an image.
func remoteMIME(contentType string, data []byte) string {
	declared, _, _ := mime.ParseMediaType(contentType)
	head := data
	if len(head) > 2048 {
		head = head[:2048]
	}
	if looksLikeSVG(head) {
		// SVG is text, so it cannot be recognised by magic bytes alone.
		if declared == "image/svg+xml" || declared == "text/xml" || declared == "application/xml" || declared == "text/plain" || declared == "" {
			return "image/svg+xml"
		}
		return ""
	}
	sniffed := http.DetectContentType(data)
	if strings.HasPrefix(sniffed, "image/") {
		return sniffed
	}
	return ""
}

func looksLikeSVG(head []byte) bool {
	h := bytes.TrimSpace(bytes.TrimPrefix(head, []byte{0xEF, 0xBB, 0xBF}))
	return bytes.HasPrefix(h, []byte("<svg")) || (bytes.HasPrefix(h, []byte("<?xml")) && bytes.Contains(h, []byte("<svg")))
}

func friendlyNetErr(err error) error {
	switch {
	case errors.Is(err, errPrivateAddress):
		return errPrivateAddress
	case errors.Is(err, context.DeadlineExceeded):
		return errors.New("the request timed out")
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return errors.New("the request timed out")
	}
	// Strip the URL from the error text: it may contain a token.
	var ue *url.Error
	if errors.As(err, &ue) {
		return ue.Err
	}
	return err
}

func humanBytes(n int64) string {
	if n >= 1<<20 {
		return fmt.Sprintf("%d MB", n>>20)
	}
	return fmt.Sprintf("%d KB", n>>10)
}

// cached returns a live cache entry. Callers hold f.mu.
func (f *HTTPFetcher) cached(key string) (*Image, error, bool) {
	el, ok := f.entries[key]
	if !ok {
		return nil, nil, false
	}
	e := el.Value.(*cacheEntry)
	if !f.opts.Now().Before(e.expires) {
		f.remove(el)
		return nil, nil, false
	}
	f.order.MoveToFront(el)
	return e.img, e.err, true
}

// store remembers a result and evicts old entries beyond the byte budget.
// Callers hold f.mu.
func (f *HTTPFetcher) store(key string, img *Image, err error) {
	if old, ok := f.entries[key]; ok {
		f.remove(old)
	}
	ttl := f.opts.SuccessTTL
	if err != nil {
		ttl = f.opts.FailureTTL
	}
	e := &cacheEntry{key: key, img: img, err: err, expires: f.opts.Now().Add(ttl)}
	f.entries[key] = f.order.PushFront(e)
	if img != nil {
		f.bytes += int64(len(img.Data))
	}
	for f.bytes > f.opts.CacheBytes && f.order.Len() > 1 {
		f.remove(f.order.Back())
	}
}

func (f *HTTPFetcher) remove(el *list.Element) {
	e := el.Value.(*cacheEntry)
	f.order.Remove(el)
	delete(f.entries, e.key)
	if e.img != nil {
		f.bytes -= int64(len(e.img.Data))
	}
}
