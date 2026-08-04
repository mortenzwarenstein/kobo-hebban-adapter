package main

import (
	"log/slog"
	"net/http"
	"os"

	"kobo-hebban-adapter/internal/config"
	"kobo-hebban-adapter/internal/kobo"
	"kobo-hebban-adapter/internal/proxy"
)

func main() {
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "/etc/kobo/config.json"
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		slog.Error("failed to load config", "err", err)
		os.Exit(1)
	}

	users := kobo.NewUserStore(cfg.Users)
	p := proxy.New("https://storeapi.kobo.com")

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", kobo.Healthz)
	mux.HandleFunc("GET /{user_token}/v1/library/sync", kobo.SyncRoute(p, users))
	mux.HandleFunc("PUT /{user_token}/v1/library/{book_id}/state", kobo.StateRoute(p, users))
	mux.HandleFunc("/{user_token}/", kobo.ProxyRoute(p))

	slog.Info("kobo-hebban-adapter starting", "port", cfg.Port, "upstream", "https://storeapi.kobo.com")
	for _, u := range users.Users() {
		slog.Info("user configured", "name", u.Name, "url_prefix", "/"+u.Token)
	}
	if err := http.ListenAndServe(":"+cfg.Port, mux); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}
