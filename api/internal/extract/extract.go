// Package extract turns a job ad, offer letter, chat screenshot or
// certificate photo into structured Facts.
//
//	image  ──► vision model (transcribe)  ──┐
//	text   ─────────────────────────────────┴─► extract model (JSON) ──► Facts
//
// If a model call fails (or LLM_MODE=stub), a deterministic heuristic
// extractor takes over so the pipeline always produces a result.
package extract

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/edkariuki/kazi-halali/api/internal/domain"
	"github.com/edkariuki/kazi-halali/api/internal/llm"
)

type Extractor struct {
	Client       llm.Client // nil = heuristic only
	VisionModel  string
	ExtractModel string
	// MaxTokens caps each call. Reasoning models spend tokens thinking
	// before they answer, so this must leave room for both. 0 = 6000.
	MaxTokens int
}

func (e *Extractor) budget() int {
	if e.MaxTokens > 0 {
		return e.MaxTokens
	}
	return 6000
}

type Image struct {
	Bytes    []byte
	MimeType string // image/png, image/jpeg, image/webp
}

type Result struct {
	Transcript string
	Facts      domain.Facts
	Trace      []domain.ModelCall
}

func (e *Extractor) Run(ctx context.Context, text string, img *Image) Result {
	var res Result
	combined := strings.TrimSpace(text)

	if img != nil && len(img.Bytes) > 0 {
		transcript, call := e.transcribe(ctx, img)
		res.Trace = append(res.Trace, call)
		res.Transcript = transcript
		if transcript != "" {
			if combined != "" {
				combined += "\n\n--- Text read from the uploaded image ---\n"
			}
			combined += transcript
		}
	}

	facts, call := e.extract(ctx, combined)
	if call != nil {
		res.Trace = append(res.Trace, *call)
	}
	res.Facts = facts
	return res
}

func (e *Extractor) transcribe(ctx context.Context, img *Image) (string, domain.ModelCall) {
	call := domain.ModelCall{Step: "vision", Model: e.VisionModel}
	if e.Client == nil {
		call.Error = "vision skipped (offline mode)"
		return "", call
	}
	dataURL := fmt.Sprintf("data:%s;base64,%s", img.MimeType, base64.StdEncoding.EncodeToString(img.Bytes))
	resp, err := e.Client.Chat(ctx, llm.Request{
		Model:       e.VisionModel,
		Temperature: 0,
		MaxTokens:   e.budget(),
		Messages: []llm.Message{
			{Role: "system", Content: visionSystemPrompt},
			{Role: "user", Parts: []llm.Part{
				{Type: "text", Text: "Transcribe this image."},
				{Type: "image_url", ImageURL: &llm.ImageURL{URL: dataURL}},
			}},
		},
	})
	fill(&call, resp, err)
	if err != nil {
		return "", call
	}
	return strings.TrimSpace(resp.Text), call
}

func (e *Extractor) extract(ctx context.Context, text string) (domain.Facts, *domain.ModelCall) {
	if text == "" {
		return domain.Facts{JobCategory: domain.JobUnknown, VisaType: domain.VisaUnknown}, nil
	}
	call := domain.ModelCall{Step: "extract", Model: e.ExtractModel}
	if e.Client == nil {
		call.Model = "heuristic"
		return Heuristic(text), &call
	}

	resp, err := e.Client.Chat(ctx, llm.Request{
		Model:       e.ExtractModel,
		Temperature: 0,
		MaxTokens:   e.budget(),
		JSON:        true,
		Messages: []llm.Message{
			{Role: "system", Content: extractSystemPrompt},
			{Role: "user", Content: "Extract the facts from this material:\n\n<<<\n" + text + "\n>>>"},
		},
	})
	fill(&call, resp, err)
	if err != nil {
		call.Error += " — fell back to heuristic extractor"
		return Heuristic(text), &call
	}

	var f domain.Facts
	if err := llm.DecodeJSON(resp.Text, &f); err != nil {
		call.Error = "unparseable JSON — fell back to heuristic extractor"
		return Heuristic(text), &call
	}
	normalize(&f)
	// The model sometimes misses contact details that regexes catch reliably.
	h := Heuristic(text)
	f.Phones = union(f.Phones, h.Phones)
	f.Emails = union(f.Emails, h.Emails)
	f.Websites = union(f.Websites, h.Websites)
	if f.AgencyLicenceNo == "" {
		f.AgencyLicenceNo = h.AgencyLicenceNo
	}
	// A missing job title makes the salary check search for something vague
	// ("general labourer"), so backfill it from the ad text.
	if strings.TrimSpace(f.JobTitle) == "" {
		f.JobTitle = h.JobTitle
	}
	if f.JobCategory == domain.JobUnknown && h.JobCategory != domain.JobUnknown {
		f.JobCategory = h.JobCategory
	}
	return f, &call
}

func fill(call *domain.ModelCall, resp llm.Response, err error) {
	call.LatencyMS = resp.Latency.Milliseconds()
	call.InputTok = resp.InputTokens
	call.OutputTok = resp.OutputTokens
	if err != nil {
		call.Error = err.Error()
	}
}

func normalize(f *domain.Facts) {
	f.AgencyName = strings.TrimSpace(f.AgencyName)
	f.AgencyLicenceNo = strings.ToUpper(strings.TrimSpace(f.AgencyLicenceNo))
	switch f.JobCategory {
	case domain.JobSkilled, domain.JobSemiSkilled, domain.JobUnskilled, domain.JobDomestic, domain.JobMilitary:
	default:
		f.JobCategory = domain.JobUnknown
	}
	switch f.VisaType {
	case domain.VisaWork, domain.VisaVisitor, domain.VisaStudent:
	default:
		f.VisaType = domain.VisaUnknown
	}
	for i := range f.ContactChannels {
		f.ContactChannels[i] = strings.ToLower(strings.TrimSpace(f.ContactChannels[i]))
	}
}

func union(a, b []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range append(append([]string{}, a...), b...) {
		k := strings.ToLower(strings.TrimSpace(s))
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, strings.TrimSpace(s))
	}
	return out
}
