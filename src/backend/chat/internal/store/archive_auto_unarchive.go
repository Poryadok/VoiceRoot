package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

// AutoUnarchiveDMRecipients clears is_archived / deleted_for_self for non-sender members in a DM when peer sends
// (text-chat.md §Архивирование — incoming message auto-unarchive; DeleteChat for-self restore; group/channel excluded).
func (s *DMStore) AutoUnarchiveDMRecipients(ctx context.Context, chatID, senderProfileID uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return errors.New("dm store: pool not configured")
	}
	_, err := s.Pool.Exec(ctx, `
UPDATE chat_members m
SET is_archived = false, deleted_for_self = false
FROM chats c
WHERE m.chat_id = c.id
  AND c.id = $1
  AND c.type = 'dm'
  AND m.profile_id <> $2
  AND (m.is_archived = true OR m.deleted_for_self = true)
`, chatID, senderProfileID)
	return err
}
