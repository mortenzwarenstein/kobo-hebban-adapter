package kobo

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"kobo-hebban-adapter/internal/proxy"
)

func TestFirstAuthor(t *testing.T) {
	cases := []struct {
		name  string
		roles []ContributorRole
		want  string
	}{
		{"empty", nil, ""},
		{"single author", []ContributorRole{{Name: "J.R.R. Tolkien", Role: "Author"}}, "J.R.R. Tolkien"},
		{
			"falls back to first when no author role",
			[]ContributorRole{{Name: "Some Illustrator", Role: "Illustrator"}},
			"Some Illustrator",
		},
		{
			"finds author role even if not first",
			[]ContributorRole{
				{Name: "Some Illustrator", Role: "Illustrator"},
				{Name: "J.R.R. Tolkien", Role: "Author"},
			},
			"J.R.R. Tolkien",
		},
		{
			"first author role wins if multiple",
			[]ContributorRole{
				{Name: "First Author", Role: "Author"},
				{Name: "Second Author", Role: "Author"},
			},
			"First Author",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := firstAuthor(tc.roles); got != tc.want {
				t.Errorf("firstAuthor(%+v) = %q, want %q", tc.roles, got, tc.want)
			}
		})
	}
}

func TestSyncHandler_ForwardsResponseAndCachesEntitlements(t *testing.T) {
	upstreamBody := `[
		{"NewEntitlement":{"BookEntitlement":{"Id":"book1"},"BookMetadata":{"Title":"The Hobbit","ContributorRoles":[{"Name":"J.R.R. Tolkien","Role":"Author"}]}}},
		{}
	]`
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(upstreamBody))
	}))
	defer upstream.Close()

	p := proxy.New(upstream.URL)
	bc := NewBookCache()

	req := httptest.NewRequest(http.MethodGet, "/v1/library/sync", nil)
	rr := httptest.NewRecorder()

	SyncHandler(p, bc)(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200", rr.Code)
	}
	if rr.Body.String() != upstreamBody {
		t.Fatalf("got body %q, want %q", rr.Body.String(), upstreamBody)
	}

	waitFor(t, time.Second, func() bool {
		_, ok := bc.Get("book1")
		return ok
	})
	meta, _ := bc.Get("book1")
	if meta.Title != "The Hobbit" || meta.Author != "J.R.R. Tolkien" {
		t.Errorf("got meta %+v, want Title=The Hobbit Author=J.R.R. Tolkien", meta)
	}
}

func TestSyncHandler_MalformedBody_DoesNotCacheOrPanic(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not json"))
	}))
	defer upstream.Close()

	p := proxy.New(upstream.URL)
	bc := NewBookCache()

	req := httptest.NewRequest(http.MethodGet, "/v1/library/sync", nil)
	rr := httptest.NewRecorder()

	SyncHandler(p, bc)(rr, req)

	if rr.Code != http.StatusOK || rr.Body.String() != "not json" {
		t.Fatalf("got status=%d body=%q, want 200 %q", rr.Code, rr.Body.String(), "not json")
	}

	time.Sleep(50 * time.Millisecond)
	if _, ok := bc.Get("book1"); ok {
		t.Fatal("expected no cache entries for a malformed sync response")
	}
}

type captureTransport struct {
	req  *http.Request
	resp *http.Response
}

func (ct *captureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ct.req = req
	return ct.resp, nil
}

func TestFetchAndCache_StripsHopByHopHeaders(t *testing.T) {
	body := `[{"NewEntitlement":{"BookEntitlement":{"Id":"book1"},"BookMetadata":{"Title":"The Hobbit","ContributorRoles":[]}}}]`
	ct := &captureTransport{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
		},
	}
	original := syncClient
	syncClient = &http.Client{Transport: ct}
	defer func() { syncClient = original }()

	headers := http.Header{
		"X-Test":     {"keep-me"},
		"Connection": {"close"},
		"Keep-Alive": {"timeout=5"},
		"Upgrade":    {"websocket"},
	}
	bc := NewBookCache()
	if err := fetchAndCache(headers, bc); err != nil {
		t.Fatalf("fetchAndCache returned error: %v", err)
	}

	if got := ct.req.Header.Get("X-Test"); got != "keep-me" {
		t.Errorf("X-Test header = %q, want keep-me to be forwarded", got)
	}
	for _, h := range []string{"Connection", "Keep-Alive", "Upgrade"} {
		if got := ct.req.Header.Get(h); got != "" {
			t.Errorf("%s header = %q, want empty (hop-by-hop headers must be stripped)", h, got)
		}
	}
}

func TestFetchAndCache_Success(t *testing.T) {
	upstreamBody := `[{"NewEntitlement":{"BookEntitlement":{"Id":"book1"},"BookMetadata":{"Title":"The Hobbit","ContributorRoles":[{"Name":"J.R.R. Tolkien","Role":"Author"}]}}}]`
	withSyncClientTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(upstreamBody))
	})

	bc := NewBookCache()
	if err := fetchAndCache(http.Header{}, bc); err != nil {
		t.Fatalf("fetchAndCache returned error: %v", err)
	}

	meta, ok := bc.Get("book1")
	if !ok {
		t.Fatal("expected book1 to be cached")
	}
	if meta.Title != "The Hobbit" || meta.Author != "J.R.R. Tolkien" {
		t.Errorf("got meta %+v, want Title=The Hobbit Author=J.R.R. Tolkien", meta)
	}
}

func TestFetchAndCache_NonOKStatus(t *testing.T) {
	withSyncClientTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})

	bc := NewBookCache()
	err := fetchAndCache(http.Header{}, bc)
	if err == nil || !strings.Contains(err.Error(), "upstream returned 500") {
		t.Fatalf("got err=%v, want an \"upstream returned 500\" error", err)
	}
}

func TestFetchAndCache_MalformedJSON(t *testing.T) {
	withSyncClientTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not json"))
	})

	bc := NewBookCache()
	err := fetchAndCache(http.Header{}, bc)
	if err == nil {
		t.Fatal("expected an error for a malformed JSON response")
	}
}
