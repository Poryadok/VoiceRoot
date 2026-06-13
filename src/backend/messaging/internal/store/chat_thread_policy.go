package store

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ChatThreadPolicy holds per-chat thread and main-feed settings from chat_db.chats.
type ChatThreadPolicy struct {
	ChatType          string
	SpaceID           *uuid.UUID
	ThreadsEnabled    bool
	AllowUserMainFeed bool
}

// SQLChatThreadPolicy loads thread settings from chat_db.
type SQLChatThreadPolicy struct {
	Pool *pgxpool.Pool
}

func (s *SQLChatThreadPolicy) Load(ctx context.Context, chatID uuid.UUID) (*ChatThreadPolicy, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("chat thread policy: pool not configured")
	}
	var chatType string
	var spaceID *uuid.UUID
	var threadsEnabled, allowUserMainFeed bool
	var spaceRaw sql.NullString
	err := s.Pool.QueryRow(ctx, `
SELECT type, space_id::text, threads_enabled, allow_user_main_feed
FROM chats
WHERE id = $1
`, chatID).Scan(&chatType, &spaceRaw, &threadsEnabled, &allowUserMainFeed)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrChatNotFound
	}
	if err != nil {
		return nil, err
	}
	if spaceRaw.Valid && spaceRaw.String != "" {
		if sid, perr := uuid.Parse(spaceRaw.String); perr == nil {
			spaceID = &sid
		}
	}
	return &ChatThreadPolicy{
		ChatType:          chatType,
		SpaceID:           spaceID,
		ThreadsEnabled:    threadsEnabled,
		AllowUserMainFeed: allowUserMainFeed,
	}, nil
}
