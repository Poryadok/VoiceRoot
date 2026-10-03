package store

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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
	tx, err := beginGovernedTx(ctx, s.Pool)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := lifecycleGateChat(ctx, tx, doc.ChatID); err != nil {
		return err
	}
	if err := lockMessageProjection(ctx, tx, doc.MessageID); err != nil {
		return err
	}
	var purged bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM search_managed_chat_message_purge_fences WHERE message_id=$1)`, doc.MessageID).Scan(&purged); err != nil {
		return err
	} else if purged {
		return tx.Commit(ctx)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO message_search_documents (message_id, chat_id, sender_profile_id, body, created_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (message_id) DO UPDATE SET
			chat_id = EXCLUDED.chat_id,
			sender_profile_id = EXCLUDED.sender_profile_id,
			body = EXCLUDED.body,
			created_at = EXCLUDED.created_at`,
		doc.MessageID, doc.ChatID, doc.SenderProfileID, doc.Body, doc.CreatedAt,
	)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *MessageSearchStore) Delete(ctx context.Context, messageID uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return fmt.Errorf("message search store unavailable")
	}
	tx, err := beginGovernedTx(ctx, s.Pool)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var chatID uuid.UUID
	err = tx.QueryRow(ctx, `SELECT chat_id FROM message_search_documents WHERE message_id=$1`, messageID).Scan(&chatID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if err == nil {
		if err := lifecycleGateChat(ctx, tx, chatID); err != nil {
			return err
		}
	}
	_, err = tx.Exec(ctx, `DELETE FROM message_search_documents WHERE message_id = $1`, messageID)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

var ErrManagedChatSearchPurgeConflict = errors.New("managed chat Search purge operation conflicts with saved request")

type ManagedChatSearchPurgeReceipt struct {
	OperationID    uuid.UUID
	ChatID         uuid.UUID
	ReceiptID      uuid.UUID
	DeletedCount   uint64
	MessageIDsHash []byte
	RequestHash    []byte
	CompletedAt    time.Time
}

// PurgeManagedChatMessages installs per-message permanent tombstones and
// removes the current projection atomically. Late JetStream upserts acquire
// the same message lock and become inert after these tombstones commit.
func (s *MessageSearchStore) PurgeManagedChatMessages(ctx context.Context, operationID, chatID uuid.UUID, messageIDs []uuid.UUID, requestHash []byte) (*ManagedChatSearchPurgeReceipt, error) {
	if s == nil || s.Pool == nil || operationID == uuid.Nil || chatID == uuid.Nil || len(requestHash) != 32 {
		return nil, errors.New("managed chat Search purge identity or request hash is invalid")
	}
	ids := append([]uuid.UUID(nil), messageIDs...)
	for index, id := range ids {
		if id == uuid.Nil || (index > 0 && ids[index-1].String() >= id.String()) {
			return nil, errors.New("managed chat Search purge message IDs must be sorted and unique")
		}
	}
	messageIDsHash := ManagedChatMessageIDsHash(ids)
	tx, err := beginGovernedTx(ctx, s.Pool)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	// Serialize even an absent operation before locking its messages. Exact
	// concurrent retries then observe the first committed receipt instead of
	// conflicting with its newly installed message fences.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text, 721805))`, operationID.String()); err != nil {
		return nil, err
	}
	var savedChat, receiptID uuid.UUID
	var savedSetHash, savedRequestHash []byte
	var count int64
	var completedAt time.Time
	err = tx.QueryRow(ctx, `SELECT chat_id,receipt_id,message_ids_sha256,request_sha256,deleted_count,completed_at FROM search_managed_chat_purge_operations WHERE operation_id=$1 FOR UPDATE`, operationID).Scan(&savedChat, &receiptID, &savedSetHash, &savedRequestHash, &count, &completedAt)
	if err == nil {
		if savedChat != chatID || string(savedSetHash) != string(messageIDsHash) || string(savedRequestHash) != string(requestHash) {
			return nil, ErrManagedChatSearchPurgeConflict
		}
		// Exact committed evidence remains readable after the parent becomes
		// terminal; a replay performs no projection mutation.
		if frozen, err := lifecycleFrozenChat(ctx, tx, chatID); err != nil {
			return nil, err
		} else if frozen && !boundMessagingPurge(ctx, operationID, requestHash) {
			return nil, ErrManagedChatSearchPurgeConflict
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return &ManagedChatSearchPurgeReceipt{OperationID: operationID, ChatID: chatID, ReceiptID: receiptID, DeletedCount: uint64(count), MessageIDsHash: savedSetHash, RequestHash: savedRequestHash, CompletedAt: completedAt}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if err := authorizeManagedChatPurge(ctx, tx, operationID, chatID, requestHash); err != nil {
		return nil, err
	}
	for _, id := range ids {
		if err := lockMessageProjection(ctx, tx, id); err != nil {
			return nil, err
		}
	}
	for _, id := range ids {
		// A bad owner work set must not create a permanent global tombstone
		// for an indexed message belonging to another chat.
		var indexedChat uuid.UUID
		err := tx.QueryRow(ctx, `SELECT chat_id FROM message_search_documents WHERE message_id=$1`, id).Scan(&indexedChat)
		if err == nil && indexedChat != chatID {
			return nil, ErrManagedChatSearchPurgeConflict
		}
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		var fencedChat uuid.UUID
		err = tx.QueryRow(ctx, `SELECT chat_id FROM search_managed_chat_message_purge_fences WHERE message_id=$1`, id).Scan(&fencedChat)
		if err == nil {
			return nil, ErrManagedChatSearchPurgeConflict
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
	}
	receiptID = uuid.New()
	completedAt = time.Now().UTC()
	if _, err := tx.Exec(ctx, `INSERT INTO search_managed_chat_purge_operations(operation_id,chat_id,message_ids_sha256,request_sha256,deleted_count,receipt_id,completed_at) VALUES($1,$2,$3,$4,$5,$6,clock_timestamp())`, operationID, chatID, messageIDsHash, requestHash, len(ids), receiptID); err != nil {
		return nil, err
	}
	for _, id := range ids {
		if _, err := tx.Exec(ctx, `INSERT INTO search_managed_chat_message_purge_fences(message_id,chat_id,operation_id) VALUES($1,$2,$3)`, id, chatID, operationID); err != nil {
			return nil, err
		}
	}
	if len(ids) > 0 {
		if _, err := tx.Exec(ctx, `DELETE FROM message_search_documents WHERE chat_id=$1 AND message_id=ANY($2)`, chatID, ids); err != nil {
			return nil, err
		}
	}
	if err := tx.QueryRow(ctx, `SELECT completed_at FROM search_managed_chat_purge_operations WHERE operation_id=$1`, operationID).Scan(&completedAt); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &ManagedChatSearchPurgeReceipt{OperationID: operationID, ChatID: chatID, ReceiptID: receiptID, DeletedCount: uint64(len(ids)), MessageIDsHash: messageIDsHash, RequestHash: append([]byte(nil), requestHash...), CompletedAt: completedAt}, nil
}

func ManagedChatMessageIDsHash(ids []uuid.UUID) []byte {
	h := sha256.New()
	_, _ = h.Write([]byte("voice.search.managed_chat_message_ids.v1\x00"))
	for _, id := range ids {
		_, _ = h.Write(id[:])
	}
	return h.Sum(nil)
}

func lockMessageProjection(ctx context.Context, tx pgx.Tx, messageID uuid.UUID) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text, 721804))`, messageID.String())
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
	tx, err := beginGovernedTx(ctx, s.Pool)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if chatID != uuid.Nil {
		if err := lifecycleGateChat(ctx, tx, chatID); err != nil {
			return nil, "", err
		}
	}
	if len(chatFilter) > 0 {
		for _, id := range chatFilter {
			if err := lifecycleGateChat(ctx, tx, id); err != nil {
				return nil, "", err
			}
		}
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

	rows, err := tx.Query(ctx, sql, args...)
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
		hit.CreatedAt = created
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
	if err := tx.Commit(ctx); err != nil {
		return nil, "", err
	}
	return hits, next, nil
}
