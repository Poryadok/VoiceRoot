package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/cyberphone/json-canonicalization/go/src/webpki.org/jsoncanonicalizer"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"voice/backend/messaging/internal/markdown"
	"voice/backend/messaging/internal/messageevents"
)

// MessageRow is a persisted messaging_db.messages row (v1 DM).
type MessageRow struct {
	ID                     uuid.UUID
	ChatID                 uuid.UUID
	ChatType               string
	SenderProfileID        uuid.UUID
	PostedAsChat           bool
	DisplayChatID          *uuid.UUID
	Content                string
	Type                   string
	ThreadParentID         *uuid.UUID
	ForwardFromID          *uuid.UUID
	ForwardFromSender      string
	AttachmentsJSON        string
	MentionsJSON           string
	ClientMessageID        *uuid.UUID
	EditedAt               *time.Time
	DeletedAt              *time.Time
	GhostOnly              bool
	IsE2E                  bool
	ContentType            string // text | photo | …; empty → infer from attachments on read
	SendSilent             bool
	GameCardJSON           *string
	GameCardSHA256         string
	GameAppID              *uuid.UUID
	GameEnvironmentID      *uuid.UUID
	GameInstallationID     *uuid.UUID
	GameBotID              *uuid.UUID
	GameCharacterBindingID *uuid.UUID
	GameCardActionsEnabled bool
	GameActionResults      []GameActionResult
	CreatedAt              time.Time
}

type MessagesStore struct {
	Pool *pgxpool.Pool
}

var ErrGameEventMessageConflict = errors.New("game event message idempotency conflict")

// InsertGameEventMessage commits a game event message only while its expiry is
// live. A retry with the same key returns the exact existing message; a changed
// normalized payload under that key is a conflict.
func (s *MessagesStore) InsertGameEventMessage(ctx context.Context, row MessageRow, expiresAt *time.Time) (*MessageRow, bool, bool, error) {
	return s.insertGameEventMessage(ctx, row, expiresAt, nil, nil)
}

func (s *MessagesStore) InsertGameEventMessageWithOutbox(ctx context.Context, row MessageRow, expiresAt *time.Time, spaceID *uuid.UUID, event messageevents.OutboxEvent) (*MessageRow, bool, bool, error) {
	return s.insertGameEventMessage(ctx, row, expiresAt, spaceID, &event)
}

