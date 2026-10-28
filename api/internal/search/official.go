package search

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/edkariuki/kazi-halali/api/internal/domain"
)

// OfficialDomains are Kenyan government sites that publish warnings about
// recruitment agencies. Searching only these means a match is an official
// statement, not a rumour. Keep this list to government sources.
var OfficialDomains = []string{
	"labour.go.ke",
	"nea.go.ke",
	"neaims.go.ke",
	"diaspora.go.ke",
	"mfa.go.ke",
	"mygov.go.ke",
	"kenyanews.go.ke",
}

// Official searches government sites for the agency named in the ad.
// It returns raw hits; deciding whether a hit is a warning is the rules'
// job (see rules.officialWarning), so it stays testable and explainable.
func (t *Tavily) Official(ctx context.Context, f domain.Facts) ([]domain.SearchHit, error) {
	name := strings.TrimSpace(f.AgencyName)
	if name == "" || t.APIKey == "" {
		return nil, nil
	}
	return t.search(ctx, fmt.Sprintf(`"%s" recruitment agency`, name), Options{
		MaxResults:     8,
		Depth:          "advanced", // better recall on PDFs and notices
		IncludeDomains: OfficialDomains,
	})
}

// Salaries searches for typical pay for a job in a country. It runs two
// phrasings in parallel — one naming the local currency, which pulls in pages
// that quote actual figures — and merges the results. The results go to the
// benchmark step, where a model reads the numbers out of them.
func (t *Tavily) Salaries(ctx context.Context, jobTitle, country string) ([]domain.SearchHit, error) {
	if t.APIKey == "" || jobTitle == "" || country == "" {
		return nil, nil
	}
	queries := []string{fmt.Sprintf("average monthly salary %s in %s", jobTitle, country)}
	if cur := LocalCurrency(country); cur != "" {
		queries = append(queries, fmt.Sprintf("%s salary %s %s per month", jobTitle, country, cur))
	}
	var (
		mu   sync.Mutex
		wg   sync.WaitGroup
		hits []domain.SearchHit
		errs []string
	)
	for _, q := range queries {
		wg.Add(1)
		go func(q string) {
			defer wg.Done()
			h, err := t.search(ctx, q, Options{MaxResults: 5, Depth: "advanced"})
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
	if len(hits) == 0 && len(errs) > 0 {
		return nil, fmt.Errorf("tavily: %s", strings.Join(errs, "; "))
	}
	return hits, nil
}

// LocalCurrency returns the ISO currency code for common destination countries.
func LocalCurrency(country string) string {
	return localCurrency[strings.ToLower(strings.TrimSpace(country))]
}

var localCurrency = map[string]string{
	"qatar": "QAR", "saudi arabia": "SAR", "united arab emirates": "AED", "uae": "AED",
	"oman": "OMR", "kuwait": "KWD", "bahrain": "BHD", "canada": "CAD",
	"united kingdom": "GBP", "united states": "USD", "germany": "EUR", "poland": "PLN",
	"australia": "AUD", "japan": "JPY", "russia": "RUB",
}

// OfficialStatic and SalaryStatic are fixed results for tests and stub mode.
type OfficialStatic []domain.SearchHit

func (s OfficialStatic) Official(context.Context, domain.Facts) ([]domain.SearchHit, error) {
	return s, nil
}

type SalaryStatic []domain.SearchHit

func (s SalaryStatic) Salaries(context.Context, string, string) ([]domain.SearchHit, error) {
	return s, nil
}
