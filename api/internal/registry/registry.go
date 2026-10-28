// Package registry matches the agency named in a job offer against the NEA
// licensed-agency list and against official "flagged / not registered" notices.
//
// The whole register is small (hundreds of rows), so it is held in memory and
// matched in Go; the database is only the system of record.
package registry

import (
	"strings"
	"unicode"

	"github.com/edkariuki/kazi-halali/api/internal/domain"
)

// LookalikeThreshold: names at least this similar to a licensed agency — but
// not the same — are reported as lookalikes. Borrowing a real agency's name
// with a small change is a known tactic.
const LookalikeThreshold = 0.84

// FlaggedThreshold is stricter: a false "flagged" claim is worse than a miss.
const FlaggedThreshold = 0.92

type Registry struct {
	agencies []domain.Agency
	flagged  []domain.FlaggedEntry
	byLic    map[string]*domain.Agency
	info     domain.RegisterInfo
}

func New(agencies []domain.Agency, flagged []domain.FlaggedEntry) *Registry {
	r := &Registry{byLic: map[string]*domain.Agency{}}
	for _, a := range agencies {
		a.NormalizedName = Normalize(a.Name)
		r.agencies = append(r.agencies, a)
	}
	for i := range r.agencies {
		if k := NormalizeLicence(r.agencies[i].NEARegNo); k != "" {
			r.byLic[k] = &r.agencies[i]
		}
	}
	for _, f := range flagged {
		f.NormalizedName = Normalize(f.Name)
		r.flagged = append(r.flagged, f)
	}
	r.info = describe(r.agencies)
	return r
}

// describe summarises the snapshot: the most recent dates and the source.
func describe(as []domain.Agency) domain.RegisterInfo {
	info := domain.RegisterInfo{Agencies: len(as)}
	for _, a := range as {
		if a.PublishedAt.After(info.PublishedAt) {
			info.PublishedAt = a.PublishedAt
		}
		if a.SnapshotAt.After(info.SnapshotAt) {
			info.SnapshotAt = a.SnapshotAt
		}
		if info.SourceURL == "" && strings.HasPrefix(a.SourceURL, "http") {
			info.SourceURL = a.SourceURL
		}
	}
	return info
}

// Info describes the loaded snapshot (shown on /api/health and in results).
func (r *Registry) Info() domain.RegisterInfo { return r.info }

func (r *Registry) Size() (agencies, flagged int) { return len(r.agencies), len(r.flagged) }

func (r *Registry) Match(f domain.Facts) domain.RegistryMatch {
	m := domain.RegistryMatch{Kind: domain.MatchNone, Register: r.info}

	if lic := NormalizeLicence(f.AgencyLicenceNo); lic != "" {
		if a, ok := r.byLic[lic]; ok {
			cp := *a
			m.LicenceOwner = &cp
		}
	}

	name := Normalize(f.AgencyName)
	if name != "" {
		var best *domain.Agency
		bestSim := 0.0
		for i := range r.agencies {
			a := &r.agencies[i]
			if a.NormalizedName == name {
				best, bestSim = a, 1
				break
			}
			if s := max(Similarity(name, a.NormalizedName), CoreSimilarity(name, a.NormalizedName)); s > bestSim {
				best, bestSim = a, s
			}
		}
		switch {
		case bestSim == 1:
			cp := *best
			m.Kind, m.Agency, m.Similarity = domain.MatchExact, &cp, 1
		case bestSim >= LookalikeThreshold:
			cp := *best
			m.Kind, m.Agency, m.Similarity = domain.MatchLookalike, &cp, round2(bestSim)
		}

		for _, fl := range r.flagged {
			if fl.NormalizedName == name || Similarity(name, fl.NormalizedName) >= FlaggedThreshold {
				m.Flagged = append(m.Flagged, fl)
			}
		}
	}

	// No usable name but a valid licence number: trust the number.
	if m.Kind == domain.MatchNone && name == "" && m.LicenceOwner != nil {
		cp := *m.LicenceOwner
		m.Kind, m.Agency, m.Similarity = domain.MatchLicenceNo, &cp, 1
	}
	return m
}

// legalSuffixes are dropped before comparing names; they carry no identity.
var legalSuffixes = map[string]bool{
	"limited": true, "ltd": true, "company": true, "co": true, "plc": true,
	"the": true, "llc": true, "inc": true, "and": true,
}

// Normalize lowercases, strips punctuation and legal suffixes.
// "Upendo Global Placements Ltd." -> "upendo global placements"
func Normalize(s string) string {
	s = strings.ToLower(strings.ReplaceAll(s, "&", " and "))
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else {
			b.WriteRune(' ')
		}
	}
	var out []string
	for _, w := range strings.Fields(b.String()) {
		if !legalSuffixes[w] {
			out = append(out, w)
		}
	}
	return strings.Join(out, " ")
}

// NormalizeLicence: "nea / pea / 0001" -> "NEAPEA0001"
func NormalizeLicence(s string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Similarity blends character-level edit similarity with token overlap, so
// both "Upendo Global Placement" and "Global Upendo Placements" score high.
func Similarity(a, b string) float64 {
	if a == "" || b == "" {
		return 0
	}
	if a == b {
		return 1
	}
	lev := 1 - float64(levenshtein(a, b))/float64(max(len([]rune(a)), len([]rune(b))))
	tok := tokenOverlap(a, b)
	if tok > lev {
		return 0.5*lev + 0.5*tok
	}
	return lev
}

// genericWords describe the industry, not the company. Two names whose
// remaining "core" is the same are probably meant to look like each other:
// "Upendo Global Placement Agency" vs "Upendo Global Placements Limited".
var genericWords = map[string]bool{
	"agency": true, "agencies": true, "recruitment": true, "recruiters": true, "recruiter": true,
	"recruiting": true, "placement": true, "placements": true, "consultants": true,
	"consultancy": true, "consulting": true, "international": true, "intl": true, "global": true,
	"overseas": true, "services": true, "solutions": true, "kenya": true, "ke": true,
	"employment": true, "manpower": true, "human": true, "resource": true, "resources": true,
	"hr": true, "jobs": true, "job": true, "workforce": true, "career": true, "careers": true,
	"link": true, "links": true, "group": true, "abroad": true,
}

func core(normalized string) string {
	var out []string
	for _, w := range strings.Fields(normalized) {
		if !genericWords[w] {
			out = append(out, w)
		}
	}
	return strings.Join(out, " ")
}

// CoreSimilarity compares only the distinctive part of two names. Used for
// lookalike detection against licensed agencies — never for "flagged"
// matching, where a false accusation would be worse than a miss.
func CoreSimilarity(a, b string) float64 {
	ca, cb := core(a), core(b)
	if len(ca) < 4 || len(cb) < 4 {
		return 0
	}
	return 0.95 * Similarity(ca, cb)
}

// tokenOverlap is the Jaccard index over lightly stemmed word sets.
func tokenOverlap(a, b string) float64 {
	sa, sb := map[string]bool{}, map[string]bool{}
	for _, t := range strings.Fields(a) {
		sa[stem(t)] = true
	}
	for _, t := range strings.Fields(b) {
		sb[stem(t)] = true
	}
	inter := 0
	for t := range sb {
		if sa[t] {
			inter++
		}
	}
	union := len(sa) + len(sb) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

func stem(t string) string { return strings.TrimSuffix(t, "s") }

func levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(rb)]
}

func round2(f float64) float64 { return float64(int(f*100+0.5)) / 100 }
