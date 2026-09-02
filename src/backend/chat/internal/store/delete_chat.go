package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// MarkChatDeletedForSelf soft-deletes a DM membership for one profile (navigation.md § Delete chat).
// Peer membership and message history are preserved. Clears archive flag and removes Quick Access /
// folder_chats rows for the deleter.
func (s *DMStore) MarkChatDeletedForSelf(ctx context.Context, chatID, profileID uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return errors.New("dm store: pool not configured")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var chatType string
	err = tx.QueryRow(ctx, `SELECT type FROM chats WHERE id = $1`, chatID).Scan(&chatType)
	if errors.Is(err, pgx.ErrNoRows) {
		return pgx.ErrNoRows
	}
	if err != nil {
		return err
	}
	if chatType != "dm" {
		return ErrDeleteChatDMOnly
	}

	ct, err := tx.Exec(ctx, `
UPDATE chat_members
SET deleted_for_self = true, is_archived = false
WHERE chat_id = $1 AND profile_id = $2 AND deleted_for_self = false
`, chatID, profileID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	if _, err := tx.Exec(ctx, `
DELETE FROM quick_access_chats WHERE profile_id = $1 AND chat_id = $2
`, profileID, chatID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
DELETE FROM folder_chats WHERE profile_id = $1 AND chat_id = $2
`, profileID, chatID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ClearDeletedForSelf clears soft-delete so the DM reappears in the caller's inbox (CreateDM/GetDM / incoming).
func (s *DMStore) ClearDeletedForSelf(ctx context.Context, chatID, profileID uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return errors.New("dm store: pool not configured")
	}
	_, err := s.Pool.Exec(ctx, `
UPDATE chat_members
SET deleted_for_self = false
WHERE chat_id = $1 AND profile_id = $2 AND deleted_for_self = true
`, chatID, profileID)
	return err
}

// IsMemberDeletedForSelf reports soft-delete for a membership row.
func (s *DMStore) IsMemberDeletedForSelf(ctx context.Context, chatID, profileID uuid.UUID) (bool, error) {
	if s == nil || s.Pool == nil {
		return false, errors.New("dm store: pool not configured")
	}
	var deleted bool
	err := s.Pool.QueryRow(ctx, `
SELECT deleted_for_self FROM chat_members WHERE chat_id = $1 AND profile_id = $2
`, chatID, profileID).Scan(&deleted)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return deleted, nil
}

var ErrDeleteChatDMOnly = errors.New("delete chat is only supported for DMs")
