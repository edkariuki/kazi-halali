// Package rules is the deterministic core of Kazi Halali.
//
// The language models read and explain; these rules decide. Every rule is a
// pure function of the extracted facts, the registry match and web-search
// hits, and every finding cites its source. That makes verdicts testable,
// reproducible and auditable — the model can never "hallucinate" a red flag
// or talk its way out of one.
package rules

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/edkariuki/kazi-halali/api/internal/domain"
	"github.com/edkariuki/kazi-halali/api/internal/registry"
)

type Input struct {
	Facts    domain.Facts
	Registry domain.RegistryMatch
	Hits     []domain.SearchHit
	// Official are search results restricted to Kenyan government sites;
	// OfficialSearched says the search actually ran (so "nothing found" means something).
	Official         []domain.SearchHit
	OfficialSearched bool
	Salary           *domain.SalaryBenchmark
}

type Rule struct {
	ID    string
	Check func(Input) []domain.Finding
}

// All is the ordered rule set. Order matters only for display.
var All = []Rule{
	{"agency.flagged", agencyFlagged},
	{"official.warning", officialWarning},
	{"agency.registry", agencyRegistry},
	{"agency.licence", agencyLicence},
	{"agency.contact", agencyContact},
	{"fees.unskilled", feesUnskilled},
	{"fees.upfront", feesUpfront},
	{"visa.visitor", visaVisitor},
	{"job.military", jobMilitary},
	{"destination.risk", destinationRisk},
	{"contact.informal", contactInformal},
	{"pressure.urgency", pressureUrgency},
	{"web.reports", webReports},
	{"salary.too_good", salaryTooGood},
	{"contract.attestation", contractAttestation},
}

func Evaluate(in Input) []domain.Finding {
	var out []domain.Finding
	for _, r := range All {
		for _, f := range r.Check(in) {
			if f.RuleID == "" {
				f.RuleID = r.ID
			}
			out = append(out, f)
		}
	}
	return out
}

// ---------------------------------------------------------------------------

func agencyFlagged(in Input) []domain.Finding {
	var out []domain.Finding
	for _, fl := range in.Registry.Flagged {
		out = append(out, domain.Finding{
			Severity: domain.SevCritical,
			TitleEN:  "Agency named in an official warning",
			TitleSW:  "Wakala ametajwa kwenye onyo rasmi",
			DetailEN: fmt.Sprintf("“%s” appears in an official notice: %s.", fl.Name, fl.Reason),
			DetailSW: fmt.Sprintf("“%s” ametajwa kwenye tangazo rasmi la serikali: %s.", fl.Name, fl.Reason),
			Evidence: in.Facts.AgencyName,
			Sources:  []domain.Source{{Title: fl.SourceTitle, URL: fl.SourceURL}},
		})
	}
	return out
}

