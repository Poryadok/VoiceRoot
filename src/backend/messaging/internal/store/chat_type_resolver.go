package store

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	chatv1 "voice.app/voice/chat/v1"
)

// SQLChatTypeResolver reads the type held by Chat's database. It is used only
// when Messaging is explicitly configured with Chat metadata SQL access.
type SQLChatTypeResolver struct {
	Pool *pgxpool.Pool
}

func (r *SQLChatTypeResolver) ResolveChatType(ctx context.Context, chatID, _ uuid.UUID) (chatv1.ChatType, error) {
	if r == nil || r.Pool == nil {
		return chatv1.ChatType_CHAT_TYPE_UNSPECIFIED, pgx.ErrNoRows
	}
	var raw string
	if err := r.Pool.QueryRow(ctx, `SELECT type FROM chats WHERE id = $1`, chatID).Scan(&raw); err != nil {
		return chatv1.ChatType_CHAT_TYPE_UNSPECIFIED, err
	}
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "dm":
		return chatv1.ChatType_CHAT_TYPE_DM, nil
	case "group":
		return chatv1.ChatType_CHAT_TYPE_GROUP, nil
	case "channel":
		return chatv1.ChatType_CHAT_TYPE_CHANNEL, nil
	default:
		return chatv1.ChatType_CHAT_TYPE_UNSPECIFIED, nil
	}
}
