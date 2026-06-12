package store

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const defaultPageSize = 20

// MessageSearchStore persists and queries message_search_documents.
type MessageSearchStore struct {
	Pool *pgxpool.Pool
}

func NewMessageSearchStore(pool *pgxpool.Pool) *MessageSearchStore {
	return &MessageSearchStore{Pool: pool}
}

func (s *MessageSearchStore) Upsert(ctx context.Context, doc MessageDocument) error {
	if s == nil || s.Pool == nil {
		return fmt.Errorf("message search store unavailable")
	}
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO message_search_documents (message_id, chat_id, sender_profile_id, body, created_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (message_id) DO UPDATE SET
			chat_id = EXCLUDED.chat_id,
			sender_profile_id = EXCLUDED.sender_profile_id,
			body = EXCLUDED.body,
			created_at = EXCLUDED.created_at`,
		doc.MessageID, doc.ChatID, doc.SenderProfileID, doc.Body, doc.CreatedAt,
	)
	return err
}

func (s *MessageSearchStore) Delete(ctx context.Context, messageID uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return fmt.Errorf("message search store unavailable")
	}
	_, err := s.Pool.Exec(ctx, `DELETE FROM message_search_documents WHERE message_id = $1`, messageID)
	return err
}

type messageCursor struct {
	CreatedAt time.Time `json:"t"`
	MessageID uuid.UUID `json:"m"`
}

func encodeMessageCursor(c messageCursor) (string, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func decodeMessageCursor(raw string) (*messageCursor, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid cursor")
	}
	var c messageCursor
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("invalid cursor")
	}
	if c.MessageID == uuid.Nil || c.CreatedAt.IsZero() {
		return nil, fmt.Errorf("invalid cursor")
	}
	return &c, nil
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return defaultPageSize
	}
	return limit
}

func (s *MessageSearchStore) SearchInChat(ctx context.Context, chatID uuid.UUID, query string, cursor *string, limit int) ([]MessageHit, string, error) {
	return s.searchMessages(ctx, chatID, query, cursor, normalizeLimit(limit), nil)
}

func (s *MessageSearchStore) SearchGlobalMessages(ctx context.Context, query string, cursor *string, limit int, chatIDs []uuid.UUID) ([]MessageHit, string, error) {
	return s.searchMessages(ctx, uuid.Nil, query, cursor, normalizeLimit(limit), chatIDs)
}

func (s *MessageSearchStore) searchMessages(ctx context.Context, chatID uuid.UUID, query string, cursorRaw *string, limit int, chatFilter []uuid.UUID) ([]MessageHit, string, error) {
	if s == nil || s.Pool == nil {
		return nil, "", fmt.Errorf("message search store unavailable")
	}
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, "", fmt.Errorf("query required")
	}

	var after *messageCursor
	if cursorRaw != nil && strings.TrimSpace(*cursorRaw) != "" {
		c, err := decodeMessageCursor(*cursorRaw)
		if err != nil {
			return nil, "", err
		}
		after = c
	}

	args := []any{q}
	where := `search_vector @@ plainto_tsquery('simple', $1)`
	if chatID != uuid.Nil {
		args = append(args, chatID)
		where += fmt.Sprintf(` AND chat_id = $%d`, len(args))
	} else if len(chatFilter) > 0 {
		args = append(args, chatFilter)
		where += fmt.Sprintf(` AND chat_id = ANY($%d)`, len(args))
	}
	if after != nil {
		args = append(args, after.CreatedAt, after.MessageID)
		where += fmt.Sprintf(` AND (created_at, message_id) < ($%d, $%d)`, len(args)-1, len(args))
	}
	args = append(args, limit+1)
	limitArg := len(args)

	sql := fmt.Sprintf(`
		SELECT message_id, chat_id,
			ts_headline('simple', body, plainto_tsquery('simple', $1),
				'HighlightAll=true, MaxWords=20, MinWords=3, StartSel=<b>, StopSel=</b>') AS snippet,
			ts_rank(search_vector, plainto_tsquery('simple', $1)) AS score,
			created_at
		FROM message_search_documents
		WHERE %s
		ORDER BY created_at DESC, message_id DESC
		LIMIT $%d`, where, limitArg)

	rows, err := s.Pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	hits := make([]MessageHit, 0, limit+1)
	createdAt := make([]time.Time, 0, limit+1)
	for rows.Next() {
		var hit MessageHit
		var created time.Time
		if err := rows.Scan(&hit.MessageID, &hit.ChatID, &hit.Snippet, &hit.Score, &created); err != nil {
			return nil, "", err
		}
		hits = append(hits, hit)
		createdAt = append(createdAt, created)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	var next string
	if len(hits) > limit {
		last := hits[limit-1]
		c, err := encodeMessageCursor(messageCursor{CreatedAt: createdAt[limit-1], MessageID: last.MessageID})
		if err != nil {
			return nil, "", err
		}
		next = c
		hits = hits[:limit]
	}
	return hits, next, nil
}
