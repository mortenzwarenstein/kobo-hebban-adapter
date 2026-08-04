package kobo

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"kobo-hebban-adapter/internal/proxy"
)

func TestStripToken(t *testing.T) {
	cases := []struct {
		name        string
		path        string
		rawPath     string
		token       string
		wantPath    string
		wantRawPath string
	}{
		{"strips matching prefix", "/tok1/v1/library/sync", "", "tok1", "/v1/library/sync", ""},
		{"no-op when prefix does not match", "/other/v1/library/sync", "", "tok1", "/other/v1/library/sync", ""},
		{
			"strips both path and rawPath when set",
			"/tok1/v1/library/a b", "/tok1/v1/library/a%20b", "tok1",
			"/v1/library/a b", "/v1/library/a%20b",
		},
		{"empty token strips only the leading slash", "/v1/library/sync", "", "", "v1/library/sync", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
			req.URL.Path = tc.path
			req.URL.RawPath = tc.rawPath

			stripToken(req, tc.token)

			if req.URL.Path != tc.wantPath {
				t.Errorf("Path = %q, want %q", req.URL.Path, tc.wantPath)
			}
			if req.URL.RawPath != tc.wantRawPath {
				t.Errorf("RawPath = %q, want %q", req.URL.RawPath, tc.wantRawPath)
			}
		})
	}
}

func TestHealthz(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()

	Healthz(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("got status %d, want 200", rr.Code)
	}
}

func TestProxyRoute_RejectsNonV1Path(t *testing.T) {
	p := proxy.New("http://unused.invalid")

	req := httptest.NewRequest(http.MethodGet, "/tok1/something-else", nil)
	req.SetPathValue("user_token", "tok1")
	rr := httptest.NewRecorder()

	ProxyRoute(p)(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("got status %d, want 404", rr.Code)
	}
}

func TestProxyRoute_PassesThroughV1Path(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/library/sync" {
			t.Errorf("upstream got path %q, want /v1/library/sync", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("upstream response"))
	}))
	defer upstream.Close()

	p := proxy.New(upstream.URL)
	req := httptest.NewRequest(http.MethodGet, "/tok1/v1/library/sync", nil)
	req.SetPathValue("user_token", "tok1")
	rr := httptest.NewRecorder()

	ProxyRoute(p)(rr, req)

	if rr.Code != http.StatusOK || rr.Body.String() != "upstream response" {
		t.Errorf("got status=%d body=%q, want 200 %q", rr.Code, rr.Body.String(), "upstream response")
	}
}

func TestSyncRoute_UnknownToken_PassesThroughWithoutCaching(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"NewEntitlement":{"BookEntitlement":{"Id":"book1"},"BookMetadata":{"Title":"The Hobbit","ContributorRoles":[]}}}]`))
	}))
	defer upstream.Close()

	p := proxy.New(upstream.URL)
	users := NewUserStore(map[string]UserEntry{})

	req := httptest.NewRequest(http.MethodGet, "/tok1/v1/library/sync", nil)
	req.SetPathValue("user_token", "tok1")
	rr := httptest.NewRecorder()

	SyncRoute(p, users)(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200", rr.Code)
	}
	// An unknown token has no BookCache at all, so there's nothing to poll —
	// this test mainly documents/verifies that lookup failure falls back to
	// a plain passthrough rather than panicking or hanging.
}

func TestSyncRoute_KnownToken_DispatchesToSyncHandler(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"NewEntitlement":{"BookEntitlement":{"Id":"book1"},"BookMetadata":{"Title":"The Hobbit","ContributorRoles":[]}}}]`))
	}))
	defer upstream.Close()

	p := proxy.New(upstream.URL)
	users := NewUserStore(map[string]UserEntry{"tok1": {Name: "Alice", HebbanToken: "h1"}})

	req := httptest.NewRequest(http.MethodGet, "/tok1/v1/library/sync", nil)
	req.SetPathValue("user_token", "tok1")
	rr := httptest.NewRecorder()

	SyncRoute(p, users)(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200", rr.Code)
	}

	_, bc, _ := users.Lookup("tok1")
	waitFor(t, time.Second, func() bool {
		_, ok := bc.Get("book1")
		return ok
	})
}

func TestStateRoute_UnknownToken_PassesThroughWithoutHebbanSync(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	p := proxy.New(upstream.URL)
	users := NewUserStore(map[string]UserEntry{})

	req := httptest.NewRequest(http.MethodPut, "/tok1/v1/library/book1/state", nil)
	req.SetPathValue("user_token", "tok1")
	req.SetPathValue("book_id", "book1")
	rr := httptest.NewRecorder()

	StateRoute(p, users)(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200", rr.Code)
	}
}

func TestStateRoute_KnownToken_DispatchesToStateHandler(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("device response"))
	}))
	defer upstream.Close()

	p := proxy.New(upstream.URL)
	users := NewUserStore(map[string]UserEntry{"tok1": {Name: "Alice", HebbanToken: ""}})
	_, bc, _ := users.Lookup("tok1")
	bc.Set("book1", BookMeta{Title: "The Hobbit", Author: "Tolkien"})

	req := httptest.NewRequest(http.MethodPut, "/tok1/v1/library/book1/state", strings.NewReader("{}"))
	req.SetPathValue("user_token", "tok1")
	req.SetPathValue("book_id", "book1")
	rr := httptest.NewRecorder()

	StateRoute(p, users)(rr, req)

	if rr.Code != http.StatusOK || rr.Body.String() != "device response" {
		t.Fatalf("got status=%d body=%q, want 200 %q", rr.Code, rr.Body.String(), "device response")
	}
}
