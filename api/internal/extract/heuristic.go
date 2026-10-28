package extract

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/edkariuki/kazi-halali/api/internal/domain"
)

// Heuristic is a deterministic, regex-based extractor. It is the fallback
// when a model call fails and the engine behind LLM_MODE=stub. It is
// deliberately conservative: it would rather miss a fact than invent one.
func Heuristic(text string) domain.Facts {
	lower := strings.ToLower(text)
	f := domain.Facts{
		JobCategory:  domain.JobUnknown,
		VisaType:     domain.VisaUnknown,
		Language:     detectLanguage(lower),
		DocumentKind: "ad",
	}

	if m := agencyRe.FindStringSubmatch(text); m != nil {
		f.AgencyName = strings.TrimSpace(m[1])
	}
	if m := licenceRe.FindString(text); m != "" {
		f.AgencyLicenceNo = strings.ToUpper(strings.Join(strings.Fields(m), ""))
	}
	f.Phones = uniq(phoneRe.FindAllString(text, -1))
	f.Emails = uniq(emailRe.FindAllString(text, -1))
	f.Websites = uniq(urlRe.FindAllString(text, -1))

	for _, ch := range []string{"whatsapp", "telegram", "facebook", "tiktok", "instagram"} {
		if strings.Contains(lower, ch) || (ch == "whatsapp" && strings.Contains(lower, "wa.me")) {
			f.ContactChannels = append(f.ContactChannels, ch)
		}
	}
	if strings.Contains(lower, "office") || strings.Contains(lower, "ofisi") {
		f.ContactChannels = append(f.ContactChannels, "office")
	}

	for _, c := range countries {
		for _, alias := range c.aliases {
			if strings.Contains(lower, alias) {
				f.DestinationCountry = c.name
				break
			}
		}
		if f.DestinationCountry != "" {
			break
		}
	}

	f.JobCategory, f.JobTitle = classifyJob(lower)

	switch {
	case containsAny(lower, "visitor visa", "visit visa", "tourist visa", "viza ya utalii", "visiting visa"):
		f.VisaType = domain.VisaVisitor
	case containsAny(lower, "work permit", "work visa", "employment visa", "kibali cha kazi"):
		f.VisaType = domain.VisaWork
	}

	// Money: split into sentences/lines, then decide fee vs salary by keywords.
	for _, seg := range splitSegments(text) {
		ls := strings.ToLower(seg)
		amounts := moneyRe.FindAllStringSubmatch(seg, -1)
		if len(amounts) == 0 {
			continue
		}
		isFee := containsAny(ls, "fee", "pay ", "payment", "processing", "registration", "lipa", "ada", "commitment", "medical", "deposit", "send ")
		isSalary := containsAny(ls, "salary", "mshahara", "per month", "monthly", "kwa mwezi", "earn")
		for _, a := range amounts {
			m, ok := parseMoney(a)
			if !ok {
				continue
			}
			switch {
			case isSalary && !isFee && f.Salary == nil:
				m.Period = "month"
				f.Salary = &m
			case isFee:
				before := true
				f.Fees = append(f.Fees, domain.Fee{Money: m, Purpose: feePurpose(ls), PayableBeforeContract: &before})
			}
		}
	}

	for _, cue := range urgencyCues {
		if strings.Contains(lower, cue) {
			f.UrgencyCues = append(f.UrgencyCues, cue)
		}
	}
	return f
}

