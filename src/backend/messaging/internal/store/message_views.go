package store

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// RecordMessageView records only the authenticated profile supplied by the handler
// after current Chat membership and per-message visibility authorization.
func (s *MessagesStore) RecordMessageView(ctx context.Context, chatID uuid.UUID, spaceID *uuid.UUID, messageID, profileID uuid.UUID) (uint64, error) {
	if s == nil || s.Pool == nil || profileID == uuid.Nil {
		return 0, errors.New("message views unavailable")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err = lockMessageMutation(ctx, tx, chatID, spaceID, &messageID); err != nil {
		return 0, err
	}
	var live bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM messages m WHERE id=$1 AND chat_id=$2 AND chat_type IN ('group','channel') AND deleted_at IS NULL AND (NOT ghost_only OR sender_profile_id=$3) AND NOT EXISTS(SELECT 1 FROM message_hides h WHERE h.message_id=m.id AND h.profile_id=$3))`, messageID, chatID, profileID).Scan(&live)
	if err != nil {
		return 0, err
	}
	if !live {
		return 0, pgx.ErrNoRows
	}
	if err = insertMessageViews(ctx, tx, chatID, profileID, []uuid.UUID{messageID}); err != nil {
		return 0, err
	}
	var count uint64
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM message_views WHERE message_id=$1`, messageID).Scan(&count); err != nil {
		return 0, err
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, err
	}
	return count, nil
}

func insertMessageViews(ctx context.Context, tx pgx.Tx, chatID, profileID uuid.UUID, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := tx.Exec(ctx, `INSERT INTO message_views(message_id,profile_id)
 SELECT id,$2 FROM messages m WHERE chat_id=$1 AND id=ANY($3::uuid[]) AND chat_type IN ('group','channel') AND deleted_at IS NULL
 AND (NOT ghost_only OR sender_profile_id=$2)
 AND NOT EXISTS(SELECT 1 FROM message_hides h WHERE h.message_id=m.id AND h.profile_id=$2)
 AND NOT EXISTS(SELECT 1 FROM managed_chat_purge_operations o JOIN managed_chat_purge_messages p ON p.operation_id=o.operation_id WHERE o.chat_id=$1 AND o.state='PENDING' AND p.message_id=m.id)
 ON CONFLICT(message_id,profile_id) DO NOTHING`, chatID, profileID, ids)
	return err
}

// MessageViewCandidates is keyset-paged across every message, including replies.
// The handler applies cross-service history entitlement and block policy per row.
func (s *MessagesStore) MessageViewCandidates(ctx context.Context, chatID, profileID, through, after uuid.UUID) ([]MessageRow, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id,sender_profile_id,created_at FROM messages m WHERE chat_id=$1 AND id<=$3 AND id>$4
 AND chat_type IN ('group','channel') AND deleted_at IS NULL AND (NOT ghost_only OR sender_profile_id=$2)
 AND NOT EXISTS(SELECT 1 FROM message_hides h WHERE h.message_id=m.id AND h.profile_id=$2)
 AND NOT EXISTS(SELECT 1 FROM message_views v WHERE v.message_id=m.id AND v.profile_id=$2)
 ORDER BY id LIMIT 200`, chatID, profileID, through, after)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]MessageRow, 0, 200)
	for rows.Next() {
		m := MessageRow{ChatID: chatID}
		if err := rows.Scan(&m.ID, &m.SenderProfileID, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *MessagesStore) hydrateMessageViews(ctx context.Context, messages []MessageRow) error {
	ids := make([]uuid.UUID, 0, len(messages))
	for _, m := range messages {
		if m.ChatType == "group" || m.ChatType == "channel" {
			ids = append(ids, m.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	rows, err := s.Pool.Query(ctx, `SELECT message_id,count(*) FROM message_views WHERE message_id=ANY($1::uuid[]) GROUP BY message_id`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	counts := map[uuid.UUID]uint64{}
	for rows.Next() {
		var id uuid.UUID
		var n uint64
		if err := rows.Scan(&id, &n); err != nil {
			return err
		}
		counts[id] = n
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for i := range messages {
		messages[i].ViewCount = counts[messages[i].ID]
	}
	return nil
}

// MessageViewProfiles bounds each authority lookup without copying User data.
func (s *MessagesStore) MessageViewProfiles(ctx context.Context, after uuid.UUID, limit int) ([]uuid.UUID, error) {
	if s == nil || s.Pool == nil || limit < 1 || limit > 500 {
		return nil, errors.New("invalid message view compaction page")
	}
	rows, err := s.Pool.Query(ctx, `SELECT DISTINCT profile_id FROM message_views WHERE profile_id>$1 ORDER BY profile_id LIMIT $2`, after, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// DeleteProfileMessageViews atomically removes rows and changes derived counts.
func (s *MessagesStore) DeleteProfileMessageViews(ctx context.Context, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	if s == nil || s.Pool == nil || len(ids) > 500 {
		return errors.New("invalid message view compaction chunk")
	}
	_, err := s.Pool.Exec(ctx, `DELETE FROM message_views WHERE profile_id=ANY($1::uuid[])`, ids)
	return err
}
