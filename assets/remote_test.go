package assets

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/png"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func pngBytes(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

const svgDoc = `<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg" width="4" height="4"><rect width="4" height="4"/></svg>`

type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *clock) advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

// server starts a test server and counts the requests it receives.
func server(t *testing.T, h http.HandlerFunc) (*httptest.Server, *atomic.Int64) {
	t.Helper()
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		h(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func serveImage(t *testing.T) (*httptest.Server, *atomic.Int64) {
	data := pngBytes(t)
	return server(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(data)
	})
}

// local returns a fetcher allowed to reach the loopback test server.
func local(o HTTPOptions) *HTTPFetcher {
	o.AllowPrivateNetworks = true
	return NewHTTPFetcher(o)
}

func TestFetchReturnsTheImage(t *testing.T) {
	srv, _ := serveImage(t)
	img, err := local(HTTPOptions{}).Fetch(context.Background(), srv.URL+"/i.png")
	if err != nil {
		t.Fatal(err)
	}
	if img.MIME != "image/png" || !bytes.Equal(img.Data, pngBytes(t)) {
		t.Fatalf("img = %s, %d bytes", img.MIME, len(img.Data))
	}
}

// The important one: by default a diagram cannot make the app connect to the
// user's own machine or network.
func TestPrivateAndLocalAddressesAreBlockedByDefault(t *testing.T) {
	srv, hits := serveImage(t) // listens on 127.0.0.1
	_, err := NewHTTPFetcher(HTTPOptions{}).Fetch(context.Background(), srv.URL+"/i.png")
	if err == nil || !strings.Contains(err.Error(), "private or local network") {
		t.Fatalf("err = %v", err)
	}
	if n := hits.Load(); n != 0 {
		t.Fatalf("the server received %d requests; the connection must be refused before anything is sent", n)
	}
}

func TestBlockedIPAddresses(t *testing.T) {
	for _, s := range []string{
		"127.0.0.1", "127.8.9.10", "::1", "10.0.0.1", "172.16.5.5", "172.31.255.255", "192.168.1.1",
		"169.254.169.254", "fe80::1", "fc00::1", "fd12::1", "100.64.0.1", "100.127.255.254",
		"0.0.0.0", "::", "224.0.0.1", "ff02::1", "::ffff:127.0.0.1", "::ffff:10.0.0.1",
	} {
		if !blockedIP(net.ParseIP(s)) {
			t.Errorf("%s should be blocked", s)
		}
	}
	for _, s := range []string{"8.8.8.8", "1.1.1.1", "93.184.216.34", "2606:4700:4700::1111", "100.63.255.255", "172.32.0.1", "192.169.0.1"} {
		if blockedIP(net.ParseIP(s)) {
			t.Errorf("%s is a public address and should be allowed", s)
		}
	}
}

func TestOnlyHTTPAndHTTPSAddressesAreFetched(t *testing.T) {
	f := local(HTTPOptions{})
	for _, raw := range []string{"ftp://example.com/a.png", "file:///etc/passwd", "data:image/png;base64,AAAA", "javascript:alert(1)", "//example.com/a.png", "http://", "not a url", ""} {
		if _, err := f.Fetch(context.Background(), raw); err == nil {
			t.Errorf("%q should be refused", raw)
		}
	}
}

func TestContentMustBeAnImage(t *testing.T) {
	cases := []struct {
		name, contentType, body string
		wantMIME                string // "" = rejected
	}{
		{"html page", "text/html", "<html><body>hi</body></html>", ""},
		{"json", "application/json", `{"a":1}`, ""},
		{"plain text", "text/plain", "just words", ""},
		{"svg", "image/svg+xml", svgDoc, "image/svg+xml"},
		{"svg served as xml", "text/xml", svgDoc, "image/svg+xml"},
		{"html claiming to be svg", "text/html", svgDoc, ""},
		{"html page labelled image", "image/png", "<html>not an image</html>", ""},
		{"png labelled plain text", "text/plain", string(pngBytes(t)), "image/png"},
		{"empty body", "image/png", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv, _ := server(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", c.contentType)
				_, _ = w.Write([]byte(c.body))
			})
			img, err := local(HTTPOptions{}).Fetch(context.Background(), srv.URL+"/x")
			if c.wantMIME == "" {
				if err == nil {
					t.Fatalf("accepted %s as %s", c.contentType, img.MIME)
				}
				return
			}
			if err != nil || img.MIME != c.wantMIME {
				t.Fatalf("img=%v err=%v, want %s", img, err, c.wantMIME)
			}
		})
	}
}

