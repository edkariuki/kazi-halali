// Package domain holds the core types shared across the pipeline.
// Nothing here talks to the network or the database.
package domain

import "time"

// ---------------------------------------------------------------------------
// Facts: what the extraction step pulls out of an ad / offer / certificate.
// Every field is optional — ads are messy and often leave things out, and
// "not stated" is itself a signal the rules can use.
// ---------------------------------------------------------------------------

type JobCategory string

const (
	JobSkilled     JobCategory = "skilled"      // nurse, engineer, chef, driver with licence
	JobSemiSkilled JobCategory = "semi_skilled" // security guard, care assistant, warehouse
	JobUnskilled   JobCategory = "unskilled"    // cleaner, farm hand, general labourer
	JobDomestic    JobCategory = "domestic"     // house help, nanny
	JobMilitary    JobCategory = "military"     // anything combat / "security contractor" in a war zone
	JobUnknown     JobCategory = "unknown"
)

type VisaType string

const (
	VisaWork    VisaType = "work"
	VisaVisitor VisaType = "visitor" // includes tourist / visit / "e-visa then convert"
	VisaStudent VisaType = "student"
	VisaUnknown VisaType = "unknown"
)

type Money struct {
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"` // ISO code where possible: KES, USD, CAD, QAR...
	Period   string  `json:"period,omitempty"`
}

type Fee struct {
	Money
	Purpose string `json:"purpose"` // "processing", "medical", "visa", "training"...
	// PayableBeforeContract is true when the fee is demanded before a signed,
	// attested contract exists — the typical scam pattern.
	PayableBeforeContract *bool `json:"payable_before_contract,omitempty"`
}

type Facts struct {
	AgencyName         string      `json:"agency_name"`
	AgencyLicenceNo    string      `json:"agency_licence_no"`
	RecruiterPerson    string      `json:"recruiter_person"` // individual "agent" name if no company
	Phones             []string    `json:"phones"`
	Emails             []string    `json:"emails"`
	Websites           []string    `json:"websites"`
	ContactChannels    []string    `json:"contact_channels"` // whatsapp, telegram, facebook, office, email, phone
	HasPhysicalOffice  *bool       `json:"has_physical_office"`
	DestinationCountry string      `json:"destination_country"`
	Employer           string      `json:"employer"`
	JobTitle           string      `json:"job_title"`
	JobCategory        JobCategory `json:"job_category"`
	Salary             *Money      `json:"salary"`
	Fees               []Fee       `json:"fees"`
	VisaType           VisaType    `json:"visa_type"`
	ContractAttested   *bool       `json:"contract_attested"`
	UrgencyCues        []string    `json:"urgency_cues"` // "limited slots", "pay today", "travel next week"
	Language           string      `json:"language"`     // en, sw, mixed
	// DocumentKind describes what was submitted: ad, offer_letter, certificate, chat, other.
	DocumentKind string `json:"document_kind"`
}

// ---------------------------------------------------------------------------
// Registry
// ---------------------------------------------------------------------------

type AgencyStatus string

const (
	StatusValid             AgencyStatus = "valid"
	StatusUnderVerification AgencyStatus = "under_verification"
	StatusExpired           AgencyStatus = "expired"
	StatusUnknown           AgencyStatus = "unknown"
)

type Agency struct {
	ID             int64        `json:"id"`
	Name           string       `json:"name"`
	NormalizedName string       `json:"-"`
	NEARegNo       string       `json:"nea_reg_no"`
	Status         AgencyStatus `json:"status"`
	SourceURL      string       `json:"source_url"`
	ServiceType    string       `json:"service_type,omitempty"`
	// Contact details as published in the register. A licensed agency's name
	// and number are easy to copy; its registered mailbox is not.
	Email   string `json:"email,omitempty"`
	Website string `json:"website,omitempty"`
	// PublishedAt is the register's own "last updated" date; SnapshotAt is
	// when we copied it. Both are shown to users so staleness is visible.
	PublishedAt time.Time `json:"published_at,omitempty"`
	SnapshotAt  time.Time `json:"snapshot_at,omitempty"`
}

// RegisterInfo describes the copy of the NEA register a check used.
type RegisterInfo struct {
	SourceURL   string    `json:"source_url,omitempty"`
	Agencies    int       `json:"agencies"`
	PublishedAt time.Time `json:"published_at,omitempty"`
	SnapshotAt  time.Time `json:"snapshot_at,omitempty"`
}

type FlaggedEntry struct {
	ID             int64     `json:"id"`
	Name           string    `json:"name"`
	NormalizedName string    `json:"-"`
	Reason         string    `json:"reason"`
	SourceTitle    string    `json:"source_title"`
	SourceURL      string    `json:"source_url"`
	PublishedAt    time.Time `json:"published_at"`
}

