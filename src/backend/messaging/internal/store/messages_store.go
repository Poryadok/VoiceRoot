package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"voice/backend/messaging/internal/markdown"
)

// MessageRow is a persisted messaging_db.messages row (v1 DM).
type MessageRow struct {
	ID                uuid.UUID
	ChatID            uuid.UUID
	ChatType          string
	SenderProfileID   uuid.UUID
	PostedAsChat      bool
	DisplayChatID     *uuid.UUID
	Content           string
	Type              string
	ThreadParentID    *uuid.UUID
	ForwardFromID     *uuid.UUID
	ForwardFromSender string
	AttachmentsJSON   string
	MentionsJSON      string
	ClientMessageID   *uuid.UUID
	EditedAt          *time.Time
	DeletedAt         *time.Time
	CreatedAt         time.Time
}

type MessagesStore struct {
	Pool *pgxpool.Pool
}

type ChatListMetadataRow struct {
	ChatID             uuid.UUID
	LastMessagePreview string
	LastMessageAt      *time.Time
	UnreadCount        int64
}

func (s *MessagesStore) MessageExists(ctx context.Context, chatID, messageID uuid.UUID) (bool, error) {
	if s == nil || s.Pool == nil {
		return false, errors.New("messages store: pool not configured")
	}
	var one int
	err := s.Pool.QueryRow(ctx, `
SELECT 1 FROM messages
WHERE chat_id = $1 AND id = $2 AND deleted_at IS NULL
LIMIT 1
`, chatID, messageID).Scan(&one)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return false, err
}

func (s *MessagesStore) GetByClientDedupKey(ctx context.Context, chatID, senderProfileID, clientMessageID uuid.UUID) (*MessageRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("messages store: pool not configured")
	}
	return scanMessageRow(s.Pool.QueryRow(ctx, messageSelectSQL+`
FROM messages
WHERE chat_id = $1 AND sender_profile_id = $2 AND client_message_id = $3
LIMIT 1
`, chatID, senderProfileID, clientMessageID))
}

func (s *MessagesStore) InsertMessage(ctx context.Context, row MessageRow) (*MessageRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("messages store: pool not configured")
	}
	if !json.Valid([]byte(row.AttachmentsJSON)) {
		return nil, errors.New("attachments_json must be valid JSON")
	}
	if !json.Valid([]byte(row.MentionsJSON)) {
		return nil, errors.New("mentions_json must be valid JSON")
	}

	var clientAny any
	if row.ClientMessageID != nil {
		clientAny = *row.ClientMessageID
	}

	var threadAny any
	if row.ThreadParentID != nil {
		threadAny = *row.ThreadParentID
	}
	var forwardFromAny any
	if row.ForwardFromID != nil {
		forwardFromAny = *row.ForwardFromID
	}
	var forwardSenderAny any
	if row.ForwardFromSender != "" {
		forwardSenderAny = row.ForwardFromSender
	}
	chatType := row.ChatType
	if chatType == "" {
		chatType = "dm"
	}
	var displayAny any
	if row.DisplayChatID != nil {
		displayAny = *row.DisplayChatID
	}

	q := `
INSERT INTO messages (
  id, chat_id, chat_type, sender_profile_id, posted_as_chat, display_chat_id,
  content, type, thread_parent_id, forward_from_id, forward_from_sender,
  attachments, mentions, client_message_id
) VALUES (
  $1, $2, $3, $4, $5, $6,
  $7, $8, $9, $10, $11, $12::jsonb, $13::jsonb, $14
)
ON CONFLICT (chat_id, sender_profile_id, client_message_id)
  WHERE client_message_id IS NOT NULL
  DO NOTHING
`
	ct, err := s.Pool.Exec(ctx, q,
		row.ID, row.ChatID, chatType, row.SenderProfileID, row.PostedAsChat, displayAny,
		row.Content, row.Type, threadAny, forwardFromAny, forwardSenderAny,
		row.AttachmentsJSON, row.MentionsJSON, clientAny,
	)
	if err != nil {
		return nil, err
	}
	if ct.RowsAffected() == 0 {
		if row.ClientMessageID == nil {
			return nil, errors.New("messages store: insert produced no row")
		}
		return scanMessageRow(s.Pool.QueryRow(ctx, messageSelectSQL+`
FROM messages
WHERE chat_id = $1 AND sender_profile_id = $2 AND client_message_id = $3
LIMIT 1
`, row.ChatID, row.SenderProfileID, *row.ClientMessageID))
	}
	return s.GetMessageByID(ctx, row.ID)
}