func (s *MessagesStore) insertGameEventMessage(ctx context.Context, row MessageRow, expiresAt *time.Time, spaceID *uuid.UUID, outboxEvent *messageevents.OutboxEvent) (*MessageRow, bool, bool, error) {
	if s == nil || s.Pool == nil || row.ChatID == uuid.Nil || row.SenderProfileID == uuid.Nil ||
		row.ClientMessageID == nil || *row.ClientMessageID == uuid.Nil || row.ID == uuid.Nil {
		return nil, false, false, errors.New("game event message has invalid identity")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, false, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockMessageMutation(ctx, tx, row.ChatID, spaceID, &row.ID); err != nil {
		return nil, false, false, err
	}
	saved, err := scanMessageRow(tx.QueryRow(ctx, messageSelectSQL+`
FROM messages WHERE chat_id=$1 AND sender_profile_id=$2 AND client_message_id=$3 FOR UPDATE`,
		row.ChatID, row.SenderProfileID, *row.ClientMessageID))
	if err == nil {
		if err := loadGameCardTx(ctx, tx, saved); err != nil {
			return nil, false, false, err
		}
		if !sameGameEventMessage(*saved, row) {
			return nil, false, false, ErrGameEventMessageConflict
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, false, false, err
		}
		return saved, false, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, false, false, err
	}
	if expiresAt != nil {
		var expired bool
		if err := tx.QueryRow(ctx, `SELECT $1::timestamptz<=clock_timestamp()`, *expiresAt).Scan(&expired); err != nil {
			return nil, false, false, err
		}
		if expired {
			return nil, true, false, nil
		}
	}
	var clientAny any = *row.ClientMessageID
	command, err := tx.Exec(ctx, `INSERT INTO messages (
		id,chat_id,chat_type,sender_profile_id,posted_as_chat,display_chat_id,content,type,thread_parent_id,
		forward_from_id,forward_from_sender,attachments,mentions,client_message_id,ghost_only,is_e2e,content_type,send_silent
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,NULL,NULL,$10::jsonb,$11::jsonb,$12,$13,$14,$15,$16)
	ON CONFLICT (chat_id,sender_profile_id,client_message_id) WHERE client_message_id IS NOT NULL DO NOTHING`,
		row.ID, row.ChatID, row.ChatType, row.SenderProfileID, row.PostedAsChat, row.DisplayChatID, row.Content,
		row.Type, row.ThreadParentID, row.AttachmentsJSON, row.MentionsJSON, clientAny, row.GhostOnly, row.IsE2E,
		row.ContentType, row.SendSilent)
	if err != nil {
		return nil, false, false, err
	}
	if command.RowsAffected() == 0 {
		saved, err = scanMessageRow(tx.QueryRow(ctx, messageSelectSQL+`
FROM messages WHERE chat_id=$1 AND sender_profile_id=$2 AND client_message_id=$3 FOR UPDATE`,
			row.ChatID, row.SenderProfileID, *row.ClientMessageID))
		if err != nil {
			return nil, false, false, err
		}
		if err := loadGameCardTx(ctx, tx, saved); err != nil {
			return nil, false, false, err
		}
		if !sameGameEventMessage(*saved, row) {
			return nil, false, false, ErrGameEventMessageConflict
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, false, false, err
		}
		return saved, false, false, nil
	}
	saved, err = scanMessageRow(tx.QueryRow(ctx, messageSelectSQL+` FROM messages WHERE id=$1 FOR UPDATE`, row.ID))
	if err != nil {
		return nil, false, false, err
	}
	if expiresAt != nil {
		var expired bool
		if err := tx.QueryRow(ctx, `SELECT $1::timestamptz<=clock_timestamp()`, *expiresAt).Scan(&expired); err != nil {
			return nil, false, false, err
		}
		if expired {
			return nil, true, false, nil
		}
	}
	if row.GameCardJSON != nil {
		if err := insertGameCardTx(ctx, tx, row); err != nil {
			return nil, false, false, err
		}
	}
	if outboxEvent != nil {
		if outboxEvent.MessageID != saved.ID || outboxEvent.ChatID != saved.ChatID {
			return nil, false, false, errors.New("game event outbox does not match inserted message")
		}
		if err := enqueueMessageEvent(ctx, tx, *outboxEvent); err != nil {
			return nil, false, false, err
		}
	}
	if err := loadGameCardTx(ctx, tx, saved); err != nil {
		return nil, false, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, false, false, err
	}
	return saved, false, true, nil
}

func sameGameEventMessage(saved, requested MessageRow) bool {
	return saved.ChatID == requested.ChatID && saved.SenderProfileID == requested.SenderProfileID &&
		saved.Content == requested.Content && saved.Type == requested.Type && saved.ChatType == requested.ChatType &&
		saved.PostedAsChat == requested.PostedAsChat && sameUUIDPointer(saved.ThreadParentID, requested.ThreadParentID) &&
		saved.AttachmentsJSON == requested.AttachmentsJSON && saved.MentionsJSON == requested.MentionsJSON &&
		saved.ContentType == requested.ContentType && saved.SendSilent == requested.SendSilent && saved.IsE2E == requested.IsE2E &&
		sameStringPointer(saved.GameCardJSON, requested.GameCardJSON) && sameUUIDPointer(saved.GameAppID, requested.GameAppID) &&
		sameUUIDPointer(saved.GameEnvironmentID, requested.GameEnvironmentID) && sameUUIDPointer(saved.GameInstallationID, requested.GameInstallationID) &&
		sameUUIDPointer(saved.GameBotID, requested.GameBotID) && sameUUIDPointer(saved.GameCharacterBindingID, requested.GameCharacterBindingID) &&
		saved.GameCardActionsEnabled == requested.GameCardActionsEnabled
}

func sameStringPointer(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	canonicalA, errA := jsoncanonicalizer.Transform([]byte(*a))
	canonicalB, errB := jsoncanonicalizer.Transform([]byte(*b))
	return errA == nil && errB == nil && string(canonicalA) == string(canonicalB)
}

func insertGameCardTx(ctx context.Context, tx pgx.Tx, row MessageRow) error {
	if row.GameCardJSON == nil || row.GameAppID == nil || row.GameEnvironmentID == nil || row.GameInstallationID == nil || row.GameBotID == nil || row.GameCardActionsEnabled {
		return errors.New("game card authority metadata is incomplete")
	}
	canonical, err := jsoncanonicalizer.Transform([]byte(*row.GameCardJSON))
	if err != nil {
		return errors.New("game card payload is not canonicalizable JSON")
	}
	digest := sha256.Sum256(canonical)
	cardHash := hex.EncodeToString(digest[:])
	_, err = tx.Exec(ctx, `INSERT INTO message_game_cards (
		message_id,app_id,environment_id,installation_id,bot_id,character_binding_id,card_json,card_sha256,actions_enabled
	) VALUES ($1,$2,$3,$4,$5,$6,$7::jsonb,$8,false)`, row.ID, *row.GameAppID, *row.GameEnvironmentID,
		*row.GameInstallationID, *row.GameBotID, row.GameCharacterBindingID, *row.GameCardJSON, cardHash)
	return err
}

func loadGameCardTx(ctx context.Context, tx pgx.Tx, row *MessageRow) error {
	if row == nil {
		return nil
	}
	var appID, envID, installationID, botID uuid.UUID
	var characterID *uuid.UUID
	var raw string
	var actionsEnabled bool
	err := tx.QueryRow(ctx, `SELECT app_id,environment_id,installation_id,bot_id,character_binding_id,card_json::text,card_sha256,actions_enabled
		FROM message_game_cards WHERE message_id=$1`, row.ID).Scan(&appID, &envID, &installationID, &botID, &characterID, &raw, &row.GameCardSHA256, &actionsEnabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	row.GameCardJSON = &raw
	row.GameAppID, row.GameEnvironmentID, row.GameInstallationID, row.GameBotID = &appID, &envID, &installationID, &botID
	row.GameCharacterBindingID, row.GameCardActionsEnabled = characterID, actionsEnabled
	return nil
}

func hydrateGameCards(ctx context.Context, db interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}, messages []MessageRow) error {
	if len(messages) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, 0, len(messages))
	index := make(map[uuid.UUID]int, len(messages))
	for i := range messages {
		ids = append(ids, messages[i].ID)
		index[messages[i].ID] = i
	}
	rows, err := db.Query(ctx, `SELECT message_id,app_id,environment_id,installation_id,bot_id,character_binding_id,card_json::text,card_sha256,actions_enabled
		FROM message_game_cards WHERE message_id=ANY($1)`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id, appID, envID, installationID, botID uuid.UUID
		var characterID *uuid.UUID
		var raw string
		var actionsEnabled bool
		var cardHash string
		if err := rows.Scan(&id, &appID, &envID, &installationID, &botID, &characterID, &raw, &cardHash, &actionsEnabled); err != nil {
			return err
		}
		message := &messages[index[id]]
		message.GameCardJSON = &raw
		message.GameCardSHA256 = cardHash
		message.GameAppID, message.GameEnvironmentID, message.GameInstallationID, message.GameBotID = &appID, &envID, &installationID, &botID
		message.GameCharacterBindingID, message.GameCardActionsEnabled = characterID, actionsEnabled
	}
	if err := rows.Err(); err != nil {
		return err
	}
	results, err := db.Query(ctx, `SELECT message_id,operation_id,action_id,result_id,state_version,status,safe_summary,recorded_at
		FROM message_game_action_results WHERE message_id=ANY($1) ORDER BY recorded_at,action_id`, ids)
	if err != nil {
		return err
	}
	defer results.Close()
	for results.Next() {
		var id uuid.UUID
		var item GameActionResult
		if err := results.Scan(&id, &item.OperationID, &item.ActionID, &item.ResultID, &item.StateVersion, &item.Status, &item.SafeSummary, &item.RecordedAt); err != nil {
			return err
		}
		if index, ok := index[id]; ok {
			messages[index].GameActionResults = append(messages[index].GameActionResults, item)
		}
	}
	return results.Err()
}

