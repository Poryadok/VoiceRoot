package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ChatRow is a persisted chat row from chat_db.chats.
type ChatRow struct {
	ID               uuid.UUID
	Type             string // dm | group | channel
	SpaceID          *uuid.UUID
	Name             *string
	AvatarURL        *string
	Topic            *string
	CreatorProfileID uuid.UUID
	SlowModeSeconds  int32
	CreatedAt        time.Time
	UpdatedAt        time.Time
	LastMessageAt    *time.Time
	InboxBucket      string
}

// DMStore persists DM chats and membership (Phase 1).
type DMStore struct {
	Pool *pgxpool.Pool
}

func orderedProfileStrings(a, b uuid.UUID) (low, high string) {
	s1, s2 := a.String(), b.String()
	if s1 <= s2 {
		return s1, s2
	}
	return s2, s1
}

// EnsureDM returns the existing DM between the two profiles or creates one (creator = caller).
// The bool is true when a new chat row and memberships were inserted.
// Uses a transaction-scoped advisory lock on the sorted pair to avoid duplicate chats under concurrency.
func (s *DMStore) EnsureDM(ctx context.Context, callerProfileID, otherProfileID uuid.UUID) (*ChatRow, bool, error) {
	if s == nil || s.Pool == nil {
		return nil, false, errors.New("dm store: pool not configured")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	low, high := orderedProfileStrings(callerProfileID, otherProfileID)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1::text), hashtext($2::text))`, low, high); err != nil {
		return nil, false, err
	}

	found, err := findDMInTx(ctx, tx, callerProfileID, otherProfileID)
	if err != nil {
		return nil, false, err
	}
	if found != nil {
		if err := tx.Commit(ctx); err != nil {
			return nil, false, err
		}
		return found, false, nil
	}

	var chatID uuid.UUID
	var createdAt, updatedAt time.Time
	err = tx.QueryRow(ctx, `
INSERT INTO chats (type, creator_profile_id, slow_mode_seconds)
VALUES ('dm', $1, 0)
RETURNING id, created_at, updated_at
`, callerProfileID).Scan(&chatID, &createdAt, &updatedAt)
	if err != nil {
		return nil, false, err
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO chat_members (chat_id, profile_id, role, inbox_bucket)
VALUES ($1, $2, 'member', 'main'), ($1, $3, 'member', 'requests')
`, chatID, callerProfileID, otherProfileID); err != nil {
		return nil, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, false, err
	}
	return &ChatRow{
		ID:               chatID,
		Type:             "dm",
		CreatorProfileID: callerProfileID,
		CreatedAt:        createdAt.UTC(),
		UpdatedAt:        updatedAt.UTC(),
		LastMessageAt:    nil,
		InboxBucket:      "main",
	}, true, nil
}

func (s *DMStore) TouchLastMessageAt(ctx context.Context, chatID uuid.UUID, at time.Time) error {
	if s == nil || s.Pool == nil {
		return errors.New("dm store: pool not configured")
	}
	_, err := s.Pool.Exec(ctx, `
UPDATE chats
SET last_message_at = CASE
    WHEN last_message_at IS NULL OR last_message_at < $2::timestamptz THEN $2::timestamptz
    ELSE last_message_at
  END,
  updated_at = CASE
    WHEN last_message_at IS NULL OR last_message_at < $2::timestamptz THEN now()
    ELSE updated_at
  END
WHERE id = $1 AND type = 'dm'
`, chatID, at.UTC())
	return err
}

func (s *DMStore) SetInboxBucket(ctx context.Context, chatID, profileID uuid.UUID, bucket string) error {
	if s == nil || s.Pool == nil {
		return errors.New("dm store: pool not configured")
	}
	ct, err := s.Pool.Exec(ctx, `
UPDATE chat_members
SET inbox_bucket = $3
WHERE chat_id = $1 AND profile_id = $2
`, chatID, profileID, bucket)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func findDMInTx(ctx context.Context, tx pgx.Tx, profileA, profileB uuid.UUID) (*ChatRow, error) {
	var id, creator uuid.UUID
	var createdAt, updatedAt time.Time
	var lastMsg sql.NullTime
	err := tx.QueryRow(ctx, `
SELECT c.id, c.creator_profile_id, c.last_message_at, c.created_at, c.updated_at
FROM chats c
INNER JOIN chat_members m1 ON m1.chat_id = c.id AND m1.profile_id = $1
INNER JOIN chat_members m2 ON m2.chat_id = c.id AND m2.profile_id = $2
WHERE c.type = 'dm'
LIMIT 1
`, profileA, profileB).Scan(&id, &creator, &lastMsg, &createdAt, &updatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var lm *time.Time
	if lastMsg.Valid {
		t := lastMsg.Time.UTC()
		lm = &t
	}
	return &ChatRow{
		ID:               id,
		Type:             "dm",
		CreatorProfileID: creator,
		CreatedAt:        createdAt.UTC(),
		UpdatedAt:        updatedAt.UTC(),
		LastMessageAt:    lm,
	}, nil
}