const messageSelectSQL = `
SELECT id, chat_id, chat_type, sender_profile_id, posted_as_chat, display_chat_id,
       content, type, thread_parent_id,
       forward_from_id, forward_from_sender,
       attachments::text, mentions::text, client_message_id, edited_at, deleted_at, created_at
`

const messageReturningCols = `id, chat_id, chat_type, sender_profile_id, posted_as_chat, display_chat_id,
       content, type, thread_parent_id,
       forward_from_id, forward_from_sender,
       attachments::text, mentions::text, client_message_id, edited_at, deleted_at, created_at`

func scanMessageRow(row pgx.Row) (*MessageRow, error) {
	var m MessageRow
	var threadID *uuid.UUID
	var forwardFromID *uuid.UUID
	var forwardSender *string
	var clientID *uuid.UUID
	var displayID *uuid.UUID
	err := row.Scan(
		&m.ID, &m.ChatID, &m.ChatType, &m.SenderProfileID, &m.PostedAsChat, &displayID,
		&m.Content, &m.Type, &threadID,
		&forwardFromID, &forwardSender,
		&m.AttachmentsJSON, &m.MentionsJSON, &clientID, &m.EditedAt, &m.DeletedAt, &m.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	m.ThreadParentID = threadID
	m.DisplayChatID = displayID
	if forwardFromID != nil {
		m.ForwardFromID = forwardFromID
	}
	if forwardSender != nil {
		m.ForwardFromSender = *forwardSender
	}
	m.ClientMessageID = clientID
	return &m, nil
}

func (s *MessagesStore) GetMessageByID(ctx context.Context, id uuid.UUID) (*MessageRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("messages store: pool not configured")
	}
	return scanMessageRow(s.Pool.QueryRow(ctx, messageSelectSQL+`
FROM messages WHERE id = $1
`, id))
}

// UpdateMessageContent sets content and edited_at for a non-deleted row owned by senderProfileID.
func (s *MessagesStore) UpdateMessageContent(ctx context.Context, messageID, senderProfileID uuid.UUID, content string) (*MessageRow, error) {
	return s.UpdateMessageContentAndMentions(ctx, messageID, senderProfileID, content, nil)
}

// UpdateMessageContentAndMentions sets content, optional mentions_json, and edited_at.
func (s *MessagesStore) UpdateMessageContentAndMentions(ctx context.Context, messageID, senderProfileID uuid.UUID, content string, mentionsJSON *string) (*MessageRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("messages store: pool not configured")
	}
	if mentionsJSON == nil {
		return scanMessageRow(s.Pool.QueryRow(ctx, `
UPDATE messages
SET content = $1, edited_at = now()
WHERE id = $2 AND sender_profile_id = $3 AND deleted_at IS NULL
RETURNING `+messageReturningCols+`
`, content, messageID, senderProfileID))
	}
	return scanMessageRow(s.Pool.QueryRow(ctx, `
UPDATE messages
SET content = $1, mentions_json = $2::jsonb, edited_at = now()
WHERE id = $3 AND sender_profile_id = $4 AND deleted_at IS NULL
RETURNING `+messageReturningCols+`
`, content, *mentionsJSON, messageID, senderProfileID))
}

// SoftDeleteMessage sets deleted_at for a non-deleted row owned by senderProfileID.
func (s *MessagesStore) SoftDeleteMessage(ctx context.Context, messageID, senderProfileID uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return errors.New("messages store: pool not configured")
	}
	ct, err := s.Pool.Exec(ctx, `
UPDATE messages
SET deleted_at = now()
WHERE id = $1 AND sender_profile_id = $2 AND deleted_at IS NULL
`, messageID, senderProfileID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *MessagesStore) HideMessageForProfile(ctx context.Context, messageID, profileID uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return errors.New("messages store: pool not configured")
	}
	_, err := s.Pool.Exec(ctx, `
INSERT INTO message_hides (message_id, profile_id)
VALUES ($1, $2)
ON CONFLICT (message_id, profile_id) DO NOTHING
`, messageID, profileID)
	return err
}

type ListMode int

