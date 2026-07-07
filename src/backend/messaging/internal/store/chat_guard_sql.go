package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SQLChatGuard checks chat_db.chat_members (app stack: DM). Pool must point at DB where chat tables exist.
type SQLChatGuard struct {
	Pool      *pgxpool.Pool
	SpacePool *pgxpool.Pool // optional space_db for chats with space_id
}

func (g *SQLChatGuard) EnsureMember(ctx context.Context, chatID, profileID uuid.UUID) error {
	if g == nil || g.Pool == nil {
		return nil
	}
	var spaceID *uuid.UUID
	err := g.Pool.QueryRow(ctx, `SELECT space_id FROM chats WHERE id = $1`, chatID).Scan(&spaceID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if spaceID != nil && g.SpacePool != nil {
		var one int
		err := g.SpacePool.QueryRow(ctx, `
SELECT 1 FROM space_members WHERE space_id = $1 AND profile_id = $2 LIMIT 1
`, *spaceID, profileID).Scan(&one)
		if err == nil {
			return nil
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotChatMember
		}
		return err
	}
	var one int
	err = g.Pool.QueryRow(ctx, `
SELECT 1 FROM chat_members
WHERE chat_id = $1 AND profile_id = $2
LIMIT 1
`, chatID, profileID).Scan(&one)
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotChatMember
	}
	return err
}

func (g *SQLChatGuard) DMOtherProfileID(ctx context.Context, chatID, profileID uuid.UUID) (uuid.UUID, error) {
	if g == nil || g.Pool == nil {
		return uuid.Nil, errors.New("chat guard: pool not configured")
	}
	if err := g.EnsureMember(ctx, chatID, profileID); err != nil {
		return uuid.Nil, err
	}
	var other uuid.UUID
	err := g.Pool.QueryRow(ctx, `
SELECT profile_id FROM chat_members
WHERE chat_id = $1 AND profile_id <> $2
LIMIT 1
`, chatID, profileID).Scan(&other)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, errors.New("dm must have exactly two members")
	}
	if err != nil {
		return uuid.Nil, err
	}
	return other, nil
}

func (g *SQLChatGuard) OtherMemberProfileIDs(ctx context.Context, chatID, profileID uuid.UUID) ([]uuid.UUID, error) {
	if g == nil || g.Pool == nil {
		return nil, errors.New("chat guard: pool not configured")
	}
	if err := g.EnsureMember(ctx, chatID, profileID); err != nil {
		return nil, err
	}
	rows, err := g.Pool.Query(ctx, `
SELECT profile_id FROM chat_members
WHERE chat_id = $1 AND profile_id <> $2
`, chatID, profileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []uuid.UUID
	for rows.Next() {
		var pid uuid.UUID
		if err := rows.Scan(&pid); err != nil {
			return nil, err
		}
		out = append(out, pid)
	}
	return out, rows.Err()
}
