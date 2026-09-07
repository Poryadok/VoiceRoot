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

const folderPinOrderNullSentinel = 1_000_000

type listFolderChatCursorPayload struct {
	P int    `json:"p"` // 0 pinned row, 1 unpinned
	O int    `json:"o"` // pin_order or sentinel
	S int    `json:"s"` // folder sort_order
	N int64  `json:"n"` // negated sort_at unix nano for DESC-as-ASC
	I string `json:"i"` // chat id
}

func encodeListFolderChatCursor(pinned bool, pinOrder *int32, folderSort int32, sortAt time.Time, chatID uuid.UUID) string {
	p := 1
	if pinned {
		p = 0
	}
	o := folderPinOrderNullSentinel
	if pinOrder != nil {
		o = int(*pinOrder)
	}
	payload := listFolderChatCursorPayload{
		P: p,
		O: o,
		S: int(folderSort),
		N: -sortAt.UTC().UnixNano(),
		I: chatID.String(),
	}
	b, _ := json.Marshal(payload)
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeListFolderChatCursor(raw string) (pinned bool, pinOrder int, folderSort int32, sortAt time.Time, chatID uuid.UUID, err error) {
	if raw == "" {
		return false, folderPinOrderNullSentinel, 0, time.Time{}, uuid.Nil, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return false, 0, 0, time.Time{}, uuid.Nil, ErrInvalidListCursor
	}
	var p listFolderChatCursorPayload
	if err := json.Unmarshal(b, &p); err != nil {
		return false, 0, 0, time.Time{}, uuid.Nil, ErrInvalidListCursor
	}
	id, err := uuid.Parse(p.I)
	if err != nil {
		return false, 0, 0, time.Time{}, uuid.Nil, ErrInvalidListCursor
	}
	pinned = p.P == 0
	return pinned, p.O, int32(p.S), time.Unix(0, -p.N).UTC(), id, nil
}

// ListChatsPageByFolder returns chats visible in a folder with pin/order overlay sorting.
func (s *DMStore) ListChatsPageByFolder(ctx context.Context, viewerProfileID, folderID uuid.UUID, cursor string, limit int, spaceIDs []uuid.UUID) (*ListChatsPage, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("dm store: pool not configured")
	}
	if limit < 1 {
		limit = 1
	}
	folder, err := s.GetFolder(ctx, viewerProfileID, folderID)
	if err != nil {
		return nil, err
	}
	if folder.FolderType == "custom" {
		return s.listCustomFolderChatsPage(ctx, viewerProfileID, folderID, cursor, limit)
	}
	cfg := parseFolderFilterConfig(folder.FilterConfigJSON)
	if folderSystemKeyNeedsSpaceMerge(cfg) && len(spaceIDs) > 0 {
		return s.listSystemFolderChatsPageWithSpaces(ctx, viewerProfileID, folderID, cfg, cursor, limit, spaceIDs)
	}
	return s.listSystemFolderChatsPage(ctx, viewerProfileID, folderID, cfg, cursor, limit)
}

func (s *DMStore) listCustomFolderChatsPage(ctx context.Context, profileID, folderID uuid.UUID, cursor string, limit int) (*ListChatsPage, error) {
	curPinned, curPinOrder, curSort, curTS, curChatID, err := decodeListFolderChatCursor(cursor)
	if err != nil {
		return nil, err
	}

	args := []any{profileID, folderID, limit + 1, folderPinOrderNullSentinel}
	cursorFilter := ""
	if cursor != "" {
		cursorFilter = `
  AND (
    (CASE WHEN fc.is_pinned THEN 0 ELSE 1 END,
     COALESCE(fc.pin_order, $4),
     fc.sort_order,
     -EXTRACT(EPOCH FROM sort_at) * 1000000000,
     c.id::text)
    > ($5, $6, $7, $8, $9::text)
  )`
		args = append(args, boolToPinRank(curPinned), curPinOrder, curSort, -curTS.UnixNano(), curChatID.String())
	}

	query := `
SELECT c.id, c.type, c.space_id, c.name, c.avatar_url, c.creator_profile_id, c.last_message_at, c.created_at, c.updated_at,
       c.slow_mode_seconds, c.threads_enabled, c.allow_user_main_feed, c.e2e_enabled, m.inbox_bucket,
       COALESCE(c.last_message_at, c.created_at) AS sort_at,
       fc.is_pinned, fc.pin_order, fc.sort_order AS folder_sort
FROM folder_chats fc
INNER JOIN chats c ON c.id = fc.chat_id
INNER JOIN chat_members m ON m.chat_id = c.id AND m.profile_id = $1
WHERE fc.profile_id = $1 AND fc.folder_id = $2
  AND m.is_archived = false AND m.deleted_for_self = false AND m.inbox_bucket = 'main'` + cursorFilter + `
ORDER BY
  CASE WHEN fc.is_pinned THEN 0 ELSE 1 END ASC,
  COALESCE(fc.pin_order, $4) ASC,
  fc.sort_order ASC,
  sort_at DESC,
  c.id DESC
LIMIT $3`

	rows, err := s.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanFolderListChatPageRows(rows, limit)
}

