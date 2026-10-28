// Package pipeline runs one check end to end:
//
//	extract (vision → JSON facts) → registry match → web search
//	→ rules → verdict → explanation → save
package pipeline

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"strings"
	"sync"
	"time"

	"github.com/edkariuki/kazi-halali/api/internal/benchmark"
	"github.com/edkariuki/kazi-halali/api/internal/domain"
	"github.com/edkariuki/kazi-halali/api/internal/extract"
	"github.com/edkariuki/kazi-halali/api/internal/registry"
	"github.com/edkariuki/kazi-halali/api/internal/rules"
	"github.com/edkariuki/kazi-halali/api/internal/search"
	"github.com/edkariuki/kazi-halali/api/internal/verdict"
)

type Saver interface {
	SaveCheck(ctx context.Context, c *domain.Check) error
}

type OfficialSearcher interface {
	Official(ctx context.Context, f domain.Facts) ([]domain.SearchHit, error)
}

type Pipeline struct {
	Extractor *extract.Extractor
	Registry  func() *registry.Registry
	Searcher  search.Searcher        // public fraud reports (optional)
	Official  OfficialSearcher       // government-site warnings (optional)
	Benchmark *benchmark.Benchmarker // salary reality check (optional)
	Explainer *verdict.Explainer
	Saver     Saver
}

func (p *Pipeline) Run(ctx context.Context, text string, img *extract.Image) (*domain.Check, error) {
	c := &domain.Check{
		ID:        NewID(),
		CreatedAt: time.Now().UTC(),
		InputText: strings.TrimSpace(text),
		HadImage:  img != nil && len(img.Bytes) > 0,
	}

	ex := p.Extractor.Run(ctx, text, img)
	c.Transcript, c.Facts = ex.Transcript, ex.Facts
	c.Trace = append(c.Trace, ex.Trace...)

	c.Registry = p.Registry().Match(c.Facts)

	// The three live checks are independent: run them in parallel so the
	// user waits for the slowest one, not the sum.
	lctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	var (
		wg sync.WaitGroup
		mu sync.Mutex
	)
	addTrace := func(calls ...domain.ModelCall) {
		mu.Lock()
		c.Trace = append(c.Trace, calls...)
		mu.Unlock()
	}

	if p.Searcher != nil && len(search.Queries(c.Facts)) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			start := time.Now()
			hits, err := p.Searcher.Reports(lctx, c.Facts)
			call := domain.ModelCall{Step: "search", Model: "tavily", LatencyMS: time.Since(start).Milliseconds()}
			if err != nil {
				call.Error = err.Error()
			}
			c.SearchHits = hits
			addTrace(call)
		}()
	}

	if p.Official != nil && strings.TrimSpace(c.Facts.AgencyName) != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			start := time.Now()
			hits, err := p.Official.Official(lctx, c.Facts)
			call := domain.ModelCall{Step: "official", Model: "tavily", LatencyMS: time.Since(start).Milliseconds()}
			if err != nil {
				call.Error = err.Error()
			} else {
				c.OfficialSearched = true
			}
			c.OfficialHits = hits
			addTrace(call)
		}()
	}

	if p.Benchmark != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			bm, calls := p.Benchmark.Run(lctx, c.Facts)
			c.Salary = bm
			addTrace(calls...)
		}()
	}
	wg.Wait()
	cancel()

	c.Findings = rules.Evaluate(rules.Input{
		Facts:            c.Facts,
		Registry:         c.Registry,
		Hits:             c.SearchHits,
		Official:         c.OfficialHits,
		OfficialSearched: c.OfficialSearched,
		Salary:           c.Salary,
	})
	c.Verdict = verdict.Decide(c.Findings)

	exp, call := p.Explainer.Explain(ctx, c.Facts, c.Findings, c.Verdict)
	c.Explanation = exp
	if call != nil {
		c.Trace = append(c.Trace, *call)
	}

	if p.Saver != nil {
		if err := p.Saver.SaveCheck(ctx, c); err != nil {
			return c, err
		}
	}
	return c, nil
}

// NewID returns a short, URL-safe, unguessable id for share links.
func NewID() string {
	b := make([]byte, 10)
	_, _ = rand.Read(b)
	return strings.ToLower(strings.TrimRight(base32.StdEncoding.EncodeToString(b), "="))
}