func agencyRegistry(in Input) []domain.Finding {
	name := strings.TrimSpace(in.Facts.AgencyName)
	m := in.Registry

	if m.Agency == nil {
		m.Kind = domain.MatchNone // zero value or explicit none
	}
	if name == "" && m.Kind == domain.MatchNone {
		who := "No recruitment company is named"
		whoSW := "Hakuna kampuni ya uajiri iliyotajwa"
		if p := in.Facts.RecruiterPerson; p != "" {
			who = fmt.Sprintf("Only an individual (“%s”) is named, not a licensed company", p)
			whoSW = fmt.Sprintf("Ni mtu binafsi tu (“%s”) aliyetajwa, si kampuni yenye leseni", p)
		}
		return []domain.Finding{{
			Severity: domain.SevHigh,
			TitleEN:  "No licensed agency identified",
			TitleSW:  "Hakuna wakala mwenye leseni aliyetambuliwa",
			DetailEN: who + ". Overseas recruitment in Kenya must go through an agency licensed by the National Employment Authority (NEA). Ask for the agency's full name and NEA number, then check them yourself.",
			DetailSW: whoSW + ". Uajiri wa kazi za nje lazima upitie wakala aliye na leseni ya Mamlaka ya Kitaifa ya Ajira (NEA). Uliza jina kamili la wakala na nambari yake ya NEA, kisha uthibitishe mwenyewe.",
			Sources:  []domain.Source{srcNEAList, srcHowToCheck},
		}}
	}

	switch m.Kind {
	case domain.MatchNone:
		// Not critical on purpose: the register lags (newly licensed agencies
		// may be missing), and a false accusation is worse than a caution.
		en, sw := registerDates(m.Register)
		return []domain.Finding{{
			Severity: domain.SevHigh,
			TitleEN:  "Agency not found on the NEA register",
			TitleSW:  "Wakala hapatikani kwenye rejista ya NEA",
			DetailEN: fmt.Sprintf("We could not find “%s” among agencies on the National Employment Authority register%s. Unlicensed agencies may not recruit Kenyans for jobs abroad. A newly licensed agency may not appear yet, so check the register yourself or call NEA on %s before paying anything.", name, en, NEAHotline),
			DetailSW: fmt.Sprintf("Hatukumpata “%s” kwenye rejista ya Mamlaka ya Kitaifa ya Ajira%s. Mawakala wasio na leseni hawaruhusiwi kuajiri Wakenya kwa kazi za nje. Wakala aliyepewa leseni hivi karibuni huenda bado hajaonekana, kwa hiyo kagua rejista mwenyewe au piga simu NEA kupitia %s kabla ya kulipa chochote.", name, sw, NEAHotline),
			Evidence: name,
			Sources:  []domain.Source{registerSource(m.Register), srcHowToCheck},
		}}

	case domain.MatchLookalike:
		return []domain.Finding{{
			Severity: domain.SevHigh,
			TitleEN:  "Name resembles a licensed agency, but is not the same",
			TitleSW:  "Jina linafanana na la wakala mwenye leseni, lakini si lile lile",
			DetailEN: fmt.Sprintf("“%s” is close to the licensed agency “%s”, but does not match it. Fraudsters sometimes borrow a real agency's name with small changes. Contact “%s” through the details on the NEA list — not the details in this ad — and ask if this job is theirs.", name, m.Agency.Name, m.Agency.Name),
			DetailSW: fmt.Sprintf("“%s” linakaribiana na wakala mwenye leseni “%s”, lakini halilingani. Matapeli wakati mwingine hutumia jina la wakala halali kwa mabadiliko madogo. Wasiliana na “%s” kupitia maelezo yaliyo kwenye orodha ya NEA — si yaliyo kwenye tangazo hili — uulize kama kazi hii ni yao.", name, m.Agency.Name, m.Agency.Name),
			Evidence: name,
			Sources:  []domain.Source{srcNEAList},
		}}
	}

	// Exact or licence-number match: report the licence status.
	a := m.Agency
	datesEN, datesSW := registerDates(m.Register)
	switch a.Status {
	case domain.StatusValid:
		return []domain.Finding{{
			Severity: domain.SevPass,
			TitleEN:  "Agency is on the NEA licensed list",
			TitleSW:  "Wakala yupo kwenye orodha ya NEA",
			DetailEN: fmt.Sprintf("“%s” is listed as licensed on the NEA register%s. That does not prove this particular job is genuine — scammers also impersonate real agencies — so confirm the vacancy with the agency directly.", a.Name, datesEN),
			DetailSW: fmt.Sprintf("“%s” ameorodheshwa kuwa na leseni kwenye rejista ya NEA%s. Hii haithibitishi kuwa kazi hii ni halali — matapeli pia hujifanya mawakala halali — kwa hiyo thibitisha nafasi hii moja kwa moja na wakala.", a.Name, datesSW),
			Sources:  []domain.Source{registerSource(m.Register)},
		}}
	case domain.StatusUnderVerification:
		return []domain.Finding{{
			Severity: domain.SevHigh,
			TitleEN:  "Agency licence still under verification",
			TitleSW:  "Leseni ya wakala bado inathibitishwa",
			DetailEN: fmt.Sprintf("“%s” is listed by NEA as still undergoing verification, not as fully licensed.", a.Name),
			DetailSW: fmt.Sprintf("“%s” ameorodheshwa na NEA kama bado anathibitishwa, si kama mwenye leseni kamili.", a.Name),
			Sources:  []domain.Source{srcNEAList},
		}}
	case domain.StatusExpired:
		en, sw := registerDates(m.Register)
		return []domain.Finding{{
			Severity: domain.SevHigh,
			TitleEN:  "Agency licence shown as expired",
			TitleSW:  "Leseni ya wakala inaonyeshwa kuwa imeisha muda",
			DetailEN: fmt.Sprintf("“%s” is on the NEA register, but its licence is marked expired%s. An agency with an expired licence may not recruit. It may have renewed since, so ask the agency for its current licence and confirm it with NEA before you pay.", a.Name, en),
			DetailSW: fmt.Sprintf("“%s” yupo kwenye rejista ya NEA, lakini leseni yake imeonyeshwa kuwa imeisha muda%s. Wakala mwenye leseni iliyoisha hawezi kuajiri. Huenda ameihuisha tangu wakati huo, kwa hiyo muulize wakala leseni yake ya sasa na uithibitishe na NEA kabla ya kulipa.", a.Name, sw),
			Sources:  []domain.Source{registerSource(m.Register), srcHowToCheck},
		}}
	}
	return nil
}

func agencyLicence(in Input) []domain.Finding {
	lic := strings.TrimSpace(in.Facts.AgencyLicenceNo)
	if lic == "" {
		return nil
	}
	owner := in.Registry.LicenceOwner
	if owner == nil {
		return []domain.Finding{{
			Severity: domain.SevHigh,
			TitleEN:  "Licence number not found on the register",
			TitleSW:  "Nambari ya leseni haipatikani kwenye rejista",
			DetailEN: fmt.Sprintf("The number “%s” does not match any agency on our copy of the NEA list. Forged NEA certificates and numbers have been reported. Check the number on the official NEA list.", lic),
			DetailSW: fmt.Sprintf("Nambari “%s” hailingani na wakala yeyote kwenye nakala yetu ya orodha ya NEA. Vyeti na nambari bandia za NEA vimeripotiwa. Thibitisha nambari hii kwenye orodha rasmi ya NEA.", lic),
			Evidence: lic,
			Sources:  []domain.Source{srcFeesAndFakeCerts, srcNEAList},
		}}
	}
	named := in.Registry.Agency
	if named != nil && named.ID == owner.ID && in.Registry.Kind != domain.MatchLookalike {
		return nil // number and name agree
	}
	// A lookalike name quoting the real agency's number is the classic
	// "borrowed licence" pattern, so it falls through to the critical finding.
	if in.Facts.AgencyName == "" {
		return nil
	}
	return []domain.Finding{{
		Severity: domain.SevCritical,
		TitleEN:  "Licence number belongs to a different agency",
		TitleSW:  "Nambari ya leseni ni ya wakala mwingine",
		DetailEN: fmt.Sprintf("The number “%s” is registered to “%s”, not to “%s”. Using another agency's licence number is a common sign of fraud.", lic, owner.Name, in.Facts.AgencyName),
		DetailSW: fmt.Sprintf("Nambari “%s” imesajiliwa kwa “%s”, si “%s”. Kutumia nambari ya leseni ya wakala mwingine ni ishara ya kawaida ya utapeli.", lic, owner.Name, in.Facts.AgencyName),
		Evidence: lic,
		Sources:  []domain.Source{srcFeesAndFakeCerts, srcNEAList},
	}}
}

