package rules

import (
	"strings"
	"testing"
	"time"

	"github.com/edkariuki/kazi-halali/api/internal/domain"
)

func ptr[T any](v T) *T { return &v }

func fired(fs []domain.Finding) map[string]domain.Severity {
	out := map[string]domain.Severity{}
	for _, f := range fs {
		out[f.RuleID] = f.Severity
	}
	return out
}

var validAgency = &domain.Agency{ID: 1, Name: "Upendo Global Placements Limited", Status: domain.StatusValid}

func TestUnskilledFeeIsCritical(t *testing.T) {
	in := Input{
		Facts: domain.Facts{
			JobCategory: domain.JobUnskilled,
			Fees:        []domain.Fee{{Money: domain.Money{Amount: 85000, Currency: "KES"}, Purpose: "processing"}},
		},
	}
	got := fired(Evaluate(in))
	if got["fees.unskilled"] != domain.SevCritical {
		t.Fatalf("fees.unskilled should be critical, got %v", got)
	}
	if _, ok := got["fees.upfront"]; ok {
		t.Fatal("fees.upfront should not double-count unskilled fees")
	}
}

func TestNoAgencyIsHigh(t *testing.T) {
	got := fired(Evaluate(Input{}))
	if got["agency.registry"] != domain.SevHigh {
		t.Fatalf("want high for no agency, got %v", got)
	}
}

// The register lags behind new licences, so "not found" must never be
// critical on its own: it would accuse a newly licensed agency.
func TestUnknownAgencyIsHighNotCritical(t *testing.T) {
	got := fired(Evaluate(Input{Facts: domain.Facts{AgencyName: "Ghost Recruiters Ltd"}}))
	if got["agency.registry"] != domain.SevHigh {
		t.Fatalf("want high, got %v", got)
	}
}

func TestValidAgencyPasses(t *testing.T) {
	in := Input{
		Facts:    domain.Facts{AgencyName: validAgency.Name},
		Registry: domain.RegistryMatch{Kind: domain.MatchExact, Agency: validAgency},
	}
	got := fired(Evaluate(in))
	if got["agency.registry"] != domain.SevPass {
		t.Fatalf("want pass, got %v", got)
	}
}

func TestBorrowedLicenceIsCritical(t *testing.T) {
	in := Input{
		Facts:    domain.Facts{AgencyName: "Upendo Global Placement Agency", AgencyLicenceNo: "NEA/PEA/0001"},
		Registry: domain.RegistryMatch{Kind: domain.MatchLookalike, Agency: validAgency, LicenceOwner: validAgency},
	}
	// Lookalike resolved to the same agency that owns the number: the name
	// still differs, so the rule must look at the facts' name, not just IDs.
	got := fired(Evaluate(in))
	if got["agency.registry"] != domain.SevHigh {
		t.Fatalf("lookalike should be high, got %v", got)
	}
	if got["agency.licence"] != domain.SevCritical {
		t.Fatalf("lookalike quoting the real agency's number should be critical, got %v", got)
	}
}

func TestLicenceOfOtherAgency(t *testing.T) {
	other := &domain.Agency{ID: 2, Name: "Mwangaza Overseas Recruitment Ltd", Status: domain.StatusValid}
	in := Input{
		Facts:    domain.Facts{AgencyName: validAgency.Name, AgencyLicenceNo: "NEA/PEA/0002"},
		Registry: domain.RegistryMatch{Kind: domain.MatchExact, Agency: validAgency, LicenceOwner: other},
	}
	if fired(Evaluate(in))["agency.licence"] != domain.SevCritical {
		t.Fatal("licence belonging to another agency should be critical")
	}
}

func TestVisitorVisaAndRussia(t *testing.T) {
	got := fired(Evaluate(Input{Facts: domain.Facts{VisaType: domain.VisaVisitor, DestinationCountry: "Russia"}}))
	if got["visa.visitor"] != domain.SevCritical || got["job.military"] != domain.SevCritical {
		t.Fatalf("want both critical, got %v", got)
	}
}

func TestUpfrontFeeSkilledIsHigh(t *testing.T) {
	in := Input{Facts: domain.Facts{
		JobCategory: domain.JobSemiSkilled,
		Fees:        []domain.Fee{{Money: domain.Money{Amount: 6000, Currency: "KES"}, PayableBeforeContract: ptr(true)}},
	}}
	if fired(Evaluate(in))["fees.upfront"] != domain.SevHigh {
		t.Fatal("upfront fee should be high")
	}
}

