package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

// SetChatE2EEnabled toggles DM opt-in E2E encryption for a chat (encryption (docs/features/encryption.md)).
func (s *DMStore) SetChatE2EEnabled(ctx context.Context, chatID uuid.UUID, enabled bool) error {
	if s == nil || s.Pool == nil {
		return errors.New("dm store: pool not configured")
	}
	_, err := s.Pool.Exec(ctx, `
UPDATE chats
SET e2e_enabled = $2, updated_at = now()
WHERE id = $1
`, chatID, enabled)
	return err
}
