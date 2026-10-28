// Package llm is a minimal client for OpenAI-compatible chat completion APIs
// (Nebius Token Factory, NVIDIA API catalog, local NIM, vLLM...).
// Standard library only.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type Part struct {
	Type     string    `json:"type"` // "text" | "image_url"
	Text     string    `json:"text,omitempty"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
}

type ImageURL struct {
	URL string `json:"url"` // https://... or data:image/png;base64,...
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"-"`
	Parts   []Part `json:"-"`
}

// MarshalJSON emits content as a string, or as a parts array for multimodal input.
func (m Message) MarshalJSON() ([]byte, error) {
	if len(m.Parts) > 0 {
		return json.Marshal(struct {
			Role    string `json:"role"`
			Content []Part `json:"content"`
		}{m.Role, m.Parts})
	}
	return json.Marshal(struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}{m.Role, m.Content})
}

type Request struct {
	Model       string
	Messages    []Message
	Temperature float64
	MaxTokens   int
	JSON        bool           // ask for a JSON object response
	Extra       map[string]any // provider-specific fields merged into the body
}

type Response struct {
	Text         string
	InputTokens  int
	OutputTokens int
	Latency      time.Duration
}

type Client interface {
	Chat(ctx context.Context, req Request) (Response, error)
}

// HTTPClient talks to an OpenAI-compatible /chat/completions endpoint.
type HTTPClient struct {
	BaseURL string
	APIKey  string
	HTTP    *http.Client
	// DefaultExtra is merged into every request body (per-request Extra wins).
	// Set from LLM_EXTRA_BODY, e.g. to switch a reasoning model's thinking off:
	//   {"chat_template_kwargs":{"enable_thinking":false}}
	DefaultExtra map[string]any
	// Retries is how many times a busy response (429/502/503/504) is retried.
	Retries int
	// Backoff is the wait before the first retry; it grows linearly. 0 = 1.5s.
	Backoff time.Duration
}

// NewHTTPClient creates a client. timeout caps each HTTP attempt; a hung
// call is abandoned after it so the pipeline can fall back promptly.
func NewHTTPClient(baseURL, apiKey string, timeout time.Duration) *HTTPClient {
	if timeout <= 0 {
		timeout = 40 * time.Second
	}
	return &HTTPClient{
		BaseURL: strings.TrimRight(baseURL, "/"),
		APIKey:  apiKey,
		HTTP:    &http.Client{Timeout: timeout},
		Retries: 2,
	}
}

func (c *HTTPClient) post(ctx context.Context, buf []byte) ([]byte, int, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/chat/completions", bytes.NewReader(buf))
	if err != nil {
		return nil, 0, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	res, err := c.HTTP.Do(httpReq)
	if err != nil {
		return nil, 0, err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	return raw, res.StatusCode, nil
}

func retryable(status int) bool {
	return status == http.StatusTooManyRequests || status == http.StatusBadGateway ||
		status == http.StatusServiceUnavailable || status == http.StatusGatewayTimeout
}

func (c *HTTPClient) retries() int {
	if c.Retries < 0 {
		return 0
	}
	return c.Retries
}

func (c *HTTPClient) backoff() time.Duration {
	if c.Backoff > 0 {
		return c.Backoff
	}
	return 1500 * time.Millisecond
}

func (c *HTTPClient) Chat(ctx context.Context, req Request) (Response, error) {
	body := map[string]any{
		"model":    req.Model,
		"messages": req.Messages,
	}
	if req.Temperature >= 0 {
		body["temperature"] = req.Temperature
	}
	if req.MaxTokens > 0 {
		body["max_tokens"] = req.MaxTokens
	}
	if req.JSON {
		body["response_format"] = map[string]string{"type": "json_object"}
	}
	for k, v := range c.DefaultExtra {
		body[k] = v
	}
	for k, v := range req.Extra {
		body[k] = v
	}
	buf, err := json.Marshal(body)
	if err != nil {
		return Response{}, err
	}

	start := time.Now()
	var raw []byte
	var status int
	for attempt := 0; ; attempt++ {
		raw, status, err = c.post(ctx, buf)
		if err == nil && !retryable(status) {
			break
		}
		if attempt >= c.retries() || err != nil {
			// Network errors and timeouts are not retried: the caller has a
			// fallback, and waiting twice is worse than falling back once.
			break
		}
		// 429 / 5xx: the shared endpoint is busy. Back off briefly and retry.
		select {
		case <-ctx.Done():
			return Response{Latency: time.Since(start)}, ctx.Err()
		case <-time.After(time.Duration(attempt+1) * c.backoff()):
		}
	}
	latency := time.Since(start)
	if err != nil {
		return Response{Latency: latency}, err
	}
	if status >= 300 {
		return Response{Latency: latency}, fmt.Errorf("llm %s: HTTP %d: %s", req.Model, status, truncate(string(raw), 400))
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return Response{Latency: latency}, fmt.Errorf("llm %s: bad response: %w", req.Model, err)
	}
	if len(parsed.Choices) == 0 {
		return Response{Latency: latency}, errors.New("llm: no choices returned")
	}
	return Response{
		Text:         StripThinking(parsed.Choices[0].Message.Content),
		InputTokens:  parsed.Usage.PromptTokens,
		OutputTokens: parsed.Usage.CompletionTokens,
		Latency:      latency,
	}, nil
}

var thinkRe = regexp.MustCompile(`(?s)<think>.*?</think>`)

// StripThinking removes reasoning traces some Nemotron variants emit inline.
func StripThinking(s string) string {
	s = thinkRe.ReplaceAllString(s, "")
	// An unclosed <think> means the answer never started; keep what follows it if any.
	if i := strings.Index(s, "</think>"); i >= 0 {
		s = s[i+len("</think>"):]
	}
	return strings.TrimSpace(s)
}

// DecodeJSON finds the first JSON object in s (tolerating code fences and
// surrounding prose) and unmarshals it into v.
func DecodeJSON(s string, v any) error {
	start := strings.Index(s, "{")
	if start < 0 {
		return errors.New("no JSON object in model output")
	}
	depth, inStr, esc := 0, false, false
	for i := start; i < len(s); i++ {
		ch := s[i]
		switch {
		case esc:
			esc = false
		case ch == '\\' && inStr:
			esc = true
		case ch == '"':
			inStr = !inStr
		case inStr:
		case ch == '{':
			depth++
		case ch == '}':
			depth--
			if depth == 0 {
				return json.Unmarshal([]byte(s[start:i+1]), v)
			}
		}
	}
	return errors.New("unterminated JSON object in model output")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
