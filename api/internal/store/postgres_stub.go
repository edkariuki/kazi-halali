//go:build nopg

package store

import (
	"context"
	"errors"
)

// Built with -tags nopg: no Postgres driver compiled in (used where the Go
// module proxy is unavailable). Run with STORE=memory.
func NewPostgres(context.Context, string) (Store, error) {
	return nil, errors.New("built without Postgres support (-tags nopg); use STORE=memory")
}
