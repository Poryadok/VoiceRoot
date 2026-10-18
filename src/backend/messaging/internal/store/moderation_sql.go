package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrMemberTimedOut = errors.New("member is timed out in space")
	ErrSlowModeActive = errors.New("slow mode active")
)

// SQLModerationGuard enforces Chat-owned slow mode. Space communication
// authority is resolved through the protected Space RPC, never through SQL.
type SQLModerationGuard struct {
	Pool     *pgxpool.Pool // legacy single-DB tests
	ChatPool *pgxpool.Pool
	MsgPool  *pgxpool.Pool
}

func (g *SQLModerationGuard) chatPool() *pgxpool.Pool {
	if g != nil && g.ChatPool != nil {
		return g.ChatPool
	}
	if g != nil {
		return g.Pool
	}
	return nil
}

func (g *SQLModerationGuard) msgPool() *pgxpool.Pool {
	if g != nil && g.MsgPool != nil {
		return g.MsgPool
	}
	if g != nil {
		return g.Pool
	}
	return nil
}

type ChatModerationMeta struct {
	SpaceID         *uuid.UUID
	SlowModeSeconds int32
}

func (g *SQLModerationGuard) ChatMeta(ctx context.Context, chatID uuid.UUID) (ChatModerationMeta, error) {
	var meta ChatModerationMeta
	pool := g.chatPool()
	if g == nil || pool == nil {
		return meta, nil
	}
	var spaceID *uuid.UUID
	err := pool.QueryRow(ctx, `
SELECT space_id, slow_mode_seconds FROM chats WHERE id = $1
`, chatID).Scan(&spaceID, &meta.SlowModeSeconds)
	if err != nil {
		return meta, err
	}
	meta.SpaceID = spaceID
	return meta, nil
}

func (g *SQLModerationGuard) LastSenderMessageAt(ctx context.Context, chatID, profileID uuid.UUID) (*time.Time, error) {
	pool := g.msgPool()
	if g == nil || pool == nil {
		return nil, nil
	}
	var at time.Time
	err := pool.QueryRow(ctx, `
SELECT created_at FROM messages
WHERE chat_id = $1 AND sender_profile_id = $2
ORDER BY created_at DESC
LIMIT 1
`, chatID, profileID).Scan(&at)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &at, nil
}

func (g *SQLModerationGuard) EnsureCanSend(ctx context.Context, chatID, profileID uuid.UUID) error {
	meta, err := g.ChatMeta(ctx, chatID)
	if err != nil {
		return err
	}
	if meta.SlowModeSeconds > 0 {
		last, err := g.LastSenderMessageAt(ctx, chatID, profileID)
		if err != nil {
			return err
		}
		if last != nil {
			elapsed := time.Since(*last)
			if elapsed < time.Duration(meta.SlowModeSeconds)*time.Second {
				return ErrSlowModeActive
			}
		}
	}
	return nil
}