func sameUUIDPointer(a, b *uuid.UUID) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

type ChatListMetadataRow struct {
	ChatID                   uuid.UUID
	LastMessagePreview       string
	LastMessageAt            *time.Time
	UnreadCount              int64
	LastMessageIsOutgoing    bool
	LastMessageDeliveryState string // none | sent | delivered | read
	LastMessageContentType   string // text | photo | ...
}

var ErrVisibilityCandidateBudgetExceeded = errors.New("messages store: visibility candidate budget exceeded")

func (s *MessagesStore) UnreadMessageSenders(ctx context.Context, viewerProfileID uuid.UUID, chatIDs []uuid.UUID, candidateLimit int) ([]uuid.UUID, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("messages store: pool not configured")
	}
	out := make([]uuid.UUID, 0)
	if len(chatIDs) == 0 {
		return out, nil
	}
	if candidateLimit <= 0 {
		return nil, ErrVisibilityCandidateBudgetExceeded
	}
	rows, err := s.Pool.Query(ctx, `
SELECT m.sender_profile_id
FROM messages m
LEFT JOIN read_positions rr ON rr.chat_id = m.chat_id AND rr.profile_id = $1
WHERE m.chat_id = ANY($2::uuid[])
  AND m.deleted_at IS NULL
  AND m.sender_profile_id <> $1
  AND (NOT COALESCE(m.ghost_only, false) OR m.sender_profile_id = $1)
  AND NOT EXISTS (SELECT 1 FROM message_hides h WHERE h.message_id = m.id AND h.profile_id = $1)
  AND (rr.last_read_message_id IS NULL OR m.id > rr.last_read_message_id)
ORDER BY m.id DESC
LIMIT $3
`, viewerProfileID, chatIDs, candidateLimit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	seen := make(map[uuid.UUID]struct{})
	candidateRows := 0
	for rows.Next() {
		if candidateRows >= candidateLimit {
			return nil, ErrVisibilityCandidateBudgetExceeded
		}
		candidateRows++
		var senderID uuid.UUID
		if err := rows.Scan(&senderID); err != nil {
			return nil, err
		}
		if _, exists := seen[senderID]; !exists {
			seen[senderID] = struct{}{}
			out = append(out, senderID)
		}
	}
	return out, rows.Err()
}