func TestFeeAfterContractNotFlagged(t *testing.T) {
	in := Input{Facts: domain.Facts{
		JobCategory: domain.JobSkilled,
		Fees:        []domain.Fee{{Money: domain.Money{Amount: 6000, Currency: "KES"}, PayableBeforeContract: ptr(false)}},
	}}
	if _, ok := fired(Evaluate(in))["fees.upfront"]; ok {
		t.Fatal("fee after contract should not fire fees.upfront")
	}
}

func TestWhatsAppOnly(t *testing.T) {
	in := Input{Facts: domain.Facts{ContactChannels: []string{"whatsapp"}, Emails: []string{"jobs@gmail.com"}}}
	if fired(Evaluate(in))["contact.informal"] != domain.SevMedium {
		t.Fatal("whatsapp + gmail only should be medium")
	}
	in.Facts.Websites = []string{"https://agency.co.ke"}
	if _, ok := fired(Evaluate(in))["contact.informal"]; ok {
		t.Fatal("a company website should clear contact.informal")
	}
}

func TestWebReportsNeedNameNearScamWord(t *testing.T) {
	in := Input{
		Facts: domain.Facts{AgencyName: "Ghost Recruiters Ltd"},
		Hits: []domain.SearchHit{
			{Title: "Ghost Recruiters Ltd opens new office", URL: "https://a"},
			{Title: "Police arrest Ghost Recruiters Ltd director over fake jobs", URL: "https://b"},
		},
	}
	fs := Evaluate(in)
	for _, f := range fs {
		if f.RuleID == "web.reports" {
			if len(f.Sources) != 1 || f.Sources[0].URL != "https://b" {
				t.Fatalf("want only the fraud report cited, got %+v", f.Sources)
			}
			return
		}
	}
	t.Fatal("web.reports did not fire")
}

// A general scam-awareness article that happens to list a licensed agency
// far from any scam word must not link that agency to fraud.
func TestWebReportsIgnoresDistantAgencyMention(t *testing.T) {
	filler := strings.Repeat("Recruitment advice for job seekers continues here. ", 10)
	in := Input{
		Facts: domain.Facts{AgencyName: "Africa Management Solutions Limited"},
		Hits: []domain.SearchHit{{Title: "Could you spot this scam?", URL: "https://guide",
			Snippet: "How to spot fake job offers and scams. " + filler + "Licensed agencies include Africa Management Solutions Limited."}},
	}
	if _, ok := fired(Evaluate(in))["web.reports"]; ok {
		t.Fatal("distant mention in a general guide must not count")
	}
}

func TestWebReportsIgnoresPlaceholderPhones(t *testing.T) {
	for _, p := range []string{"0712 345 678", "0700000000", "+254 711 111 111"} {
		in := Input{
			Facts: domain.Facts{Phones: []string{p}},
			Hits:  []domain.SearchHit{{Title: "Could you spot this scam?", URL: "https://guide", Snippet: "Scam example: call " + p + " to pay."}},
		}
		if _, ok := fired(Evaluate(in))["web.reports"]; ok {
			t.Errorf("placeholder %s must not count", p)
		}
	}
}

func TestWebReportsPhoneMatchesWholeNumbersOnly(t *testing.T) {
	real := "0798 413 552"
	in := Input{Facts: domain.Facts{Phones: []string{real}}}
	// The digits of the number appear, but split across a date and another number.
	in.Hits = []domain.SearchHit{{Title: "Scam alert", URL: "https://x", Snippet: "On 07/98 fraud: call 0741 355 211 or ref 2"}}
	if _, ok := fired(Evaluate(in))["web.reports"]; ok {
		t.Fatal("digits run together across separate numbers must not match")
	}
	in.Hits = []domain.SearchHit{{Title: "Scam alert", URL: "https://y", Snippet: "Job fraud: victims paid +254-798-413-552 via M-Pesa."}}
	if fired(Evaluate(in))["web.reports"] != domain.SevHigh {
		t.Fatal("the same number in another format should match")
	}
}

