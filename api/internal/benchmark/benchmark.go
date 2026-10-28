// Package benchmark checks whether the pay in a job ad is far above what
// that job normally pays — a classic scam hook.
//
//	Tavily: "average monthly salary <job> in <country>"
//	   │
//	   ▼
//	Nemotron reads the figures out of the snippets (it does NOT judge)
//	   │
//	   ▼
//	Go normalises periods, converts currencies, computes the ratio
//	   │
//	   ▼
//	rules.salaryTooGood decides
package benchmark

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/edkariuki/kazi-halali/api/internal/domain"
	"github.com/edkariuki/kazi-halali/api/internal/llm"
)

type SalarySearcher interface {
	Salaries(ctx context.Context, jobTitle, country string) ([]domain.SearchHit, error)
}

type Benchmarker struct {
	Search    SalarySearcher
	Client    llm.Client
	Model     string
	MaxTokens int
	// CacheTTL keeps a found salary range per job+country, so the same ad gets
	// the same answer and repeat checks cost no search. 0 = 24h.
	CacheTTL time.Duration

	mu    sync.Mutex
	cache map[string]cachedRange
}

type cachedRange struct {
	low, high float64 // monthly
	currency  string
	sources   []domain.Source
	at        time.Time
}

// Run returns nil when there is nothing to compare (no salary, job or
// country in the ad) or when no search/model is configured.
func (b *Benchmarker) Run(ctx context.Context, f domain.Facts) (*domain.SalaryBenchmark, []domain.ModelCall) {
	if b == nil || b.Search == nil || b.Client == nil {
		return nil, nil
	}
	if f.Salary == nil || f.Salary.Amount <= 0 || f.DestinationCountry == "" {
		return nil, nil
	}
	job := strings.TrimSpace(f.JobTitle)
	if job == "" {
		job = jobFromCategory(f.JobCategory)
	}
	if job == "" {
		return nil, nil
	}
	bm := &domain.SalaryBenchmark{
		JobTitle:   job,
		Country:    f.DestinationCountry,
		AdMonthly:  ToMonthly(f.Salary.Amount, f.Salary.Period),
		AdCurrency: strings.ToUpper(f.Salary.Currency),
	}
	key := strings.ToLower(job + "|" + f.DestinationCountry)

	if c, ok := b.lookup(key); ok {
		apply(bm, c)
		return bm, []domain.ModelCall{{Step: "salary", Model: "cache"}}
	}

	var trace []domain.ModelCall
	start := time.Now()
	hits, err := b.Search.Salaries(ctx, job, f.DestinationCountry)
	sc := domain.ModelCall{Step: "salary", Model: "tavily", LatencyMS: time.Since(start).Milliseconds()}
	if err != nil {
		sc.Error = err.Error()
	}
	trace = append(trace, sc)
	if len(hits) == 0 {
		bm.Note = "no salary data found"
		return bm, trace
	}

	read, call := b.read(ctx, job, f.DestinationCountry, hits)
	trace = append(trace, call)
	if read == nil || !read.Found {
		bm.Note = "salary figures not found in search results"
		return bm, trace
	}

	low := ToMonthly(read.Low, read.Period)
	high := ToMonthly(read.High, read.Period)
	if high <= 0 {
		high = low
	}
	if low > high {
		low, high = high, low
	}
	if low <= 0 || high <= 0 {
		bm.Note = "salary figures not usable"
		return bm, trace
	}

	// Keep only sources the model said it used, and only if we actually
	// sent them — never a URL the model made up.
	sent := map[string]domain.SearchHit{}
	for _, h := range hits {
		sent[h.URL] = h
	}
	var sources []domain.Source
	for _, u := range read.SourceURLs {
		if h, ok := sent[u]; ok {
			sources = append(sources, domain.Source{Title: h.Title, URL: h.URL})
		}
	}
	if len(sources) == 0 {
		bm.Note = "figures could not be tied to a source"
		return bm, trace
	}

	c := cachedRange{low: low, high: high, currency: strings.ToUpper(read.Currency), sources: sources, at: time.Now()}
	b.store(key, c)
	apply(bm, c)
	return bm, trace
}

// apply fills in the typical range and computes the ratio in Go.
func apply(bm *domain.SalaryBenchmark, c cachedRange) {
	bm.Found = true
	bm.MonthlyLow, bm.MonthlyHigh = round(c.low), round(c.high)
	bm.Currency = c.currency
	bm.Sources = c.sources
	adKES, ok1 := ToKES(bm.AdMonthly, bm.AdCurrency)
	highKES, ok2 := ToKES(c.high, c.currency)
	if ok1 && ok2 && highKES > 0 {
		bm.Ratio = float64(int(adKES/highKES*10+0.5)) / 10
	} else {
		bm.Note = "currency not supported for comparison"
	}
}