func feesUnskilled(in Input) []domain.Finding {
	cat := in.Facts.JobCategory
	if cat != domain.JobDomestic && cat != domain.JobUnskilled {
		return nil
	}
	if len(in.Facts.Fees) == 0 {
		return nil
	}
	return []domain.Finding{{
		Severity: domain.SevCritical,
		TitleEN:  "Fee charged for an unskilled or domestic job",
		TitleSW:  "Ada inatozwa kwa kazi isiyohitaji ujuzi au ya nyumbani",
		DetailEN: fmt.Sprintf("This %s job asks you to pay %s. According to NEA, agencies are not supposed to charge job seekers anything for unskilled jobs such as domestic work.", humanCategory(cat), feeSummary(in.Facts.Fees)),
		DetailSW: fmt.Sprintf("Kazi hii (%s) inakutaka ulipe %s. Kwa mujibu wa NEA, mawakala hawapaswi kumtoza mtafuta kazi chochote kwa kazi zisizohitaji ujuzi kama kazi za nyumbani.", humanCategorySW(cat), feeSummary(in.Facts.Fees)),
		Evidence: feeSummary(in.Facts.Fees),
		Sources:  []domain.Source{srcFeesAndFakeCerts},
	}}
}

func feesUpfront(in Input) []domain.Finding {
	cat := in.Facts.JobCategory
	if cat == domain.JobDomestic || cat == domain.JobUnskilled {
		return nil // already covered, more strongly, by fees.unskilled
	}
	var upfront []domain.Fee
	for _, f := range in.Facts.Fees {
		if f.PayableBeforeContract == nil || *f.PayableBeforeContract {
			upfront = append(upfront, f)
		}
	}
	if len(upfront) == 0 {
		return nil
	}
	return []domain.Finding{{
		Severity: domain.SevHigh,
		TitleEN:  "Money requested before you have a contract",
		TitleSW:  "Pesa zinaombwa kabla ya kupata mkataba",
		DetailEN: fmt.Sprintf("You are asked to pay %s before a signed, attested contract exists. In reported scams, victims paid “processing” fees and the job never existed. Do not pay until you have verified the agency, the employer and the contract.", feeSummary(upfront)),
		DetailSW: fmt.Sprintf("Unaombwa kulipa %s kabla ya kuwa na mkataba uliosainiwa na kuthibitishwa. Katika utapeli ulioripotiwa, waathiriwa walilipa ada za “processing” na kazi haikuwepo. Usilipe hadi uthibitishe wakala, mwajiri na mkataba.", feeSummary(upfront)),
		Evidence: feeSummary(upfront),
		Sources:  []domain.Source{srcNEAApprovesAds, srcAttestation},
	}}
}

func visaVisitor(in Input) []domain.Finding {
	if in.Facts.VisaType != domain.VisaVisitor {
		return nil
	}
	return []domain.Finding{{
		Severity: domain.SevCritical,
		TitleEN:  "You would travel on a visitor visa",
		TitleSW:  "Utasafiri kwa viza ya mgeni",
		DetailEN: "The offer involves travelling on a visitor or tourist visa. The Labour Cabinet Secretary has warned young people against using visitor visas to look for work abroad — it leaves you without legal protection and exposed to exploitation.",
		DetailSW: "Ofa hii inahusisha kusafiri kwa viza ya mgeni au ya utalii. Waziri wa Leba ameonya vijana dhidi ya kutumia viza za wageni kutafuta kazi nje — inakuacha bila ulinzi wa kisheria na hatarini kunyonywa.",
		Sources:  []domain.Source{srcVisitorVisa},
	}}
}

func jobMilitary(in Input) []domain.Finding {
	if in.Facts.JobCategory != domain.JobMilitary && !strings.EqualFold(in.Facts.DestinationCountry, "Russia") {
		return nil
	}
	return []domain.Finding{{
		Severity: domain.SevCritical,
		TitleEN:  "Possible recruitment into a war zone",
		TitleSW:  "Huenda ni uajiri wa kupelekwa vitani",
		DetailEN: "The government has warned that Kenyans have been lured with job offers and ended up recruited into the Russia–Ukraine war, with injuries and people stranded. Treat any “job” in Russia or any military / security-contractor role with extreme caution.",
		DetailSW: "Serikali imeonya kuwa Wakenya wamevutwa kwa ofa za kazi na kuishia kuandikishwa katika vita vya Urusi na Ukraine, wengine wakijeruhiwa na kukwama. Kuwa mwangalifu sana na “kazi” yoyote nchini Urusi au kazi yoyote ya kijeshi.",
		Evidence: firstNonEmpty(in.Facts.JobTitle, in.Facts.DestinationCountry),
		Sources:  []domain.Source{srcRussia},
	}}
}

