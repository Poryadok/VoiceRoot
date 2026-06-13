package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"voice/backend/messaging/internal/mentions"
)

// SQLChatMentionsMeta loads chat type, optional space_id, and member profile IDs from chat_db.
type SQLChatMentionsMeta struct {
	Pool      *pgxpool.Pool
	SpacePool *pgxpool.Pool // optional space_db for chats with space_id
}

func (s *SQLChatMentionsMeta) LoadChatMeta(ctx context.Context, chatID uuid.UUID) (mentions.ChatMeta, error) {
	if s == nil || s.Pool == nil {
		return mentions.ChatMeta{}, errors.New("chat mentions meta: pool not configured")
	}
	var chatType string
	var spaceID *uuid.UUID
	err := s.Pool.QueryRow(ctx, `
SELECT type, space_id FROM chats WHERE id = $1
`, chatID).Scan(&chatType, &spaceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return mentions.ChatMeta{}, ErrNotChatMember
	}
	if err != nil {
		return mentions.ChatMeta{}, err
	}
	rows, err := s.Pool.Query(ctx, `
SELECT profile_id FROM chat_members WHERE chat_id = $1
`, chatID)
	if err != nil {
		return mentions.ChatMeta{}, err
	}
	defer rows.Close()
	var members []uuid.UUID
	for rows.Next() {
		var pid uuid.UUID
		if err := rows.Scan(&pid); err != nil {
			return mentions.ChatMeta{}, err
		}
		members = append(members, pid)
	}
	if err := rows.Err(); err != nil {
		return mentions.ChatMeta{}, err
	}
	if spaceID != nil && len(members) == 0 && s.SpacePool != nil {
		members, err = s.loadSpaceMembers(ctx, *spaceID)
		if err != nil {
			return mentions.ChatMeta{}, err
		}
	}
	return mentions.ChatMeta{
		ChatID:   chatID,
		ChatType: chatType,
		SpaceID:  spaceID,
		Members:  members,
	}, nil
}

func (s *SQLChatMentionsMeta) loadSpaceMembers(ctx context.Context, spaceID uuid.UUID) ([]uuid.UUID, error) {
	if s == nil || s.SpacePool == nil {
		return nil, errors.New("chat mentions meta: space pool not configured")
	}
	rows, err := s.SpacePool.Query(ctx, `
SELECT profile_id FROM space_members WHERE space_id = $1
`, spaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var members []uuid.UUID
	for rows.Next() {
		var pid uuid.UUID
		if err := rows.Scan(&pid); err != nil {
			return nil, err
		}
		members = append(members, pid)
	}
	return members, rows.Err()
}
