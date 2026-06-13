package store

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"voice/backend/messaging/internal/markdown"
)

// SharedMediaKind matches proto SharedMediaKind buckets.
type SharedMediaKind int

const (
	SharedMediaKindMedia SharedMediaKind = iota + 1
	SharedMediaKindFiles
	SharedMediaKindLinks
	SharedMediaKindVoice
)

// SharedMediaRow is a raw attachment or link item before file enrichment.
type SharedMediaRow struct {
	MessageID       uuid.UUID
	SenderProfileID uuid.UUID
	CreatedAt       time.Time
	SortOrder       int32
	FileID          *uuid.UUID
	AttachmentType  string
	ExternalURL     string
	Title           string
}

// SharedMediaStore lists shared media items per chat.
type SharedMediaStore struct {
	Pool *pgxpool.Pool
}

func attachmentTypesForKind(kind SharedMediaKind) []string {
	switch kind {
	case SharedMediaKindMedia:
		return []string{"image", "video"}
	case SharedMediaKindFiles:
		return []string{"document", "other"}
	case SharedMediaKindVoice:
		return []string{"audio", "voice_message"}
	default:
		return nil
	}
}

// List returns up to limit items; cursor format: message_id:sort_order.
func (s *SharedMediaStore) List(
	ctx context.Context,
	chatID uuid.UUID,
	kind SharedMediaKind,
	cursor string,
	limit int32,
) ([]SharedMediaRow, string, bool, error) {
	if s == nil || s.Pool == nil {
		return nil, "", false, errors.New("shared media store not configured")
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if kind == SharedMediaKindLinks {
		return s.listLinks(ctx, chatID, cursor, limit)
	}
	return s.listAttachments(ctx, chatID, kind, cursor, limit)
}

func (s *SharedMediaStore) listAttachments(
	ctx context.Context,
	chatID uuid.UUID,
	kind SharedMediaKind,
	cursor string,
	limit int32,
) ([]SharedMediaRow, string, bool, error) {
	types := attachmentTypesForKind(kind)
	if len(types) == 0 {
		return nil, "", false, fmt.Errorf("unsupported attachment kind")
	}

	cursorMsgID, cursorSort, err := parseSharedMediaCursor(cursor)
	if err != nil {
		return nil, "", false, err
	}

	query := `
SELECT m.id, m.sender_profile_id, m.created_at,
       att.elem->>'file_id', att.elem->>'type', (att.ordinality - 1)::int
FROM messages m
CROSS JOIN LATERAL jsonb_array_elements(m.attachments) WITH ORDINALITY AS att(elem, ordinality)
WHERE m.chat_id = $1
  AND m.deleted_at IS NULL
  AND att.elem->>'type' = ANY($2)
`
	args := []any{chatID, types}
	argN := 3
	if cursorMsgID != uuid.Nil {
		query += fmt.Sprintf(`
  AND (
    m.id < $%d
    OR (m.id = $%d AND (att.ordinality - 1) > $%d)
  )`, argN, argN, argN+1)
		args = append(args, cursorMsgID, cursorSort)
		argN += 2
	}
	query += fmt.Sprintf(`
ORDER BY m.id DESC, att.ordinality ASC
LIMIT $%d`, argN)
	args = append(args, limit+1)

	rows, err := s.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, "", false, err
	}
	defer rows.Close()

	var out []SharedMediaRow
	for rows.Next() {
		var row SharedMediaRow
		var fileIDRaw *string
		var attType string
		if err := rows.Scan(
			&row.MessageID, &row.SenderProfileID, &row.CreatedAt,
			&fileIDRaw, &attType, &row.SortOrder,
		); err != nil {
			return nil, "", false, err
		}
		row.AttachmentType = strings.TrimSpace(attType)
		if fileIDRaw != nil && strings.TrimSpace(*fileIDRaw) != "" {
			fid, err := uuid.Parse(strings.TrimSpace(*fileIDRaw))
			if err != nil {
				continue
			}
			row.FileID = &fid
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, "", false, err
	}

	hasMore := len(out) > int(limit)
	if hasMore {
		out = out[:limit]
	}
	nextCursor := ""
	if hasMore && len(out) > 0 {
		last := out[len(out)-1]
		nextCursor = formatSharedMediaCursor(last.MessageID, last.SortOrder)
	}
	return out, nextCursor, hasMore, nil
}

func (s *SharedMediaStore) listLinks(
	ctx context.Context,
	chatID uuid.UUID,
	cursor string,
	limit int32,
) ([]SharedMediaRow, string, bool, error) {
	cursorMsgID, cursorSort, err := parseSharedMediaCursor(cursor)
	if err != nil {
		return nil, "", false, err
	}

	const batchSize = 100
	var out []SharedMediaRow
	var lastScannedID *uuid.UUID

	for len(out) < int(limit)+1 {
		query := `
SELECT id, sender_profile_id, created_at, content
FROM messages
WHERE chat_id = $1
  AND deleted_at IS NULL
  AND content ~ 'https?://'
`
		args := []any{chatID}
		if lastScannedID != nil {
			query += ` AND id < $2`
			args = append(args, *lastScannedID)
		}
		query += ` ORDER BY id DESC LIMIT $` + strconv.Itoa(len(args)+1)
		args = append(args, batchSize)

		rows, err := s.Pool.Query(ctx, query, args...)
		if err != nil {
			return nil, "", false, err
		}

		var batchIDs []uuid.UUID
		type msgRow struct {
			id        uuid.UUID
			sender    uuid.UUID
			createdAt time.Time
			content   string
		}
		var batch []msgRow
		for rows.Next() {
			var m msgRow
			if err := rows.Scan(&m.id, &m.sender, &m.createdAt, &m.content); err != nil {
				rows.Close()
				return nil, "", false, err
			}
			batch = append(batch, m)
			batchIDs = append(batchIDs, m.id)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, "", false, err
		}
		if len(batch) == 0 {
			break
		}

		for _, msg := range batch {
			urls := markdown.ExtractURLs(msg.content)
			for i, u := range urls {
				sortOrder := int32(i)
				if !linkItemAfterCursor(msg.id, sortOrder, cursorMsgID, cursorSort) {
					continue
				}
				out = append(out, SharedMediaRow{
					MessageID:       msg.id,
					SenderProfileID: msg.sender,
					CreatedAt:       msg.createdAt,
					SortOrder:       sortOrder,
					ExternalURL:     u.URL,
					Title:           u.Title,
				})
				if len(out) >= int(limit)+1 {
					break
				}
			}
			if len(out) >= int(limit)+1 {
				break
			}
		}

		if len(out) >= int(limit)+1 {
			break
		}
		lastID := batch[len(batch)-1].id
		lastScannedID = &lastID
		if len(batch) < batchSize {
			break
		}
	}

	hasMore := len(out) > int(limit)
	if hasMore {
		out = out[:limit]
	}
	nextCursor := ""
	if hasMore && len(out) > 0 {
		last := out[len(out)-1]
		nextCursor = formatSharedMediaCursor(last.MessageID, last.SortOrder)
	}
	return out, nextCursor, hasMore, nil
}

// linkItemAfterCursor reports whether (msgID, sort) comes after cursor in DESC list order.
func linkItemAfterCursor(msgID uuid.UUID, sort int32, cursorMsgID uuid.UUID, cursorSort int32) bool {
	if cursorMsgID == uuid.Nil {
		return true
	}
	if bytes.Compare(msgID[:], cursorMsgID[:]) < 0 {
		return true
	}
	if msgID == cursorMsgID && sort > cursorSort {
		return true
	}
	return false
}

func parseSharedMediaCursor(raw string) (uuid.UUID, int32, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return uuid.Nil, 0, nil
	}
	parts := strings.SplitN(raw, ":", 2)
	if len(parts) != 2 {
		return uuid.Nil, 0, fmt.Errorf("invalid shared media cursor")
	}
	msgID, err := uuid.Parse(strings.TrimSpace(parts[0]))
	if err != nil {
		return uuid.Nil, 0, fmt.Errorf("invalid shared media cursor")
	}
	sortOrder, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 32)
	if err != nil {
		return uuid.Nil, 0, fmt.Errorf("invalid shared media cursor")
	}
	return msgID, int32(sortOrder), nil
}

func formatSharedMediaCursor(msgID uuid.UUID, sortOrder int32) string {
	return msgID.String() + ":" + strconv.FormatInt(int64(sortOrder), 10)
}

// InsertMessageAttachments is a test helper to seed attachment JSON without file service.
func InsertMessageAttachments(
	ctx context.Context,
	pool *pgxpool.Pool,
	msgID, chatID, sender uuid.UUID,
	attachments []map[string]string,
	content string,
) error {
	if content == "" {
		content = " "
	}
	b, err := json.Marshal(attachments)
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `
INSERT INTO messages (id, chat_id, chat_type, sender_profile_id, content, attachments, mentions)
VALUES ($1, $2, 'dm', $3, $4, $5::jsonb, '[]'::jsonb)
`, msgID, chatID, sender, content, string(b))
	return err
}