type LatestPreviewMessageCandidate struct {
	MessageID       uuid.UUID
	SenderProfileID uuid.UUID
}

func (s *MessagesStore) LatestPreviewMessageCandidates(ctx context.Context, chatID, viewerProfileID uuid.UUID, blockedSenderIDs []uuid.UUID, limit int) ([]LatestPreviewMessageCandidate, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("messages store: pool not configured")
	}
	if limit <= 0 {
		limit = 1
	}
	rows, err := s.Pool.Query(ctx, `
SELECT id, sender_profile_id
FROM messages
WHERE chat_id = $1
  AND deleted_at IS NULL
  AND (NOT COALESCE(ghost_only, false) OR sender_profile_id = $2)
  AND NOT EXISTS (SELECT 1 FROM message_hides h WHERE h.message_id = messages.id AND h.profile_id = $2)
  AND NOT (sender_profile_id = ANY($3::uuid[]))
ORDER BY id DESC
LIMIT $4
`, chatID, viewerProfileID, blockedSenderIDs, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]LatestPreviewMessageCandidate, 0, limit)
	for rows.Next() {
		var candidate LatestPreviewMessageCandidate
		if err := rows.Scan(&candidate.MessageID, &candidate.SenderProfileID); err != nil {
			return nil, err
		}
		out = append(out, candidate)
	}
	return out, rows.Err()
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
	saved, err := insertMessageDB(ctx, s.Pool, row)
	if err != nil {
		return nil, err
	}
	return s.GetMessageByID(ctx, saved.ID)
}

type messageInsertDB interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

func insertMessageDB(ctx context.Context, db messageInsertDB, row MessageRow) (*MessageRow, error) {
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
	contentType := strings.TrimSpace(row.ContentType)
	var contentTypeAny any
	if contentType != "" {
		contentTypeAny = contentType
	}

	q := `
INSERT INTO messages (
  id, chat_id, chat_type, sender_profile_id, posted_as_chat, display_chat_id,
  content, type, thread_parent_id, forward_from_id, forward_from_sender,
  attachments, mentions, client_message_id, ghost_only, is_e2e, content_type, send_silent
) VALUES (
  $1, $2, $3, $4, $5, $6,
  $7, $8, $9, $10, $11, $12::jsonb, $13::jsonb, $14, $15, $16, $17, $18
)
ON CONFLICT (chat_id, sender_profile_id, client_message_id)
  WHERE client_message_id IS NOT NULL
  DO NOTHING
`
	ct, err := db.Exec(ctx, q,
		row.ID, row.ChatID, chatType, row.SenderProfileID, row.PostedAsChat, displayAny,
		row.Content, row.Type, threadAny, forwardFromAny, forwardSenderAny,
		row.AttachmentsJSON, row.MentionsJSON, clientAny, row.GhostOnly, row.IsE2E, contentTypeAny, row.SendSilent,
	)
	if err != nil {
		return nil, err
	}
	if ct.RowsAffected() == 0 {
		if row.ClientMessageID == nil {
			return nil, errors.New("messages store: insert produced no row")
		}
		return scanMessageRow(db.QueryRow(ctx, messageSelectSQL+`
FROM messages
WHERE chat_id = $1 AND sender_profile_id = $2 AND client_message_id = $3
LIMIT 1
`, row.ChatID, row.SenderProfileID, *row.ClientMessageID))
	}
	return scanMessageRow(db.QueryRow(ctx, messageSelectSQL+` FROM messages WHERE id=$1`, row.ID))
}

const messageSelectSQL = `
SELECT id, chat_id, chat_type, sender_profile_id, posted_as_chat, display_chat_id,
       content, type, thread_parent_id,
       forward_from_id, forward_from_sender,
       attachments::text, mentions::text, client_message_id, edited_at, deleted_at, created_at, is_e2e,
       COALESCE(content_type, ''), send_silent
`