// asiaRisk: destinations named in government warnings about fake adverts
// "purported to be in the Asian continent". Keep this list sourced.
var asiaRisk = map[string]bool{"Myanmar": true, "Thailand": true, "Laos": true, "Cambodia": true}

func destinationRisk(in Input) []domain.Finding {
	if !asiaRisk[in.Facts.DestinationCountry] {
		return nil
	}
	return []domain.Finding{{
		Severity: domain.SevHigh,
		TitleEN:  "Destination matches a known scam pattern",
		TitleSW:  "Nchi inayotajwa inalingana na mtindo wa utapeli unaojulikana",
		DetailEN: fmt.Sprintf("The government has warned about fake online adverts for jobs in Asia that lead to exploitation and trafficking. Be extremely careful with offers for %s.", in.Facts.DestinationCountry),
		DetailSW: fmt.Sprintf("Serikali imeonya kuhusu matangazo bandia ya kazi barani Asia yanayoishia kwenye unyonyaji na usafirishaji haramu wa binadamu. Kuwa mwangalifu sana na ofa za %s.", in.Facts.DestinationCountry),
		Evidence: in.Facts.DestinationCountry,
		Sources:  []domain.Source{srcAsiaAdverts},
	}}
}

func contactInformal(in Input) []domain.Finding {
	f := in.Facts
	if f.HasPhysicalOffice != nil && *f.HasPhysicalOffice {
		return nil
	}
	social := 0
	formal := len(f.Websites) > 0
	for _, e := range f.Emails {
		d := strings.ToLower(e[strings.LastIndex(e, "@")+1:])
		if d != "gmail.com" && d != "yahoo.com" && d != "outlook.com" && d != "hotmail.com" {
			formal = true
		}
	}
	for _, c := range f.ContactChannels {
		switch c {
		case "whatsapp", "telegram", "facebook", "tiktok", "instagram":
			social++
		case "office":
			formal = true
		}
	}
	if social == 0 || formal {
		return nil
	}
	return []domain.Finding{{
		Severity: domain.SevMedium,
		TitleEN:  "Contact only through social media or WhatsApp",
		TitleSW:  "Mawasiliano ni kupitia mitandao ya kijamii au WhatsApp tu",
		DetailEN: "No office address, company website or company email is given. Licensed agencies must have a physical office; be careful with recruiters who only meet in cafés or chat on WhatsApp.",
		DetailSW: "Hakuna anwani ya ofisi, tovuti ya kampuni wala barua pepe ya kampuni. Mawakala wenye leseni lazima wawe na ofisi; kuwa mwangalifu na wanaokutana nawe mikahawani au kuongea kwenye WhatsApp tu.",
		Evidence: strings.Join(f.ContactChannels, ", "),
		Sources:  []domain.Source{srcHowToCheck},
	}}
}

func pressureUrgency(in Input) []domain.Finding {
	if len(in.Facts.UrgencyCues) == 0 {
		return nil
	}
	cues := strings.Join(in.Facts.UrgencyCues, "”, “")
	return []domain.Finding{{
		Severity: domain.SevMedium,
		TitleEN:  "Pressure to decide or pay quickly",
		TitleSW:  "Shinikizo la kuamua au kulipa haraka",
		DetailEN: fmt.Sprintf("The message pushes you to act fast (“%s”). Genuine employers give you time to verify. Pressure is a common way to stop people from checking.", cues),
		DetailSW: fmt.Sprintf("Ujumbe unakusukuma kuchukua hatua haraka (“%s”). Waajiri halali hukupa muda wa kuthibitisha. Shinikizo ni njia ya kawaida ya kuzuia watu kuchunguza.", cues),
		Evidence: cues,
		Sources:  []domain.Source{srcAsiaAdverts},
	}}
}

var scamWords = []string{"scam", "fraud", "fake", "conned", "duped", "deregister", "arrest", "not registered", "unlicensed", "rogue", "matapeli", "utapeli"}

