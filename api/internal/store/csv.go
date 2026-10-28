package store

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/edkariuki/kazi-halali/api/internal/domain"
)

// ReadAgenciesCSV reads name,nea_reg_no,status,source_url and, optionally,
// service_type,email,website,published_at,snapshot_at (dates as YYYY-MM-DD).
func ReadAgenciesCSV(path string) ([]domain.Agency, error) {
	rows, err := readCSV(path, []string{"name", "nea_reg_no", "status", "source_url"})
	if err != nil {
		return nil, err
	}
	var out []domain.Agency
	for i, r := range rows {
		st := domain.AgencyStatus(strings.ToLower(strings.TrimSpace(r["status"])))
		switch st {
		case domain.StatusValid, domain.StatusUnderVerification, domain.StatusExpired, domain.StatusUnknown:
		case "":
			st = domain.StatusUnknown
		default:
			return nil, fmt.Errorf("%s row %d: unknown status %q", path, i+2, st)
		}
		pub, err := optDate(r["published_at"])
		if err != nil {
			return nil, fmt.Errorf("%s row %d: published_at: %w", path, i+2, err)
		}
		snap, err := optDate(r["snapshot_at"])
		if err != nil {
			return nil, fmt.Errorf("%s row %d: snapshot_at: %w", path, i+2, err)
		}
		out = append(out, domain.Agency{
			Name:        strings.TrimSpace(r["name"]),
			NEARegNo:    strings.TrimSpace(r["nea_reg_no"]),
			Status:      st,
			SourceURL:   strings.TrimSpace(r["source_url"]),
			ServiceType: strings.TrimSpace(r["service_type"]),
			Email:       placeholderToEmpty(r["email"]),
			Website:     placeholderToEmpty(r["website"]),
			PublishedAt: pub,
			SnapshotAt:  snap,
		})
	}
	return out, nil
}

// placeholderToEmpty turns the register's "none", "n/a", "-" into "".
func placeholderToEmpty(s string) string {
	s = strings.TrimSpace(s)
	switch strings.ToLower(s) {
	case "", "none", "n/a", "na", "nil", "-", "null":
		return ""
	}
	return s
}

func optDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, nil
	}
	return time.Parse("2006-01-02", s)
}

// ReadFlaggedCSV reads name,reason,source_title,source_url,published_at.
func ReadFlaggedCSV(path string) ([]domain.FlaggedEntry, error) {
	rows, err := readCSV(path, []string{"name", "reason", "source_title", "source_url", "published_at"})
	if err != nil {
		return nil, err
	}
	var out []domain.FlaggedEntry
	for i, r := range rows {
		if strings.TrimSpace(r["source_url"]) == "" {
			return nil, fmt.Errorf("%s row %d: source_url is required for flagged entries", path, i+2)
		}
		var pub time.Time
		if s := strings.TrimSpace(r["published_at"]); s != "" {
			if pub, err = time.Parse("2006-01-02", s); err != nil {
				return nil, fmt.Errorf("%s row %d: published_at: %w", path, i+2, err)
			}
		}
		out = append(out, domain.FlaggedEntry{
			Name:        strings.TrimSpace(r["name"]),
			Reason:      strings.TrimSpace(r["reason"]),
			SourceTitle: strings.TrimSpace(r["source_title"]),
			SourceURL:   strings.TrimSpace(r["source_url"]),
			PublishedAt: pub,
		})
	}
	return out, nil
}

func readCSV(path string, required []string) ([]map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.TrimLeadingSpace = true
	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	idx := map[string]int{}
	for i, h := range header {
		idx[strings.ToLower(strings.TrimSpace(strings.TrimPrefix(h, "\uFEFF")))] = i
	}
	for _, col := range required {
		if _, ok := idx[col]; !ok {
			return nil, fmt.Errorf("%s: missing column %q", path, col)
		}
	}
	var rows []map[string]string
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		row := map[string]string{}
		for col, i := range idx {
			if i < len(rec) {
				row[col] = rec[i]
			}
		}
		if strings.TrimSpace(row["name"]) == "" {
			continue
		}
		rows = append(rows, row)
	}
	return rows, nil
}