func TestSizeLimitIsEnforcedWithOrWithoutContentLength(t *testing.T) {
	big := bytes.Repeat([]byte{0x89}, 5000)
	withLength, _ := server(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(big)
	})
	chunked, _ := server(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.(http.Flusher).Flush() // forces chunked encoding: no Content-Length
		_, _ = w.Write(big)
	})
	for name, srv := range map[string]*httptest.Server{"content-length": withLength, "chunked": chunked} {
		_, err := local(HTTPOptions{MaxImageBytes: 1024}).Fetch(context.Background(), srv.URL+"/big")
		if err == nil || !strings.Contains(err.Error(), "larger than") {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestBadStatusIsAnError(t *testing.T) {
	srv, _ := server(t, func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) })
	_, err := local(HTTPOptions{}).Fetch(context.Background(), srv.URL+"/missing.png")
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("err = %v", err)
	}
}

func TestRedirectLoopsStop(t *testing.T) {
	var srv *httptest.Server
	srv, _ = server(t, func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, srv.URL+"/again", http.StatusFound) })
	_, err := local(HTTPOptions{}).Fetch(context.Background(), srv.URL+"/start")
	if err == nil || !strings.Contains(err.Error(), "redirects") {
		t.Fatalf("err = %v", err)
	}
}

func TestSlowServersTimeOut(t *testing.T) {
	srv, _ := server(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(3 * time.Second):
		case <-r.Context().Done():
		}
	})
	start := time.Now()
	_, err := local(HTTPOptions{Timeout: 100 * time.Millisecond}).Fetch(context.Background(), srv.URL+"/slow")
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("err = %v", err)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("the timeout did not cut the request short")
	}
}

func TestSuccessIsCachedUntilItExpires(t *testing.T) {
	srv, hits := serveImage(t)
	c := &clock{t: time.Unix(1_000_000, 0)}
	f := local(HTTPOptions{Now: c.now, SuccessTTL: time.Minute})
	for i := 0; i < 5; i++ {
		if _, err := f.Fetch(context.Background(), srv.URL+"/i.png"); err != nil {
			t.Fatal(err)
		}
	}
	if hits.Load() != 1 {
		t.Fatalf("5 fetches made %d requests", hits.Load())
	}
	c.advance(2 * time.Minute)
	if _, err := f.Fetch(context.Background(), srv.URL+"/i.png"); err != nil {
		t.Fatal(err)
	}
	if hits.Load() != 2 {
		t.Fatalf("an expired entry should be refetched; requests = %d", hits.Load())
	}
}

