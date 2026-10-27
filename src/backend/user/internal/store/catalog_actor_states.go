package store

import (
	"context"
	"errors"
	"github.com/google/uuid"
)

// CatalogActorSnapshot is read-only and contains no Auth account eligibility.
type CatalogActorSnapshot struct {
	AccountID uuid.UUID
	Deleted   bool
}

func (s *ProfileStore) CatalogActorStates(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]CatalogActorSnapshot, error) {
	if s == nil || s.pool == nil {
		return nil, errors.New("catalog actor store unavailable")
	}
	rows, err := s.pool.Query(ctx, `SELECT id, account_id, deleted_at IS NOT NULL FROM profiles WHERE id = ANY($1::uuid[])`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[uuid.UUID]CatalogActorSnapshot, len(ids))
	for rows.Next() {
		var id uuid.UUID
		var entry CatalogActorSnapshot
		if err := rows.Scan(&id, &entry.AccountID, &entry.Deleted); err != nil {
			return nil, err
		}
		result[id] = entry
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
