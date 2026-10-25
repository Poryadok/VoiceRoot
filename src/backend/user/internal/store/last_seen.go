package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type LastSeenStore struct{ pool *pgxpool.Pool }

func NewLastSeenStore(pool *pgxpool.Pool) *LastSeenStore { return &LastSeenStore{pool: pool} }

func (s *LastSeenStore) Observe(ctx context.Context, id uuid.UUID, at time.Time) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("last seen store not configured")
	}
	_, err := s.pool.Exec(ctx, `INSERT INTO profile_last_seen (profile_id, last_seen_at)
 VALUES ($1,$2) ON CONFLICT (profile_id) DO UPDATE
 SET last_seen_at=GREATEST(profile_last_seen.last_seen_at, EXCLUDED.last_seen_at)`, id, at.UTC())
	return err
}

func (s *LastSeenStore) GetMany(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]int64, error) {
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("last seen store not configured")
	}
	rows, err := s.pool.Query(ctx, `SELECT profile_id,last_seen_at FROM profile_last_seen WHERE profile_id=ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[uuid.UUID]int64, len(ids))
	for rows.Next() {
		var id uuid.UUID
		var at time.Time
		if err := rows.Scan(&id, &at); err != nil {
			return nil, err
		}
		out[id] = at.Unix()
	}
	return out, rows.Err()
}