type MatchKind string

const (
	MatchNone      MatchKind = "none"
	MatchExact     MatchKind = "exact"     // same normalized name
	MatchLicenceNo MatchKind = "licence"   // matched on NEA number
	MatchLookalike MatchKind = "lookalike" // very similar but not the same — a known scam tactic
)

type RegistryMatch struct {
	Kind       MatchKind `json:"kind"`
	Agency     *Agency   `json:"agency,omitempty"`
	Similarity float64   `json:"similarity,omitempty"`
	// LicenceOwner is the registered agency that owns the licence number
	// quoted in the material, if any. When it differs from Agency, someone is
	// borrowing a real agency's number.
	LicenceOwner *Agency        `json:"licence_owner,omitempty"`
	Flagged      []FlaggedEntry `json:"flagged,omitempty"`
	Register     RegisterInfo   `json:"register"`
}

// ---------------------------------------------------------------------------
// Web search
// ---------------------------------------------------------------------------

type SearchHit struct {
	Title   string  `json:"title"`
	URL     string  `json:"url"`
	Snippet string  `json:"snippet"`
	Score   float64 `json:"score"`
}

// ---------------------------------------------------------------------------
// Salary benchmark
// ---------------------------------------------------------------------------

// SalaryBenchmark compares the pay in an ad with typical pay found online.
// The model only reads figures out of search results; currency conversion
// and the ratio are computed in Go.
type SalaryBenchmark struct {
	JobTitle    string   `json:"job_title"`
	Country     string   `json:"country"`
	Found       bool     `json:"found"`
	MonthlyLow  float64  `json:"monthly_low"`
	MonthlyHigh float64  `json:"monthly_high"`
	Currency    string   `json:"currency"`
	AdMonthly   float64  `json:"ad_monthly"`
	AdCurrency  string   `json:"ad_currency"`
	Ratio       float64  `json:"ratio"` // ad monthly pay / top of typical range, both in KES
	Sources     []Source `json:"sources"`
	Note        string   `json:"note,omitempty"`
}

// ---------------------------------------------------------------------------
// Findings & verdict
// ---------------------------------------------------------------------------

type Severity string

const (
	SevCritical Severity = "critical" // on its own, enough to say "do not proceed"
	SevHigh     Severity = "high"
	SevMedium   Severity = "medium"
	SevInfo     Severity = "info" // a reminder / good practice, not a red flag
	SevPass     Severity = "pass" // a check that came back clean
)

type Source struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

type Finding struct {
	RuleID   string   `json:"rule_id"`
	Severity Severity `json:"severity"`
	TitleEN  string   `json:"title_en"`
	TitleSW  string   `json:"title_sw"`
	DetailEN string   `json:"detail_en"`
	DetailSW string   `json:"detail_sw"`
	Evidence string   `json:"evidence,omitempty"` // the phrase / value that triggered it
	Sources  []Source `json:"sources"`
}

type Verdict string

const (
	VerdictDanger   Verdict = "danger"  // HATARI
	VerdictCaution  Verdict = "caution" // TAHADHARI
	VerdictLikelyOK Verdict = "likely_ok"
)

type Explanation struct {
	SummaryEN   string   `json:"summary_en"`
	SummarySW   string   `json:"summary_sw"`
	NextStepsEN []string `json:"next_steps_en"`
	NextStepsSW []string `json:"next_steps_sw"`
	Generated   bool     `json:"generated"` // false = template fallback was used
}

// ModelCall records which model did what, for the transparency panel and
// the README's "how we used Nemotron" section.
type ModelCall struct {
	Step      string `json:"step"` // vision | extract | explain
	Model     string `json:"model"`
	LatencyMS int64  `json:"latency_ms"`
	InputTok  int    `json:"input_tokens"`
	OutputTok int    `json:"output_tokens"`
	Error     string `json:"error,omitempty"`
}

type Check struct {
	ID         string        `json:"id"`
	CreatedAt  time.Time     `json:"created_at"`
	InputText  string        `json:"input_text"`
	HadImage   bool          `json:"had_image"`
	Transcript string        `json:"transcript,omitempty"` // text read from the image
	Facts      Facts         `json:"facts"`
	Registry   RegistryMatch `json:"registry"`
	SearchHits []SearchHit   `json:"search_hits"`
	// OfficialHits are results restricted to Kenyan government sites.
	OfficialHits     []SearchHit      `json:"official_hits"`
	OfficialSearched bool             `json:"official_searched"`
	Salary           *SalaryBenchmark `json:"salary_benchmark,omitempty"`
	Findings         []Finding        `json:"findings"`
	Verdict          Verdict          `json:"verdict"`
	Explanation      Explanation      `json:"explanation"`
	Trace            []ModelCall      `json:"trace"`
}
