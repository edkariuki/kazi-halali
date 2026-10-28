// Package store persists the registry snapshot and check results.
package store

import (
	"context"
	"errors"

	"github.com/edkariuki/kazi-halali/api/internal/domain"
)

var ErrNotFound = errors.New("not found")

type Store interface {
	LoadRegistry(ctx context.Context) ([]domain.Agency, []domain.FlaggedEntry, error)
	UpsertAgencies(ctx context.Context, as []domain.Agency) (int, error)
	UpsertFlagged(ctx context.Context, fs []domain.FlaggedEntry) (int, error)
	SaveCheck(ctx context.Context, c *domain.Check) error
	GetCheck(ctx context.Context, id string) (*domain.Check, error)
	Stats(ctx context.Context) (Stats, error)
	Close()
}

type Stats struct {
	Checks   int `json:"checks"`
	Danger   int `json:"danger"`
	Caution  int `json:"caution"`
	LikelyOK int `json:"likely_ok"`
}
