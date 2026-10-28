package store

import (
	"context"
	"sync"

	"github.com/edkariuki/kazi-halali/api/internal/domain"
)

// Memory is a non-persistent store for development and tests.
type Memory struct {
	mu       sync.RWMutex
	agencies []domain.Agency
	flagged  []domain.FlaggedEntry
	checks   map[string]*domain.Check
}

func NewMemory() *Memory { return &Memory{checks: map[string]*domain.Check{}} }

func (m *Memory) LoadRegistry(context.Context) ([]domain.Agency, []domain.FlaggedEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]domain.Agency(nil), m.agencies...), append([]domain.FlaggedEntry(nil), m.flagged...), nil
}

func (m *Memory) UpsertAgencies(_ context.Context, as []domain.Agency) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range as {
		as[i].ID = int64(len(m.agencies) + 1)
		m.agencies = append(m.agencies, as[i])
	}
	return len(as), nil
}

func (m *Memory) UpsertFlagged(_ context.Context, fs []domain.FlaggedEntry) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range fs {
		fs[i].ID = int64(len(m.flagged) + 1)
		m.flagged = append(m.flagged, fs[i])
	}
	return len(fs), nil
}

func (m *Memory) SaveCheck(_ context.Context, c *domain.Check) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *c
	m.checks[c.ID] = &cp
	return nil
}

func (m *Memory) GetCheck(_ context.Context, id string) (*domain.Check, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c, ok := m.checks[id]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *c
	return &cp, nil
}

func (m *Memory) Stats(context.Context) (Stats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var s Stats
	for _, c := range m.checks {
		s.Checks++
		switch c.Verdict {
		case domain.VerdictDanger:
			s.Danger++
		case domain.VerdictCaution:
			s.Caution++
		default:
			s.LikelyOK++
		}
	}
	return s, nil
}

func (m *Memory) Close() {}
