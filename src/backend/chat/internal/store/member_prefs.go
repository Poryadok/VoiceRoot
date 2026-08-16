package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// SetMemberArchived sets chat_members.is_archived for one membership row.
func (s *DMStore) SetMemberArchived(ctx context.Context, chatID, profileID uuid.UUID, archived bool) error {
	if s == nil || s.Pool == nil {
		return errors.New("dm store: pool not configured")
	}
	ct, err := s.Pool.Exec(ctx, `
UPDATE chat_members
SET is_archived = $3
WHERE chat_id = $1 AND profile_id = $2
`, chatID, profileID, archived)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// SetMemberMutedUntil sets or clears chat_members.muted_until for one membership row.
// A nil until clears the mute (unmute).
func (s *DMStore) SetMemberMutedUntil(ctx context.Context, chatID, profileID uuid.UUID, until *time.Time) error {
	if s == nil || s.Pool == nil {
		return errors.New("dm store: pool not configured")
	}
	var muted any
	if until != nil {
		muted = until.UTC()
	}
	ct, err := s.Pool.Exec(ctx, `
UPDATE chat_members
SET muted_until = $3
WHERE chat_id = $1 AND profile_id = $2
`, chatID, profileID, muted)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
