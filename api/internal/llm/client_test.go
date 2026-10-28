package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestDecodeJSONToleratesFencesAndProse(t *testing.T) {
	var v struct {
		A string `json:"a"`
		B []int  `json:"b"`
	}
	in := "Sure! Here you go:\n```json\n{\"a\": \"x}y\", \"b\": [1,2]}\n```\nAnything else?"
	if err := DecodeJSON(in, &v); err != nil || v.A != "x}y" || len(v.B) != 2 {
		t.Fatalf("got %+v err=%v", v, err)
	}
}

func TestStripThinking(t *testing.T) {
	if got := StripThinking("<think>hmm</think>\n{\"ok\":true}"); got != `{"ok":true}` {
		t.Fatalf("got %q", got)
	}
}

func TestMultimodalMarshal(t *testing.T) {
	m := Message{Role: "user", Parts: []Part{{Type: "text", Text: "hi"}, {Type: "image_url", ImageURL: &ImageURL{URL: "data:x"}}}}
	b, _ := json.Marshal(m)
	want := `{"role":"user","content":[{"type":"text","text":"hi"},{"type":"image_url","image_url":{"url":"data:x"}}]}`
	if string(b) != want {
		t.Fatalf("got %s", b)
	}
}

func TestDefaultExtraMergedAndOverridable(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, "k", 0)
	c.DefaultExtra = map[string]any{"reasoning_budget": 512, "keep": true}
	_, err := c.Chat(context.Background(), Request{Model: "m", Extra: map[string]any{"reasoning_budget": 64}})
	if err != nil {
		t.Fatal(err)
	}
	if got["keep"] != true || got["reasoning_budget"] != float64(64) {
		t.Fatalf("body = %v", got)
	}
}

func TestRetriesBusyThenSucceeds(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":"ResourceExhausted"}`))
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, "k", 0)
	c.Backoff = time.Millisecond
	resp, err := c.Chat(context.Background(), Request{Model: "m"})
	if err != nil || resp.Text != "ok" || calls.Load() != 3 {
		t.Fatalf("resp=%q err=%v calls=%d", resp.Text, err, calls.Load())
	}
}

func TestGivesUpAfterRetries(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, "k", 0)
	c.Backoff = time.Millisecond
	if _, err := c.Chat(context.Background(), Request{Model: "m"}); err == nil {
		t.Fatal("expected an error")
	}
	if calls.Load() != 3 {
		t.Fatalf("calls = %d, want 3 (1 + 2 retries)", calls.Load())
	}
}

func TestBadRequestNotRetried(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, "k", 0)
	_, _ = c.Chat(context.Background(), Request{Model: "m"})
	if calls.Load() != 1 {
		t.Fatalf("calls = %d, want 1", calls.Load())
	}
}
