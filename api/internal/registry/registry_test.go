package registry

import (
	"testing"
	"time"

	"github.com/edkariuki/kazi-halali/api/internal/domain"
)

func testRegistry() *Registry {
	return New(
		[]domain.Agency{
			{ID: 1, Name: "Upendo Global Placements Limited", NEARegNo: "NEA/PEA/0001", Status: domain.StatusValid},
			{ID: 2, Name: "Mwangaza Overseas Recruitment Ltd", NEARegNo: "NEA/PEA/0002", Status: domain.StatusValid},
			{ID: 3, Name: "Pwani Career Links Limited", NEARegNo: "NEA/PEA/0005", Status: domain.StatusExpired},
		},
		[]domain.FlaggedEntry{{ID: 1, Name: "Sunrise Dream Jobs Abroad Ltd", Reason: "not registered", SourceURL: "https://example.invalid"}},
	)
}

func TestNormalize(t *testing.T) {
	cases := map[string]string{
		"Upendo Global Placements Ltd.":     "upendo global placements",
		"UPENDO  GLOBAL PLACEMENTS LIMITED": "upendo global placements",
		"A&B Recruiters Co.":                "a b recruiters",
	}
	for in, want := range cases {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMatchExact(t *testing.T) {
	m := testRegistry().Match(domain.Facts{AgencyName: "upendo global placements ltd"})
	if m.Kind != domain.MatchExact || m.Agency.ID != 1 {
		t.Fatalf("want exact match on 1, got %+v", m)
	}
}

func TestMatchLookalike(t *testing.T) {
	m := testRegistry().Match(domain.Facts{AgencyName: "Upendo Global Placement Agency"})
	if m.Kind != domain.MatchLookalike || m.Agency.ID != 1 {
		t.Fatalf("want lookalike of 1, got %+v", m)
	}
}

func TestMatchUnrelatedNameIsNone(t *testing.T) {
	m := testRegistry().Match(domain.Facts{AgencyName: "Golden Horizon Travel"})
	if m.Kind != domain.MatchNone {
		t.Fatalf("want no match, got %+v", m)
	}
}

func TestLicenceOwnerDetected(t *testing.T) {
	m := testRegistry().Match(domain.Facts{AgencyName: "Some Other Agency", AgencyLicenceNo: "nea / pea / 0001"})
	if m.LicenceOwner == nil || m.LicenceOwner.ID != 1 {
		t.Fatalf("want licence owner 1, got %+v", m.LicenceOwner)
	}
}

func TestLicenceOnlyMatch(t *testing.T) {
	m := testRegistry().Match(domain.Facts{AgencyLicenceNo: "NEA/PEA/0002"})
	if m.Kind != domain.MatchLicenceNo || m.Agency.ID != 2 {
		t.Fatalf("want licence match on 2, got %+v", m)
	}
}

func TestFlagged(t *testing.T) {
	m := testRegistry().Match(domain.Facts{AgencyName: "Sunrise Dream Jobs Abroad Limited"})
	if len(m.Flagged) != 1 {
		t.Fatalf("want 1 flagged entry, got %+v", m.Flagged)
	}
}

func TestRegisterInfoTakesLatestDatesAndSource(t *testing.T) {
	pub := time.Date(2026, 5, 19, 0, 0, 0, 0, time.UTC)
	snap := time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC)
	r := New([]domain.Agency{
		{ID: 1, Name: "A", SourceURL: "https://register.example", PublishedAt: pub, SnapshotAt: snap},
		{ID: 2, Name: "B"},
	}, nil)
	info := r.Match(domain.Facts{AgencyName: "Unknown"}).Register
	if info.Agencies != 2 || !info.PublishedAt.Equal(pub) || !info.SnapshotAt.Equal(snap) || info.SourceURL != "https://register.example" {
		t.Fatalf("info = %+v", info)
	}
}
