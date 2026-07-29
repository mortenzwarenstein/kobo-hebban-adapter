package kobo

import (
	"bytes"
	"encoding/json"
	"io"
	"kobo-hebban-adapter/hebban"
	"kobo-hebban-adapter/proxy"
	"log/slog"
	"net/http"
)

type ProgressResponse struct {
	ReadingStates []ReadingState `json:"ReadingStates"`
}

type ReadingState struct {
	StatusInfo struct {
		Status string `json:"Status"`
	} `json:"StatusInfo"`
}

func (pr *ProgressResponse) hebbanStatus() string {
	for _, rs := range pr.ReadingStates {
		switch rs.StatusInfo.Status {
		case "Finished":
			return "read"
		case "Reading":
			return "reading"
		}
	}
	return ""
}

func StateHandler(p *proxy.Proxy, hc *hebban.Client, bc *BookCache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bookID := r.PathValue("book_id")

		body, err := io.ReadAll(r.Body)
		if err != nil {
			slog.Error("failed to read state body", "book_id", bookID, "err", err)
			http.Error(w, "failed to read body", http.StatusInternalServerError)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		r.ContentLength = int64(len(body))

		p.Forward(w, r)

		var state ProgressResponse
		if err := json.Unmarshal(body, &state); err != nil {
			slog.Error("failed to parse reading state", "book_id", bookID, "err", err)
			return
		}

		status := state.hebbanStatus()
		if status == "" {
			slog.Info("skipping Hebban update — no actionable status", "book_id", bookID)
			return
		}

		koboHeaders := r.Header.Clone()

		meta, ok := bc.Get(bookID)
		if !ok {
			slog.Info("book not in cache, fetching sync from upstream", "book_id", bookID)
		}

		go func() {
			if !ok {
				if err := fetchAndCache(koboHeaders, bc); err != nil {
					slog.Error("on-demand sync failed", "book_id", bookID, "err", err)
					return
				}
				meta, ok = bc.Get(bookID)
				if !ok {
					slog.Warn("book not found even after sync", "book_id", bookID)
					return
				}
			}
			slog.Info("syncing to Hebban", "book_id", bookID, "title", meta.Title, "status", status)
			if err := hc.UpdateReadingStatus(meta.Title, meta.Author, status); err != nil {
				slog.Error("Hebban sync failed", "book_id", bookID, "title", meta.Title, "err", err)
			} else {
				slog.Info("Hebban sync succeeded", "book_id", bookID, "title", meta.Title, "status", status)
			}
		}()
	}
}