var (
	// Name words may be initials ("A.F.") or "&"; the legal suffix may be in capitals.
	agencyRe = regexp.MustCompile(`((?:(?:[A-Z][A-Za-z&'.-]*|&)\s+){1,7}(?i:Ltd|Limited|Agency|Agencies|Consultants|Recruiters|Recruitment|Placements))\b`)
	// Real NEA numbers look like NA/2025/11/3 or RA/2025/11/61; older ones NEA/PEA/0001.
	licenceRe = regexp.MustCompile(`(?i)\b(?:(?:NA|RA)\s*/\s*\d{4}\s*/\s*\d{1,2}\s*/\s*\d{1,3}|NEA\s*[/\-]\s*[A-Z/\-]*\s*\d{2,})\b`)
	phoneRe   = regexp.MustCompile(`(?:\+?254|0)\s?(?:7|1)\d{2}\s?\d{3}\s?\d{3}`)
	emailRe   = regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`)
	urlRe     = regexp.MustCompile(`(?i)\bhttps?://[^\s)]+|\bwww\.[^\s)]+|\bwa\.me/[^\s)]+`)
	moneyRe   = regexp.MustCompile(`(?i)\b(KES|KSH|KSh|Ksh|Sh|USD|US\$|\$|CAD|QAR|SAR|AED|OMR|EUR|GBP|€|£)\.?\s?([\d][\d,]*(?:\.\d+)?)\s?(k)?\b|\b([\d][\d,]*(?:\.\d+)?)\s?(k)?\s?(KES|KSH|Ksh|bob|USD|CAD|QAR|SAR|AED|OMR|EUR|GBP)\b`)
)

var currencyMap = map[string]string{
	"kes": "KES", "ksh": "KES", "sh": "KES", "bob": "KES",
	"usd": "USD", "us$": "USD", "$": "USD",
	"cad": "CAD", "qar": "QAR", "sar": "SAR", "aed": "AED", "omr": "OMR",
	"eur": "EUR", "€": "EUR", "gbp": "GBP", "£": "GBP",
}

func parseMoney(m []string) (domain.Money, bool) {
	cur, num, k := m[1], m[2], m[3]
	if num == "" {
		num, k, cur = m[4], m[5], m[6]
	}
	v, err := strconv.ParseFloat(strings.ReplaceAll(num, ",", ""), 64)
	if err != nil || v <= 0 {
		return domain.Money{}, false
	}
	if strings.EqualFold(k, "k") {
		v *= 1000
	}
	code, ok := currencyMap[strings.ToLower(cur)]
	if !ok {
		code = strings.ToUpper(cur)
	}
	return domain.Money{Amount: v, Currency: code}, true
}

func feePurpose(s string) string {
	for _, p := range []string{"processing", "registration", "medical", "visa", "training", "ticket", "commitment", "deposit"} {
		if strings.Contains(s, p) {
			return p
		}
	}
	return "unspecified"
}

// jobTerm maps a word in an ad to a job category and a plain English job
// title (used for salary searches). Order matters: the first match wins.
type jobTerm struct {
	word  string
	cat   domain.JobCategory
	title string
}

var jobTerms = []jobTerm{
	{"soldier", domain.JobMilitary, "soldier"},
	{"military", domain.JobMilitary, "soldier"},
	{"army", domain.JobMilitary, "soldier"},
	{"combat", domain.JobMilitary, "soldier"},
	{"frontline", domain.JobMilitary, "soldier"},
	{"war zone", domain.JobMilitary, "soldier"},
	{"house help", domain.JobDomestic, "housemaid"},
	{"househelp", domain.JobDomestic, "housemaid"},
	{"housemaid", domain.JobDomestic, "housemaid"},
	{"house maid", domain.JobDomestic, "housemaid"},
	{"house girl", domain.JobDomestic, "housemaid"},
	{"housegirl", domain.JobDomestic, "housemaid"},
	{"nanny", domain.JobDomestic, "nanny"},
	{"domestic worker", domain.JobDomestic, "domestic worker"},
	{"maid", domain.JobDomestic, "housemaid"},
	{"mfanyakazi wa nyumbani", domain.JobDomestic, "domestic worker"},
	{"kazi za nyumbani", domain.JobDomestic, "domestic worker"},
	{"kazi ya nyumbani", domain.JobDomestic, "domestic worker"},
	{"msaidizi wa nyumbani", domain.JobDomestic, "domestic worker"},
	{"mfanyakazi wa ndani", domain.JobDomestic, "domestic worker"},
	{"cleaner", domain.JobUnskilled, "cleaner"},
	{"farm worker", domain.JobUnskilled, "farm worker"},
	{"farm hand", domain.JobUnskilled, "farm worker"},
	{"fruit picker", domain.JobUnskilled, "fruit picker"},
	{"picker", domain.JobUnskilled, "fruit picker"},
	{"packer", domain.JobUnskilled, "packer"},
	{"general worker", domain.JobUnskilled, "general labourer"},
	{"labourer", domain.JobUnskilled, "general labourer"},
	{"laborer", domain.JobUnskilled, "general labourer"},
	{"loader", domain.JobUnskilled, "loader"},
	{"shamba", domain.JobUnskilled, "farm worker"},
	{"security guard", domain.JobSemiSkilled, "security guard"},
	{"driver", domain.JobSemiSkilled, "driver"},
	{"care assistant", domain.JobSemiSkilled, "care assistant"},
	{"caregiver", domain.JobSemiSkilled, "caregiver"},
	{"waitress", domain.JobSemiSkilled, "waiter"},
	{"waiter", domain.JobSemiSkilled, "waiter"},
	{"warehouse", domain.JobSemiSkilled, "warehouse worker"},
	{"nurse", domain.JobSkilled, "nurse"},
	{"engineer", domain.JobSkilled, "engineer"},
	{"teacher", domain.JobSkilled, "teacher"},
	{"electrician", domain.JobSkilled, "electrician"},
	{"accountant", domain.JobSkilled, "accountant"},
	{"chef", domain.JobSkilled, "chef"},
	{"doctor", domain.JobSkilled, "doctor"},
	{"pharmacist", domain.JobSkilled, "pharmacist"},
	{"plumber", domain.JobSkilled, "plumber"},
	{"welder", domain.JobSkilled, "welder"},
}

