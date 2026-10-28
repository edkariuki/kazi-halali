package verdict

import (
	"context"
	"testing"

	"github.com/edkariuki/kazi-halali/api/internal/domain"
	"github.com/edkariuki/kazi-halali/api/internal/llm"
)

func TestParseExplanationShapes(t *testing.T) {
	cases := map[string]string{
		"plain":   `{"summary_en":"Be careful.","summary_sw":"Kuwa mwangalifu.","next_steps_en":["Call NEA."],"next_steps_sw":["Piga NEA."]}`,
		"wrapped": "Here:\n```json\n{\"explanation\":{\"summary_en\":\"Be careful.\",\"summary_sw\":\"Kuwa mwangalifu.\",\"next_steps_en\":[{\"step\":\"Call NEA.\"}],\"next_steps_sw\":[{\"action\":\"Piga NEA.\"}]}}\n```",
		"split":   `{"summary_en":["Be careful."],"summary_sw":"Kuwa mwangalifu.","next_steps_en":"Call NEA.","next_steps_sw":["Piga NEA."]}`,
	}
	for name, in := range cases {
		e, ok := parseExplanation(in)
		if !ok || e.SummaryEN != "Be careful." || len(e.NextStepsEN) != 1 || e.NextStepsSW[0] != "Piga NEA." {
			t.Errorf("%s: ok=%v e=%+v", name, ok, e)
		}
	}
	if _, ok := parseExplanation(`{"summary_sw":"Kiswahili tu"}`); ok {
		t.Error("missing English summary must be rejected")
	}
}

type fakeClient struct{ text string }

func (f fakeClient) Chat(context.Context, llm.Request) (llm.Response, error) {
	return llm.Response{Text: f.text}, nil
}

func TestKiswahiliAlwaysFromTemplate(t *testing.T) {
	findings := []domain.Finding{{RuleID: "fees.unskilled", Severity: domain.SevCritical, TitleEN: "Fee for an unskilled job", TitleSW: "Ada kwa kazi isiyo ya kitaalamu"}}
	e := &Explainer{Client: fakeClient{`{"summary_en":"Do not pay.","summary_sw":"Kiswahili mbaya","next_steps_en":["Stop paying."]}`}, Model: "m"}
	got, call := e.Explain(context.Background(), domain.Facts{}, findings, domain.VerdictDanger)
	want := Template(findings, domain.VerdictDanger)
	if call.Error != "" || !got.Generated || got.SummaryEN != "Do not pay." {
		t.Fatalf("got %+v, call %+v", got, call)
	}
	if got.SummarySW != want.SummarySW || len(got.NextStepsSW) != len(want.NextStepsSW) {
		t.Fatalf("Kiswahili must come from the template: %+v", got)
	}
	if !containsHotline(got.NextStepsEN) {
		t.Fatal("hotline step missing from English steps")
	}
}