func (s *DMStore) listSystemFolderChatsPage(ctx context.Context, profileID, folderID uuid.UUID, cfg folderFilterConfig, cursor string, limit int) (*ListChatsPage, error) {
	types, requireSpace := systemFolderTypeFilter(cfg)
	fetch := limit + 1
	curPinned, curPinOrder, curSort, curTS, curChatID, err := decodeListFolderChatCursor(cursor)
	if err != nil {
		return nil, err
	}

	args := []any{profileID, folderID, types, fetch, folderPinOrderNullSentinel}
	cursorFilter := ""
	if cursor != "" {
		cursorFilter = `
  AND (
    (CASE WHEN COALESCE(fc.is_pinned, false) THEN 0 ELSE 1 END,
     COALESCE(fc.pin_order, $5),
     COALESCE(fc.sort_order, 0),
     -EXTRACT(EPOCH FROM sort_at) * 1000000000,
     c.id::text)
    > ($6, $7, $8, $9, $10::text)
  )`
		args = append(args, boolToPinRank(curPinned), curPinOrder, curSort, -curTS.UnixNano(), curChatID.String())
	}
	spaceFilter := ""
	if requireSpace {
		spaceFilter = " AND c.space_id IS NOT NULL"
	}

	query := `
SELECT c.id, c.type, c.space_id, c.name, c.avatar_url, c.creator_profile_id, c.last_message_at, c.created_at, c.updated_at,
       c.slow_mode_seconds, c.threads_enabled, c.allow_user_main_feed, c.e2e_enabled, m.inbox_bucket,
       COALESCE(c.last_message_at, c.created_at) AS sort_at,
       COALESCE(fc.is_pinned, false), fc.pin_order, COALESCE(fc.sort_order, 0) AS folder_sort
FROM chats c
INNER JOIN chat_members m ON m.chat_id = c.id AND m.profile_id = $1
LEFT JOIN folder_chats fc ON fc.profile_id = $1 AND fc.folder_id = $2 AND fc.chat_id = c.id
WHERE m.is_archived = false AND m.deleted_for_self = false AND m.inbox_bucket = 'main'
  AND c.type = ANY($3)` + spaceFilter + cursorFilter + `
ORDER BY
  CASE WHEN COALESCE(fc.is_pinned, false) THEN 0 ELSE 1 END ASC,
  COALESCE(fc.pin_order, $5) ASC,
  COALESCE(fc.sort_order, 0) ASC,
  sort_at DESC,
  c.id DESC
LIMIT $4`

	rows, err := s.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanFolderListChatPageRows(rows, limit)
}

