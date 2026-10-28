package rules

import "github.com/edkariuki/kazi-halali/api/internal/domain"

// Sources cited by rules. Each finding shown to a user links to at least one,
// so every claim Kazi Halali makes can be checked. Keep this list current.
var (
	srcNEAList = domain.Source{
		Title: "NEAIMS — List of Private Employment Agencies (black = licensed, red = under verification)",
		URL:   "https://www.neaims.go.ke/EmploymentAgencyList.aspx",
	}
	srcHowToCheck = domain.Source{
		Title: "How to check if a recruitment agency is licensed by NEA (Business Radar)",
		URL:   "https://businessradar.co.ke/how-to-check-if-a-recruitment-agency-is-licensed-nea-kenya/",
	}
	srcFeesAndFakeCerts = domain.Source{
		Title: "MPs unearth how rogue agencies dupe job seekers (People Daily)",
		URL:   "https://peopledaily.digital/news/mps-unearth-how-rogue-agencies-dupe-job-seekers",
	}
	srcAttestation = domain.Source{
		Title: "State flags 16 companies defrauding job-seekers with overseas work (People Daily)",
		URL:   "https://peopledaily.digital/state-flags-16-companies-defrauding-job-seekers-with-overseas-work",
	}
	srcNEAApprovesAds = domain.Source{
		Title: "Labour Ministry flags 16 companies after Kenyans lose millions in Canada scam (Kenyans.co.ke)",
		URL:   "https://www.kenyans.co.ke/news/103720-labour-ministry-publishes-investigation-findings-after-300-kenyans-lose-millions-canada",
	}
	srcVisitorVisa = domain.Source{
		Title: "CS Mutua warns youth against using visitor visas to seek jobs abroad (People Daily)",
		URL:   "https://peopledaily.digital/news/alfred-mutua-warns-youth-against-using-visitor-visas-to-seek-jobs-abroad",
	}
	srcRussia = domain.Source{
		Title: "Govt warns Kenyans against fake jobs amid Russia–Ukraine conflict casualties (AllAfrica)",
		URL:   "https://allafrica.com/stories/202602060030.html",
	}
	srcAsiaAdverts = domain.Source{
		Title: "Fake overseas jobs: what Kenyans must know (People Daily)",
		URL:   "https://peopledaily.digital/insights/fake-overseas-jobs-what-kenyans-must-know/amp",
	}
	srcOmanForgery = domain.Source{
		Title: "How Kenyans were nearly trafficked to Oman through a fake job scheme (Capital FM)",
		URL:   "https://capitalfm.africa/how-kenyans-were-nearly-trafficked-to-oman-through-a-fake-job-scheme/",
	}
)

// NEAHotline is the National Employment Authority toll-free line quoted by the
// Labour CS (Capital FM, 30 Jan 2026). Verify before launch.
const NEAHotline = "0800 222 223"
