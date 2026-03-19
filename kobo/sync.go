package kobo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"kobo-hebban-adapter/proxy"
)

type SyncItem struct {
	NewEntitlement *NewEntitlement `json:"NewEntitlement"`
}

type NewEntitlement struct {
	BookEntitlement BookEntitlement `json:"BookEntitlement"`
	BookMetadata    BookMetadata    `json:"BookMetadata"`
}

type BookEntitlement struct {
	Id string `json:"Id"`
}

type BookMetadata struct {
	Title            string            `json:"Title"`
	ContributorRoles []ContributorRole `json:"ContributorRoles"`
}

type ContributorRole struct {
	Name string `json:"Name"`
	Role string `json:"Role"`
}

func SyncHandler(p *proxy.Proxy, bc *BookCache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Header.Del("Accept-Encoding")

		rec := newResponseRecorder()
		p.Forward(rec, r)

		for k, vs := range rec.header {
			for _, v := range vs {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(rec.code)
		body := rec.buf.Bytes()
		w.Write(body) //nolint:errcheck

		go func() {
			var items []SyncItem
			if err := json.Unmarshal(body, &items); err != nil {
				slog.Error("failed to parse sync response for caching", "err", err)
				return
			}
			count := 0
			for _, item := range items {
				if item.NewEntitlement == nil {
					continue
				}
				ne := item.NewEntitlement
				id := ne.BookEntitlement.Id
				author := firstAuthor(ne.BookMetadata.ContributorRoles)
				bc.Set(id, BookMeta{Title: ne.BookMetadata.Title, Author: author})
				slog.Debug("cached book metadata", "id", id, "title", ne.BookMetadata.Title, "author", author)
				count++
			}
			slog.Info("book cache updated from sync", "count", count)
		}()
	}
}

const koboStoreSync = "https://storeapi.kobo.com/v1/library/sync"

var syncClient = &http.Client{Timeout: 20 * time.Second}

var hopByHop = map[string]bool{
	"Connection":          true,
	"Keep-Alive":          true,
	"Proxy-Authenticate":  true,
	"Proxy-Authorization": true,
	"Te":                  true,
	"Trailers":            true,
	"Transfer-Encoding":   true,
	"Upgrade":             true,
}

func fetchAndCache(koboHeaders http.Header, bc *BookCache) error {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, koboStoreSync, nil)
	if err != nil {
		return err
	}
	req.Host = "storeapi.kobo.com"

	for k, vs := range koboHeaders {
		if !hopByHop[k] {
			req.Header[k] = vs
		}
	}
	req.Header.Del("Accept-Encoding")

	resp, err := syncClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("upstream returned %d: %s", resp.StatusCode, string(body))
	}

	var items []SyncItem
	if err := json.Unmarshal(body, &items); err != nil {
		return err
	}

	count := 0
	for _, item := range items {
		if item.NewEntitlement == nil {
			continue
		}
		ne := item.NewEntitlement
		id := ne.BookEntitlement.Id
		author := firstAuthor(ne.BookMetadata.ContributorRoles)
		bc.Set(id, BookMeta{Title: ne.BookMetadata.Title, Author: author})
		count++
	}
	slog.Info("on-demand sync populated cache", "count", count)
	return nil
}

func firstAuthor(roles []ContributorRole) string {
	fallback := ""
	for _, cr := range roles {
		if fallback == "" {
			fallback = cr.Name
		}
		if cr.Role == "Author" {
			return cr.Name
		}
	}
	return fallback
}

type responseRecorder struct {
	header http.Header
	buf    *bytes.Buffer
	code   int
}

func newResponseRecorder() *responseRecorder {
	return &responseRecorder{header: make(http.Header), buf: &bytes.Buffer{}, code: http.StatusOK}
}

func (r *responseRecorder) Header() http.Header         { return r.header }
func (r *responseRecorder) WriteHeader(code int)        { r.code = code }
func (r *responseRecorder) Write(b []byte) (int, error) { return r.buf.Write(b) }
func (r *responseRecorder) Flush()                      {}
