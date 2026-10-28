package extract

import (
	"testing"

	"github.com/edkariuki/kazi-halali/api/internal/domain"
)

func TestHeuristicCanadaScam(t *testing.T) {
	f := Heuristic("CANADA FARM JOBS. Fruit pickers needed. Salary CAD 3,200 per month. Processing fee KSh 85,000 via M-Pesa. Limited slots, pay today! WhatsApp 0712 345 678.")
	if f.DestinationCountry != "Canada" {
		t.Errorf("country = %q", f.DestinationCountry)
	}
	if f.JobCategory != domain.JobUnskilled {
		t.Errorf("category = %q", f.JobCategory)
	}
	if len(f.Fees) != 1 || f.Fees[0].Amount != 85000 || f.Fees[0].Currency != "KES" {
		t.Errorf("fees = %+v", f.Fees)
	}
	if f.Salary == nil || f.Salary.Currency != "CAD" {
		t.Errorf("salary = %+v", f.Salary)
	}
	if len(f.Phones) != 1 {
		t.Errorf("phones = %v", f.Phones)
	}
	if len(f.UrgencyCues) == 0 {
		t.Error("expected urgency cues")
	}
}

func TestHeuristicAgencyAndLicence(t *testing.T) {
	f := Heuristic("Upendo Global Placements Limited (NEA/PEA/0001) is recruiting nurses for Germany.")
	if f.AgencyName != "Upendo Global Placements Limited" {
		t.Errorf("agency = %q", f.AgencyName)
	}
	if f.AgencyLicenceNo != "NEA/PEA/0001" {
		t.Errorf("licence = %q", f.AgencyLicenceNo)
	}
	if f.JobCategory != domain.JobSkilled {
		t.Errorf("category = %q", f.JobCategory)
	}
}

func TestHeuristicVisitorVisa(t *testing.T) {
	f := Heuristic("Security guards wanted in Dubai. You will travel on a visitor visa.")
	if f.VisaType != domain.VisaVisitor || f.DestinationCountry != "United Arab Emirates" {
		t.Errorf("got visa=%q country=%q", f.VisaType, f.DestinationCountry)
	}
}

func TestHeuristicSwahili(t *testing.T) {
	f := Heuristic("Kazi ya nyumbani Saudi. Mshahara SAR 1,500 kwa mwezi. Ada ya usajili KES 15,000, tuma na M-Pesa.")
	if f.JobCategory != domain.JobDomestic {
		t.Errorf("category = %q", f.JobCategory)
	}
	if len(f.Fees) == 0 {
		t.Error("expected the registration fee")
	}
	if f.Language == "en" {
		t.Errorf("language = %q", f.Language)
	}
}

func TestHeuristicJobTitle(t *testing.T) {
	cases := map[string]string{
		"Hiring cleaners in Qatar":            "cleaner",
		"Kazi ya nyumbani Saudi":              "domestic worker",
		"Housemaid needed in Oman":            "housemaid",
		"Heavy truck drivers wanted in Qatar": "driver",
	}
	for in, want := range cases {
		if got := Heuristic(in).JobTitle; got != want {
			t.Errorf("Heuristic(%q).JobTitle = %q, want %q", in, got, want)
		}
	}
}

func TestHeuristicAgencyNamesWithInitialsAndAmpersand(t *testing.T) {
	cases := map[string]string{
		"A.F. & R Manpower Limited is hiring nurses":     "A.F. & R Manpower Limited",
		"A&J GLOBAL LIMITED is recruiting drivers":       "A&J GLOBAL LIMITED",
		"Apply through Upendo Global Placements Ltd now": "Upendo Global Placements Ltd",
	}
	for in, want := range cases {
		if got := Heuristic(in).AgencyName; got != want {
			t.Errorf("Heuristic(%q).AgencyName = %q, want %q", in, got, want)
		}
	}
}

func TestHeuristicRealLicenceFormats(t *testing.T) {
	cases := map[string]string{
		"Licence NA/2025/11/3, hiring nurses":     "NA/2025/11/3",
		"(RA/2025/11/61) drivers wanted":          "RA/2025/11/61",
		"NEA reg no: na / 2026 / 03 / 1":          "NA/2026/03/1",
		"Upendo Ltd (NEA/PEA/0001) is recruiting": "NEA/PEA/0001",
	}
	for in, want := range cases {
		if got := Heuristic(in).AgencyLicenceNo; got != want {
			t.Errorf("Heuristic(%q).AgencyLicenceNo = %q, want %q", in, got, want)
		}
	}
}