const messageReturningCols = `id, chat_id, chat_type, sender_profile_id, posted_as_chat, display_chat_id,
       content, type, thread_parent_id,
       forward_from_id, forward_from_sender,
       attachments::text, mentions::text, client_message_id, edited_at, deleted_at, created_at, is_e2e,
       COALESCE(content_type, ''), send_silent`

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
		&m.AttachmentsJSON, &m.MentionsJSON, &clientID, &m.EditedAt, &m.DeletedAt, &m.CreatedAt, &m.IsE2E,
		&m.ContentType, &m.SendSilent,
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

func (s *MessagesStore) IsHiddenForProfile(ctx context.Context, messageID, profileID uuid.UUID) (bool, error) {
	if s == nil || s.Pool == nil {
		return false, errors.New("messages store: pool not configured")
	}
	var hidden bool
	err := s.Pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM message_hides WHERE message_id = $1 AND profile_id = $2)`, messageID, profileID).Scan(&hidden)
	return hidden, err
}

func (s *MessagesStore) GetMessageByID(ctx context.Context, id uuid.UUID) (*MessageRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("messages store: pool not configured")
	}
	row, err := scanMessageRow(s.Pool.QueryRow(ctx, messageSelectSQL+`
FROM messages WHERE id = $1
`, id))
	if err != nil {
		return nil, err
	}
	single := []MessageRow{*row}
	if err := hydrateGameCards(ctx, s.Pool, single); err != nil {
		return nil, err
	}
	*row = single[0]
	return row, nil
}

// UpdateMessageContent sets content and edited_at for a non-deleted row owned by senderProfileID.
func (s *MessagesStore) UpdateMessageContent(ctx context.Context, messageID, senderProfileID uuid.UUID, content string) (*MessageRow, error) {
	return s.UpdateMessageContentAndMentions(ctx, messageID, senderProfileID, content, nil)
}

// UpdateMessageContentAndMentions sets content, optional mentions, and edited_at.
func (s *MessagesStore) UpdateMessageContentAndMentions(ctx context.Context, messageID, senderProfileID uuid.UUID, content string, mentionsJSON *string) (*MessageRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("messages store: pool not configured")
	}
	if mentionsJSON == nil {
		return scanMessageRow(s.Pool.QueryRow(ctx, `
UPDATE messages
SET content = $1, edited_at = now()
WHERE id = $2 AND sender_profile_id = $3 AND deleted_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM message_game_cards gc WHERE gc.message_id = messages.id)
RETURNING `+messageReturningCols+`
`, content, messageID, senderProfileID))
	}
	return scanMessageRow(s.Pool.QueryRow(ctx, `
UPDATE messages
SET content = $1, mentions = $2::jsonb, edited_at = now()
WHERE id = $3 AND sender_profile_id = $4 AND deleted_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM message_game_cards gc WHERE gc.message_id = messages.id)
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

// ThreadSummaryRow is a thread listing entry for a text channel.
type ThreadSummaryRow struct {
	ThreadParentID   uuid.UUID
	ReplyCount       int32
	LastReplyAt      time.Time
	LastReplyPreview string
}

// ListThreads returns thread roots in a chat ordered by latest reply.
func (s *MessagesStore) ListThreads(ctx context.Context, chatID uuid.UUID, limit int) ([]ThreadSummaryRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("messages store: pool not configured")
	}
	if limit < 1 {
		limit = 20
	}
	rows, err := s.Pool.Query(ctx, `
SELECT m.thread_parent_id,
       COUNT(*)::int AS reply_count,
       MAX(m.created_at) AS last_reply_at,
       (SELECT LEFT(content, 120) FROM messages lm
        WHERE lm.chat_id = $1 AND lm.thread_parent_id = m.thread_parent_id AND lm.deleted_at IS NULL
        ORDER BY lm.created_at DESC LIMIT 1) AS last_preview
FROM messages m
WHERE m.chat_id = $1 AND m.thread_parent_id IS NOT NULL AND m.deleted_at IS NULL
GROUP BY m.thread_parent_id
ORDER BY last_reply_at DESC
LIMIT $2
`, chatID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ThreadSummaryRow
	for rows.Next() {
		var row ThreadSummaryRow
		var preview *string
		if err := rows.Scan(&row.ThreadParentID, &row.ReplyCount, &row.LastReplyAt, &preview); err != nil {
			return nil, err
		}
		if preview != nil {
			row.LastReplyPreview = *preview
		}
		out = append(out, row)
	}
	return out, rows.Err()
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
  AND (NOT COALESCE(ghost_only, false) OR sender_profile_id = $`+itoa(argN+1)+`)
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
  AND (NOT COALESCE(ghost_only, false) OR sender_profile_id = $`+itoa(argN+2)+`)
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
  AND (NOT COALESCE(ghost_only, false) OR sender_profile_id = $`+itoa(argN+2)+`)
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
			&m.AttachmentsJSON, &m.MentionsJSON, &clientID, &m.EditedAt, &m.DeletedAt, &m.CreatedAt, &m.IsE2E,
			&m.ContentType, &m.SendSilent,
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
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := hydrateGameCards(ctx, s.Pool, out); err != nil {
		return nil, err
	}
	return out, nil
}

