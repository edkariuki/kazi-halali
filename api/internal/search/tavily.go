// Package search looks for public reports about an agency or phone number
// using the Tavily search API.
package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/edkariuki/kazi-halali/api/internal/domain"
)

type Searcher interface {
	Reports(ctx context.Context, f domain.Facts) ([]domain.SearchHit, error)
}

type Tavily struct {
	APIKey string
	HTTP   *http.Client
	// BaseURL defaults to the Tavily API; tests point it at a local server.
	BaseURL string
}

func NewTavily(key string) *Tavily {
	return &Tavily{APIKey: key, HTTP: &http.Client{Timeout: 20 * time.Second}}
}

// Options tunes a single Tavily search.
type Options struct {
	MaxResults     int
	Depth          string   // "basic" | "advanced"
	IncludeDomains []string // restrict results to these sites
}

func (t *Tavily) endpoint() string {
	if t.BaseURL != "" {
		return strings.TrimRight(t.BaseURL, "/") + "/search"
	}
	return "https://api.tavily.com/search"
}

// Queries builds the searches we run for a set of facts. Exported for tests
// and so the UI can show what was searched.
func Queries(f domain.Facts) []string {
	var q []string
	if n := strings.TrimSpace(f.AgencyName); n != "" {
		q = append(q, fmt.Sprintf(`"%s" Kenya recruitment agency scam OR fraud OR deregistered`, n))
	}
	if len(f.Phones) > 0 {
		q = append(q, fmt.Sprintf(`"%s" job scam Kenya`, f.Phones[0]))
	}
	return q
}

func (t *Tavily) Reports(ctx context.Context, f domain.Facts) ([]domain.SearchHit, error) {
	qs := Queries(f)
	if len(qs) == 0 || t.APIKey == "" {
		return nil, nil
	}
	var (
		mu   sync.Mutex
		wg   sync.WaitGroup
		hits []domain.SearchHit
		errs []string
	)
	for _, q := range qs {
		wg.Add(1)
		go func(q string) {
			defer wg.Done()
			h, err := t.search(ctx, q, Options{})
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, err.Error())
				return
			}
			hits = append(hits, h...)
		}(q)
	}
	wg.Wait()
	hits = dedupe(hits)
	if len(errs) > 0 && len(hits) == 0 {
		return nil, fmt.Errorf("tavily: %s", strings.Join(errs, "; "))
	}
	return hits, nil
}

func (t *Tavily) search(ctx context.Context, query string, o Options) ([]domain.SearchHit, error) {
	if o.MaxResults == 0 {
		o.MaxResults = 6
	}
	if o.Depth == "" {
		o.Depth = "basic"
	}
	payload := map[string]any{
		"query":          query,
		"search_depth":   o.Depth,
		"max_results":    o.MaxResults,
		"include_answer": false,
	}
	if len(o.IncludeDomains) > 0 {
		payload["include_domains"] = o.IncludeDomains
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.endpoint(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+t.APIKey)
	res, err := t.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", res.StatusCode, string(raw))
	}
	var parsed struct {
		Results []struct {
			Title   string  `json:"title"`
			URL     string  `json:"url"`
			Content string  `json:"content"`
			Score   float64 `json:"score"`
		} `json:"results"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, err
	}
	out := make([]domain.SearchHit, 0, len(parsed.Results))
	for _, r := range parsed.Results {
		out = append(out, domain.SearchHit{Title: r.Title, URL: r.URL, Snippet: r.Content, Score: r.Score})
	}
	return out, nil
}

func dedupe(in []domain.SearchHit) []domain.SearchHit {
	seen := map[string]bool{}
	var out []domain.SearchHit
	for _, h := range in {
		if seen[h.URL] {
			continue
		}
		seen[h.URL] = true
		out = append(out, h)
	}
	return out
}

// Static returns fixed hits; used in stub mode and tests.
type Static []domain.SearchHit

func (s Static) Reports(context.Context, domain.Facts) ([]domain.SearchHit, error) { return s, nil }
