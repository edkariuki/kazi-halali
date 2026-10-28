// Package httpapi exposes the check pipeline over HTTP.
//
//	GET  /api/health
//	POST /api/checks        JSON {"text": "..."} or multipart (text, image)
//	GET  /api/checks/{id}
//	GET  /api/stats
package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/edkariuki/kazi-halali/api/internal/extract"
	"github.com/edkariuki/kazi-halali/api/internal/pipeline"
	"github.com/edkariuki/kazi-halali/api/internal/registry"
	"github.com/edkariuki/kazi-halali/api/internal/store"
)

const (
	maxTextChars  = 8000
	maxImageBytes = 8 << 20
)

type Server struct {
	Pipeline   *pipeline.Pipeline
	Store      store.Store
	Registry   func() *registry.Registry
	CORSOrigin string
	Info       map[string]any // shown on /api/health (models, mode)
	limiter    *limiter
}

func (s *Server) Handler() http.Handler {
	s.limiter = newLimiter(30, time.Hour) // per IP: protects model credits during the demo
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.health)
	mux.HandleFunc("POST /api/checks", s.createCheck)
	mux.HandleFunc("GET /api/checks/{id}", s.getCheck)
	mux.HandleFunc("GET /api/stats", s.stats)
	return s.cors(logRequests(mux))
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	reg := s.Registry()
	_, f := reg.Size()
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":       true,
		"registry": map[string]any{"register": reg.Info(), "flagged": f},
		"info":     s.Info,
	})
}

func (s *Server) createCheck(w http.ResponseWriter, r *http.Request) {
	if !s.limiter.allow(clientIP(r)) {
		writeErr(w, http.StatusTooManyRequests, "Too many checks from this network. Please try again later.")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxImageBytes+maxTextChars*4+1<<16)

	var text string
	var img *extract.Image

	ct := r.Header.Get("Content-Type")
	switch {
	case strings.HasPrefix(ct, "multipart/form-data"):
		if err := r.ParseMultipartForm(maxImageBytes); err != nil {
			writeErr(w, http.StatusBadRequest, "Could not read the upload (max 8 MB).")
			return
		}
		text = r.FormValue("text")
		if file, _, err := r.FormFile("image"); err == nil {
			defer file.Close()
			b, err := io.ReadAll(io.LimitReader(file, maxImageBytes+1))
			if err != nil || len(b) > maxImageBytes {
				writeErr(w, http.StatusBadRequest, "Image is too large (max 8 MB).")
				return
			}
			mt := http.DetectContentType(b)
			if mt != "image/png" && mt != "image/jpeg" && mt != "image/webp" && mt != "image/gif" {
				writeErr(w, http.StatusBadRequest, "Please upload a PNG, JPEG or WebP image.")
				return
			}
			img = &extract.Image{Bytes: b, MimeType: mt}
		}
	default:
		var body struct {
			Text string `json:"text"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, http.StatusBadRequest, "Send JSON {\"text\": \"...\"} or a multipart form.")
			return
		}
		text = body.Text
	}

	text = strings.TrimSpace(text)
	if text == "" && img == nil {
		writeErr(w, http.StatusBadRequest, "Paste the job ad or upload a screenshot.")
		return
	}
	if len([]rune(text)) > maxTextChars {
		writeErr(w, http.StatusBadRequest, "Text is too long (max 8,000 characters).")
		return
	}

	check, err := s.Pipeline.Run(r.Context(), text, img)
	if err != nil {
		// The check itself completed; only saving failed. Still return it.
		log.Printf("save check %s: %v", check.ID, err)
	}
	writeJSON(w, http.StatusCreated, check)
}

func (s *Server) getCheck(w http.ResponseWriter, r *http.Request) {
	c, err := s.Store.GetCheck(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "Check not found.")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Could not load this check.")
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) stats(w http.ResponseWriter, r *http.Request) {
	st, err := s.Store.Stats(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Could not load stats.")
		return
	}
	writeJSON(w, http.StatusOK, st)
}

// ---------------------------------------------------------------------------

func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && allowedOrigin(s.CORSOrigin, origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// allowedOrigin checks origin against CORS_ORIGIN: "*", one origin, or a
// comma-separated list ("https://a.app,https://b.app"). A leading "*." on a
// list entry allows subdomains ("https://*.vercel.app" for preview builds).
func allowedOrigin(config, origin string) bool {
	origin = strings.TrimRight(origin, "/")
	for _, o := range strings.Split(config, ",") {
		o = strings.TrimRight(strings.TrimSpace(o), "/")
		switch {
		case o == "":
		case o == "*" || o == origin:
			return true
		case strings.Contains(o, "://*."):
			scheme, suffix, _ := strings.Cut(o, "*.")
			if strings.HasPrefix(origin, scheme) && strings.HasSuffix(origin, "."+suffix) {
				return true
			}
		}
	}
	return false
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func clientIP(r *http.Request) string {
	if xf := r.Header.Get("X-Forwarded-For"); xf != "" {
		return strings.TrimSpace(strings.Split(xf, ",")[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// limiter is a fixed-window per-key counter. Good enough for a demo.
type limiter struct {
	mu     sync.Mutex
	max    int
	window time.Duration
	hits   map[string][]time.Time
}

func newLimiter(max int, window time.Duration) *limiter {
	return &limiter{max: max, window: window, hits: map[string][]time.Time{}}
}

func (l *limiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	cut := now.Add(-l.window)
	kept := l.hits[key][:0]
	for _, t := range l.hits[key] {
		if t.After(cut) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= l.max {
		l.hits[key] = kept
		return false
	}
	l.hits[key] = append(kept, now)
	return true
}
