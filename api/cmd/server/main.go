// Command server runs the Kazi Halali API.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/edkariuki/kazi-halali/api/internal/benchmark"
	"github.com/edkariuki/kazi-halali/api/internal/config"
	"github.com/edkariuki/kazi-halali/api/internal/extract"
	"github.com/edkariuki/kazi-halali/api/internal/httpapi"
	"github.com/edkariuki/kazi-halali/api/internal/llm"
	"github.com/edkariuki/kazi-halali/api/internal/pipeline"
	"github.com/edkariuki/kazi-halali/api/internal/registry"
	"github.com/edkariuki/kazi-halali/api/internal/search"
	"github.com/edkariuki/kazi-halali/api/internal/store"
	"github.com/edkariuki/kazi-halali/api/internal/verdict"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	st, err := openStore(ctx, cfg)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer st.Close()

	agencies, flagged, err := st.LoadRegistry(ctx)
	if err != nil {
		log.Fatalf("load registry: %v", err)
	}
	reg := registry.New(agencies, flagged)
	if len(agencies) == 0 {
		log.Printf("WARNING: agency registry is empty — run `make ingest` (or use STORE=memory)")
	}
	log.Printf("registry: %d agencies, %d flagged entries", len(agencies), len(flagged))
	if info := reg.Info(); !info.PublishedAt.IsZero() || !info.SnapshotAt.IsZero() {
		log.Printf("register published %s, snapshot taken %s",
			info.PublishedAt.Format("2006-01-02"), info.SnapshotAt.Format("2006-01-02"))
		if age := time.Since(info.SnapshotAt); !info.SnapshotAt.IsZero() && age > 14*24*time.Hour {
			log.Printf("WARNING: register snapshot is %d days old — re-export it (see scripts/export-nea-register.js)", int(age.Hours()/24))
		}
	}

	var client llm.Client
	if cfg.LLMMode != "stub" {
		hc := llm.NewHTTPClient(cfg.LLMBaseURL, cfg.LLMAPIKey, time.Duration(cfg.LLMTimeout)*time.Second)
		hc.DefaultExtra = cfg.ExtraBody
		if len(cfg.ExtraBody) > 0 {
			log.Printf("LLM_EXTRA_BODY applied to every model request: %v", cfg.ExtraBody)
		}
		client = hc
	} else {
		log.Printf("LLM_MODE=stub: heuristic extraction + template explanations, no model calls")
	}

	var searcher search.Searcher
	var official pipeline.OfficialSearcher
	var bench *benchmark.Benchmarker
	if cfg.TavilyAPIKey != "" {
		tv := search.NewTavily(cfg.TavilyAPIKey)
		searcher, official = tv, tv
		if client != nil {
			bench = &benchmark.Benchmarker{Search: tv, Client: client, Model: cfg.ModelExtract, MaxTokens: cfg.MaxTokens}
		}
	} else {
		log.Printf("TAVILY_API_KEY not set: web reports check disabled")
	}

	p := &pipeline.Pipeline{
		Extractor: &extract.Extractor{Client: client, VisionModel: cfg.ModelVision, ExtractModel: cfg.ModelExtract, MaxTokens: cfg.MaxTokens},
		Registry:  func() *registry.Registry { return reg },
		Searcher:  searcher,
		Official:  official,
		Benchmark: bench,
		Explainer: &verdict.Explainer{Client: client, Model: cfg.ModelReason, MaxTokens: cfg.MaxTokens},
		Saver:     st,
	}

	srv := &httpapi.Server{
		Pipeline:   p,
		Store:      st,
		Registry:   func() *registry.Registry { return reg },
		CORSOrigin: cfg.CORSOrigin,
		Info: map[string]any{
			"llm_mode": modeName(cfg.LLMMode),
			"models": map[string]string{
				"vision":  cfg.ModelVision,
				"extract": cfg.ModelExtract,
				"reason":  cfg.ModelReason,
			},
			"web_search": searcher != nil,
			"store":      cfg.Store,
		},
	}

	httpSrv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      150 * time.Second, // vision + extraction + search + explanation
	}
	go func() {
		log.Printf("listening on :%s", cfg.Port)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shutdownCtx)
}

func openStore(ctx context.Context, cfg config.Config) (store.Store, error) {
	if cfg.Store != "memory" {
		return store.NewPostgres(ctx, cfg.DatabaseURL)
	}
	m := store.NewMemory()
	as, err := store.ReadAgenciesCSV(cfg.AgenciesCSV)
	if err != nil {
		return nil, err
	}
	fs, err := store.ReadFlaggedCSV(cfg.FlaggedCSV)
	if err != nil {
		return nil, err
	}
	_, _ = m.UpsertAgencies(ctx, as)
	_, _ = m.UpsertFlagged(ctx, fs)
	return m, nil
}

func modeName(m string) string {
	if m == "stub" {
		return "stub"
	}
	return "live"
}
