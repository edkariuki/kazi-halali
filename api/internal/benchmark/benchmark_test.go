package benchmark

import (
	"context"
	"testing"

	"github.com/edkariuki/kazi-halali/api/internal/domain"
	"github.com/edkariuki/kazi-halali/api/internal/llm"
	"github.com/edkariuki/kazi-halali/api/internal/search"
)

type fakeLLM struct{ out string }

func (f fakeLLM) Chat(context.Context, llm.Request) (llm.Response, error) {
	return llm.Response{Text: f.out}, nil
}

var hits = search.SalaryStatic{
	{Title: "Cleaner salary in Qatar", URL: "https://salaries.example/qatar-cleaner", Snippet: "Cleaners in Qatar earn QAR 1,200 to 2,000 per month."},
}

func facts(amount float64, cur, period string) domain.Facts {
	return domain.Facts{
		JobTitle: "cleaner", DestinationCountry: "Qatar",
		Salary: &domain.Money{Amount: amount, Currency: cur, Period: period},
	}
}

func TestRatioComputedInGo(t *testing.T) {
	b := &Benchmarker{Search: hits, Client: fakeLLM{`{"found":true,"low":1200,"high":2000,"currency":"QAR","period":"month","source_urls":["https://salaries.example/qatar-cleaner"]}`}, Model: "m"}
	bm, trace := b.Run(context.Background(), facts(9000, "QAR", "month"))
	if bm == nil || !bm.Found || bm.Ratio != 4.5 {
		t.Fatalf("bm = %+v", bm)
	}
	if len(trace) != 2 || len(bm.Sources) != 1 {
		t.Fatalf("trace=%d sources=%d", len(trace), len(bm.Sources))
	}
}

func TestCrossCurrency(t *testing.T) {
	b := &Benchmarker{Search: hits, Client: fakeLLM{`{"found":true,"low":1200,"high":2000,"currency":"QAR","period":"month","source_urls":["https://salaries.example/qatar-cleaner"]}`}, Model: "m"}
	// KES 70,800 ≈ QAR 2,000 → ratio ~1
	bm, _ := b.Run(context.Background(), facts(70800, "KES", "month"))
	if bm == nil || bm.Ratio < 0.9 || bm.Ratio > 1.1 {
		t.Fatalf("bm = %+v", bm)
	}
}

func TestInventedSourceRejected(t *testing.T) {
	b := &Benchmarker{Search: hits, Client: fakeLLM{`{"found":true,"low":100,"high":200,"currency":"QAR","period":"month","source_urls":["https://made-up.example/x"]}`}, Model: "m"}
	bm, _ := b.Run(context.Background(), facts(9000, "QAR", "month"))
	if bm == nil || bm.Found || bm.Ratio != 0 {
		t.Fatalf("figures without a real source must not be used: %+v", bm)
	}
}

func TestAnnualFiguresNormalised(t *testing.T) {
	b := &Benchmarker{Search: hits, Client: fakeLLM{`{"found":true,"low":14400,"high":24000,"currency":"QAR","period":"year","source_urls":["https://salaries.example/qatar-cleaner"]}`}, Model: "m"}
	bm, _ := b.Run(context.Background(), facts(2000, "QAR", "month"))
	if bm == nil || bm.MonthlyHigh != 2000 || bm.Ratio != 1 {
		t.Fatalf("bm = %+v", bm)
	}
}

func TestSkippedWithoutSalary(t *testing.T) {
	b := &Benchmarker{Search: hits, Client: fakeLLM{}, Model: "m"}
	if bm, _ := b.Run(context.Background(), domain.Facts{JobTitle: "cleaner", DestinationCountry: "Qatar"}); bm != nil {
		t.Fatal("no salary in the ad, nothing to compare")
	}
}

type countingSearch struct{ n int }

func (c *countingSearch) Salaries(context.Context, string, string) ([]domain.SearchHit, error) {
	c.n++
	return hits, nil
}

func TestCacheGivesSameAnswerWithoutSearching(t *testing.T) {
	cs := &countingSearch{}
	b := &Benchmarker{Search: cs, Client: fakeLLM{`{"found":true,"low":1200,"high":2000,"currency":"QAR","period":"month","source_urls":["https://salaries.example/qatar-cleaner"]}`}, Model: "m"}
	first, _ := b.Run(context.Background(), facts(9000, "QAR", "month"))
	second, trace := b.Run(context.Background(), facts(4000, "QAR", "month"))
	if cs.n != 1 {
		t.Fatalf("searched %d times, want 1", cs.n)
	}
	if first.Ratio != 4.5 || second.Ratio != 2 {
		t.Fatalf("ratios %v %v — the ratio must be recomputed for each ad", first.Ratio, second.Ratio)
	}
	if len(trace) != 1 || trace[0].Model != "cache" {
		t.Fatalf("trace = %+v", trace)
	}
}

func TestNotFoundIsNotCached(t *testing.T) {
	cs := &countingSearch{}
	b := &Benchmarker{Search: cs, Client: fakeLLM{`{"found":false}`}, Model: "m"}
	b.Run(context.Background(), facts(9000, "QAR", "month"))
	b.Run(context.Background(), facts(9000, "QAR", "month"))
	if cs.n != 2 {
		t.Fatalf("a miss should be retried next time, searched %d times", cs.n)
	}
}