const (
	ListLatest ListMode = iota
	ListBeforeID
	ListAfterID
)

func (s *MessagesStore) ListMessages(ctx context.Context, chatID, viewerProfileID uuid.UUID, mode ListMode, refID *uuid.UUID, limit int) ([]MessageRow, error) {
	return s.listMessagesFiltered(ctx, chatID, viewerProfileID, mode, refID, limit, true, nil)
}

func (s *MessagesStore) ListThreadMessages(ctx context.Context, chatID, viewerProfileID, threadParentID uuid.UUID, mode ListMode, refID *uuid.UUID, limit int) ([]MessageRow, error) {
	return s.listMessagesFiltered(ctx, chatID, viewerProfileID, mode, refID, limit, false, &threadParentID)
}

func (s *MessagesStore) ThreadHasReplies(ctx context.Context, chatID, threadParentID uuid.UUID) (bool, error) {
	if s == nil || s.Pool == nil {
		return false, errors.New("messages store: pool not configured")
	}
	var exists bool
	err := s.Pool.QueryRow(ctx, `
SELECT EXISTS(
  SELECT 1 FROM messages
  WHERE chat_id = $1 AND thread_parent_id = $2 AND deleted_at IS NULL
)
`, chatID, threadParentID).Scan(&exists)
	return exists, err
}

func (s *MessagesStore) listMessagesFiltered(ctx context.Context, chatID, viewerProfileID uuid.UUID, mode ListMode, refID *uuid.UUID, limit int, mainFeedOnly bool, threadParentID *uuid.UUID) ([]MessageRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("messages store: pool not configured")
	}
	if limit < 1 {
		limit = 1
	}
	fetch := limit + 1

	threadClause := ""
	argsBase := []any{chatID}
	argN := 2
	if mainFeedOnly {
		threadClause = " AND thread_parent_id IS NULL"
	} else if threadParentID != nil {
		threadClause = " AND thread_parent_id = $" + itoa(argN)
		argsBase = append(argsBase, *threadParentID)
		argN++
	}

	var rows pgx.Rows
	var err error
	switch mode {
	case ListLatest:
		args := append(append([]any{}, argsBase...), fetch, viewerProfileID)
		rows, err = s.Pool.Query(ctx, messageSelectSQL+`
FROM messages
WHERE chat_id = $1 AND deleted_at IS NULL`+threadClause+`
  AND NOT EXISTS (
    SELECT 1 FROM message_hides h
    WHERE h.message_id = messages.id AND h.profile_id = $`+itoa(argN+1)+`
  )
ORDER BY id DESC
LIMIT $`+itoa(argN)+`
`, args...)
	case ListBeforeID:
		args := append(append([]any{}, argsBase...), *refID, fetch, viewerProfileID)
		rows, err = s.Pool.Query(ctx, messageSelectSQL+`
FROM messages
WHERE chat_id = $1 AND deleted_at IS NULL AND id < $`+itoa(argN)+`::uuid`+threadClause+`
  AND NOT EXISTS (
    SELECT 1 FROM message_hides h
    WHERE h.message_id = messages.id AND h.profile_id = $`+itoa(argN+2)+`
  )
ORDER BY id DESC
LIMIT $`+itoa(argN+1)+`
`, args...)
	case ListAfterID:
		args := append(append([]any{}, argsBase...), *refID, fetch, viewerProfileID)
		rows, err = s.Pool.Query(ctx, messageSelectSQL+`
FROM messages
WHERE chat_id = $1 AND deleted_at IS NULL AND id > $`+itoa(argN)+`::uuid`+threadClause+`
  AND NOT EXISTS (
    SELECT 1 FROM message_hides h
    WHERE h.message_id = messages.id AND h.profile_id = $`+itoa(argN+2)+`
  )
ORDER BY id ASC
LIMIT $`+itoa(argN+1)+`
`, args...)
	default:
		return nil, errors.New("messages store: unknown list mode")
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []MessageRow
	for rows.Next() {
		var m MessageRow
		var threadID *uuid.UUID
		var forwardFromID *uuid.UUID
		var forwardSender *string
		var clientID *uuid.UUID
		var displayID *uuid.UUID
		if err := rows.Scan(
			&m.ID, &m.ChatID, &m.ChatType, &m.SenderProfileID, &m.PostedAsChat, &displayID,
			&m.Content, &m.Type, &threadID,
			&forwardFromID, &forwardSender,
			&m.AttachmentsJSON, &m.MentionsJSON, &clientID, &m.EditedAt, &m.DeletedAt, &m.CreatedAt,
		); err != nil {
			return nil, err
		}
		m.ThreadParentID = threadID
		m.DisplayChatID = displayID
		if forwardFromID != nil {
			m.ForwardFromID = forwardFromID
		}
		if forwardSender != nil {
			m.ForwardFromSender = *forwardSender
		}
		m.ClientMessageID = clientID
		out = append(out, m)
	}
	return out, rows.Err()
}

