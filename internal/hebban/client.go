package hebban

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"
)

const baseURL = "https://www.hebban.nl/api/1"

var httpClient = &http.Client{Timeout: 15 * time.Second}

type Client struct {
	authToken string
}

func NewClient(authToken string) *Client {
	return &Client{authToken: authToken}
}

func (c *Client) UpdateReadingStatus(title, author, status string) error {
	if c.authToken == "" {
		return fmt.Errorf("no Hebban token configured")
	}

	query := title
	if author != "" {
		query = title + " " + author
	}

	items, err := c.search(query)
	if err != nil {
		return fmt.Errorf("search %q: %w", query, err)
	}

	workID, matched, ok := bestMatch(items, title, author)
	if !ok {
		return fmt.Errorf("no matching work found for %q by %q (%d results)", title, author, len(items))
	}

	slog.Info("matched work on Hebban", "work_id", workID, "matched_title", matched, "status", status)
	return c.setStatus(workID, status)
}

type searchItem struct {
	ID     int    `json:"id"`
	Title  string `json:"title"`
	Author string `json:"author"`
}

type searchResponse struct {
	Items []searchItem `json:"items"`
}

func (c *Client) search(query string) ([]searchItem, error) {
	endpoint := fmt.Sprintf("%s/works?filter=query:%s&limit=5", baseURL, url.QueryEscape(query))

	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	c.addAuth(req)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	return result.Items, nil
}

func (c *Client) setStatus(workID int, status string) error {
	endpoint := fmt.Sprintf("%s/work/%d/status", baseURL, workID)
	payload, _ := json.Marshal(map[string]string{"status": status})

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	c.addAuth(req)

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func (c *Client) addAuth(req *http.Request) {
	req.AddCookie(&http.Cookie{Name: "hebban-authorization-token", Value: c.authToken})
}

func bestMatch(items []searchItem, title, author string) (int, string, bool) {
	if len(items) == 0 {
		return 0, "", false
	}

	normTitle := normalize(title)
	normAuthor := normalize(author)

	bestScore := 0
	bestID := 0
	bestTitle := ""

	for _, item := range items {
		score := wordOverlap(normalize(item.Title), normTitle)
		if normAuthor != "" {
			score += wordOverlap(normalize(item.Author), normAuthor)
		}
		if score > bestScore {
			bestScore = score
			bestID = item.ID
			bestTitle = item.Title
		}
	}

	return bestID, bestTitle, bestScore > 0
}

func normalize(s string) string {
	var b strings.Builder
	prevWasSpace := true
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			prevWasSpace = false
		} else if !prevWasSpace {
			b.WriteRune(' ')
			prevWasSpace = true
		}
	}
	return strings.TrimSpace(b.String())
}

func wordOverlap(a, b string) int {
	set := make(map[string]bool)
	for _, w := range strings.Fields(b) {
		set[w] = true
	}
	count := 0
	for _, w := range strings.Fields(a) {
		if set[w] {
			count++
		}
	}
	return count
}
