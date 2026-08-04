package kobo

import (
	"log/slog"
	"net/http"
	"strings"

	"kobo-hebban-adapter/internal/proxy"
)

func Healthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func SyncRoute(p *proxy.Proxy, users *UserStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.PathValue("user_token")
		stripToken(r, token)
		_, bc, ok := users.Lookup(token)
		if !ok {
			p.Handler()(w, r)
			return
		}
		SyncHandler(p, bc)(w, r)
	}
}

func StateRoute(p *proxy.Proxy, users *UserStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.PathValue("user_token")
		stripToken(r, token)
		hc, bc, ok := users.Lookup(token)
		if !ok {
			p.Handler()(w, r)
			return
		}
		StateHandler(p, hc, bc)(w, r)
	}
}

func ProxyRoute(p *proxy.Proxy) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.PathValue("user_token")
		stripToken(r, token)
		if !strings.HasPrefix(r.URL.Path, "/v1/") {
			slog.Warn("rejected request to non-kobo path", "path", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		p.Handler()(w, r)
	}
}

func stripToken(r *http.Request, token string) {
	r.URL.Path = strings.TrimPrefix(r.URL.Path, "/"+token)
	if r.URL.RawPath != "" {
		r.URL.RawPath = strings.TrimPrefix(r.URL.RawPath, "/"+token)
	}
}
