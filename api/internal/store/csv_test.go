package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadAgenciesCSVWithRegisterColumns(t *testing.T) {
	p := filepath.Join(t.TempDir(), "agencies.csv")
	csv := "name,nea_reg_no,status,source_url,service_type,email,website,published_at,snapshot_at\n" +
		"\"A&J GLOBAL LIMITED\",RA/2025/04/39,expired,https://register.example,Foreign & Local Recruitment,admin@ajglobalconsult.com,www.ajglobalconsult.com,2026-05-19,2026-10-27\n" +
		"ABAMWE INVESTMENTS LIMITED,NA/2021/12/22,valid,https://register.example,Foreign & Local Recruitment,abamweinvestmentsltd@gmail.com,n/a,2026-05-19,2026-10-27\n"
	if err := os.WriteFile(p, []byte(csv), 0o644); err != nil {
		t.Fatal(err)
	}
	as, err := ReadAgenciesCSV(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(as) != 2 || as[0].Website != "www.ajglobalconsult.com" || as[1].Website != "" || as[0].PublishedAt.Year() != 2026 {
		t.Fatalf("got %+v", as)
	}
}

func TestReadAgenciesCSVOldFormatStillWorks(t *testing.T) {
	as, err := ReadAgenciesCSV("../../../data/sample/agencies.csv")
	if err != nil || len(as) == 0 {
		t.Fatalf("as=%d err=%v", len(as), err)
	}
}
