//go:build !nopg

package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/edkariuki/kazi-halali/api/internal/domain"
	"github.com/edkariuki/kazi-halali/api/internal/registry"
)

type Postgres struct{ pool *pgxpool.Pool }

func NewPostgres(ctx context.Context, url string) (Store, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	pctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &Postgres{pool: pool}, nil
}

func (p *Postgres) Close() { p.pool.Close() }

func (p *Postgres) LoadRegistry(ctx context.Context) ([]domain.Agency, []domain.FlaggedEntry, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT id, name, nea_reg_no, status::text, source_url,
		       service_type, email, website, published_at, snapshot_at
		FROM agencies ORDER BY id`)
	if err != nil {
		return nil, nil, err
	}
	var agencies []domain.Agency
	for rows.Next() {
		var a domain.Agency
		var st string
		var pub *time.Time
		if err := rows.Scan(&a.ID, &a.Name, &a.NEARegNo, &st, &a.SourceURL,
			&a.ServiceType, &a.Email, &a.Website, &pub, &a.SnapshotAt); err != nil {
			rows.Close()
			return nil, nil, err
		}
		a.Status = domain.AgencyStatus(st)
		if pub != nil {
			a.PublishedAt = *pub
		}
		agencies = append(agencies, a)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	rows, err = p.pool.Query(ctx, `SELECT id, name, reason, source_title, source_url, published_at FROM flagged_entities ORDER BY id`)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var flagged []domain.FlaggedEntry
	for rows.Next() {
		var f domain.FlaggedEntry
		var pub *time.Time
		if err := rows.Scan(&f.ID, &f.Name, &f.Reason, &f.SourceTitle, &f.SourceURL, &pub); err != nil {
			return nil, nil, err
		}
		if pub != nil {
			f.PublishedAt = *pub
		}
		flagged = append(flagged, f)
	}
	return agencies, flagged, rows.Err()
}

func (p *Postgres) UpsertAgencies(ctx context.Context, as []domain.Agency) (int, error) {
	b := &pgx.Batch{}
	for _, a := range as {
		var pub *time.Time
		if !a.PublishedAt.IsZero() {
			t := a.PublishedAt
			pub = &t
		}
		snap := a.SnapshotAt
		if snap.IsZero() {
			snap = time.Now()
		}
		b.Queue(`
			INSERT INTO agencies (name, normalized_name, nea_reg_no, status, source_url,
			                      service_type, email, website, published_at, snapshot_at)
			VALUES ($1, $2, $3, $4::agency_status, $5, $6, $7, $8, $9, $10)
			ON CONFLICT (normalized_name) DO UPDATE
			SET name = EXCLUDED.name, nea_reg_no = EXCLUDED.nea_reg_no, status = EXCLUDED.status,
			    source_url = EXCLUDED.source_url, service_type = EXCLUDED.service_type,
			    email = EXCLUDED.email, website = EXCLUDED.website,
			    published_at = EXCLUDED.published_at, snapshot_at = EXCLUDED.snapshot_at`,
			a.Name, registry.Normalize(a.Name), a.NEARegNo, string(a.Status), a.SourceURL,
			a.ServiceType, a.Email, a.Website, pub, snap)
	}
	return p.runBatch(ctx, b, len(as))
}

func (p *Postgres) UpsertFlagged(ctx context.Context, fs []domain.FlaggedEntry) (int, error) {
	b := &pgx.Batch{}
	for _, f := range fs {
		var pub *time.Time
		if !f.PublishedAt.IsZero() {
			t := f.PublishedAt
			pub = &t
		}
		b.Queue(`
			INSERT INTO flagged_entities (name, normalized_name, reason, source_title, source_url, published_at)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (normalized_name, source_url) DO UPDATE
			SET name = EXCLUDED.name, reason = EXCLUDED.reason,
			    source_title = EXCLUDED.source_title, published_at = EXCLUDED.published_at`,
			f.Name, registry.Normalize(f.Name), f.Reason, f.SourceTitle, f.SourceURL, pub)
	}
	return p.runBatch(ctx, b, len(fs))
}

func (p *Postgres) runBatch(ctx context.Context, b *pgx.Batch, n int) (int, error) {
	if n == 0 {
		return 0, nil
	}
	br := p.pool.SendBatch(ctx, b)
	defer br.Close()
	for i := 0; i < n; i++ {
		if _, err := br.Exec(); err != nil {
			return i, err
		}
	}
	return n, nil
}

func (p *Postgres) SaveCheck(ctx context.Context, c *domain.Check) error {
	raw, err := json.Marshal(c)
	if err != nil {
		return err
	}
	_, err = p.pool.Exec(ctx, `
		INSERT INTO checks (id, created_at, verdict, agency_name, destination, had_image, result)
		VALUES ($1, $2, $3::verdict, $4, $5, $6, $7::jsonb)`,
		c.ID, c.CreatedAt, string(c.Verdict), c.Facts.AgencyName, c.Facts.DestinationCountry, c.HadImage, string(raw))
	return err
}

func (p *Postgres) GetCheck(ctx context.Context, id string) (*domain.Check, error) {
	var raw string
	err := p.pool.QueryRow(ctx, `SELECT result::text FROM checks WHERE id = $1`, id).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var c domain.Check
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		return nil, err
	}
	return &c, nil
}

func (p *Postgres) Stats(ctx context.Context) (Stats, error) {
	var s Stats
	err := p.pool.QueryRow(ctx, `
		SELECT count(*),
		       count(*) FILTER (WHERE verdict = 'danger'),
		       count(*) FILTER (WHERE verdict = 'caution'),
		       count(*) FILTER (WHERE verdict = 'likely_ok')
		FROM checks`).Scan(&s.Checks, &s.Danger, &s.Caution, &s.LikelyOK)
	return s, err
}
