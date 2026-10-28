// Package verdict turns findings into a verdict (deterministically) and an
// explanation (with a language model, falling back to templates).
package verdict

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/edkariuki/kazi-halali/api/internal/domain"
	"github.com/edkariuki/kazi-halali/api/internal/llm"
	"github.com/edkariuki/kazi-halali/api/internal/rules"
)

// Decide is pure and deterministic: the model never chooses the colour.
func Decide(findings []domain.Finding) domain.Verdict {
	var crit, high, med int
	for _, f := range findings {
		switch f.Severity {
		case domain.SevCritical:
			crit++
		case domain.SevHigh:
			high++
		case domain.SevMedium:
			med++
		}
	}
	switch {
	case crit > 0:
		return domain.VerdictDanger
	case high > 0 || med >= 2:
		return domain.VerdictCaution
	}
	return domain.VerdictLikelyOK
}

type Explainer struct {
	Client    llm.Client // nil = templates only
	Model     string
	MaxTokens int // 0 = 6000; leaves room for reasoning before the answer
}

func (e *Explainer) Explain(ctx context.Context, f domain.Facts, findings []domain.Finding, v domain.Verdict) (domain.Explanation, *domain.ModelCall) {
	fallback := Template(findings, v)
	if e.Client == nil {
		return fallback, nil
	}
	call := &domain.ModelCall{Step: "explain", Model: e.Model}

	type brief struct {
		Severity string `json:"severity"`
		Title    string `json:"title"`
		Detail   string `json:"detail"`
	}
	var bs []brief
	for _, x := range findings {
		bs = append(bs, brief{string(x.Severity), x.TitleEN, x.DetailEN})
	}
	payload, _ := json.MarshalIndent(map[string]any{
		"verdict":     v,
		"job":         map[string]string{"title": f.JobTitle, "destination": f.DestinationCountry, "agency": f.AgencyName},
		"findings":    bs,
		"nea_hotline": rules.NEAHotline,
	}, "", "  ")

	resp, err := e.Client.Chat(ctx, llm.Request{
		Model:       e.Model,
		Temperature: 0.2,
		MaxTokens:   e.budget(),
		JSON:        true,
		Messages: []llm.Message{
			{Role: "system", Content: explainSystemPrompt},
			{Role: "user", Content: string(payload)},
		},
	})
	call.LatencyMS = resp.Latency.Milliseconds()
	call.InputTok, call.OutputTok = resp.InputTokens, resp.OutputTokens
	if err != nil {
		call.Error = err.Error() + " — used template explanation"
		return fallback, call
	}
	out, ok := parseExplanation(resp.Text)
	if !ok {
		call.Error = fmt.Sprintf("unusable model output %q — used template explanation", snippet(resp.Text, 160))
		return fallback, call
	}
	out.Generated = true
	// Kiswahili comes from the hand-written templates, not the model. We tested
	// the Nemotron models available on Token Factory and their Kiswahili was not
	// reliable enough for safety advice (wrong words, mistranslated warnings).
	// The template text is specific to this check: it names the findings, each
	// with a hand-written Kiswahili title.
	out.SummarySW, out.NextStepsSW = fallback.SummarySW, fallback.NextStepsSW
	// The hotline step is non-negotiable; make sure it survived.
	if len(out.NextStepsEN) == 0 || !containsHotline(out.NextStepsEN) {
		out.NextStepsEN = append(out.NextStepsEN, fallback.NextStepsEN[len(fallback.NextStepsEN)-1])
	}
	return out, call
}

const explainSystemPrompt = `You explain the result of an overseas job-offer check to a Kenyan job seeker.

You receive a verdict ("danger", "caution" or "likely_ok") and a list of findings produced by fixed rules. Write for someone who may have only secondary-school English: short sentences, warm but direct.

Return ONE JSON object, in English:
{
  "summary_en": "2-3 sentences",
  "next_steps_en": ["3 to 4 concrete actions"]
}

Hard rules:
- Use ONLY the findings given. Do not add facts, statistics, names or laws that are not in the input.
- Never change or soften the verdict. For "danger", clearly say not to pay or send documents. For "likely_ok", still say the job is not proven genuine and must be confirmed directly with the agency.
- Never say a job is "safe", "legit" or "guaranteed".
- One next step must be to call the National Employment Authority on the hotline given.
- Output JSON only.`

