package search

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/edkariuki/kazi-halali/api/internal/domain"
)

func TestOfficialRestrictsToGovernmentDomains(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(`{"results":[{"title":"Notice","url":"https://labour.go.ke/n","content":"x"}]}`))
	}))
	defer srv.Close()

	tv := NewTavily("k")
	tv.BaseURL = srv.URL
	hits, err := tv.Official(context.Background(), domain.Facts{AgencyName: "Ghost Recruiters Ltd"})
	if err != nil || len(hits) != 1 {
		t.Fatalf("hits=%v err=%v", hits, err)
	}
	domains, _ := body["include_domains"].([]any)
	if len(domains) != len(OfficialDomains) {
		t.Fatalf("include_domains = %v", body["include_domains"])
	}
}

func TestOfficialSkippedWithoutAgency(t *testing.T) {
	tv := NewTavily("k")
	tv.BaseURL = "http://127.0.0.1:1" // must not be called
	if hits, err := tv.Official(context.Background(), domain.Facts{}); hits != nil || err != nil {
		t.Fatalf("hits=%v err=%v", hits, err)
	}
}
