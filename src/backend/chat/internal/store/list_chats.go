package store

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ErrInvalidListCursor is returned when ListChatsPage receives a non-empty cursor that cannot be decoded.
var ErrInvalidListCursor = errors.New("invalid list chats cursor")

// ListChatsPage holds one page of dm/group/channel chats for the caller's inbox filter.
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

// ListChatsPage returns chats the profile is a member of (dm, group, channel), ordered by recent activity.
// inbox: main | requests (non-archived, filtered by inbox_bucket) | archive (is_archived only).
// When inbox=main and spaceIDs is non-empty, membership rows and visible space chats are paginated in one SQL union (R3-A16).
// sort key: COALESCE(last_message_at, created_at) DESC, id DESC. Cursor is opaque (see encodeListChatCursor).
func (s *DMStore) ListChatsPage(ctx context.Context, viewerProfileID uuid.UUID, cursor string, limit int, inbox string, spaceIDs []uuid.UUID) (*ListChatsPage, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("dm store: pool not configured")
	}
	if limit < 1 {
		limit = 1
	}
	if inbox == "" {
		inbox = "main"
	}
	if inbox == "main" && len(spaceIDs) > 0 {
		return s.listChatsPageMainWithSpaces(ctx, viewerProfileID, cursor, limit, spaceIDs)
	}
	fetch := limit + 1
	archivedOnly := inbox == "archive"

	sortTS, chatID, err := decodeListChatCursor(cursor)
	if err != nil {
		return nil, err
	}

	var rows pgx.Rows
	if cursor == "" {
		if archivedOnly {
			rows, err = s.Pool.Query(ctx, `
SELECT c.id, c.type, c.space_id, c.name, c.avatar_url, c.creator_profile_id, c.last_message_at, c.created_at, c.updated_at,
       c.slow_mode_seconds, c.threads_enabled, c.allow_user_main_feed, c.e2e_enabled, m.inbox_bucket,
       COALESCE(c.last_message_at, c.created_at) AS sort_at
FROM chats c
INNER JOIN chat_members m ON m.chat_id = c.id AND m.profile_id = $1
WHERE c.type IN ('dm', 'group', 'channel') AND m.is_archived = true
ORDER BY sort_at DESC, c.id DESC
LIMIT $2
`, viewerProfileID, fetch)
		} else {
			rows, err = s.Pool.Query(ctx, `
SELECT c.id, c.type, c.space_id, c.name, c.avatar_url, c.creator_profile_id, c.last_message_at, c.created_at, c.updated_at,
       c.slow_mode_seconds, c.threads_enabled, c.allow_user_main_feed, c.e2e_enabled, m.inbox_bucket,
       COALESCE(c.last_message_at, c.created_at) AS sort_at
FROM chats c
INNER JOIN chat_members m ON m.chat_id = c.id AND m.profile_id = $1
WHERE c.type IN ('dm', 'group', 'channel') AND m.is_archived = false AND m.inbox_bucket = $3
ORDER BY sort_at DESC, c.id DESC
LIMIT $2
`, viewerProfileID, fetch, inbox)
		}
	} else if archivedOnly {
		rows, err = s.Pool.Query(ctx, `
SELECT c.id, c.type, c.space_id, c.name, c.avatar_url, c.creator_profile_id, c.last_message_at, c.created_at, c.updated_at,
       c.slow_mode_seconds, c.threads_enabled, c.allow_user_main_feed, c.e2e_enabled, m.inbox_bucket,
       COALESCE(c.last_message_at, c.created_at) AS sort_at
FROM chats c
INNER JOIN chat_members m ON m.chat_id = c.id AND m.profile_id = $1
WHERE c.type IN ('dm', 'group', 'channel') AND m.is_archived = true
  AND (
    COALESCE(c.last_message_at, c.created_at) < $2::timestamptz
    OR (
      COALESCE(c.last_message_at, c.created_at) = $2::timestamptz
      AND c.id < $3::uuid
    )
  )
ORDER BY sort_at DESC, c.id DESC
LIMIT $4
`, viewerProfileID, sortTS, chatID, fetch)
	} else {
		rows, err = s.Pool.Query(ctx, `
SELECT c.id, c.type, c.space_id, c.name, c.avatar_url, c.creator_profile_id, c.last_message_at, c.created_at, c.updated_at,
       c.slow_mode_seconds, c.threads_enabled, c.allow_user_main_feed, c.e2e_enabled, m.inbox_bucket,
       COALESCE(c.last_message_at, c.created_at) AS sort_at
FROM chats c
INNER JOIN chat_members m ON m.chat_id = c.id AND m.profile_id = $1
WHERE c.type IN ('dm', 'group', 'channel') AND m.is_archived = false AND m.inbox_bucket = $5
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

	out, err := scanListChatPageRows(rows)
	if err != nil {
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

const listChatsSelectColumns = `
       c.id, c.type, c.space_id, c.name, c.avatar_url, c.creator_profile_id, c.last_message_at, c.created_at, c.updated_at,
       c.slow_mode_seconds, c.threads_enabled, c.allow_user_main_feed, c.e2e_enabled, m.inbox_bucket,
       COALESCE(c.last_message_at, c.created_at) AS sort_at`

func (s *DMStore) listChatsPageMainWithSpaces(ctx context.Context, viewerProfileID uuid.UUID, cursor string, limit int, spaceIDs []uuid.UUID) (*ListChatsPage, error) {
	fetch := limit + 1
	sortTS, chatID, err := decodeListChatCursor(cursor)
	if err != nil {
		return nil, err
	}

	cursorFilter := ""
	args := []any{viewerProfileID, spaceIDs, fetch}
	if cursor != "" {
		cursorFilter = `
  AND (
    sort_at < $4::timestamptz
    OR (sort_at = $4::timestamptz AND id < $5::uuid)
  )`
		args = append(args, sortTS, chatID)
	}

	query := `
WITH candidates AS (
  SELECT` + listChatsSelectColumns + `
  FROM chats c
  INNER JOIN chat_members m ON m.chat_id = c.id AND m.profile_id = $1
  WHERE c.type IN ('dm', 'group', 'channel') AND m.is_archived = false AND m.inbox_bucket = 'main'

  UNION ALL

  SELECT c.id, c.type, c.space_id, c.name, c.avatar_url, c.creator_profile_id, c.last_message_at, c.created_at, c.updated_at,
         c.slow_mode_seconds, c.threads_enabled, c.allow_user_main_feed, c.e2e_enabled, 'main'::text AS inbox_bucket,
         COALESCE(c.last_message_at, c.created_at) AS sort_at
  FROM chats c
  WHERE c.space_id = ANY($2) AND c.type IN ('group', 'channel')
    AND NOT EXISTS (
      SELECT 1 FROM chat_members m
      WHERE m.chat_id = c.id AND m.profile_id = $1 AND m.is_archived = true
    )
    AND NOT EXISTS (
      SELECT 1 FROM chat_members m
      WHERE m.chat_id = c.id AND m.profile_id = $1 AND m.is_archived = false AND m.inbox_bucket = 'main'
    )
),
deduped AS (
  SELECT DISTINCT ON (id) id, type, space_id, name, avatar_url, creator_profile_id, last_message_at, created_at, updated_at,
         slow_mode_seconds, threads_enabled, allow_user_main_feed, e2e_enabled, inbox_bucket, sort_at
  FROM candidates
  ORDER BY id, sort_at DESC
)
SELECT id, type, space_id, name, avatar_url, creator_profile_id, last_message_at, created_at, updated_at,
       slow_mode_seconds, threads_enabled, allow_user_main_feed, e2e_enabled, inbox_bucket, sort_at
FROM deduped
WHERE true` + cursorFilter + `
ORDER BY sort_at DESC, id DESC
LIMIT $3`

	rows, err := s.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out, err := scanListChatPageRows(rows)
	if err != nil {
		return nil, err
	}

	next := ""
	if len(out) > limit {
		last := out[limit-1]
		sk := last.CreatedAt
		if last.LastMessageAt != nil {
			sk = *last.LastMessageAt
		}
		next = encodeListChatCursor(sk, last.ID)
		out = out[:limit]
	}
	return &ListChatsPage{Rows: out, NextCursor: next}, nil
}

func scanListChatPageRows(rows pgx.Rows) ([]*ChatRow, error) {
	var out []*ChatRow
	for rows.Next() {
		var id, creator uuid.UUID
		var chatType string
		var spaceID *uuid.UUID
		var name, avatarURL sql.NullString
		var lastMsg sql.NullTime
		var createdAt, updatedAt time.Time
		var slowMode int32
		var threadsEnabled, allowMainFeed, e2eEnabled bool
		var inboxBucket string
		var sortAt time.Time
		if err := rows.Scan(&id, &chatType, &spaceID, &name, &avatarURL, &creator, &lastMsg, &createdAt, &updatedAt,
			&slowMode, &threadsEnabled, &allowMainFeed, &e2eEnabled, &inboxBucket, &sortAt); err != nil {
			return nil, err
		}
		var lm *time.Time
		if lastMsg.Valid {
			t := lastMsg.Time.UTC()
			lm = &t
		}
		row := &ChatRow{
			ID:                id,
			Type:              chatType,
			SpaceID:           spaceID,
			CreatorProfileID:  creator,
			CreatedAt:         createdAt.UTC(),
			UpdatedAt:         updatedAt.UTC(),
			LastMessageAt:     lm,
			InboxBucket:       inboxBucket,
			SlowModeSeconds:   slowMode,
			ThreadsEnabled:    threadsEnabled,
			AllowUserMainFeed: allowMainFeed,
			E2EEnabled:        e2eEnabled,
		}
		if name.Valid {
			n := name.String
			row.Name = &n
		}
		if avatarURL.Valid {
			a := avatarURL.String
			row.AvatarURL = &a
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// ListSpaceChatsForProfile returns space-bound group and channel chats visible via space membership.
// Chats archived by the viewer in chat_members are excluded.
func (s *DMStore) ListSpaceChatsForProfile(ctx context.Context, viewerProfileID uuid.UUID, spaceIDs []uuid.UUID) ([]*ChatRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("dm store: pool not configured")
	}
	if len(spaceIDs) == 0 {
		return nil, nil
	}
	rows, err := s.Pool.Query(ctx, `
SELECT c.id, c.type, c.space_id, c.name, c.avatar_url, c.creator_profile_id, c.last_message_at, c.created_at, c.updated_at,
       c.threads_enabled, c.allow_user_main_feed, c.e2e_enabled, c.slow_mode_seconds,
       COALESCE(c.last_message_at, c.created_at) AS sort_at
FROM chats c
WHERE c.space_id = ANY($1) AND c.type IN ('group', 'channel')
  AND NOT EXISTS (
    SELECT 1 FROM chat_members m
    WHERE m.chat_id = c.id AND m.profile_id = $2 AND m.is_archived = true
  )
ORDER BY sort_at DESC, c.id DESC
`, spaceIDs, viewerProfileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*ChatRow
	for rows.Next() {
		var id, creator uuid.UUID
		var chatType string
		var spaceID *uuid.UUID
		var name, avatarURL sql.NullString
		var lastMsg sql.NullTime
		var createdAt, updatedAt time.Time
		var threadsEnabled, allowMainFeed, e2eEnabled bool
		var slowMode int32
		var sortAt time.Time
		if err := rows.Scan(&id, &chatType, &spaceID, &name, &avatarURL, &creator, &lastMsg, &createdAt, &updatedAt,
			&threadsEnabled, &allowMainFeed, &e2eEnabled, &slowMode, &sortAt); err != nil {
			return nil, err
		}
		var lm *time.Time
		if lastMsg.Valid {
			t := lastMsg.Time.UTC()
			lm = &t
		}
		row := &ChatRow{
			ID:                id,
			Type:              chatType,
			SpaceID:           spaceID,
			CreatorProfileID:  creator,
			CreatedAt:         createdAt.UTC(),
			UpdatedAt:         updatedAt.UTC(),
			LastMessageAt:     lm,
			InboxBucket:       "main",
			ThreadsEnabled:    threadsEnabled,
			AllowUserMainFeed: allowMainFeed,
			E2EEnabled:        e2eEnabled,
			SlowModeSeconds:   slowMode,
		}
		if name.Valid {
			n := name.String
			row.Name = &n
		}
		if avatarURL.Valid {
			a := avatarURL.String
			row.AvatarURL = &a
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func MergeListChatRows(primary, extra []*ChatRow, limit int) []*ChatRow {
	seen := make(map[uuid.UUID]struct{}, len(primary)+len(extra))
	merged := make([]*ChatRow, 0, len(primary)+len(extra))
	add := func(row *ChatRow) {
		if row == nil {
			return
		}
		if _, ok := seen[row.ID]; ok {
			return
		}
		seen[row.ID] = struct{}{}
		merged = append(merged, row)
	}
	for _, row := range primary {
		add(row)
	}
	for _, row := range extra {
		add(row)
	}
	sort.Slice(merged, func(i, j int) bool {
		si := merged[i].CreatedAt
		if merged[i].LastMessageAt != nil {
			si = *merged[i].LastMessageAt
		}
		sj := merged[j].CreatedAt
		if merged[j].LastMessageAt != nil {
			sj = *merged[j].LastMessageAt
		}
		if si.Equal(sj) {
			return merged[i].ID.String() > merged[j].ID.String()
		}
		return si.After(sj)
	})
	if limit > 0 && len(merged) > limit {
		merged = merged[:limit]
	}
	return merged
}

// ListChatsPageCursorFromRows truncates rows to limit and returns the opaque cursor for the next page.
func ListChatsPageCursorFromRows(rows []*ChatRow, limit int) ([]*ChatRow, string) {
	if limit < 1 || len(rows) <= limit {
		return rows, ""
	}
	last := rows[limit-1]
	sk := last.CreatedAt
	if last.LastMessageAt != nil {
		sk = *last.LastMessageAt
	}
	return rows[:limit], encodeListChatCursor(sk, last.ID)
}
