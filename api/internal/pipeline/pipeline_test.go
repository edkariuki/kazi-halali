package pipeline

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/edkariuki/kazi-halali/api/internal/benchmark"
	"github.com/edkariuki/kazi-halali/api/internal/domain"
	"github.com/edkariuki/kazi-halali/api/internal/extract"
	"github.com/edkariuki/kazi-halali/api/internal/llm"
	"github.com/edkariuki/kazi-halali/api/internal/registry"
	"github.com/edkariuki/kazi-halali/api/internal/search"
	"github.com/edkariuki/kazi-halali/api/internal/store"
	"github.com/edkariuki/kazi-halali/api/internal/verdict"
)

// fakeLLM answers by model name, so we exercise the real code paths.
type fakeLLM struct {
	byModel map[string]string
	fail    map[string]bool
	calls   []string
}

func (f *fakeLLM) Chat(_ context.Context, r llm.Request) (llm.Response, error) {
	f.calls = append(f.calls, r.Model)
	if f.fail[r.Model] {
		return llm.Response{}, errors.New("boom")
	}
	return llm.Response{Text: f.byModel[r.Model], InputTokens: 10, OutputTokens: 5}, nil
}

func newPipeline(c llm.Client, st *store.Memory) *Pipeline {
	reg := registry.New([]domain.Agency{
		{ID: 1, Name: "Upendo Global Placements Limited", NEARegNo: "NEA/PEA/0001", Status: domain.StatusValid},
	}, nil)
	return &Pipeline{
		Extractor: &extract.Extractor{Client: c, VisionModel: "vl", ExtractModel: "nano"},
		Registry:  func() *registry.Registry { return reg },
		Searcher:  search.Static{{Title: "Police arrest Ghost Agency boss over fake Canada jobs", URL: "https://news.example/1"}},
		Explainer: &verdict.Explainer{Client: c, Model: "super"},
		Saver:     st,
	}
}

func TestPipelineWithModels(t *testing.T) {
	fake := &fakeLLM{byModel: map[string]string{
		"vl":    "GHOST AGENCY — Canada farm jobs. Processing fee KES 50,000. WhatsApp 0711 000 111\nVISUAL NOTES: no logo",
		"nano":  "<think>reading</think>```json\n{\"agency_name\":\"Ghost Agency\",\"destination_country\":\"Canada\",\"job_category\":\"unskilled\",\"fees\":[{\"amount\":50000,\"currency\":\"KES\",\"purpose\":\"processing\",\"payable_before_contract\":true}],\"contact_channels\":[\"whatsapp\"],\"visa_type\":\"unknown\"}\n```",
		"super": `{"summary_en":"Do not pay.","summary_sw":"Usilipe.","next_steps_en":["Stop."],"next_steps_sw":["Acha."]}`,
	}}
	st := store.NewMemory()
	p := newPipeline(fake, st)

	c, err := p.Run(context.Background(), "", &extract.Image{Bytes: []byte("png"), MimeType: "image/png"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(fake.calls, ",") != "vl,nano,super" {
		t.Fatalf("model routing = %v", fake.calls)
	}
	if c.Verdict != domain.VerdictDanger {
		t.Fatalf("verdict = %s", c.Verdict)
	}
	if !c.Explanation.Generated || c.Explanation.SummaryEN != "Do not pay." || !strings.HasPrefix(c.Explanation.SummarySW, "Usilipe pesa yoyote") { // Kiswahili comes from the template
		t.Fatalf("explanation = %+v", c.Explanation)
	}
	// Hotline step must be appended even if the model forgot it.
	if !strings.Contains(strings.Join(c.Explanation.NextStepsEN, " "), "0800") {
		t.Fatalf("hotline missing: %v", c.Explanation.NextStepsEN)
	}
	if !c.HadImage || c.Transcript == "" {
		t.Fatal("image transcript not recorded")
	}
	got, err := st.GetCheck(context.Background(), c.ID)
	if err != nil || got.Verdict != c.Verdict {
		t.Fatalf("not saved: %v", err)
	}
	var web bool
	for _, f := range c.Findings {
		if f.RuleID == "web.reports" {
			web = true
		}
	}
	if !web {
		t.Fatal("expected web.reports finding from the static search hit")
	}
}

func TestPipelineFallsBackWhenModelsFail(t *testing.T) {
	fake := &fakeLLM{fail: map[string]bool{"nano": true, "super": true}}
	p := newPipeline(fake, store.NewMemory())
	c, err := p.Run(context.Background(), "House help needed in Saudi. Registration fee KES 15,000. WhatsApp 0798 111 222.", nil)
	if err != nil {
		t.Fatal(err)
	}
	if c.Verdict != domain.VerdictDanger {
		t.Fatalf("verdict = %s", c.Verdict)
	}
	if c.Explanation.Generated {
		t.Fatal("should have used the template")
	}
	if c.Explanation.SummaryEN == "" || c.Explanation.SummarySW == "" {
		t.Fatal("template explanation empty")
	}
	var sawErr bool
	for _, tr := range c.Trace {
		if tr.Error != "" {
			sawErr = true
		}
	}
	if !sawErr {
		t.Fatal("trace should record the model failures")
	}
}

func TestPipelineLiveChecksInParallel(t *testing.T) {
	fake := &fakeLLM{byModel: map[string]string{
		"nano":  `{"agency_name":"Ghost Recruiters Ltd","job_title":"cleaner","destination_country":"Qatar","job_category":"unskilled","salary":{"amount":9000,"currency":"QAR","period":"month"},"visa_type":"work"}`,
		"bench": `{"found":true,"low":1200,"high":2000,"currency":"QAR","period":"month","source_urls":["https://salaries.example/q"]}`,
		"super": `{"summary_en":"Do not pay.","summary_sw":"Usilipe.","next_steps_en":["Stop."],"next_steps_sw":["Acha."]}`,
	}}
	p := newPipeline(fake, store.NewMemory())
	p.Official = search.OfficialStatic{{
		Title: "Ministry notice", URL: "https://labour.go.ke/n",
		Snippet: "Ghost Recruiters Limited has been deregistered and is not authorised to recruit.",
	}}
	p.Benchmark = &benchmark.Benchmarker{
		Search: search.SalaryStatic{{Title: "Cleaner pay Qatar", URL: "https://salaries.example/q", Snippet: "QAR 1,200-2,000"}},
		Client: fake, Model: "bench",
	}

	c, err := p.Run(context.Background(), "Ghost Recruiters Ltd: cleaners for Qatar, QAR 9,000 a month.", nil)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]domain.Severity{}
	for _, f := range c.Findings {
		got[f.RuleID] = f.Severity
	}
	if got["official.warning"] != domain.SevCritical {
		t.Fatalf("official.warning = %v", got["official.warning"])
	}
	if got["salary.too_good"] != domain.SevHigh {
		t.Fatalf("salary.too_good = %v", got["salary.too_good"])
	}
	steps := map[string]bool{}
	for _, tr := range c.Trace {
		steps[tr.Step] = true
	}
	for _, s := range []string{"extract", "search", "official", "salary", "explain"} {
		if !steps[s] {
			t.Errorf("trace missing step %q", s)
		}
	}
}