func itoa(n int) string {
	return strconv.Itoa(n)
}

func (s *MessagesStore) UpsertReadReceipt(ctx context.Context, chatID, profileID, lastReadMessageID uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return errors.New("messages store: pool not configured")
	}
	_, err := s.Pool.Exec(ctx, `
INSERT INTO read_receipts (chat_id, profile_id, last_read_message_id, last_delivered_message_id, updated_at)
VALUES ($1, $2, $3, $3, now())
ON CONFLICT (chat_id, profile_id) DO UPDATE SET
  last_read_message_id = CASE
    WHEN read_receipts.last_read_message_id IS NULL OR read_receipts.last_read_message_id < EXCLUDED.last_read_message_id THEN EXCLUDED.last_read_message_id
    ELSE read_receipts.last_read_message_id
  END,
  last_delivered_message_id = CASE
    WHEN read_receipts.last_delivered_message_id IS NULL OR read_receipts.last_delivered_message_id < EXCLUDED.last_read_message_id THEN EXCLUDED.last_read_message_id
    ELSE read_receipts.last_delivered_message_id
  END,
  updated_at = CASE
    WHEN read_receipts.last_read_message_id IS NULL OR read_receipts.last_read_message_id < EXCLUDED.last_read_message_id THEN now()
    ELSE read_receipts.updated_at
  END
`, chatID, profileID, lastReadMessageID)
	return err
}

// UpsertReadPosition records the reader's private progress. Unlike a public
// read receipt it is always updated, because unread badges must still clear
// when the reader opts out of DM receipt visibility.
func (s *MessagesStore) UpsertReadPosition(ctx context.Context, chatID, profileID, lastReadMessageID uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return errors.New("messages store: pool not configured")
	}
	_, err := s.Pool.Exec(ctx, `
INSERT INTO read_positions (chat_id, profile_id, last_read_message_id, updated_at)
VALUES ($1, $2, $3, now())
ON CONFLICT (chat_id, profile_id) DO UPDATE SET
  last_read_message_id = CASE
    WHEN read_positions.last_read_message_id < EXCLUDED.last_read_message_id THEN EXCLUDED.last_read_message_id
    ELSE read_positions.last_read_message_id
  END,
  updated_at = CASE
    WHEN read_positions.last_read_message_id < EXCLUDED.last_read_message_id THEN now()
    ELSE read_positions.updated_at
  END
`, chatID, profileID, lastReadMessageID)
	return err
}

func (s *MessagesStore) GetReadReceipt(ctx context.Context, chatID, profileID uuid.UUID) (lastRead *uuid.UUID, updatedAt *time.Time, err error) {
	if s == nil || s.Pool == nil {
		return nil, nil, errors.New("messages store: pool not configured")
	}
	var lid *uuid.UUID
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
	if lid == nil {
		return nil, nil, nil
	}
	u := upd.UTC()
	return lid, &u, nil
}

// PublicReadReceipt is a receipt that was visible to the other DM participant.
type PublicReadReceipt struct {
	ChatID             uuid.UUID
	ProfileID          uuid.UUID // receipt reader whose cursor was visible
	MessageID          uuid.UUID
	RecipientProfileID uuid.UUID // participant that must remove this rendered tick
}

// ReadReceiptChatIDsForProfile returns candidate chat IDs from Messaging-owned
// receipt state. Callers must resolve which candidates are DMs via Chat.
func (s *MessagesStore) ReadReceiptChatIDsForProfile(ctx context.Context, profileID uuid.UUID) ([]uuid.UUID, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("messages store: pool not configured")
	}
	rows, err := s.Pool.Query(ctx, `SELECT DISTINCT chat_id FROM read_receipts WHERE profile_id = $1`, profileID)
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

