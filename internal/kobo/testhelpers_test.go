package kobo

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

// waitFor polls cond until it returns true or timeout elapses, failing the
// test if the condition is never met. Needed because SyncHandler and
// StateHandler do their cache/Hebban work in an unawaited goroutine.
func waitFor(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !cond() {
		t.Fatal("condition not met within timeout")
	}
}

// redirectTransport rewrites every outgoing request to hit the given test
// server instead of the real host, so the unexported package-level
// syncClient can be pointed at an httptest.Server without changing
// production code.
type redirectTransport struct {
	target *url.URL
}

func (rt *redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = rt.target.Scheme
	req.URL.Host = rt.target.Host
	return http.DefaultTransport.RoundTrip(req)
}

// withSyncClientTestServer points the package-level syncClient (used by
// fetchAndCache) at an httptest.Server for the duration of the test.
func withSyncClientTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	target, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}

	original := syncClient
	syncClient = &http.Client{Transport: &redirectTransport{target: target}}
	t.Cleanup(func() { syncClient = original })

	return srv
}
