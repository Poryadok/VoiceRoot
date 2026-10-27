package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

// ProfileDeletionStates returns only existing profile rows, with true indicating
// a persisted deleted_at. Missing map entries are unknown, never deleted.
func (s *ProfileStore) ProfileDeletionStates(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]bool, error) {
	if s == nil || s.pool == nil {
		return nil, errors.New("profile deletion store unavailable")
	}
	rows, err := s.pool.Query(ctx, `SELECT id, deleted_at IS NOT NULL FROM profiles WHERE id = ANY($1::uuid[])`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	states := make(map[uuid.UUID]bool, len(ids))
	for rows.Next() {
		var id uuid.UUID
		var deleted bool
		if err := rows.Scan(&id, &deleted); err != nil {
			return nil, err
		}
		states[id] = deleted
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return states, nil
}
