package store

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"voice/backend/messaging/internal/messageevents"
)

// ReactionAggregate is one emoji counter row for reactions_json wire format.
type ReactionAggregate struct {
	Emoji       string `json:"emoji"`
	Count       int    `json:"count"`
	ReactedByMe bool   `json:"reacted_by_me"`
}

// ReactionsStore persists per-message emoji reactions.
type ReactionsStore struct {
	Pool *pgxpool.Pool
}

// UpsertReaction inserts a reaction; duplicate (message, profile, emoji) is a no-op.
func (s *ReactionsStore) UpsertReaction(ctx context.Context, messageID, profileID uuid.UUID, emoji string) error {
	if s == nil || s.Pool == nil {
		return ErrStoreNotConfigured
	}
	_, err := s.Pool.Exec(ctx, `
INSERT INTO reactions (message_id, profile_id, emoji)
VALUES ($1, $2, $3)
ON CONFLICT (message_id, profile_id, emoji) DO NOTHING
`, messageID, profileID, emoji)
	return err
}

// DeleteReaction removes one reaction row; missing row is not an error.
func (s *ReactionsStore) DeleteReaction(ctx context.Context, messageID, profileID uuid.UUID, emoji string) error {
	if s == nil || s.Pool == nil {
		return ErrStoreNotConfigured
	}
	_, err := s.Pool.Exec(ctx, `
DELETE FROM reactions WHERE message_id = $1 AND profile_id = $2 AND emoji = $3
`, messageID, profileID, emoji)
	return err
}

func (s *ReactionsStore) MutateReactionWithOutbox(ctx context.Context, chatID, spaceID, messageID, profileID uuid.UUID, emoji string, add bool, event messageevents.OutboxEvent) error {
	if s == nil || s.Pool == nil {
		return ErrStoreNotConfigured
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var scope *uuid.UUID
	if spaceID != uuid.Nil {
		scope = &spaceID
	}
	if err := lockMessageMutation(ctx, tx, chatID, scope, &messageID); err != nil {
		return err
	}
	if err := requireLiveMessage(ctx, tx, chatID, messageID); err != nil {
		return err
	}
	if add {
		_, err = tx.Exec(ctx, `INSERT INTO reactions(message_id,profile_id,emoji) VALUES($1,$2,$3) ON CONFLICT(message_id,profile_id,emoji) DO NOTHING`, messageID, profileID, emoji)
	} else {
		_, err = tx.Exec(ctx, `DELETE FROM reactions WHERE message_id=$1 AND profile_id=$2 AND emoji=$3`, messageID, profileID, emoji)
	}
	if err != nil {
		return err
	}
	if event.MessageID != messageID || event.ChatID != chatID {
		return errors.New("reaction outbox event identity mismatch")
	}
	if err := enqueueMessageEvent(ctx, tx, event); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ReactionsJSONByMessageIDs returns reactions_json per message id for the viewer profile.
func (s *ReactionsStore) ReactionsJSONByMessageIDs(ctx context.Context, messageIDs []uuid.UUID, viewerProfileID uuid.UUID) (map[uuid.UUID]string, error) {
	out := make(map[uuid.UUID]string, len(messageIDs))
	if s == nil || s.Pool == nil || len(messageIDs) == 0 {
		return out, nil
	}
	rows, err := s.Pool.Query(ctx, `
SELECT message_id, emoji, COUNT(*)::int AS cnt, BOOL_OR(profile_id = $2) AS reacted_by_me
FROM reactions
WHERE message_id = ANY($1)
GROUP BY message_id, emoji
ORDER BY message_id, emoji
`, messageIDs, viewerProfileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type bucket struct {
		aggs []ReactionAggregate
	}
	byMsg := make(map[uuid.UUID]*bucket)
	for rows.Next() {
		var msgID uuid.UUID
		var emoji string
		var cnt int
		var reactedByMe bool
		if err := rows.Scan(&msgID, &emoji, &cnt, &reactedByMe); err != nil {
			return nil, err
		}
		b := byMsg[msgID]
		if b == nil {
			b = &bucket{}
			byMsg[msgID] = b
		}
		b.aggs = append(b.aggs, ReactionAggregate{
			Emoji:       emoji,
			Count:       cnt,
			ReactedByMe: reactedByMe,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for id, b := range byMsg {
		if len(b.aggs) == 0 {
			out[id] = "[]"
			continue
		}
		raw, err := json.Marshal(b.aggs)
		if err != nil {
			return nil, err
		}
		out[id] = string(raw)
	}
	return out, nil
}
