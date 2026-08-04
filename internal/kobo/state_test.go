package kobo

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"kobo-hebban-adapter/internal/hebban"
	"kobo-hebban-adapter/internal/proxy"
)

func TestProgressResponse_HebbanStatus(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"no reading states", `{"ReadingStates":[]}`, ""},
		{"finished", `{"ReadingStates":[{"StatusInfo":{"Status":"Finished"}}]}`, "read"},
		{"reading", `{"ReadingStates":[{"StatusInfo":{"Status":"Reading"}}]}`, "reading"},
		{"unknown status is skipped", `{"ReadingStates":[{"StatusInfo":{"Status":"Unread"}}]}`, ""},
		{
			"first actionable match wins",
			`{"ReadingStates":[{"StatusInfo":{"Status":"Reading"}},{"StatusInfo":{"Status":"Finished"}}]}`,
			"reading",
		},
		{
			"skips non-actionable entries before an actionable one",
			`{"ReadingStates":[{"StatusInfo":{"Status":"Unread"}},{"StatusInfo":{"Status":"Finished"}}]}`,
			"read",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var pr ProgressResponse
			if err := json.Unmarshal([]byte(tc.body), &pr); err != nil {
				t.Fatalf("failed to unmarshal test fixture: %v", err)
			}
			if got := pr.hebbanStatus(); got != tc.want {
				t.Errorf("hebbanStatus() = %q, want %q", got, tc.want)
			}
		})
	}
}

func newStateRequest(t *testing.T, bookID, body string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPut, "/v1/library/"+bookID+"/state", strings.NewReader(body))
	req.SetPathValue("book_id", bookID)
	return req
}

func TestStateHandler_ForwardsToDeviceImmediately(t *testing.T) {
	const upstreamBody = `{"ok":true}`
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(upstreamBody))
	}))
	defer upstream.Close()

	p := proxy.New(upstream.URL)
	bc := NewBookCache()
	bc.Set("book1", BookMeta{Title: "The Hobbit", Author: "Tolkien"})
	hc := hebban.NewClient("")

	req := newStateRequest(t, "book1", `{"ReadingStates":[{"StatusInfo":{"Status":"Reading"}}]}`)
	rr := httptest.NewRecorder()

	StateHandler(p, hc, bc)(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("got status %d, want 201", rr.Code)
	}
	if rr.Body.String() != upstreamBody {
		t.Fatalf("got body %q, want %q", rr.Body.String(), upstreamBody)
	}
}

func TestStateHandler_MalformedBody_StillForwardsAndDoesNotPanic(t *testing.T) {
	const upstreamBody = `{"ok":true}`
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(upstreamBody))
	}))
	defer upstream.Close()

	p := proxy.New(upstream.URL)
	bc := NewBookCache()
	hc := hebban.NewClient("")

	req := newStateRequest(t, "book1", "not json")
	rr := httptest.NewRecorder()

	StateHandler(p, hc, bc)(rr, req)

	if rr.Code != http.StatusOK || rr.Body.String() != upstreamBody {
		t.Fatalf("got status=%d body=%q, want 200 %q", rr.Code, rr.Body.String(), upstreamBody)
	}
}

func TestStateHandler_SkipsHebbanSyncWhenStatusNotActionable(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	p := proxy.New(upstream.URL)
	bc := NewBookCache()
	bc.Set("book1", BookMeta{Title: "The Hobbit", Author: "Tolkien"})
	hc := hebban.NewClient("")

	req := newStateRequest(t, "book1", `{"ReadingStates":[{"StatusInfo":{"Status":"Unread"}}]}`)
	rr := httptest.NewRecorder()

	StateHandler(p, hc, bc)(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200", rr.Code)
	}
}

func TestStateHandler_CacheMissTriggersOnDemandFetch(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	syncBody := `[{"NewEntitlement":{"BookEntitlement":{"Id":"book1"},"BookMetadata":{"Title":"The Hobbit","ContributorRoles":[{"Name":"J.R.R. Tolkien","Role":"Author"}]}}}]`
	withSyncClientTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(syncBody))
	})

	p := proxy.New(upstream.URL)
	bc := NewBookCache() // deliberately empty: book1 is not cached yet
	hc := hebban.NewClient("")

	req := newStateRequest(t, "book1", `{"ReadingStates":[{"StatusInfo":{"Status":"Reading"}}]}`)
	rr := httptest.NewRecorder()

	StateHandler(p, hc, bc)(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200", rr.Code)
	}

	waitFor(t, time.Second, func() bool {
		_, ok := bc.Get("book1")
		return ok
	})
	meta, _ := bc.Get("book1")
	if meta.Title != "The Hobbit" || meta.Author != "J.R.R. Tolkien" {
		t.Errorf("got meta %+v after on-demand fetch, want Title=The Hobbit Author=J.R.R. Tolkien", meta)
	}
}