func (b *Benchmarker) lookup(key string) (cachedRange, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	c, ok := b.cache[key]
	ttl := b.CacheTTL
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	if !ok || time.Since(c.at) > ttl {
		return cachedRange{}, false
	}
	return c, true
}

func (b *Benchmarker) store(key string, c cachedRange) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.cache == nil {
		b.cache = map[string]cachedRange{}
	}
	b.cache[key] = c
}

type reading struct {
	Found      bool     `json:"found"`
	Low        float64  `json:"low"`
	High       float64  `json:"high"`
	Currency   string   `json:"currency"`
	Period     string   `json:"period"`
	SourceURLs []string `json:"source_urls"`
}

func (b *Benchmarker) read(ctx context.Context, job, country string, hits []domain.SearchHit) (*reading, domain.ModelCall) {
	call := domain.ModelCall{Step: "salary", Model: b.Model}
	var sb strings.Builder
	for i, h := range hits {
		fmt.Fprintf(&sb, "[%d] %s\nURL: %s\n%s\n\n", i+1, h.Title, h.URL, truncate(h.Snippet, 900))
	}
	max := b.MaxTokens
	if max <= 0 {
		max = 6000
	}
	resp, err := b.Client.Chat(ctx, llm.Request{
		Model:       b.Model,
		Temperature: 0,
		MaxTokens:   max,
		JSON:        true,
		Messages: []llm.Message{
			{Role: "system", Content: readPrompt},
			{Role: "user", Content: fmt.Sprintf("Job: %s\nCountry: %s\n\nSearch results:\n%s", job, country, sb.String())},
		},
	})
	call.LatencyMS = resp.Latency.Milliseconds()
	call.InputTok, call.OutputTok = resp.InputTokens, resp.OutputTokens
	if err != nil {
		call.Error = err.Error() + " — salary check skipped"
		return nil, call
	}
	var r reading
	if err := llm.DecodeJSON(resp.Text, &r); err != nil {
		call.Error = "unparseable JSON — salary check skipped"
		return nil, call
	}
	return &r, call
}

const readPrompt = `You read salary figures out of web search results. You do not judge job offers.

Return ONE JSON object:
{
  "found": true|false,
  "low": number,         // lower end of typical pay for this job in this country
  "high": number,        // upper end of typical pay
  "currency": "ISO code", // currency the figures are stated in, e.g. QAR, CAD, USD
  "period": "month"|"year"|"hour"|"week"|"day",
  "source_urls": ["the URLs of the results you took the figures from"]
}

Rules:
- Use ONLY figures that appear in the search results. Never use your own knowledge or estimate.
- The figures must be for this job (or a very close equivalent) in this country. Ignore other countries.
- If results give a single average, use it for both low and high.
- Copy the URLs exactly as given. If no usable figure is present, return {"found": false}.
- Output JSON only.`

// ToMonthly converts a pay amount to a monthly figure.
func ToMonthly(amount float64, period string) float64 {
	switch strings.ToLower(strings.TrimSpace(period)) {
	case "hour", "hourly", "hr":
		return amount * 173 // ~40h/week
	case "day", "daily":
		return amount * 22
	case "week", "weekly":
		return amount * 4.33
	case "year", "annual", "annually", "yearly":
		return amount / 12
	}
	return amount // month, monthly, or unspecified
}

// kesPerUnit holds APPROXIMATE exchange rates, used only to compare an ad's
// salary with typical pay. The salary rule only fires at 2x or more, so
// rate drift of a few percent cannot change a result. Refresh before launch.
var kesPerUnit = map[string]float64{
	"KES": 1, "USD": 129, "CAD": 93, "EUR": 140, "GBP": 170,
	"QAR": 35.4, "SAR": 34.4, "AED": 35.1, "OMR": 335, "KWD": 420, "BHD": 342,
	"AUD": 85, "JPY": 0.86, "PLN": 33, "RUB": 1.5,
}

// ToKES converts an amount to Kenyan shillings using kesPerUnit.
func ToKES(amount float64, currency string) (float64, bool) {
	r, ok := kesPerUnit[strings.ToUpper(strings.TrimSpace(currency))]
	if !ok {
		return 0, false
	}
	return amount * r, true
}

func jobFromCategory(c domain.JobCategory) string {
	switch c {
	case domain.JobDomestic:
		return "domestic worker"
	case domain.JobUnskilled:
		return "general labourer"
	}
	return ""
}

func round(f float64) float64 { return float64(int64(f + 0.5)) }

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