func webReports(in Input) []domain.Finding {
	name := registry.Normalize(in.Facts.AgencyName)
	var phones []string
	for _, p := range in.Facts.Phones {
		if n := phoneKey(p); n != "" && !placeholderPhone(n) {
			phones = append(phones, n)
		}
	}
	if name == "" && len(phones) == 0 {
		return nil
	}
	var srcs []domain.Source
	var evidence []string
	for _, h := range in.Hits {
		if listingSite(h.URL) {
			continue
		}
		raw := stripBoilerplate(h.Title + " " + h.Snippet)
		// The agency's name must appear close to a scam word: a general
		// article on job scams that lists many agencies is not a report about
		// each of them.
		hit := name != "" && near(flatten(raw), name, scamWords, warningWindow)
		// Phone numbers are compared as whole numbers found in the text, never
		// as digits run together across separate numbers or dates.
		if !hit && len(phones) > 0 && containsAny(strings.ToLower(raw), scamWords...) {
			found := map[string]bool{}
			for _, t := range phoneInText.FindAllString(raw, -1) {
				found[phoneKey(t)] = true
			}
			for _, p := range phones {
				if found[p] {
					hit = true
				}
			}
		}
		if !hit {
			continue
		}
		srcs = append(srcs, domain.Source{Title: h.Title, URL: h.URL})
		evidence = append(evidence, h.Title)
		if len(srcs) == 3 {
			break
		}
	}
	if len(srcs) == 0 {
		return nil
	}
	return []domain.Finding{{
		Severity: domain.SevHigh,
		TitleEN:  "Linked to fraud reports online",
		TitleSW:  "Imehusishwa na ripoti za utapeli mtandaoni",
		DetailEN: "We found online reports that mention this agency or phone number together with fraud or scam warnings. Read them before you go further.",
		DetailSW: "Tumepata ripoti mtandaoni zinazotaja wakala huyu au nambari hii ya simu pamoja na maonyo ya utapeli. Zisome kabla ya kuendelea.",
		Evidence: strings.Join(evidence, " | "),
		Sources:  srcs,
	}}
}

// listingHosts are business directories and "trust score" sites. Their
// pages are profiles of each company, built from anonymous or automated
// input, and their menus say "report fake listing" or "is it a scam?" on
// every page. They are not fraud reports, so they never count as one.
var listingHosts = []string{
	"trustscore.co.ke", "scamadviser.com", "scam-detector.com", "trustpilot.com",
	"sitejabber.com", "scamdoc.com", "gridinsoft.com", "kenyabizdirectory.co.ke",
	"businesslist.co.ke", "yellowpageskenya.com", "cybo.com", "infoisinfo.co.ke",
}

func listingSite(u string) bool {
	h := host(u)
	for _, l := range listingHosts {
		if h == l || strings.HasSuffix(h, "."+l) {
			return true
		}
	}
	return false
}

// boilerplate matches site chrome that contains scam words without saying
// anything about the company: "Report a listing that may be fake",
// "Is X a scam or legit?", "Report a scam", "How to avoid job scams".
var boilerplate = regexp.MustCompile(`(?i)^\s*(?:(?:report|flag)\s+(?:a|an|this|the)?\s*(?:listing|profile|business|company|review|scam|fraud|problem|abuse|website)\b|is\b.{0,80}\b(?:a\s+)?(?:scam|fake|legit)\b.{0,20}\?)|\b(?:scam or legit|legit or scam|real or fake|fake or real|scam check|may be fake)\b`)

var sentenceSplit = regexp.MustCompile(`[.!?\n]+`)

// stripBoilerplate drops every sentence that is site chrome (see boilerplate).
func stripBoilerplate(s string) string {
	var keep []string
	for _, sent := range sentenceSplit.Split(s, -1) {
		if strings.TrimSpace(sent) == "" || boilerplate.MatchString(sent) {
			continue
		}
		keep = append(keep, sent)
	}
	return strings.Join(keep, ". ")
}

// phoneInText finds Kenyan mobile numbers written in any common style.
var phoneInText = regexp.MustCompile(`(?:\+?254|\b0)[\s-]?[17]\d{2}[\s-]?\d{3}[\s-]?\d{3}\b`)

// phoneKey reduces a number to its last 9 digits ("0712 345 678" → "712345678").
func phoneKey(p string) string {
	d := digits(p)
	if len(d) < 9 {
		return ""
	}
	return d[len(d)-9:]
}

// placeholderPhone spots dummy numbers used in examples and awareness
// material (0712 345 678, 0700 000 000, 0711 111 111): matching those
// says nothing about a real scammer.
func placeholderPhone(n string) bool {
	tail := n[1:] // after the leading 7 or 1
	if strings.Contains("0123456789", tail[:6]) || strings.Contains("9876543210", tail[:6]) {
		return true
	}
	distinct := map[rune]bool{}
	for _, r := range tail {
		distinct[r] = true
	}
	return len(distinct) <= 2
}

func contractAttestation(in Input) []domain.Finding {
	sev := domain.SevInfo
	title, titleSW := "Get your contract attested before you travel", "Hakikisha mkataba wako umethibitishwa kabla ya kusafiri"
	if in.Facts.ContractAttested != nil && !*in.Facts.ContractAttested {
		sev = domain.SevHigh
		title, titleSW = "Contract is not attested", "Mkataba haujathibitishwa"
	}
	return []domain.Finding{{
		Severity: sev,
		TitleEN:  title,
		TitleSW:  titleSW,
		DetailEN: fmt.Sprintf("Foreign employment contracts must be attested by labour officers at the Ministry of Labour before you buy a ticket. Attestation is done online through NEAIMS. If unsure, call the National Employment Authority on %s.", NEAHotline),
		DetailSW: fmt.Sprintf("Mikataba ya ajira ya nje lazima ithibitishwe na maafisa wa leba wa Wizara ya Leba kabla ya kununua tiketi. Uthibitishaji hufanywa mtandaoni kupitia NEAIMS. Ukiwa na shaka, piga simu NEA kupitia %s.", NEAHotline),
		Sources:  []domain.Source{srcAttestation, srcOmanForgery},
	}}
}

// ---------------------------------------------------------------------------