func itoa(n int) string {
	return strconv.Itoa(n)
}

func (s *MessagesStore) UpsertReadReceipt(ctx context.Context, chatID, profileID, lastReadMessageID uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return errors.New("messages store: pool not configured")
	}
	_, err := s.Pool.Exec(ctx, `
INSERT INTO read_receipts (chat_id, profile_id, last_read_message_id, updated_at)
VALUES ($1, $2, $3, now())
ON CONFLICT (chat_id, profile_id) DO UPDATE SET
  last_read_message_id = CASE
    WHEN read_receipts.last_read_message_id < EXCLUDED.last_read_message_id THEN EXCLUDED.last_read_message_id
    ELSE read_receipts.last_read_message_id
  END,
  updated_at = CASE
    WHEN read_receipts.last_read_message_id < EXCLUDED.last_read_message_id THEN now()
    ELSE read_receipts.updated_at
  END
`, chatID, profileID, lastReadMessageID)
	return err
}

func (s *MessagesStore) GetReadReceipt(ctx context.Context, chatID, profileID uuid.UUID) (lastRead *uuid.UUID, updatedAt *time.Time, err error) {
	if s == nil || s.Pool == nil {
		return nil, nil, errors.New("messages store: pool not configured")
	}
	var lid uuid.UUID
	var upd time.Time
	qerr := s.Pool.QueryRow(ctx, `
SELECT last_read_message_id, updated_at FROM read_receipts
WHERE chat_id = $1 AND profile_id = $2
`, chatID, profileID).Scan(&lid, &upd)
	if errors.Is(qerr, pgx.ErrNoRows) {
		return nil, nil, nil
	}
	if qerr != nil {
		return nil, nil, qerr
	}
	u := upd.UTC()
	return &lid, &u, nil
}

func (s *MessagesStore) GetChatListMetadata(ctx context.Context, viewerProfileID uuid.UUID, chatIDs []uuid.UUID) (map[uuid.UUID]ChatListMetadataRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("messages store: pool not configured")
	}
	out := make(map[uuid.UUID]ChatListMetadataRow, len(chatIDs))
	for _, chatID := range chatIDs {
		var preview sql.NullString
		var lastAt sql.NullTime
		var unread int64
		err := s.Pool.QueryRow(ctx, `
WITH latest AS (
  SELECT content, created_at
  FROM messages
  WHERE chat_id = $1 AND deleted_at IS NULL
  ORDER BY id DESC
  LIMIT 1
), unread AS (
  SELECT count(*)::bigint AS unread_count
  FROM messages m
  LEFT JOIN read_receipts rr
    ON rr.chat_id = m.chat_id AND rr.profile_id = $2
  WHERE m.chat_id = $1
    AND m.deleted_at IS NULL
    AND m.sender_profile_id <> $2
    AND (rr.last_read_message_id IS NULL OR m.id > rr.last_read_message_id)
)
SELECT latest.content, latest.created_at, unread.unread_count
FROM unread
LEFT JOIN latest ON true
`, chatID, viewerProfileID).Scan(&preview, &lastAt, &unread)
		if err != nil {
			return nil, err
		}
		row := ChatListMetadataRow{ChatID: chatID, UnreadCount: unread}
		if preview.Valid {
			row.LastMessagePreview = truncatePreview(preview.String)
		}
		if lastAt.Valid {
			t := lastAt.Time.UTC()
			row.LastMessageAt = &t
		}
		out[chatID] = row
	}
	return out, nil
}

func truncatePreview(s string) string {
	const maxRunes = 160
	plain := markdown.StripForPreview(s)
	r := []rune(plain)
	if len(r) <= maxRunes {
		return plain
	}
	return string(r[:maxRunes])
}
