// Package config reads settings from the environment.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	LLMBaseURL   string
	LLMAPIKey    string
	LLMMode      string // "" for real calls, "stub" for offline canned output
	ModelVision  string
	ModelExtract string
	ModelReason  string
	MaxTokens    int
	LLMTimeout   int            // seconds per model HTTP attempt (LLM_TIMEOUT_SECONDS)
	ExtraBody    map[string]any // LLM_EXTRA_BODY: JSON merged into every model request

	TavilyAPIKey string

	Store       string // postgres | memory
	DatabaseURL string
	AgenciesCSV string
	FlaggedCSV  string

	Port       string
	CORSOrigin string
}

// DefaultExtraBody is merged into every model request unless LLM_EXTRA_BODY is set.
const DefaultExtraBody = `{"chat_template_kwargs":{"enable_thinking":false}}`

func Load() (Config, error) {
	c := Config{
		LLMBaseURL:   env("LLM_BASE_URL", "https://api.tokenfactory.nebius.com/v1"),
		LLMAPIKey:    os.Getenv("LLM_API_KEY"),
		LLMMode:      strings.ToLower(os.Getenv("LLM_MODE")),
		ModelVision:  env("MODEL_VISION", "nvidia/Nemotron-3.5-Super-VL"),
		ModelExtract: env("MODEL_EXTRACT", "nvidia/NVIDIA-Nemotron-3-Nano-30B-A3B"),
		ModelReason:  env("MODEL_REASON", "nvidia/nemotron-3-super-120b-a12b"),
		MaxTokens:    envInt("LLM_MAX_TOKENS", 6000),
		LLMTimeout:   envInt("LLM_TIMEOUT_SECONDS", 40),
		TavilyAPIKey: os.Getenv("TAVILY_API_KEY"),
		Store:        env("STORE", "postgres"),
		DatabaseURL:  os.Getenv("DATABASE_URL"),
		AgenciesCSV:  env("AGENCIES_CSV", "../data/sample/agencies.csv"),
		FlaggedCSV:   env("FLAGGED_CSV", "../data/sample/flagged.csv"),
		Port:         env("PORT", "8080"),
		CORSOrigin:   env("CORS_ORIGIN", "http://localhost:3000"),
	}
	// LLM_EXTRA_BODY unset: switch Nemotron's thinking off (tested on Token
	// Factory: about half the latency, and the VL model returns an empty
	// answer with thinking on). Set it to "" or {} to send nothing extra.
	raw, set := os.LookupEnv("LLM_EXTRA_BODY")
	if !set {
		raw = DefaultExtraBody
	}
	if raw = strings.TrimSpace(raw); raw != "" {
		if err := json.Unmarshal([]byte(raw), &c.ExtraBody); err != nil {
			return c, fmt.Errorf("LLM_EXTRA_BODY is not valid JSON: %w", err)
		}
	}
	if c.LLMMode != "stub" && c.LLMAPIKey == "" {
		return c, fmt.Errorf("LLM_API_KEY is empty (set it, or LLM_MODE=stub for offline dev)")
	}
	if c.Store == "postgres" && c.DatabaseURL == "" {
		return c, fmt.Errorf("DATABASE_URL is empty (set it, or STORE=memory)")
	}
	return c, nil
}

func env(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if n, err := strconv.Atoi(strings.TrimSpace(os.Getenv(key))); err == nil && n > 0 {
		return n
	}
	return def
}
