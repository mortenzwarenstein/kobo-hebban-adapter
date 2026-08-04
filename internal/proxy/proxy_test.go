package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandler_ForwardsRequestAndRewritesHost(t *testing.T) {
	var gotHost, gotPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHost = r.Host
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusTeapot)
		w.Write([]byte("upstream response"))
	}))
	defer upstream.Close()

	p := New(upstream.URL)
	req := httptest.NewRequest(http.MethodGet, "/v1/library/sync", nil)
	req.Host = "some-other-host.example"
	rr := httptest.NewRecorder()

	p.Handler()(rr, req)

	if rr.Code != http.StatusTeapot || rr.Body.String() != "upstream response" {
		t.Fatalf("got status=%d body=%q, want 418 %q", rr.Code, rr.Body.String(), "upstream response")
	}
	if gotPath != "/v1/library/sync" {
		t.Errorf("upstream saw path %q, want /v1/library/sync", gotPath)
	}
	wantHost := p.target.Host
	if gotHost != wantHost {
		t.Errorf("upstream saw Host %q, want %q (director must rewrite Host to the proxy target)", gotHost, wantHost)
	}
}

func TestForward_BehavesLikeHandler(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("forwarded"))
	}))
	defer upstream.Close()

	p := New(upstream.URL)
	req := httptest.NewRequest(http.MethodGet, "/v1/library/sync", nil)
	rr := httptest.NewRecorder()

	p.Forward(rr, req)

	if rr.Code != http.StatusOK || rr.Body.String() != "forwarded" {
		t.Fatalf("got status=%d body=%q, want 200 %q", rr.Code, rr.Body.String(), "forwarded")
	}
}

func TestNew_PanicsOnInvalidTarget(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected New to panic on an invalid target URL")
		}
	}()
	New("://%zz-not-a-valid-url")
}
