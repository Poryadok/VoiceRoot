package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

// PromoteDeclinedDMRecipients moves non-sender members from declined → requests when peer re-contacts
// (text-chat.md §«Запросы сообщений» re-contact after DeclineDMRequest).
func (s *DMStore) PromoteDeclinedDMRecipients(ctx context.Context, chatID, senderProfileID uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return errors.New("dm store: pool not configured")
	}
	_, err := s.Pool.Exec(ctx, `
UPDATE chat_members m
SET inbox_bucket = 'requests'
FROM chats c
WHERE m.chat_id = c.id
  AND c.id = $1
  AND c.type = 'dm'
  AND m.profile_id <> $2
  AND m.inbox_bucket = 'declined'
`, chatID, senderProfileID)
	return err
}