// Template is the no-model explanation. It is also what we show if the model
// call fails, so it must always be complete and correct.
func Template(findings []domain.Finding, v domain.Verdict) domain.Explanation {
	var top, topSW []string
	for _, sev := range []domain.Severity{domain.SevCritical, domain.SevHigh, domain.SevMedium} {
		for _, f := range findings {
			if f.Severity == sev && len(top) < 3 {
				top = append(top, strings.ToLower(f.TitleEN[:1])+f.TitleEN[1:])
				topSW = append(topSW, strings.ToLower(f.TitleSW[:1])+f.TitleSW[1:])
			}
		}
	}
	reasons := strings.Join(top, "; ")
	reasonsSW := strings.Join(topSW, "; ")

	var e domain.Explanation
	switch v {
	case domain.VerdictDanger:
		e.SummaryEN = fmt.Sprintf("Do not pay any money or send your documents. This offer shows serious warning signs: %s.", reasons)
		e.SummarySW = fmt.Sprintf("Usilipe pesa yoyote wala kutuma hati zako. Ofa hii ina dalili nzito za hatari: %s.", reasonsSW)
		e.NextStepsEN = []string{"Stop all payments to this recruiter.", "Do not hand over your passport, ID or certificates.", "Report the ad to NEA through the “Report a fake recruitment agency” option on NEAIMS."}
		e.NextStepsSW = []string{"Acha kufanya malipo yoyote kwa wakala huyu.", "Usikabidhi pasipoti, kitambulisho au vyeti vyako.", "Ripoti tangazo hili kwa NEA kupitia sehemu ya “Report a fake recruitment agency” kwenye NEAIMS."}
	case domain.VerdictCaution:
		e.SummaryEN = fmt.Sprintf("Be careful. We could not confirm this offer, and we found warning signs: %s.", reasons)
		e.SummarySW = fmt.Sprintf("Kuwa mwangalifu. Hatukuweza kuthibitisha ofa hii, na tumepata dalili za tahadhari: %s.", reasonsSW)
		e.NextStepsEN = []string{"Do not pay anything until every point above is answered.", "Check the agency's exact name and number on the NEA list yourself.", "Ask for the employer's details and a written contract."}
		e.NextStepsSW = []string{"Usilipe chochote hadi kila jambo hapo juu lijibiwe.", "Thibitisha mwenyewe jina kamili na nambari ya wakala kwenye orodha ya NEA.", "Omba maelezo ya mwajiri na mkataba ulioandikwa."}
	default:
		e.SummaryEN = "We did not find red flags in what you shared, and the agency appears on the NEA list. That does not prove this specific job is real — confirm it directly with the agency using the contacts on the NEA list."
		e.SummarySW = "Hatukupata dalili za hatari katika ulichotuma, na wakala yupo kwenye orodha ya NEA. Hii haithibitishi kuwa kazi hii mahususi ni ya kweli — ithibitishe moja kwa moja na wakala ukitumia mawasiliano yaliyo kwenye orodha ya NEA."
		e.NextStepsEN = []string{"Call the agency using the contacts on the NEA list, not the ones in the ad.", "Make sure your contract is attested before you buy a ticket."}
		e.NextStepsSW = []string{"Piga simu kwa wakala ukitumia mawasiliano yaliyo kwenye orodha ya NEA, si yaliyo kwenye tangazo.", "Hakikisha mkataba wako umethibitishwa kabla ya kununua tiketi."}
	}
	e.NextStepsEN = append(e.NextStepsEN, fmt.Sprintf("If in doubt, call the National Employment Authority on %s.", rules.NEAHotline))
	e.NextStepsSW = append(e.NextStepsSW, fmt.Sprintf("Ukiwa na shaka, piga simu NEA kupitia %s.", rules.NEAHotline))
	return e
}

func containsHotline(steps []string) bool {
	h := strings.ReplaceAll(rules.NEAHotline, " ", "")
	for _, s := range steps {
		if strings.Contains(strings.ReplaceAll(s, " ", ""), h) {
			return true
		}
	}
	return false
}

func (e *Explainer) budget() int {
	if e.MaxTokens > 0 {
		return e.MaxTokens
	}
	return 6000
}

// parseExplanation accepts the shapes models actually return: the object we
// asked for, the same object wrapped in another ({"explanation": {...}}),
// and next steps written as objects ({"step": "..."}) instead of strings.
func parseExplanation(text string) (domain.Explanation, bool) {
	var raw map[string]any
	if err := llm.DecodeJSON(text, &raw); err != nil {
		return domain.Explanation{}, false
	}
	obj := findObjectWith(raw, "summary_en", 3)
	if obj == nil {
		return domain.Explanation{}, false
	}
	out := domain.Explanation{
		SummaryEN:   asText(obj["summary_en"]),
		SummarySW:   asText(obj["summary_sw"]),
		NextStepsEN: asList(obj["next_steps_en"]),
		NextStepsSW: asList(obj["next_steps_sw"]),
	}
	if out.SummaryEN == "" {
		return domain.Explanation{}, false
	}
	return out, true
}

// findObjectWith returns the first object (breadth-first, up to depth) that
// has the given key.
func findObjectWith(m map[string]any, key string, depth int) map[string]any {
	if _, ok := m[key]; ok {
		return m
	}
	if depth == 0 {
		return nil
	}
	for _, v := range m {
		if child, ok := v.(map[string]any); ok {
			if found := findObjectWith(child, key, depth-1); found != nil {
				return found
			}
		}
	}
	return nil
}

func asText(v any) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case []any: // a summary split into sentences
		var parts []string
		for _, x := range t {
			if s := asText(x); s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, " ")
	case map[string]any: // {"text": "..."} or similar: take the first string field
		for _, k := range []string{"text", "step", "action", "description", "content"} {
			if s, ok := t[k].(string); ok && strings.TrimSpace(s) != "" {
				return strings.TrimSpace(s)
			}
		}
		for _, x := range t {
			if s, ok := x.(string); ok && strings.TrimSpace(s) != "" {
				return strings.TrimSpace(s)
			}
		}
	}
	return ""
}

func asList(v any) []string {
	switch t := v.(type) {
	case []any:
		var out []string
		for _, x := range t {
			if s := asText(x); s != "" {
				out = append(out, s)
			}
		}
		return out
	case string:
		if s := strings.TrimSpace(t); s != "" {
			return []string{s}
		}
	}
	return nil
}

func snippet(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