func (s *DMStore) listSystemFolderChatsPageWithSpaces(ctx context.Context, profileID, folderID uuid.UUID, cfg folderFilterConfig, cursor string, limit int, spaceIDs []uuid.UUID) (*ListChatsPage, error) {
	types, requireSpace := systemFolderTypeFilter(cfg)
	fetch := limit + 1
	curPinned, curPinOrder, curSort, curTS, curChatID, err := decodeListFolderChatCursor(cursor)
	if err != nil {
		return nil, err
	}

	args := []any{profileID, folderID, types, spaceIDs, fetch, folderPinOrderNullSentinel}
	cursorFilter := ""
	if cursor != "" {
		cursorFilter = `
  AND (
    (pin_rank,
     COALESCE(pin_order, $6),
     folder_sort,
     -EXTRACT(EPOCH FROM sort_at) * 1000000000,
     id::text)
    > ($7, $8, $9, $10, $11::text)
  )`
		args = append(args, boolToPinRank(curPinned), curPinOrder, curSort, -curTS.UnixNano(), curChatID.String())
	}
	spaceFilterMembership := ""
	spaceFilterUnion := ""
	if requireSpace {
		spaceFilterMembership = " AND c.space_id IS NOT NULL"
		spaceFilterUnion = " AND c.space_id IS NOT NULL"
	}

	query := `
WITH candidates AS (
  SELECT c.id, c.type, c.space_id, c.name, c.avatar_url, c.creator_profile_id, c.last_message_at, c.created_at, c.updated_at,
         c.slow_mode_seconds, c.threads_enabled, c.allow_user_main_feed, c.e2e_enabled, m.inbox_bucket,
         COALESCE(c.last_message_at, c.created_at) AS sort_at,
         COALESCE(fc.is_pinned, false) AS is_pinned, fc.pin_order, COALESCE(fc.sort_order, 0) AS folder_sort
  FROM chats c
  INNER JOIN chat_members m ON m.chat_id = c.id AND m.profile_id = $1
  LEFT JOIN folder_chats fc ON fc.profile_id = $1 AND fc.folder_id = $2 AND fc.chat_id = c.id
  WHERE m.is_archived = false AND m.deleted_for_self = false AND m.inbox_bucket = 'main'
    AND c.type = ANY($3)` + spaceFilterMembership + `

  UNION ALL

  SELECT c.id, c.type, c.space_id, c.name, c.avatar_url, c.creator_profile_id, c.last_message_at, c.created_at, c.updated_at,
         c.slow_mode_seconds, c.threads_enabled, c.allow_user_main_feed, c.e2e_enabled, 'main'::text AS inbox_bucket,
         COALESCE(c.last_message_at, c.created_at) AS sort_at,
         COALESCE(fc.is_pinned, false), fc.pin_order, COALESCE(fc.sort_order, 0)
  FROM chats c
  LEFT JOIN folder_chats fc ON fc.profile_id = $1 AND fc.folder_id = $2 AND fc.chat_id = c.id
  WHERE c.space_id = ANY($4) AND c.type = ANY($3)` + spaceFilterUnion + `
    AND NOT EXISTS (
      SELECT 1 FROM chat_members m
      WHERE m.chat_id = c.id AND m.profile_id = $1 AND (m.is_archived = true OR m.deleted_for_self = true)
    )
    AND NOT EXISTS (
      SELECT 1 FROM chat_members m
      WHERE m.chat_id = c.id AND m.profile_id = $1 AND m.is_archived = false AND m.deleted_for_self = false AND m.inbox_bucket = 'main'
    )
),
deduped AS (
  SELECT DISTINCT ON (id) *
  FROM candidates
  ORDER BY id, sort_at DESC
),
ranked AS (
  SELECT *,
    CASE WHEN is_pinned THEN 0 ELSE 1 END AS pin_rank
  FROM deduped
)
SELECT id, type, space_id, name, avatar_url, creator_profile_id, last_message_at, created_at, updated_at,
       slow_mode_seconds, threads_enabled, allow_user_main_feed, e2e_enabled, inbox_bucket, sort_at,
       is_pinned, pin_order, folder_sort
FROM ranked
WHERE true` + cursorFilter + `
ORDER BY pin_rank ASC, COALESCE(pin_order, $6) ASC, folder_sort ASC, sort_at DESC, id DESC
LIMIT $5`

	rows, err := s.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanFolderListChatPageRows(rows, limit)
}

func boolToPinRank(pinned bool) int {
	if pinned {
		return 0
	}
	return 1
}

func scanFolderListChatPageRows(rows pgx.Rows, limit int) (*ListChatsPage, error) {
	var out []*ChatRow
	var pinStates []folderPinState
	for rows.Next() {
		row, pin, err := scanFolderListChatRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
		pinStates = append(pinStates, pin)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	next := ""
	if len(out) > limit {
		last := out[limit-1]
		pin := pinStates[limit-1]
		sk := last.CreatedAt
		if last.LastMessageAt != nil {
			sk = *last.LastMessageAt
		}
		var pinOrder *int32
		if pin.PinOrder != nil {
			pinOrder = pin.PinOrder
		}
		next = encodeListFolderChatCursor(pin.IsPinned, pinOrder, pin.FolderSort, sk, last.ID)
		out = out[:limit]
	}
	return &ListChatsPage{Rows: out, NextCursor: next}, nil
}

type folderPinState struct {
	IsPinned   bool
	PinOrder   *int32
	FolderSort int32
}

func scanFolderListChatRow(rows pgx.Rows) (*ChatRow, folderPinState, error) {
	var id, creator uuid.UUID
	var chatType string
	var spaceID *uuid.UUID
	var name, avatarURL sql.NullString
	var lastMsg sql.NullTime
	var createdAt, updatedAt time.Time
	var slowMode int32
	var threadsEnabled, allowMainFeed, e2eEnabled, isPinned bool
	var pinOrder sql.NullInt32
	var folderSort int32
	var inboxBucket string
	var sortAt time.Time
	if err := rows.Scan(&id, &chatType, &spaceID, &name, &avatarURL, &creator, &lastMsg, &createdAt, &updatedAt,
		&slowMode, &threadsEnabled, &allowMainFeed, &e2eEnabled, &inboxBucket, &sortAt,
		&isPinned, &pinOrder, &folderSort); err != nil {
		return nil, folderPinState{}, err
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
		IsPinned:          isPinned,
	}
	if name.Valid {
		n := name.String
		row.Name = &n
	}
	if avatarURL.Valid {
		a := avatarURL.String
		row.AvatarURL = &a
	}
	pin := folderPinState{IsPinned: isPinned, FolderSort: folderSort}
	if pinOrder.Valid {
		v := pinOrder.Int32
		pin.PinOrder = &v
	}
	return row, pin, nil
}
