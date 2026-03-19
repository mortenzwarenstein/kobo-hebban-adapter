package main

import (
	"log/slog"
	"net/http"
	"os"

	"kobo-hebban-adapter/hebban"
	"kobo-hebban-adapter/kobo"
	"kobo-hebban-adapter/proxy"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	authToken := os.Getenv("HEBBAN_AUTH_TOKEN")
	if authToken == "" {
		slog.Warn("HEBBAN_AUTH_TOKEN not set — Hebban sync will be disabled")
	}

	hebbanClient := hebban.NewClient(authToken)
	bookCache := kobo.NewBookCache()
	p := proxy.New("https://storeapi.kobo.com")

	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("GET /v1/library/sync", kobo.SyncHandler(p, bookCache))
	mux.HandleFunc("PUT /v1/library/{book_id}/state", kobo.StateHandler(p, hebbanClient, bookCache))
	mux.HandleFunc("/", p.Handler())

	slog.Info("kobo-hebban-adapter starting", "port", port, "upstream", "https://storeapi.kobo.com")
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}