// Without this, every keystroke would re-request a dead URL.
func TestFailuresAreCachedBrieflyThenRetried(t *testing.T) {
	var ok atomic.Bool
	srv, hits := server(t, func(w http.ResponseWriter, r *http.Request) {
		if !ok.Load() {
			http.Error(w, "down", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngBytes(t))
	})
	c := &clock{t: time.Unix(1_000_000, 0)}
	f := local(HTTPOptions{Now: c.now, FailureTTL: 30 * time.Second})

	for i := 0; i < 4; i++ {
		if _, err := f.Fetch(context.Background(), srv.URL+"/i.png"); err == nil {
			t.Fatal("expected a failure")
		}
	}
	if hits.Load() != 1 {
		t.Fatalf("a failing URL was requested %d times in a burst", hits.Load())
	}

	ok.Store(true)
	c.advance(31 * time.Second)
	if _, err := f.Fetch(context.Background(), srv.URL+"/i.png"); err != nil {
		t.Fatalf("after the failure expired it should be retried and succeed: %v", err)
	}
}

func TestCancelledFetchIsNotRemembered(t *testing.T) {
	srv, hits := serveImage(t)
	f := local(HTTPOptions{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := f.Fetch(ctx, srv.URL+"/i.png"); err == nil {
		t.Fatal("expected an error from a cancelled context")
	}
	if _, err := f.Fetch(context.Background(), srv.URL+"/i.png"); err != nil {
		t.Fatalf("a later fetch must not inherit the cancellation: %v", err)
	}
	if hits.Load() != 1 {
		t.Fatalf("requests = %d", hits.Load())
	}
}

func TestConcurrentFetchesOfOneURLShareARequest(t *testing.T) {
	release := make(chan struct{})
	srv, hits := server(t, func(w http.ResponseWriter, r *http.Request) {
		<-release
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngBytes(t))
	})
	f := local(HTTPOptions{})
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := f.Fetch(context.Background(), srv.URL+"/i.png"); err != nil {
				t.Error(err)
			}
		}()
	}
	time.Sleep(100 * time.Millisecond) // let them all queue behind the first
	close(release)
	wg.Wait()
	if hits.Load() != 1 {
		t.Fatalf("20 concurrent fetches made %d requests", hits.Load())
	}
}

func TestCacheStaysWithinItsByteBudget(t *testing.T) {
	srv, hits := server(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngBytes(t))
	})
	size := int64(len(pngBytes(t)))
	f := local(HTTPOptions{CacheBytes: size*2 + 1})
	for i := 0; i < 6; i++ {
		if _, err := f.Fetch(context.Background(), fmt.Sprintf("%s/%d.png", srv.URL, i)); err != nil {
			t.Fatal(err)
		}
	}
	f.mu.Lock()
	kept, total := f.order.Len(), f.bytes
	f.mu.Unlock()
	if total > size*2+1 || kept > 2 {
		t.Fatalf("cache holds %d entries / %d bytes, budget %d", kept, total, size*2+1)
	}
	// The newest are kept (no refetch), the oldest were dropped (refetched).
	before := hits.Load()
	_, _ = f.Fetch(context.Background(), srv.URL+"/5.png")
	if hits.Load() != before {
		t.Error("the most recent image should still be cached")
	}
	_, _ = f.Fetch(context.Background(), srv.URL+"/0.png")
	if hits.Load() != before+1 {
		t.Error("the oldest image should have been evicted")
	}
}

func TestURLsAreRedactedInMessages(t *testing.T) {
	if got := RedactURL("https://user:pass@icons.example/a.png?token=SECRET#frag"); got != "https://icons.example/a.png" {
		t.Fatalf("RedactURL = %q", got)
	}
	srv, _ := server(t, func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) })
	_, err := local(HTTPOptions{}).Fetch(context.Background(), srv.URL+"/a.png?token=SECRET")
	if err != nil && strings.Contains(err.Error(), "SECRET") {
		t.Fatalf("the query string leaked into an error: %v", err)
	}
	_, err = NewHTTPFetcher(HTTPOptions{}).Fetch(context.Background(), srv.URL+"/a.png?token=SECRET")
	if err == nil || strings.Contains(err.Error(), "SECRET") {
		t.Fatalf("err = %v", err)
	}
}

func TestErrorsAreFriendly(t *testing.T) {
	for _, e := range []error{errNotImage, errPrivateAddress} {
		if errors.Is(e, context.Canceled) || e.Error() == "" {
			t.Errorf("bad error %v", e)
		}
	}
}