func feeSummary(fees []domain.Fee) string {
	var parts []string
	for _, f := range fees {
		s := fmt.Sprintf("%s %s", f.Currency, formatAmount(f.Amount))
		if f.Purpose != "" && f.Purpose != "unspecified" {
			s += " (" + f.Purpose + ")"
		}
		parts = append(parts, strings.TrimSpace(s))
	}
	return strings.Join(parts, " + ")
}

func formatAmount(v float64) string {
	n := int64(v + 0.5)
	s := fmt.Sprintf("%d", n)
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	return b.String()
}

func humanCategory(c domain.JobCategory) string {
	if c == domain.JobDomestic {
		return "domestic"
	}
	return "unskilled"
}

func humanCategorySW(c domain.JobCategory) string {
	if c == domain.JobDomestic {
		return "ya nyumbani"
	}
	return "isiyohitaji ujuzi"
}

func containsAny(s string, subs ...string) bool {
	for _, x := range subs {
		if strings.Contains(s, x) {
			return true
		}
	}
	return false
}

func digits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}

// ---------------------------------------------------------------------------
// Live checks (Tavily)
// ---------------------------------------------------------------------------

var warningWords = []string{
	"not registered", "unregistered", "deregistered", "de registered", "revoked", "suspended",
	"fake", "fraud", "unlicensed", "not licensed", "not authorised", "not authorized",
	"blacklist", "struck off", "warning", "rogue", "closed down",
}

// warningWindow is how close (in characters) a warning word must be to the
// agency's name. Official pages often list many agencies; proximity stops a
// licensed agency being accused because a deregistered one is on the same page.
const warningWindow = 220

func officialWarning(in Input) []domain.Finding {
	name := registry.Normalize(in.Facts.AgencyName)
	if name == "" {
		return nil
	}
	var srcs []domain.Source
	var evidence []string
	for _, h := range in.Official {
		text := flatten(h.Title + " " + h.Snippet)
		if near(text, name, warningWords, warningWindow) {
			srcs = append(srcs, domain.Source{Title: h.Title, URL: h.URL})
			evidence = append(evidence, h.Title)
		}
		if len(srcs) == 3 {
			break
		}
	}
	if len(srcs) > 0 {
		return []domain.Finding{{
			Severity: domain.SevCritical,
			TitleEN:  "Named in a warning on a government website",
			TitleSW:  "Ametajwa kwenye onyo katika tovuti ya serikali",
			DetailEN: fmt.Sprintf("A Kenyan government website mentions “%s” together with a warning (for example deregistered, not registered or fake). Read the official page before you go further.", in.Facts.AgencyName),
			DetailSW: fmt.Sprintf("Tovuti ya serikali ya Kenya inamtaja “%s” pamoja na onyo (kwa mfano amefutiwa usajili, hajasajiliwa au ni bandia). Soma ukurasa rasmi kabla ya kuendelea.", in.Facts.AgencyName),
			Evidence: strings.Join(evidence, " | "),
			Sources:  srcs,
		}}
	}
	if in.OfficialSearched {
		return []domain.Finding{{
			Severity: domain.SevPass,
			TitleEN:  "No warnings found on government websites",
			TitleSW:  "Hakuna maonyo yaliyopatikana kwenye tovuti za serikali",
			DetailEN: fmt.Sprintf("We searched Labour Ministry, NEA and Diaspora Affairs websites for “%s” just now and found no warning about it. New warnings can take time to appear, so this alone does not prove the agency is safe.", in.Facts.AgencyName),
			DetailSW: fmt.Sprintf("Tumetafuta “%s” sasa hivi kwenye tovuti za Wizara ya Leba, NEA na Masuala ya Diaspora na hatukupata onyo lolote. Maonyo mapya huchukua muda kuonekana, kwa hiyo hili peke yake halithibitishi kuwa wakala ni salama.", in.Facts.AgencyName),
			Sources:  []domain.Source{srcNEAList},
		}}
	}
	return nil
}

// SalaryRatioThreshold: pay at least this many times the top of the typical
// range is flagged. Set high on purpose — exchange rates are approximate and
// genuine overseas jobs do pay more than at home.
const SalaryRatioThreshold = 2.0

// SalaryAboveRange: pay more than this many times the top of the typical
// range is reported as "higher than typical" (medium) rather than as a pass.
// Below it the ad is within a believable range, allowing for rough exchange
// rates and the spread of salary data.
const SalaryAboveRange = 1.2