// A directory profile of a licensed agency, whose menu says "Report a listing
// that may be fake", is not a fraud report about that agency.
func TestWebReportsIgnoresListingSitesAndBoilerplate(t *testing.T) {
	name := "Upendo Global Placements Limited"
	profile := "Report a listing that may be fake, closed, duplicated or inaccurate. " + name + " — Registered office. Recruitment agencies in Nairobi. 0 customer experiences."
	cases := []domain.SearchHit{
		{Title: name + " | TrustScore Kenya", URL: "https://trustscore.co.ke/businesses/upendo", Snippet: profile},
		{Title: name + " | Some Directory", URL: "https://directory.example/upendo", Snippet: profile},
		{Title: "Is " + name + " a scam or legit?", URL: "https://checker.example/upendo", Snippet: name + " recruitment agency profile."},
	}
	for _, h := range cases {
		in := Input{Facts: domain.Facts{AgencyName: name}, Hits: []domain.SearchHit{h}}
		if _, ok := fired(Evaluate(in))["web.reports"]; ok {
			t.Errorf("%s must not count as a fraud report", h.URL)
		}
	}
	// A real report that starts with "Report" still counts.
	in := Input{Facts: domain.Facts{AgencyName: "Ghost Recruiters Ltd"}, Hits: []domain.SearchHit{{
		Title: "Report: Ghost Recruiters Ltd conned 40 job seekers", URL: "https://news.example/ghost",
		Snippet: "Police say Ghost Recruiters Ltd took fees for fake jobs in Qatar.",
	}}}
	if fired(Evaluate(in))["web.reports"] != domain.SevHigh {
		t.Fatal("a news report must still fire")
	}
}

func TestEveryFindingHasSources(t *testing.T) {
	in := Input{Facts: domain.Facts{
		AgencyName: "Ghost Recruiters Ltd", AgencyLicenceNo: "NEA/X/9", JobCategory: domain.JobDomestic,
		Fees: []domain.Fee{{Money: domain.Money{Amount: 1, Currency: "KES"}}}, VisaType: domain.VisaVisitor,
		DestinationCountry: "Myanmar", ContactChannels: []string{"whatsapp"}, UrgencyCues: []string{"pay today"},
	}}
	for _, f := range Evaluate(in) {
		if len(f.Sources) == 0 {
			t.Errorf("finding %s has no sources", f.RuleID)
		}
		if f.TitleSW == "" || f.DetailSW == "" {
			t.Errorf("finding %s is missing Swahili text", f.RuleID)
		}
	}
}

func TestOfficialWarningNeedsProximity(t *testing.T) {
	in := Input{
		Facts:            domain.Facts{AgencyName: "Ghost Recruiters Ltd"},
		OfficialSearched: true,
		Official: []domain.SearchHit{{
			Title: "Ministry notice", URL: "https://labour.go.ke/notice",
			Snippet: "The following agencies are deregistered and not authorised to recruit: Ghost Recruiters Limited, Shadow Jobs Ltd.",
		}},
	}
	if fired(Evaluate(in))["official.warning"] != domain.SevCritical {
		t.Fatal("warning next to the name should be critical")
	}
}

func TestOfficialWarningIgnoresDistantMention(t *testing.T) {
	far := strings.Repeat("licensed agencies list continues here ", 20)
	in := Input{
		Facts:            domain.Facts{AgencyName: "Upendo Global Placements Ltd"},
		OfficialSearched: true,
		Official: []domain.SearchHit{{
			Title: "Agency list", URL: "https://nea.go.ke/list",
			Snippet: "Shadow Jobs Ltd was deregistered. " + far + "Upendo Global Placements Limited, valid licence.",
		}},
	}
	if got := fired(Evaluate(in))["official.warning"]; got != domain.SevPass {
		t.Fatalf("a licensed agency on the same page as a deregistered one must not be accused, got %v", got)
	}
}

func TestOfficialPassOnlyWhenSearched(t *testing.T) {
	in := Input{Facts: domain.Facts{AgencyName: "Upendo Global Placements Ltd"}}
	if _, ok := fired(Evaluate(in))["official.warning"]; ok {
		t.Fatal("no search ran, so no pass should be claimed")
	}
}

func TestSalaryTooGood(t *testing.T) {
	b := &domain.SalaryBenchmark{
		Found: true, JobTitle: "cleaner", Country: "Qatar",
		MonthlyLow: 1200, MonthlyHigh: 2000, Currency: "QAR",
		AdMonthly: 9000, AdCurrency: "QAR", Ratio: 4.5,
		Sources: []domain.Source{{Title: "Salary survey", URL: "https://example.org/s"}},
	}
	if fired(Evaluate(Input{Salary: b}))["salary.too_good"] != domain.SevHigh {
		t.Fatal("4.5x typical pay should be high")
	}
	b.Ratio = 1.5 // e.g. SAR 4,500 against a typical SAR 2,000–3,000
	if fired(Evaluate(Input{Salary: b}))["salary.too_good"] != domain.SevMedium {
		t.Fatal("1.5x typical pay must not be called believable; want medium")
	}
	b.Ratio = 1.2
	if fired(Evaluate(Input{Salary: b}))["salary.too_good"] != domain.SevPass {
		t.Fatal("1.2x typical pay should pass")
	}
	b.Found = false
	if _, ok := fired(Evaluate(Input{Salary: b}))["salary.too_good"]; ok {
		t.Fatal("no benchmark found, no finding")
	}
}

