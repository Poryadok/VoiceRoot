package store

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"time"
)

// ValidateRoleOverrideTarget checks only Chat-owned resource and lifecycle
// state. It does not resolve actors, members or permissions through Role.
func (s *DMStore) ValidateRoleOverrideTarget(ctx context.Context, spaceID, chatID uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return errors.New("chat target store unavailable")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()
	if err := lockSpaceLifecycleMutation(ctx, tx, spaceID); err != nil {
		return err
	}
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM chats WHERE id=$1 AND space_id=$2 AND type IN ('group','channel'))`, chatID, spaceID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return pgx.ErrNoRows
	}
	return tx.Commit(ctx)
}
