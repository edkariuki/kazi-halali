// Command eval runs every case in eval/cases against a running API and
// reports how many scams were caught and how many genuine offers were
// wrongly flagged.
//
//	go run ./cmd/eval -api http://localhost:8080 -cases ../eval/cases
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Case struct {
	ID              string `json:"id"`
	Label           string `json:"label"` // "scam" | "legit"
	ExpectedVerdict string `json:"expected_verdict"`
	InputText       string `json:"input_text"`
	Source          string `json:"source"`
	Notes           string `json:"notes"`
}

type result struct {
	Case      Case
	Verdict   string
	Rules     []string
	Err       string
	Took      time.Duration
	Extractor string   // model that produced the facts, or "heuristic"
	Fallbacks []string // model steps that failed and fell back
}

func main() {
	api := flag.String("api", "http://localhost:8080", "API base URL")
	dir := flag.String("cases", "../eval/cases", "directory of *.json cases")
	out := flag.String("out", "", "optional path to write a JSON report")
	flag.Parse()

	files, _ := filepath.Glob(filepath.Join(*dir, "*.json"))
	sort.Strings(files)
	if len(files) == 0 {
		log.Fatalf("no cases in %s", *dir)
	}

	client := &http.Client{Timeout: 180 * time.Second}
	var results []result
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			log.Fatal(err)
		}
		var c Case
		if err := json.Unmarshal(raw, &c); err != nil {
			log.Fatalf("%s: %v", f, err)
		}
		results = append(results, run(client, *api, c))
	}

	var scams, caught, strict, legit, falseAlarms, exact int
	fmt.Printf("\n%-32s %-6s %-10s %-10s %-6s %s\n", "case", "label", "expected", "got", "secs", "rules fired")
	fmt.Println(strings.Repeat("-", 110))
	var modelRuns, fallbackRuns int
	for _, r := range results {
		got := r.Verdict
		if r.Err != "" {
			got = "ERROR"
		}
		fmt.Printf("%-32s %-6s %-10s %-10s %-6.1f %s\n", r.Case.ID, r.Case.Label, r.Case.ExpectedVerdict, got, r.Took.Seconds(), strings.Join(r.Rules, ", "))
		if r.Err != "" {
			fmt.Printf("    error: %s\n", r.Err)
		}
		if r.Extractor != "" && r.Extractor != "heuristic" && len(r.Fallbacks) == 0 {
			modelRuns++
		}
		for _, fb := range r.Fallbacks {
			fmt.Printf("    fallback: %s\n", fb)
		}
		if len(r.Fallbacks) > 0 {
			fallbackRuns++
		}
		if got == r.Case.ExpectedVerdict {
			exact++
		}
		switch r.Case.Label {
		case "scam":
			scams++
			if got == "danger" || got == "caution" {
				caught++
			}
			if got == "danger" {
				strict++
			}
		case "legit":
			legit++
			if got == "danger" {
				falseAlarms++
			}
		}
	}
	fmt.Println(strings.Repeat("-", 110))
	fmt.Printf("Cases fully run by models:         %d/%d\n", modelRuns, len(results))
	if fallbackRuns > 0 {
		fmt.Printf("Cases with a model fallback:       %d/%d  (see 'fallback' lines)\n", fallbackRuns, len(results))
	}
	fmt.Printf("Scams flagged (danger or caution): %d/%d\n", caught, scams)
	fmt.Printf("Scams marked danger:               %d/%d\n", strict, scams)
	fmt.Printf("Genuine offers marked danger:      %d/%d\n", falseAlarms, legit)
	fmt.Printf("Exact verdict matches:             %d/%d\n", exact, len(results))

	if *out != "" {
		b, _ := json.MarshalIndent(results, "", "  ")
		if err := os.WriteFile(*out, b, 0o644); err != nil {
			log.Fatal(err)
		}
	}
}

func run(client *http.Client, api string, c Case) result {
	r := result{Case: c}
	body, _ := json.Marshal(map[string]string{"text": c.InputText})
	start := time.Now()
	resp, err := client.Post(strings.TrimRight(api, "/")+"/api/checks", "application/json", bytes.NewReader(body))
	r.Took = time.Since(start)
	if err != nil {
		r.Err = err.Error()
		return r
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		r.Err = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, raw)
		return r
	}
	var parsed struct {
		Trace []struct {
			Step  string `json:"step"`
			Model string `json:"model"`
			Error string `json:"error"`
		} `json:"trace"`
		Verdict  string `json:"verdict"`
		Findings []struct {
			RuleID   string `json:"rule_id"`
			Severity string `json:"severity"`
		} `json:"findings"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		r.Err = err.Error()
		return r
	}
	r.Verdict = parsed.Verdict
	for _, t := range parsed.Trace {
		if t.Step == "extract" {
			r.Extractor = t.Model
		}
		if t.Error != "" && t.Step != "search" {
			r.Fallbacks = append(r.Fallbacks, t.Step+": "+t.Error)
		}
	}
	for _, f := range parsed.Findings {
		if f.Severity != "info" && f.Severity != "pass" {
			r.Rules = append(r.Rules, f.RuleID)
		}
	}
	return r
}
