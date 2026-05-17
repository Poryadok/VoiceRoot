package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ChatMemberRow is one row from chat_members joined with chat type check.
type ChatMemberRow struct {
	ProfileID  uuid.UUID
	Role       string
	JoinedAt   time.Time
	MutedUntil sql.NullTime
	IsArchived bool
}

// FindDMChatByID loads a DM chat row by id, or returns nil if not found or not type dm.
func (s *DMStore) FindDMChatByID(ctx context.Context, chatID uuid.UUID) (*ChatRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("dm store: pool not configured")
	}
	var id, creator uuid.UUID
	var createdAt, updatedAt time.Time
	var lastMsg sql.NullTime
	err := s.Pool.QueryRow(ctx, `
SELECT id, creator_profile_id, last_message_at, created_at, updated_at
FROM chats
WHERE id = $1 AND type = 'dm'
`, chatID).Scan(&id, &creator, &lastMsg, &createdAt, &updatedAt)
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
		CreatorProfileID: creator,
		CreatedAt:        createdAt.UTC(),
		UpdatedAt:        updatedAt.UTC(),
		LastMessageAt:    lm,
	}, nil
}

// IsChatMember reports whether profileID is a member of chatID (any chat type in DB).
func (s *DMStore) IsChatMember(ctx context.Context, chatID, profileID uuid.UUID) (bool, error) {
	if s == nil || s.Pool == nil {
		return false, errors.New("dm store: pool not configured")
	}
	var one int
	err := s.Pool.QueryRow(ctx, `
SELECT 1 FROM chat_members WHERE chat_id = $1 AND profile_id = $2 LIMIT 1
`, chatID, profileID).Scan(&one)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// ListChatMembers returns all members for a chat ordered by joined_at, profile_id.
func (s *DMStore) ListChatMembers(ctx context.Context, chatID uuid.UUID) ([]ChatMemberRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("dm store: pool not configured")
	}
	rows, err := s.Pool.Query(ctx, `
SELECT profile_id, role, joined_at, muted_until, is_archived
FROM chat_members
WHERE chat_id = $1
ORDER BY joined_at ASC, profile_id ASC
`, chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ChatMemberRow
	for rows.Next() {
		var r ChatMemberRow
		if err := rows.Scan(&r.ProfileID, &r.Role, &r.JoinedAt, &r.MutedUntil, &r.IsArchived); err != nil {
			return nil, err
		}
		r.JoinedAt = r.JoinedAt.UTC()
		out = append(out, r)
	}
	return out, rows.Err()
}