func salaryTooGood(in Input) []domain.Finding {
	b := in.Salary
	if b == nil || !b.Found || b.Ratio <= 0 {
		return nil
	}
	typical := fmt.Sprintf("%s %s–%s a month", b.Currency, formatAmount(b.MonthlyLow), formatAmount(b.MonthlyHigh))
	offered := fmt.Sprintf("%s %s a month", b.AdCurrency, formatAmount(b.AdMonthly))
	if b.Ratio < SalaryRatioThreshold && b.Ratio > SalaryAboveRange {
		return []domain.Finding{{
			Severity: domain.SevMedium,
			TitleEN:  "Salary is higher than typical",
			TitleSW:  "Mshahara ni wa juu kuliko kawaida",
			DetailEN: fmt.Sprintf("The ad offers %s — about %.1f times the top of the typical range for a %s in %s (%s). Ask the agency to explain the pay, and make sure it is written in your contract.", offered, b.Ratio, b.JobTitle, b.Country, typical),
			DetailSW: fmt.Sprintf("Tangazo linatoa %s — takriban mara %.1f ya kiwango cha juu cha malipo ya kawaida ya %s nchini %s (%s). Muulize wakala aeleze mshahara huu, na uhakikishe umeandikwa kwenye mkataba wako.", offered, b.Ratio, b.JobTitle, b.Country, typical),
			Evidence: offered,
			Sources:  b.Sources,
		}}
	}
	if b.Ratio <= SalaryAboveRange {
		return []domain.Finding{{
			Severity: domain.SevPass,
			TitleEN:  "Salary is within a believable range",
			TitleSW:  "Mshahara uko katika kiwango kinachoaminika",
			DetailEN: fmt.Sprintf("The ad offers %s. Typical pay for a %s in %s is about %s.", offered, b.JobTitle, b.Country, typical),
			DetailSW: fmt.Sprintf("Tangazo linatoa %s. Malipo ya kawaida ya %s nchini %s ni takriban %s.", offered, b.JobTitle, b.Country, typical),
			Sources:  b.Sources,
		}}
	}
	return []domain.Finding{{
		Severity: domain.SevHigh,
		TitleEN:  "Salary looks too good to be true",
		TitleSW:  "Mshahara unaonekana mzuri kupita kiasi",
		DetailEN: fmt.Sprintf("The ad offers %s — about %.1f times the top of the typical range for a %s in %s (%s). Unusually high pay is a common way to hook people into paying fees.", offered, b.Ratio, b.JobTitle, b.Country, typical),
		DetailSW: fmt.Sprintf("Tangazo linatoa %s — takriban mara %.1f ya kiwango cha juu cha malipo ya kawaida ya %s nchini %s (%s). Mshahara mkubwa usio wa kawaida ni njia ya kawaida ya kuwavutia watu walipe ada.", offered, b.Ratio, b.JobTitle, b.Country, typical),
		Evidence: offered,
		Sources:  b.Sources,
	}}
}

// flatten normalises text exactly as registry.Normalize normalises names
// (lowercase, punctuation to spaces, legal suffixes dropped), so an agency
// name can be found in it directly.
func flatten(s string) string {
	return " " + registry.Normalize(s) + " "
}

// near reports whether any word appears within window characters of an
// occurrence of name in text. Both must already be flattened/normalized.
func near(text, name string, words []string, window int) bool {
	needle := " " + name + " "
	for i := 0; ; {
		j := strings.Index(text[i:], needle)
		if j < 0 {
			return false
		}
		pos := i + j
		lo, hi := max(0, pos-window), min(len(text), pos+len(needle)+window)
		seg := text[lo:hi]
		for _, w := range words {
			if strings.Contains(seg, " "+w+" ") || strings.Contains(seg, " "+w) {
				return true
			}
		}
		i = pos + 1
	}
}

// ---------------------------------------------------------------------------
// Register details
// ---------------------------------------------------------------------------

// agencyContact compares the email addresses and websites in the ad with
// those the register lists for the matched agency. Copying a licensed
// agency's name and number is easy; receiving mail at its registered address
// is not.
func agencyContact(in Input) []domain.Finding {
	m := in.Registry
	if m.Agency == nil || (m.Kind != domain.MatchExact && m.Kind != domain.MatchLicenceNo) {
		return nil // lookalikes and unknowns are handled by agency.registry
	}
	a := m.Agency
	regEmail := strings.ToLower(strings.TrimSpace(a.Email))
	regSites := map[string]bool{}
	if d := emailDomain(regEmail); d != "" && !freeMail[d] {
		regSites[d] = true
	}
	if h := host(a.Website); h != "" {
		regSites[h] = true
	}
	if regEmail == "" && len(regSites) == 0 {
		return nil // nothing registered to compare with
	}

	var matched, other []string
	for _, e := range in.Facts.Emails {
		e = strings.ToLower(strings.TrimSpace(e))
		if e == "" {
			continue
		}
		if e == regEmail || sameSite(emailDomain(e), regSites) && !freeMail[emailDomain(e)] {
			matched = append(matched, e)
		} else {
			other = append(other, e)
		}
	}
	for _, w := range in.Facts.Websites {
		h := host(w)
		if h == "" || socialHosts[h] {
			continue
		}
		if sameSite(h, regSites) {
			matched = append(matched, h)
		} else {
			other = append(other, h)
		}
	}
	if len(matched) == 0 && len(other) == 0 {
		return nil // the ad gives no email or website
	}

	registered := describeContacts(a)
	en, sw := registerDates(m.Register)
	if len(matched) > 0 {
		return []domain.Finding{{
			Severity: domain.SevPass,
			TitleEN:  "Contact details match the NEA register",
			TitleSW:  "Mawasiliano yanalingana na rejista ya NEA",
			DetailEN: fmt.Sprintf("The ad uses %s, which matches what the register lists for “%s”.", strings.Join(matched, ", "), a.Name),
			DetailSW: fmt.Sprintf("Tangazo linatumia %s, ambayo inalingana na yaliyoorodheshwa kwenye rejista kwa “%s”.", strings.Join(matched, ", "), a.Name),
			Sources:  []domain.Source{registerSource(m.Register)},
		}}
	}
	return []domain.Finding{{
		Severity: domain.SevHigh,
		TitleEN:  "Contact details don't match the NEA register",
		TitleSW:  "Mawasiliano hayalingani na rejista ya NEA",
		DetailEN: fmt.Sprintf("The register lists %s for “%s”%s, but this ad uses %s. Scammers often copy a licensed agency's name and licence number but give their own email or website. Contact the agency using the register's details and ask whether this job is really theirs.", registered, a.Name, en, strings.Join(other, ", ")),
		DetailSW: fmt.Sprintf("Rejista inaorodhesha %s kwa “%s”%s, lakini tangazo hili linatumia %s. Matapeli mara nyingi hunakili jina na nambari ya leseni ya wakala halali lakini hutoa barua pepe au tovuti yao wenyewe. Wasiliana na wakala ukitumia maelezo ya rejista na uulize kama kazi hii ni yao kweli.", registered, a.Name, sw, strings.Join(other, ", ")),
		Evidence: strings.Join(other, ", "),
		Sources:  []domain.Source{registerSource(m.Register)},
	}}
}

