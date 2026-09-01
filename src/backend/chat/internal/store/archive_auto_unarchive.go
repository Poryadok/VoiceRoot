package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

// AutoUnarchiveDMRecipients clears is_archived for non-sender members in a DM when peer sends
// (text-chat.md §Архивирование — incoming message auto-unarchive; group/channel excluded).
func (s *DMStore) AutoUnarchiveDMRecipients(ctx context.Context, chatID, senderProfileID uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return errors.New("dm store: pool not configured")
	}
	_, err := s.Pool.Exec(ctx, `
UPDATE chat_members m
SET is_archived = false
FROM chats c
WHERE m.chat_id = c.id
  AND c.id = $1
  AND c.type = 'dm'
  AND m.profile_id <> $2
  AND m.is_archived = true
`, chatID, senderProfileID)
	return err
}
