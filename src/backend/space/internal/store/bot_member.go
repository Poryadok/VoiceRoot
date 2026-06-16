package store

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// AddBotMember inserts a bot actor profile into space_members (idempotent).
func (s *SpaceStore) AddBotMember(ctx context.Context, spaceID, profileID uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return pgx.ErrNoRows
	}
	var exists int
	err := s.Pool.QueryRow(ctx, `
SELECT COUNT(*)::int FROM space_members WHERE space_id = $1 AND profile_id = $2`, spaceID, profileID).Scan(&exists)
	if err != nil {
		return err
	}
	if exists > 0 {
		return nil
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `
INSERT INTO space_members (space_id, profile_id) VALUES ($1, $2)`, spaceID, profileID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
UPDATE spaces SET member_count = member_count + 1, updated_at = now() WHERE id = $1`, spaceID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// RemoveBotMember removes a bot actor from space_members.
func (s *SpaceStore) RemoveBotMember(ctx context.Context, spaceID, profileID uuid.UUID) error {
	return s.RemoveMember(ctx, spaceID, profileID)
}