// classifyJob returns the job category and a plain job title for the first
// job word found in s (already lowercased).
func classifyJob(s string) (domain.JobCategory, string) {
	for _, t := range jobTerms {
		if strings.Contains(s, t.word) {
			return t.cat, t.title
		}
	}
	return domain.JobUnknown, ""
}

type country struct {
	name    string
	aliases []string
}

var countries = []country{
	{"Canada", []string{"canada"}},
	{"Qatar", []string{"qatar", "doha"}},
	{"Saudi Arabia", []string{"saudi", "riyadh", "jeddah", "ksa"}},
	{"United Arab Emirates", []string{"uae", "dubai", "abu dhabi", "sharjah"}},
	{"Oman", []string{"oman", "muscat"}},
	{"Kuwait", []string{"kuwait"}},
	{"Bahrain", []string{"bahrain"}},
	{"Lebanon", []string{"lebanon", "beirut"}},
	{"Jordan", []string{"jordan"}},
	{"Israel", []string{"israel"}},
	{"Russia", []string{"russia", "moscow", "urusi"}},
	{"Germany", []string{"germany", "ujerumani"}},
	{"Poland", []string{"poland"}},
	{"United Kingdom", []string{"united kingdom", " uk ", "london", "uingereza"}},
	{"United States", []string{"usa", "united states", "america", "marekani"}},
	{"Australia", []string{"australia"}},
	{"Japan", []string{"japan"}},
	{"Thailand", []string{"thailand", "bangkok"}},
	{"Myanmar", []string{"myanmar", "burma"}},
	{"Laos", []string{"laos"}},
	{"Cambodia", []string{"cambodia"}},
}

var urgencyCues = []string{
	"limited slots", "few slots", "slots remaining", "only today", "pay today", "pay now",
	"urgent", "hurry", "first come", "deadline today", "travel next week", "travel this week",
	"haraka", "leo tu", "nafasi chache",
}

func detectLanguage(s string) string {
	sw := 0
	for _, w := range []string{" na ", " kwa ", " kazi ", " ya ", " wa ", "mshahara", " ni ", "lipa", "nafasi", "tuma"} {
		if strings.Contains(" "+s+" ", w) {
			sw++
		}
	}
	switch {
	case sw >= 4:
		return "sw"
	case sw >= 2:
		return "mixed"
	}
	return "en"
}

func splitSegments(s string) []string {
	return regexp.MustCompile(`[\n.!?;]+`).Split(s, -1)
}

func containsAny(s string, subs ...string) bool {
	for _, x := range subs {
		if strings.Contains(s, x) {
			return true
		}
	}
	return false
}

func uniq(in []string) []string {
	return union(nil, in)
}
