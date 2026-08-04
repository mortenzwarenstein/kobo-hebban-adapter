package hebban

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// redirectTransport rewrites every outgoing request to hit the given test
// server instead of the real host, so the unexported package-level
// httpClient can be pointed at an httptest.Server without changing
// production code.
type redirectTransport struct {
	target *url.URL
}

func (rt *redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = rt.target.Scheme
	req.URL.Host = rt.target.Host
	return http.DefaultTransport.RoundTrip(req)
}

func withTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	target, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}

	original := httpClient
	httpClient = &http.Client{Transport: &redirectTransport{target: target}}
	t.Cleanup(func() { httpClient = original })

	return srv
}

func TestNormalize(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"lowercases", "The Hobbit", "the hobbit"},
		{"strips punctuation", "The Hobbit!!", "the hobbit"},
		{"collapses whitespace", "  multiple   spaces  ", "multiple spaces"},
		{"keeps digits", "Part 2: The Return", "part 2 the return"},
		{"keeps accented letters", "Ålvar Núñez", "ålvar núñez"},
		{"empty string", "", ""},
		{"only punctuation", "!!! ---", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalize(tc.in); got != tc.want {
				t.Errorf("normalize(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestWordOverlap(t *testing.T) {
	cases := []struct {
		name string
		a, b string
		want int
	}{
		{"identical", "the hobbit", "the hobbit", 2},
		{"partial overlap", "the hobbit movie", "the hobbit book", 2},
		{"no overlap", "foo bar", "baz qux", 0},
		{"empty a", "", "the hobbit", 0},
		{"empty b", "the hobbit", "", 0},
		{"both empty", "", "", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := wordOverlap(tc.a, tc.b); got != tc.want {
				t.Errorf("wordOverlap(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func TestBestMatch(t *testing.T) {
	t.Run("no items", func(t *testing.T) {
		_, _, ok := bestMatch(nil, "The Hobbit", "Tolkien")
		if ok {
			t.Fatal("expected ok=false for empty items")
		}
	})

	t.Run("picks highest combined title+author score", func(t *testing.T) {
		items := []searchItem{
			{ID: 1, Title: "The Hobbit Movie Tie-In", Author: "Someone Else"},
			{ID: 2, Title: "The Hobbit", Author: "J.R.R. Tolkien"},
		}
		id, title, ok := bestMatch(items, "The Hobbit", "Tolkien")
		if !ok {
			t.Fatal("expected a match")
		}
		if id != 2 || title != "The Hobbit" {
			t.Errorf("got id=%d title=%q, want id=2 title=\"The Hobbit\"", id, title)
		}
	})

	t.Run("no overlap at all yields no match", func(t *testing.T) {
		items := []searchItem{{ID: 1, Title: "Completely Unrelated", Author: "Nobody"}}
		_, _, ok := bestMatch(items, "The Hobbit", "Tolkien")
		if ok {
			t.Fatal("expected ok=false when there is zero word overlap")
		}
	})

	t.Run("empty author is ignored in scoring", func(t *testing.T) {
		items := []searchItem{{ID: 1, Title: "The Hobbit", Author: "Whoever"}}
		id, _, ok := bestMatch(items, "The Hobbit", "")
		if !ok || id != 1 {
			t.Fatalf("got id=%d ok=%v, want id=1 ok=true", id, ok)
		}
	})
}

func TestUpdateReadingStatus_NoToken(t *testing.T) {
	c := NewClient("")
	err := c.UpdateReadingStatus("The Hobbit", "Tolkien", "reading")
	if err == nil || !strings.Contains(err.Error(), "no Hebban token configured") {
		t.Fatalf("got err=%v, want \"no Hebban token configured\"", err)
	}
}

func TestUpdateReadingStatus_Success(t *testing.T) {
	var gotSearchPath, gotFilter string
	var gotStatusPath, gotCookie string
	var gotBody map[string]string

	withTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/works"):
			gotSearchPath = r.URL.Path
			gotFilter = r.URL.Query().Get("filter")
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(searchResponse{Items: []searchItem{
				{ID: 42, Title: "The Hobbit", Author: "J.R.R. Tolkien"},
			}})
		case strings.Contains(r.URL.Path, "/status"):
			gotStatusPath = r.URL.Path
			if ck, err := r.Cookie("hebban-authorization-token"); err == nil {
				gotCookie = ck.Value
			}
			body, _ := io.ReadAll(r.Body)
			json.Unmarshal(body, &gotBody)
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected request path %q", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	})

	c := NewClient("secret-token")
	err := c.UpdateReadingStatus("The Hobbit", "Tolkien", "reading")
	if err != nil {
		t.Fatalf("UpdateReadingStatus returned error: %v", err)
	}

	if gotSearchPath != "/api/1/works" {
		t.Errorf("search path = %q, want /api/1/works", gotSearchPath)
	}
	if gotFilter != "query:The Hobbit Tolkien" {
		t.Errorf("search filter = %q, want %q", gotFilter, "query:The Hobbit Tolkien")
	}
	if gotStatusPath != "/api/1/work/42/status" {
		t.Errorf("status path = %q, want /api/1/work/42/status", gotStatusPath)
	}
	if gotCookie != "secret-token" {
		t.Errorf("auth cookie = %q, want secret-token", gotCookie)
	}
	if gotBody["status"] != "reading" {
		t.Errorf("posted status = %q, want reading", gotBody["status"])
	}
}

func TestUpdateReadingStatus_NoMatch(t *testing.T) {
	withTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(searchResponse{Items: []searchItem{
			{ID: 1, Title: "Completely Unrelated", Author: "Nobody"},
		}})
	})

	c := NewClient("secret-token")
	err := c.UpdateReadingStatus("The Hobbit", "Tolkien", "reading")
	if err == nil || !strings.Contains(err.Error(), "no matching work found") {
		t.Fatalf("got err=%v, want \"no matching work found\"", err)
	}
}

func TestUpdateReadingStatus_SearchHTTPError(t *testing.T) {
	withTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})

	c := NewClient("secret-token")
	err := c.UpdateReadingStatus("The Hobbit", "Tolkien", "reading")
	if err == nil || !strings.Contains(err.Error(), "HTTP 500") {
		t.Fatalf("got err=%v, want an HTTP 500 error", err)
	}
}

func TestUpdateReadingStatus_SetStatusHTTPError(t *testing.T) {
	withTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/works"):
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(searchResponse{Items: []searchItem{
				{ID: 42, Title: "The Hobbit", Author: "J.R.R. Tolkien"},
			}})
		case strings.Contains(r.URL.Path, "/status"):
			http.Error(w, "nope", http.StatusForbidden)
		}
	})

	c := NewClient("secret-token")
	err := c.UpdateReadingStatus("The Hobbit", "Tolkien", "reading")
	if err == nil || !strings.Contains(err.Error(), "HTTP 403") {
		t.Fatalf("got err=%v, want an HTTP 403 error", err)
	}
}

func TestSearch_MalformedJSON(t *testing.T) {
	withTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	})

	c := NewClient("secret-token")
	err := c.UpdateReadingStatus("The Hobbit", "Tolkien", "reading")
	if err == nil || !strings.Contains(err.Error(), "decode") {
		t.Fatalf("got err=%v, want a decode error", err)
	}
}