var freeMail = map[string]bool{
	"gmail.com": true, "googlemail.com": true, "yahoo.com": true, "ymail.com": true,
	"outlook.com": true, "hotmail.com": true, "live.com": true, "msn.com": true,
	"icloud.com": true, "aol.com": true, "proton.me": true, "protonmail.com": true,
	"mail.com": true, "gmx.com": true,
}

// socialHosts are links to chat apps and social networks, not agency sites.
var socialHosts = map[string]bool{
	"wa.me": true, "whatsapp.com": true, "chat.whatsapp.com": true, "facebook.com": true,
	"fb.com": true, "m.facebook.com": true, "instagram.com": true, "tiktok.com": true,
	"t.me": true, "telegram.me": true, "x.com": true, "twitter.com": true,
	"linkedin.com": true, "youtube.com": true, "youtu.be": true,
}

func emailDomain(e string) string {
	i := strings.LastIndex(e, "@")
	if i < 0 {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(e[i+1:]))
}

// host reduces "https://www.Agency.co.ke/jobs?x=1" to "agency.co.ke".
func host(u string) string {
	u = strings.ToLower(strings.TrimSpace(u))
	if u == "" || u == "none" || u == "n/a" {
		return ""
	}
	if i := strings.Index(u, "://"); i >= 0 {
		u = u[i+3:]
	}
	if i := strings.IndexAny(u, "/?#"); i >= 0 {
		u = u[:i]
	}
	if i := strings.LastIndex(u, ":"); i >= 0 {
		u = u[:i]
	}
	u = strings.TrimPrefix(u, "www.")
	if !strings.Contains(u, ".") {
		return ""
	}
	return u
}

// sameSite reports whether h is one of the registered sites or a subdomain of one.
func sameSite(h string, sites map[string]bool) bool {
	if h == "" {
		return false
	}
	for s := range sites {
		if h == s || strings.HasSuffix(h, "."+s) {
			return true
		}
	}
	return false
}

func describeContacts(a *domain.Agency) string {
	var parts []string
	if a.Email != "" {
		parts = append(parts, "“"+a.Email+"”")
	}
	if a.Website != "" {
		parts = append(parts, "“"+a.Website+"”")
	}
	return strings.Join(parts, " and ")
}

// registerDates returns a phrase like " (register published 19 May 2026;
// our copy taken 27 Oct 2026)" in English and Kiswahili, or "" when unknown.
func registerDates(r domain.RegisterInfo) (en, sw string) {
	var e, s []string
	if !r.PublishedAt.IsZero() {
		e = append(e, "register published "+r.PublishedAt.Format("2 Jan 2006"))
		s = append(s, "rejista ilichapishwa "+dateSW(r.PublishedAt))
	}
	if !r.SnapshotAt.IsZero() {
		e = append(e, "our copy taken "+r.SnapshotAt.Format("2 Jan 2006"))
		s = append(s, "nakala yetu ilichukuliwa "+dateSW(r.SnapshotAt))
	}
	if len(e) == 0 {
		return "", ""
	}
	return " (" + strings.Join(e, "; ") + ")", " (" + strings.Join(s, "; ") + ")"
}

var swMonths = [...]string{"Januari", "Februari", "Machi", "Aprili", "Mei", "Juni",
	"Julai", "Agosti", "Septemba", "Oktoba", "Novemba", "Desemba"}

func dateSW(t time.Time) string {
	return fmt.Sprintf("%d %s %d", t.Day(), swMonths[t.Month()-1], t.Year())
}

// registerSource links to the register copy we actually used, when known.
func registerSource(r domain.RegisterInfo) domain.Source {
	if r.SourceURL == "" {
		return srcNEAList
	}
	title := "National Employment Authority register of agencies"
	var notes []string
	if h := host(r.SourceURL); h != "" {
		notes = append(notes, h)
	}
	if !r.PublishedAt.IsZero() {
		notes = append(notes, "published "+r.PublishedAt.Format("2 Jan 2006"))
	}
	if len(notes) > 0 {
		title += " (" + strings.Join(notes, ", ") + ")"
	}
	return domain.Source{Title: title, URL: r.SourceURL}
}
