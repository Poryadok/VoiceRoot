package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const MaxPinsPerChat = 50

var (
	ErrPinLimitReached = errors.New("pin limit reached for chat")
)

// PinRow is one pinned message in a chat.
type PinRow struct {
	ChatID    uuid.UUID
	MessageID uuid.UUID
	PinnedBy  uuid.UUID
	PinnedAt  time.Time
}

// PinsStore persists chat message pins.
type PinsStore struct {
	Pool *pgxpool.Pool
}

// UpsertPin pins a message; duplicate (chat, message) updates pinned_by and pinned_at.
func (s *PinsStore) UpsertPin(ctx context.Context, chatID, messageID, pinnedBy uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return ErrStoreNotConfigured
	}
	var exists bool
	err := s.Pool.QueryRow(ctx, `
SELECT EXISTS(SELECT 1 FROM pins WHERE chat_id = $1 AND message_id = $2)
`, chatID, messageID).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		var count int
		err = s.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM pins WHERE chat_id = $1`, chatID).Scan(&count)
		if err != nil {
			return err
		}
		if count >= MaxPinsPerChat {
			return ErrPinLimitReached
		}
	}
	_, err = s.Pool.Exec(ctx, `
INSERT INTO pins (chat_id, message_id, pinned_by, pinned_at)
VALUES ($1, $2, $3, now())
ON CONFLICT (chat_id, message_id) DO UPDATE
SET pinned_by = EXCLUDED.pinned_by, pinned_at = now()
`, chatID, messageID, pinnedBy)
	return err
}

// DeletePin removes a pin; missing row is not an error.
func (s *PinsStore) DeletePin(ctx context.Context, chatID, messageID uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return ErrStoreNotConfigured
	}
	_, err := s.Pool.Exec(ctx, `
DELETE FROM pins WHERE chat_id = $1 AND message_id = $2
`, chatID, messageID)
	return err
}

// ListPins returns pins for a chat ordered by pinned_at descending.
func (s *PinsStore) ListPins(ctx context.Context, chatID uuid.UUID) ([]PinRow, error) {
	if s == nil || s.Pool == nil {
		return nil, ErrStoreNotConfigured
	}
	rows, err := s.Pool.Query(ctx, `
SELECT chat_id, message_id, pinned_by, pinned_at
FROM pins
WHERE chat_id = $1
ORDER BY pinned_at DESC
`, chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PinRow
	for rows.Next() {
		var row PinRow
		if err := rows.Scan(&row.ChatID, &row.MessageID, &row.PinnedBy, &row.PinnedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// PinnedSetForMessageIDs returns which message IDs are pinned in the chat.
func (s *PinsStore) PinnedSetForMessageIDs(ctx context.Context, chatID uuid.UUID, messageIDs []uuid.UUID) (map[uuid.UUID]bool, error) {
	out := make(map[uuid.UUID]bool, len(messageIDs))
	if s == nil || s.Pool == nil || len(messageIDs) == 0 {
		return out, nil
	}
	rows, err := s.Pool.Query(ctx, `
SELECT message_id FROM pins WHERE chat_id = $1 AND message_id = ANY($2)
`, chatID, messageIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

// DeletePinIfMessageMissing removes pins whose message was deleted (housekeeping).
func (s *PinsStore) DeletePinIfMessageMissing(ctx context.Context, chatID, messageID uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return ErrStoreNotConfigured
	}
	var deletedAt *time.Time
	err := s.Pool.QueryRow(ctx, `SELECT deleted_at FROM messages WHERE id = $1 AND chat_id = $2`, messageID, chatID).Scan(&deletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return s.DeletePin(ctx, chatID, messageID)
	}
	if err != nil {
		return err
	}
	if deletedAt != nil {
		return s.DeletePin(ctx, chatID, messageID)
	}
	return nil
}