func TestExpiredIsHighAndShowsDates(t *testing.T) {
	exp := &domain.Agency{ID: 9, Name: "A&J Global Limited", Status: domain.StatusExpired}
	in := Input{
		Facts: domain.Facts{AgencyName: exp.Name},
		Registry: domain.RegistryMatch{Kind: domain.MatchExact, Agency: exp, Register: domain.RegisterInfo{
			SourceURL:   "https://register.example/agencies",
			PublishedAt: time.Date(2026, 5, 19, 0, 0, 0, 0, time.UTC),
			SnapshotAt:  time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC),
		}},
	}
	for _, f := range Evaluate(in) {
		if f.RuleID != "agency.registry" {
			continue
		}
		if f.Severity != domain.SevHigh {
			t.Fatalf("severity = %s", f.Severity)
		}
		if !strings.Contains(f.DetailEN, "19 May 2026") || !strings.Contains(f.DetailSW, "19 Mei 2026") {
			t.Fatalf("dates missing:\n%s\n%s", f.DetailEN, f.DetailSW)
		}
		if f.Sources[0].URL != "https://register.example/agencies" {
			t.Fatalf("source should be the register copy used, got %s", f.Sources[0].URL)
		}
		return
	}
	t.Fatal("agency.registry did not fire")
}

func registered(email, site string) Input {
	a := &domain.Agency{ID: 1, Name: "A&J Global Limited", Status: domain.StatusValid, Email: email, Website: site}
	return Input{Registry: domain.RegistryMatch{Kind: domain.MatchExact, Agency: a}, Facts: domain.Facts{AgencyName: a.Name}}
}

func TestContactMismatchOnCompanyDomain(t *testing.T) {
	in := registered("admin@ajglobalconsult.com", "www.ajglobalconsult.com")
	in.Facts.Emails = []string{"jobs@ajglobal-recruit.com"}
	if fired(Evaluate(in))["agency.contact"] != domain.SevHigh {
		t.Fatal("different company domain should be high")
	}
}

func TestContactMatchesSubdomainAndOtherMailbox(t *testing.T) {
	in := registered("admin@ajglobalconsult.com", "www.ajglobalconsult.com")
	in.Facts.Emails = []string{"careers@ajglobalconsult.com"}
	in.Facts.Websites = []string{"https://jobs.ajglobalconsult.com/qatar"}
	if fired(Evaluate(in))["agency.contact"] != domain.SevPass {
		t.Fatal("same company domain should pass")
	}
}

func TestContactFreeMailMustMatchExactly(t *testing.T) {
	in := registered("afrmanpower@gmail.com", "")
	in.Facts.Emails = []string{"afr.jobs.abroad@gmail.com"}
	if fired(Evaluate(in))["agency.contact"] != domain.SevHigh {
		t.Fatal("a different gmail address is not the registered one")
	}
	in.Facts.Emails = []string{"AFRmanpower@gmail.com"}
	if fired(Evaluate(in))["agency.contact"] != domain.SevPass {
		t.Fatal("the registered gmail address (any case) should pass")
	}
}

func TestContactIgnoresSocialLinksAndEmptyRegister(t *testing.T) {
	in := registered("admin@ajglobalconsult.com", "")
	in.Facts.Websites = []string{"https://wa.me/254700000000"}
	if _, ok := fired(Evaluate(in))["agency.contact"]; ok {
		t.Fatal("a WhatsApp link is not an agency website")
	}
	in = registered("", "")
	in.Facts.Emails = []string{"x@y.com"}
	if _, ok := fired(Evaluate(in))["agency.contact"]; ok {
		t.Fatal("nothing registered, nothing to compare")
	}
}

func TestContactSkipsLookalikes(t *testing.T) {
	in := registered("admin@ajglobalconsult.com", "")
	in.Registry.Kind = domain.MatchLookalike
	in.Facts.Emails = []string{"x@other.com"}
	if _, ok := fired(Evaluate(in))["agency.contact"]; ok {
		t.Fatal("lookalikes are already flagged by agency.registry")
	}
}
