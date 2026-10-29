# Kazi Halali

**Kazi halali? Test before you pay (Check an overseas job offer before you pay).**

Young Kenyans looking for work abroad are targeted by fake recruiters: "processing fees" for jobs that don't exist, forged NEA certificates, visitor-visa traps, and recruitment into war zones. The Kenyan government has deregistered hundreds of rogue agencies, and the Labour CS's advice is simple — check the agency on the National Employment Authority (NEA) list before you engage. Few people do, and fewer know the other rules.

Kazi Halali makes that check take ten seconds. Paste the WhatsApp forward or upload a screenshot of the ad, offer letter or "NEA certificate". You get a clear verdict — **Do not pay / Be careful / No red flags found** — in English and Kiswahili, with every reason linked to its source, and a link you can share back to the group chat.

> Built solo for the Nebius × NVIDIA Global AI Hackathon (Best Apps and Agents track).

**Live demo:** https://kazi-halali.vercel.app · **Video:** _add YouTube link_ · All model calls run on **NVIDIA Nemotron via Nebius Token Factory**.

---

## How it works

```
Ad text / screenshot / offer letter / certificate photo
        │
        ▼
 Nemotron Super VL ── reads the image (text + visual notes: stamps, logos, edits)
 Nemotron Nano     ── extracts structured facts as JSON:
                      agency, licence no., destination, job type, salary,
                      fees, visa type, contact channels, pressure cues
        │
        ▼
 Registry matcher  ── NEA licensed list: exact / licence-number / lookalike name
                      + official "flagged / not registered" notices
        │
        ├─ in parallel ──────────────────────────────────────────────
        │  Tavily: government sites only ── live official warnings
        │  Tavily: news & web            ── public fraud reports
        │  Tavily + Nemotron Nano        ── salary reality check
        │     (Tavily finds pay data, Nemotron reads the figures,
        │      Go converts currency and computes the ratio)
        │
        ▼
 Rules engine (Go) ── 12 deterministic, source-cited rules  ──► verdict
        │
        ▼
 Nemotron Super    ── plain-language explanation + next steps (EN; SW from checked text)
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
| Agency named in an official warning | critical | Ministry / NEA notices (registry snapshot) |
| Named in a warning on a government website, found live | critical | Tavily search restricted to Kenyan government domains |
| Agency not on the NEA register / licence shown as expired | high | NEA register (with its dates shown) |
| Agency is licensed but the ad's email or website isn't the registered one | high | Registered contact details on the NEA register |
| Licence number belongs to a different agency (incl. lookalike name + borrowed number) | critical | Reported forged NEA certificates |
| Fee charged for an unskilled or domestic job | critical | NEA: no fees for unskilled jobs |
| Visitor / tourist visa for work | critical | Labour CS warning |
| Russia / military-style "jobs" | critical | Government warnings on war recruitment |
| Name looks like a licensed agency but isn't | high | Known impersonation tactic |
| Licence under verification / number not on register | high | NEAIMS |
| Money requested before a contract exists | high | Ministry investigations |
| Linked to fraud reports online (Tavily) | high | Live search |
| Salary far above typical pay for the job and country (2× or more) | high | Tavily salary data, read by Nemotron |
| Salary above typical pay (1.2×–2×) | medium | Tavily salary data, read by Nemotron |
| Destination matches known Asia scam pattern | high | Government advisory |
| WhatsApp / social-only contact, no office | medium | NEA agency requirements |
| Pressure to decide or pay quickly | medium | Government advisory |
| Contract must be attested before travel | reminder | Ministry of Labour |

Sources are listed in [`api/internal/rules/sources.go`](api/internal/rules/sources.go).

---

## How Kazi Halali uses NVIDIA Nemotron and Nebius Token Factory

All model calls go through **Nebius Token Factory**'s OpenAI-compatible API. Each step uses the smallest model that does the job well, so the app stays fast and cheap enough to offer free to job seekers:

| Step | Model | Why this model |
|---|---|---|
| Read screenshots / certificate photos | `nvidia/Nemotron-3.5-Super-VL` | Vision-language; transcribes ads and describes stamps/edits on certificates |
| Extract structured facts | `nvidia/NVIDIA-Nemotron-3-Nano-30B-A3B` | High-volume, low-latency JSON extraction; also reads salary figures from search results |
| Explain the result | `nvidia/nemotron-3-super-120b-a12b` | Clear plain-English summary and next steps (Kiswahili comes from checked text, see Results) |

Every result records which model ran each step, with latency and token counts. You can see this under **How this check was done** on any result page.

Model IDs can be changed in `.env` (`MODEL_VISION`, `MODEL_EXTRACT`, `MODEL_REASON`). Thinking is switched off by default (`LLM_EXTRA_BODY`).

### How Kazi Halali uses Tavily

Tavily makes three live checks, run in parallel, so a result reflects what is true *today* rather than when the app's data was last updated:

1. **Official warnings, live.** A search restricted to Kenyan government domains (Labour Ministry, NEA, Diaspora Affairs and others) for the agency's name. The rule fires only when a warning word ("deregistered", "not registered", "fake"…) appears *near* the agency's name, so a licensed agency listed on the same page as a deregistered one is never accused. When nothing is found, the result says so, and that no-warning result is shown to the user as a passed check.
2. **Too good to be true?** Tavily finds typical pay for the job in the destination country. Nemotron reads the figures out of the search results, and is told to use nothing else. Go code then normalises pay periods, converts currencies and computes the ratio. Figures are only used if the model cites a URL the app actually sent it, so a model can't invent a salary or a source.
3. **Public fraud reports.** A search for the agency name and phone number alongside scam terms, reporting only results that mention both.

Every Tavily result that triggers a finding is shown to the user as a source link.

---

## Run it

Requirements: Go 1.25+, Node 20+, Docker (for Postgres).

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

### Deploy

See [DEPLOY.md](DEPLOY.md): the API runs on Render from a private Docker image, calling Nebius Token Factory for every model step, and the web app runs on
Vercel. `make up` runs both locally in Docker exactly as deployed.

### Tests and evaluation

```bash
make test        # unit tests: rules, registry matching, extraction, pipeline
make eval        # runs eval/cases against the running API
```

## Results

**Against the real NEA register** (1,295 licensed agencies, register published 19 May 2026),
on the deployed API with live Nemotron models on Token Factory and live Tavily search:

```
Cases fully run by models:         6/6
Scams flagged (danger or caution): 4/4
Genuine offers marked danger:      0/2
Exact verdict matches:             6/6
```

The six cases are realistic ads that name **real licensed agencies**: two genuine-looking
offers using the agency's registered contacts, and four scams that *impersonate* those
agencies (a Gmail address instead of the registered one, a fee for domestic work, a
lookalike name, an expired listing with mismatched contacts). They are kept out of this
public repo because they name real companies (`eval/real/`, git-ignored).
`eval/cases/` holds a similar public set with fictional agencies (`make eval`).

A screenshot of a WhatsApp-style ad (fictional agency) runs through all seven steps:

| Step | Model / service | Time |
|---|---|---|
| Read the screenshot | Nemotron 3.5 Super VL | 2.8 s |
| Extract facts | Nemotron 3 Nano 30B | 4.4 s |
| Fraud reports, official warnings, salary data (parallel) | Tavily | ~6.7 s |
| Read salary figures | Nemotron 3 Nano 30B | 4.5 s |
| Explain | Nemotron 3 Super 120B | 1.1 s |

Verdict **Do not pay**: a fee for domestic work and a visitor visa for work (critical),
a licence number not on the register (high), WhatsApp-only contact, pressure to pay
today, and pay 1.5× the typical range for the job (medium).

### What I changed after testing

- **Thinking off.** Nemotron's reasoning mode roughly doubled latency, and the VL model
  returned an empty answer with it on. The API sends `enable_thinking: false` by default.
- **Kiswahili from checked text, not the model.** I compared the Kiswahili written by
  Nemotron Super and Ultra, with and without reasoning. All of them made errors a
  Kenyan reader would notice, some of which changed the meaning of the warning. Safety
  advice can't be wrong, so the model writes the English explanation and the Kiswahili
  comes from hand-written text that still names this ad's specific findings.
- **No accusations from directory pages.** A business-directory profile of a licensed
  agency was matched as a "fraud report" because the site's menu says *"Report a
  listing that may be fake"*. Directory and trust-score sites are now ignored, and site
  boilerplate is removed before matching.
- **No false "believable salary".** Pay between 1.2× and 2× the typical range used to be
  shown as a pass; it is now reported as "higher than typical".

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

**The register can be out of date.** The NEA register states its own "last updated" date, and
it can lag months behind new licences, renewals and changes of contact details. So Kazi Halali
never makes a critical finding from the register alone: an agency that is missing or shown as
expired gets a warning that states the register's publication date and when the app's copy was taken,
and tells the user to confirm with NEA. Critical findings are reserved for things that don't go
stale this way: official warnings, a licence number registered to a different agency, fees for
unskilled jobs, visitor visas and war-zone recruitment.


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
  internal/search   Tavily client (fraud reports, official warnings, salaries)
  internal/benchmark salary reality check (Tavily + Nemotron + currency maths)
  migrations/       Postgres schema
web/                Next.js + Tailwind frontend
data/               registry CSVs
eval/cases/         evaluation cases
```

## License

MIT — see [LICENSE](LICENSE).