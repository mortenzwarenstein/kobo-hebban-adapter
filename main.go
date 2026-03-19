package main

import (
	"log/slog"
	"net/http"
	"os"
	"strings"

	"kobo-hebban-adapter/kobo"
	"kobo-hebban-adapter/proxy"
)

func stripToken(r *http.Request, token string) {
	r.URL.Path = strings.TrimPrefix(r.URL.Path, "/"+token)
	if r.URL.RawPath != "" {
		r.URL.RawPath = strings.TrimPrefix(r.URL.RawPath, "/"+token)
	}
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	usersConfig := os.Getenv("USERS_CONFIG")
	if usersConfig == "" {
		usersConfig = "/etc/kobo/users.json"
	}

	users, err := kobo.LoadUserStore(usersConfig)
	if err != nil {
		slog.Error("failed to load users config", "err", err)
		os.Exit(1)
	}

	p := proxy.New("https://storeapi.kobo.com")

	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("GET /{user_token}/v1/library/sync", func(w http.ResponseWriter, r *http.Request) {
		token := r.PathValue("user_token")
		stripToken(r, token)
		_, bc, ok := users.Lookup(token)
		if !ok {
			p.Handler()(w, r)
			return
		}
		kobo.SyncHandler(p, bc)(w, r)
	})

	mux.HandleFunc("PUT /{user_token}/v1/library/{book_id}/state", func(w http.ResponseWriter, r *http.Request) {
		token := r.PathValue("user_token")
		stripToken(r, token)
		hc, bc, ok := users.Lookup(token)
		if !ok {
			p.Handler()(w, r)
			return
		}
		kobo.StateHandler(p, hc, bc)(w, r)
	})

	mux.HandleFunc("/{user_token}/", func(w http.ResponseWriter, r *http.Request) {
		token := r.PathValue("user_token")
		stripToken(r, token)
		p.Handler()(w, r)
	})

	slog.Info("kobo-hebban-adapter starting", "port", port, "upstream", "https://storeapi.kobo.com")
	for _, u := range users.Users() {
		slog.Info("user configured", "name", u.Name, "url_prefix", "/"+u.Token)
	}
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}
