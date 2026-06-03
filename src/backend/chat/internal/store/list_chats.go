package store

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ErrInvalidListCursor is returned when ListChatsPage receives a non-empty cursor that cannot be decoded.
var ErrInvalidListCursor = errors.New("invalid list chats cursor")

// ListChatsPage holds one page of the caller's non-archived DM chats.
type ListChatsPage struct {
	Rows       []*ChatRow
	NextCursor string
}

type listChatCursorPayload struct {
	S string `json:"s"` // RFC3339Nano UTC, sort key = COALESCE(last_message_at, created_at)
	I string `json:"i"` // chat id UUID
}

func encodeListChatCursor(sortKey time.Time, chatID uuid.UUID) string {
	p := listChatCursorPayload{
		S: sortKey.UTC().Format(time.RFC3339Nano),
		I: chatID.String(),
	}
	b, _ := json.Marshal(p)
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeListChatCursor(raw string) (time.Time, uuid.UUID, error) {
	if raw == "" {
		return time.Time{}, uuid.Nil, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return time.Time{}, uuid.Nil, ErrInvalidListCursor
	}
	var p listChatCursorPayload
	if err := json.Unmarshal(b, &p); err != nil {
		return time.Time{}, uuid.Nil, ErrInvalidListCursor
	}
	ts, err := time.Parse(time.RFC3339Nano, p.S)
	if err != nil {
		return time.Time{}, uuid.Nil, ErrInvalidListCursor
	}
	id, err := uuid.Parse(p.I)
	if err != nil {
		return time.Time{}, uuid.Nil, ErrInvalidListCursor
	}
	return ts.UTC(), id, nil
}

// ListChatsPage returns DM chats the profile is a member of (non-archived), ordered by recent activity.
// sort key: COALESCE(last_message_at, created_at) DESC, id DESC. Cursor is opaque (see encodeListChatCursor).
func (s *DMStore) ListChatsPage(ctx context.Context, viewerProfileID uuid.UUID, cursor string, limit int, inbox string) (*ListChatsPage, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("dm store: pool not configured")
	}
	if limit < 1 {
		limit = 1
	}
	fetch := limit + 1
	if inbox == "" {
		inbox = "main"
	}

	sortTS, chatID, err := decodeListChatCursor(cursor)
	if err != nil {
		return nil, err
	}

	var rows pgx.Rows
	if cursor == "" {
		rows, err = s.Pool.Query(ctx, `
SELECT c.id, c.creator_profile_id, c.last_message_at, c.created_at, c.updated_at, m.inbox_bucket,
       COALESCE(c.last_message_at, c.created_at) AS sort_at
FROM chats c
INNER JOIN chat_members m ON m.chat_id = c.id AND m.profile_id = $1
WHERE c.type = 'dm' AND m.is_archived = false AND m.inbox_bucket = $3
ORDER BY sort_at DESC, c.id DESC
LIMIT $2
`, viewerProfileID, fetch, inbox)
	} else {
		rows, err = s.Pool.Query(ctx, `
SELECT c.id, c.creator_profile_id, c.last_message_at, c.created_at, c.updated_at, m.inbox_bucket,
       COALESCE(c.last_message_at, c.created_at) AS sort_at
FROM chats c
INNER JOIN chat_members m ON m.chat_id = c.id AND m.profile_id = $1
WHERE c.type = 'dm' AND m.is_archived = false AND m.inbox_bucket = $5
  AND (
    COALESCE(c.last_message_at, c.created_at) < $2::timestamptz
    OR (
      COALESCE(c.last_message_at, c.created_at) = $2::timestamptz
      AND c.id < $3::uuid
    )
  )
ORDER BY sort_at DESC, c.id DESC
LIMIT $4
`, viewerProfileID, sortTS, chatID, fetch, inbox)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*ChatRow
	for rows.Next() {
		var id, creator uuid.UUID
		var lastMsg sql.NullTime
		var createdAt, updatedAt time.Time
		var inboxBucket string
		var sortAt time.Time
		if err := rows.Scan(&id, &creator, &lastMsg, &createdAt, &updatedAt, &inboxBucket, &sortAt); err != nil {
			return nil, err
		}
		var lm *time.Time
		if lastMsg.Valid {
			t := lastMsg.Time.UTC()
			lm = &t
		}
		out = append(out, &ChatRow{
			ID:               id,
			CreatorProfileID: creator,
			CreatedAt:        createdAt.UTC(),
			UpdatedAt:        updatedAt.UTC(),
			LastMessageAt:    lm,
			InboxBucket:      inboxBucket,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	next := ""
	if len(out) > limit {
		last := out[limit-1]
		// recompute sort key for cursor (same as SQL COALESCE)
		sk := last.CreatedAt
		if last.LastMessageAt != nil {
			sk = *last.LastMessageAt
		}
		next = encodeListChatCursor(sk, last.ID)
		out = out[:limit]
	}

	return &ListChatsPage{Rows: out, NextCursor: next}, nil
}
