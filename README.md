# Kazi Halali

**Check an overseas job offer before you pay.**

Young Kenyans looking for work abroad are targeted by fake recruiters: "processing fees" for jobs that don't exist, forged NEA certificates, visitor-visa traps, and recruitment into war zones. The Kenyan government has deregistered hundreds of rogue agencies, and the Labour CS's advice is simple — check the agency on the National Employment Authority (NEA) list before you engage. Few people do, and fewer know the other rules.

Kazi Halali makes that check take ten seconds. Paste the WhatsApp forward or upload a screenshot of the ad, offer letter or "NEA certificate". You get a clear verdict — **Do not pay / Be careful / No red flags found** — in English and Kiswahili, with every reason linked to its source, and a link you can share back to the group chat.

> Built for the Nebius × NVIDIA Global AI Hackathon (Best Apps and Agents track).

---

## How it works

```
Ad text / screenshot / offer letter / certificate photo
        │
        ▼
 Nemotron Nano VL  ── reads the image (text + visual notes: stamps, logos, edits)
 Nemotron Nano     ── extracts structured facts as JSON:
                      agency, licence no., destination, job type, salary,
                      fees, visa type, contact channels, pressure cues
        │
        ▼
 Registry matcher  ── NEA licensed list: exact / licence-number / lookalike name
                      + official "flagged / not registered" notices
 Tavily search     ── public fraud reports about the agency or phone number
        │
        ▼
 Rules engine (Go) ── 12 deterministic, source-cited rules  ──► verdict
        │
        ▼
 Nemotron Super    ── plain-language explanation + next steps (EN + SW)
```

### The key design decision: models read, rules decide

The language models never choose the verdict. They do what they're good at — reading messy ads and screenshots in English, Kiswahili and Sheng, and explaining results in plain language. The verdict comes from a **deterministic rules engine** where every rule cites an official or reported source. That means:

- verdicts are reproducible and unit-tested;
- a model can't hallucinate a red flag or talk its way out of one;
- every claim shown to a user links to where it comes from.

If any model call fails, the pipeline falls back to a regex extractor and template explanations, so a user always gets an answer.

### The rules

| Rule | Severity | Based on |
|---|---|---|
| Agency named in an official warning | critical | Ministry / NEA notices |
| Agency not on the NEA list / licence expired | critical | NEAIMS agency list |
| Licence number belongs to a different agency (incl. lookalike name + borrowed number) | critical | Reported forged NEA certificates |
| Fee charged for an unskilled or domestic job | critical | NEA: no fees for unskilled jobs |
| Visitor / tourist visa for work | critical | Labour CS warning |
| Russia / military-style "jobs" | critical | Government warnings on war recruitment |
| Name looks like a licensed agency but isn't | high | Known impersonation tactic |
| Licence under verification / number not on register | high | NEAIMS |
| Money requested before a contract exists | high | Ministry investigations |
| Linked to fraud reports online (Tavily) | high | Live search |
| Destination matches known Asia scam pattern | high | Government advisory |
| WhatsApp / social-only contact, no office | medium | NEA agency requirements |
| Pressure to decide or pay quickly | medium | Government advisory |
| Contract must be attested before travel | reminder | Ministry of Labour |

Sources are listed in [`api/internal/rules/sources.go`](api/internal/rules/sources.go).

---

## How we use NVIDIA Nemotron and Nebius Token Factory

All model calls go through **Nebius Token Factory**'s OpenAI-compatible API. Each step uses the smallest model that does the job well, so the app stays fast and cheap enough to offer free to job seekers:

| Step | Model | Why this model |
|---|---|---|
| Read screenshots / certificate photos | `Nemotron Nano VL` | Vision-language; transcribes ads and describes stamps/edits on certificates |
| Extract structured facts | `Nemotron 3 Nano` | High-volume, low-latency JSON extraction |
| Explain the result (EN + SW) | `Nemotron 3 Super` | Better reasoning and Kiswahili writing for the user-facing summary |

Every result records which model ran each step, with latency and token counts. You can see this under **How this check was done** on any result page.

Model IDs are set in `.env` (`MODEL_VISION`, `MODEL_EXTRACT`, `MODEL_REASON`).

**Tavily** powers the "linked to fraud reports online" rule: it searches for the agency name and phone number alongside scam/fraud terms, and only reports hits that mention both.

---

## Run it

Requirements: Go 1.22+, Node 20+, Docker (for Postgres).

```bash
cp .env.example .env          # add LLM_API_KEY and TAVILY_API_KEY
make deps                     # go mod tidy + npm install
make db                       # Postgres; schema in api/migrations is applied on first start
make ingest                   # load the registry CSVs
make api                      # API on :8080
make web                      # web app on :3000 (in another terminal)
```

**No credentials yet?** Run everything offline with the sample registry:

```bash
cd api && STORE=memory LLM_MODE=stub go run ./cmd/server
cd web && npm run dev
```

Stub mode uses the heuristic extractor and template explanations: no model calls, no database.

### Tests and evaluation

```bash
make test        # unit tests: rules, registry matching, extraction, pipeline
make eval        # runs eval/cases against the running API
```

Current eval (sample registry, stub mode):

```
Scams flagged (danger or caution): 5/5
Scams marked danger:               4/5
Genuine offers marked danger:      0/1
```

### API

| Method | Path | Body |
|---|---|---|
| `POST` | `/api/checks` | JSON `{"text": "..."}` or multipart `text` + `image` |
| `GET` | `/api/checks/{id}` | — |
| `GET` | `/api/stats` | — |
| `GET` | `/api/health` | — |

---

## Data

- `data/sample/` holds **fictional** agencies so the app runs end to end. None of the names refer to real companies.
- The real registry snapshot goes in `data/real/` (git-ignored). See [`data/README.md`](data/README.md).
- Only names that appear in an official notice go into the flagged list, always with the source link.

## Limits

Kazi Halali points out warning signs; it cannot prove a job is genuine. A clean result still tells the user to confirm the vacancy with the agency using the contacts on the NEA list, and to have the contract attested before travelling.

## Project layout

```
api/                Go API (standard library + pgx)
  cmd/server        HTTP server
  cmd/ingest        load registry CSVs into Postgres
  cmd/eval          run eval cases against the API
  internal/extract  vision + JSON extraction (+ heuristic fallback)
  internal/registry NEA list matching, lookalike detection
  internal/rules    the deterministic rules engine
  internal/verdict  verdict + explanation
  internal/search   Tavily client
  migrations/       Postgres schema
web/                Next.js + Tailwind frontend
data/               registry CSVs
eval/cases/         evaluation cases
```

## License

MIT — see [LICENSE](LICENSE).