// ClearPublicReadReceiptsForProfile revokes previously exposed DM receipt
// cursors after a participant opts out. Private read_positions remain intact.
func (s *MessagesStore) ClearPublicReadReceiptsForProfile(ctx context.Context, profileID uuid.UUID, chatIDs []uuid.UUID) ([]PublicReadReceipt, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("messages store: pool not configured")
	}
	if len(chatIDs) == 0 {
		return nil, nil
	}
	// chatIDs were resolved through Chat/S2S by the caller. Messaging owns only
	// receipt rows and must not join chat_db tables here.
	rows, err := s.Pool.Query(ctx, `
WITH own_before AS (
  SELECT rr.chat_id, rr.profile_id, rr.last_read_message_id
  FROM read_receipts rr
  WHERE rr.profile_id = $1 AND rr.chat_id = ANY($2) AND rr.last_read_message_id IS NOT NULL
  FOR UPDATE
), cleared AS (
  UPDATE read_receipts rr
  SET last_read_message_id = NULL, updated_at = now()
  FROM own_before ob
  WHERE rr.chat_id = ob.chat_id AND rr.profile_id = ob.profile_id
), own_revocations AS (
  SELECT ob.chat_id, ob.profile_id, ob.last_read_message_id, $1::uuid AS recipient_profile_id
  FROM own_before ob
), peer_revocations AS (
  SELECT rr.chat_id, rr.profile_id, rr.last_read_message_id, $1::uuid AS recipient_profile_id
  FROM read_receipts rr
  WHERE rr.profile_id <> $1 AND rr.chat_id = ANY($2) AND rr.last_read_message_id IS NOT NULL
)
SELECT chat_id, profile_id, last_read_message_id, recipient_profile_id FROM own_revocations
UNION ALL
SELECT chat_id, profile_id, last_read_message_id, recipient_profile_id FROM peer_revocations
`, profileID, chatIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PublicReadReceipt
	for rows.Next() {
		var r PublicReadReceipt
		if err := rows.Scan(&r.ChatID, &r.ProfileID, &r.MessageID, &r.RecipientProfileID); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetReadPosition returns the caller's private cursor for unread calculation
// and own read-state APIs; it is never exposed to a DM peer directly.
func (s *MessagesStore) GetReadPosition(ctx context.Context, chatID, profileID uuid.UUID) (lastRead *uuid.UUID, updatedAt *time.Time, err error) {
	if s == nil || s.Pool == nil {
		return nil, nil, errors.New("messages store: pool not configured")
	}
	var lid uuid.UUID
	var upd time.Time
	qerr := s.Pool.QueryRow(ctx, `
SELECT last_read_message_id, updated_at FROM read_positions
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

func (s *MessagesStore) UpsertDeliveredCursor(ctx context.Context, chatID, profileID, messageID uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return errors.New("messages store: pool not configured")
	}
	_, err := s.Pool.Exec(ctx, `
INSERT INTO read_receipts (chat_id, profile_id, last_read_message_id, last_delivered_message_id, updated_at)
VALUES ($1, $2, $3, $4, now())
ON CONFLICT (chat_id, profile_id) DO UPDATE SET
  last_delivered_message_id = CASE
    WHEN read_receipts.last_delivered_message_id IS NULL OR read_receipts.last_delivered_message_id < EXCLUDED.last_delivered_message_id THEN EXCLUDED.last_delivered_message_id
    ELSE read_receipts.last_delivered_message_id
  END,
  updated_at = CASE
    WHEN read_receipts.last_delivered_message_id IS NULL OR read_receipts.last_delivered_message_id < EXCLUDED.last_delivered_message_id THEN now()
    ELSE read_receipts.updated_at
  END
`, chatID, profileID, nil, messageID)
	return err
}

func uuidAtLeast(a, b uuid.UUID) bool {
	for i := 0; i < len(a); i++ {
		if a[i] > b[i] {
			return true
		}
		if a[i] < b[i] {
			return false
		}
	}
	return true
}

func deriveLastMessageDeliveryState(isOutgoing bool, lastMsgID uuid.UUID, peerRead, peerDelivered *uuid.UUID) string {
	if !isOutgoing {
		return "none"
	}
	if peerRead != nil && *peerRead != uuid.Nil && uuidAtLeast(*peerRead, lastMsgID) {
		return "read"
	}
	if peerDelivered != nil && uuidAtLeast(*peerDelivered, lastMsgID) {
		return "delivered"
	}
	return "sent"
}

func (s *MessagesStore) GetChatListMetadata(ctx context.Context, viewerProfileID uuid.UUID, chatIDs []uuid.UUID, blockedByChat ...map[uuid.UUID][]uuid.UUID) (map[uuid.UUID]ChatListMetadataRow, error) {
	blockedProfiles := make([]uuid.UUID, 0)
	if len(blockedByChat) > 0 {
		seen := make(map[uuid.UUID]struct{})
		for _, profileIDs := range blockedByChat[0] {
			for _, profileID := range profileIDs {
				if _, exists := seen[profileID]; exists {
					continue
				}
				seen[profileID] = struct{}{}
				blockedProfiles = append(blockedProfiles, profileID)
			}
		}
	}
	return s.GetChatListMetadataWithVisibility(ctx, viewerProfileID, chatIDs, blockedProfiles)
}

func (s *MessagesStore) GetChatListMetadataWithVisibility(ctx context.Context, viewerProfileID uuid.UUID, chatIDs []uuid.UUID, blockedProfileIDs []uuid.UUID) (map[uuid.UUID]ChatListMetadataRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("messages store: pool not configured")
	}
	out := make(map[uuid.UUID]ChatListMetadataRow, len(chatIDs))
	for _, chatID := range chatIDs {
		var preview sql.NullString
		var lastAt sql.NullTime
		var unread int64
		var lastMsgID uuid.UUID
		var lastSender uuid.UUID
		var lastAttachments sql.NullString
		var lastContentType sql.NullString
		var lastChatType sql.NullString
		var peerRead *uuid.UUID
		var peerDelivered *uuid.UUID
		err := s.Pool.QueryRow(ctx, `
WITH latest AS (
  SELECT id, content, created_at, sender_profile_id, attachments, content_type, chat_type
  FROM messages
  WHERE chat_id = $1 AND deleted_at IS NULL
    AND (NOT COALESCE(ghost_only, false) OR sender_profile_id = $2)
    AND NOT EXISTS (SELECT 1 FROM message_hides h WHERE h.message_id = messages.id AND h.profile_id = $2)
    AND NOT (sender_profile_id = ANY($3::uuid[]))
  ORDER BY id DESC
  LIMIT 1
), peer AS (
  SELECT profile_id AS peer_id
  FROM read_receipts
  WHERE chat_id = $1 AND profile_id <> $2
  LIMIT 1
), peer_rr AS (
  SELECT rr.last_read_message_id, rr.last_delivered_message_id
  FROM read_receipts rr
  INNER JOIN peer ON rr.chat_id = $1 AND rr.profile_id = peer.peer_id
), unread AS (
  SELECT count(*)::bigint AS unread_count
  FROM messages m
  LEFT JOIN read_positions rr
    ON rr.chat_id = m.chat_id AND rr.profile_id = $2
  WHERE m.chat_id = $1
    AND m.deleted_at IS NULL
    AND m.sender_profile_id <> $2
    AND (NOT COALESCE(m.ghost_only, false) OR m.sender_profile_id = $2)
    AND NOT EXISTS (SELECT 1 FROM message_hides h WHERE h.message_id = m.id AND h.profile_id = $2)
    AND NOT (m.sender_profile_id = ANY($3::uuid[]))
    AND (rr.last_read_message_id IS NULL OR m.id > rr.last_read_message_id)
)
SELECT latest.content, latest.created_at, latest.id, latest.sender_profile_id, latest.attachments, latest.content_type, latest.chat_type,
       peer_rr.last_read_message_id, peer_rr.last_delivered_message_id,
       unread.unread_count
FROM unread
LEFT JOIN latest ON true
LEFT JOIN peer_rr ON true
		`, chatID, viewerProfileID, blockedProfileIDs).Scan(&preview, &lastAt, &lastMsgID, &lastSender, &lastAttachments, &lastContentType, &lastChatType, &peerRead, &peerDelivered, &unread)
		if err != nil {
			return nil, err
		}
		row := ChatListMetadataRow{ChatID: chatID, UnreadCount: unread}
		if preview.Valid {
			row.LastMessagePreview = truncatePreview(preview.String)
		}
		attJSON := "[]"
		if lastAttachments.Valid {
			attJSON = lastAttachments.String
		}
		content := ""
		if preview.Valid {
			content = preview.String
		}
		storedType := ""
		if lastContentType.Valid {
			storedType = strings.TrimSpace(lastContentType.String)
		}
		row.LastMessageContentType = EffectiveContentType(storedType, content, attJSON)
		if lastAt.Valid {
			t := lastAt.Time.UTC()
			row.LastMessageAt = &t
		}
		if lastMsgID != uuid.Nil {
			row.LastMessageIsOutgoing = lastSender == viewerProfileID
			if lastChatType.String == "dm" {
				row.LastMessageDeliveryState = deriveLastMessageDeliveryState(
					row.LastMessageIsOutgoing, lastMsgID, peerRead, peerDelivered,
				)
			} else {
				row.LastMessageDeliveryState = "none"
			}
		} else {
			row.LastMessageDeliveryState = "none"
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
