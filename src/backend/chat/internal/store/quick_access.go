package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const maxQuickAccessSlots = 15

// ErrQuickAccessLimit is returned when a profile already has 15 quick-access slots.
var ErrQuickAccessLimit = errors.New("quick access limit reached")

// QuickAccessRow is one quick-access slot for a profile.
type QuickAccessRow struct {
	ChatID    uuid.UUID
	SortOrder int32
	AddedAt   time.Time
}

// ListQuickAccess returns quick-access slots ordered by sort_order ASC.
func (s *DMStore) ListQuickAccess(ctx context.Context, profileID uuid.UUID) ([]QuickAccessRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("dm store: pool not configured")
	}
	rows, err := s.Pool.Query(ctx, `
SELECT chat_id, sort_order, added_at
FROM quick_access_chats
WHERE profile_id = $1
ORDER BY sort_order ASC, added_at ASC
`, profileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []QuickAccessRow
	for rows.Next() {
		var row QuickAccessRow
		if err := rows.Scan(&row.ChatID, &row.SortOrder, &row.AddedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// AddQuickAccess adds a chat to quick access. Idempotent when the chat is already present.
func (s *DMStore) AddQuickAccess(ctx context.Context, profileID, chatID uuid.UUID, sortOrder *int32) error {
	if s == nil || s.Pool == nil {
		return errors.New("dm store: pool not configured")
	}
	var archived bool
	err := s.Pool.QueryRow(ctx, `
SELECT COALESCE(is_archived, false)
FROM chat_members
WHERE chat_id = $1 AND profile_id = $2
`, chatID, profileID).Scan(&archived)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return pgx.ErrNoRows
		}
		return err
	}
	if archived {
		return fmt.Errorf("archived chat cannot be added to quick access")
	}

	var existing int
	err = s.Pool.QueryRow(ctx, `
SELECT 1 FROM quick_access_chats WHERE profile_id = $1 AND chat_id = $2
`, profileID, chatID).Scan(&existing)
	if err == nil {
		if sortOrder == nil {
			return nil
		}
		_, err = s.Pool.Exec(ctx, `
UPDATE quick_access_chats SET sort_order = $3 WHERE profile_id = $1 AND chat_id = $2
`, profileID, chatID, *sortOrder)
		return err
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}

	var count int
	if err := s.Pool.QueryRow(ctx, `
SELECT COUNT(*) FROM quick_access_chats WHERE profile_id = $1
`, profileID).Scan(&count); err != nil {
		return err
	}
	if count >= maxQuickAccessSlots {
		return ErrQuickAccessLimit
	}

	order := int32(count)
	if sortOrder != nil {
		order = *sortOrder
	}
	_, err = s.Pool.Exec(ctx, `
INSERT INTO quick_access_chats (profile_id, chat_id, sort_order)
VALUES ($1, $2, $3)
`, profileID, chatID, order)
	return err
}

// RemoveQuickAccess removes a chat from quick access. Missing rows are a no-op.
func (s *DMStore) RemoveQuickAccess(ctx context.Context, profileID, chatID uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return errors.New("dm store: pool not configured")
	}
	_, err := s.Pool.Exec(ctx, `
DELETE FROM quick_access_chats WHERE profile_id = $1 AND chat_id = $2
`, profileID, chatID)
	return err
}

// ReorderQuickAccess replaces the sort order for the caller's quick-access list.
func (s *DMStore) ReorderQuickAccess(ctx context.Context, profileID uuid.UUID, chatIDs []uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return errors.New("dm store: pool not configured")
	}
	if len(chatIDs) > maxQuickAccessSlots {
		return fmt.Errorf("quick access reorder: too many chat_ids")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	for i, chatID := range chatIDs {
		ct, err := tx.Exec(ctx, `
UPDATE quick_access_chats
SET sort_order = $3
WHERE profile_id = $1 AND chat_id = $2
`, profileID, chatID, int32(i))
		if err != nil {
			return err
		}
		if ct.RowsAffected() == 0 {
			return pgx.ErrNoRows
		}
	}
	return tx.Commit(ctx)
}
