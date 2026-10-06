package store

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"voice/backend/messaging/internal/messageevents"
	"voice/backend/pkg/spacemutationlock"
)

var (
	ErrMessageEventOutboxUnavailable = errors.New("message event outbox unavailable")
	ErrMessageMutationPurging        = errors.New("message mutation is inside a frozen purge work set")
)

// MessageEventOutboxClaimStore is the durable lease/PubAck boundary consumed
// by the JetStream dispatcher. PubAck state is distinct from consumer progress.
type MessageEventOutboxClaimStore interface {
	ClaimMessageEventOutbox(context.Context, int, time.Duration) ([]messageevents.OutboxEvent, error)
	RecordMessageEventPubAck(context.Context, uuid.UUID, uuid.UUID, uint64) (bool, error)
	RetryMessageEventOutbox(context.Context, uuid.UUID, uuid.UUID, time.Time) (bool, error)
}

func enqueueMessageEvent(ctx context.Context, tx pgx.Tx, event messageevents.OutboxEvent) error {
	if event.EventID == uuid.Nil || event.MessageID == uuid.Nil || event.ChatID == uuid.Nil || event.Subject == "" || len(event.Payload) == 0 {
		return ErrMessageEventOutboxUnavailable
	}
	// The schema requires headers to be a JSON object. A nil Go map marshals
	// as JSON null, which would reject the domain mutation and its outbox row.
	headersValue := event.Headers
	if headersValue == nil {
		headersValue = map[string]string{}
	}
	headers, err := json.Marshal(headersValue)
	if err != nil {
		return fmt.Errorf("encode message outbox headers: %w", err)
	}
	digest := sha256.Sum256(event.Payload)
	_, err = tx.Exec(ctx, `INSERT INTO message_event_outbox(event_id,subject,message_id,chat_id,payload_bytes,payload_sha256,headers)
VALUES($1,$2,$3,$4,$5,$6,$7::jsonb)`, event.EventID, event.Subject, event.MessageID, event.ChatID, event.Payload, digest[:], headers)
	return err
}

func requireLiveMessage(ctx context.Context, tx pgx.Tx, chatID, messageID uuid.UUID) error {
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM messages WHERE id=$1 AND chat_id=$2 AND deleted_at IS NULL)`, messageID, chatID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *MessagesStore) HideMessageForProfileWithMutationFence(ctx context.Context, chatID, spaceID, messageID, profileID uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return errors.New("messages store: pool not configured")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
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
	if _, err := tx.Exec(ctx, `INSERT INTO message_hides(message_id,profile_id) VALUES($1,$2) ON CONFLICT(message_id,profile_id) DO NOTHING`, messageID, profileID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *MessagesStore) UpsertDeliveredCursorWithMutationFence(ctx context.Context, chatID, spaceID, profileID, messageID uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return errors.New("messages store: pool not configured")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var scope *uuid.UUID
	if spaceID != uuid.Nil {
		scope = &spaceID
	}
	if err := lockMessageMutation(ctx, tx, chatID, scope, &messageID); err != nil {
		return err
	}
	if err := requireLiveMessage(ctx, tx, chatID, messageID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO read_receipts(chat_id,profile_id,last_read_message_id,last_delivered_message_id,updated_at) VALUES($1,$2,NULL,$3,now())
ON CONFLICT(chat_id,profile_id) DO UPDATE SET last_delivered_message_id=CASE WHEN read_receipts.last_delivered_message_id IS NULL OR read_receipts.last_delivered_message_id<EXCLUDED.last_delivered_message_id THEN EXCLUDED.last_delivered_message_id ELSE read_receipts.last_delivered_message_id END,updated_at=CASE WHEN read_receipts.last_delivered_message_id IS NULL OR read_receipts.last_delivered_message_id<EXCLUDED.last_delivered_message_id THEN now() ELSE read_receipts.updated_at END`, chatID, profileID, messageID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *MessagesStore) InsertMessageWithOutbox(ctx context.Context, row MessageRow, spaceID *uuid.UUID, events []messageevents.OutboxEvent) (*MessageRow, bool, error) {
	if s == nil || s.Pool == nil {
		return nil, false, errors.New("messages store: pool not configured")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := lockMessageMutation(ctx, tx, row.ChatID, spaceID, &row.ID); err != nil {
		return nil, false, err
	}
	saved, err := insertMessageDB(ctx, tx, row)
	if err != nil {
		return nil, false, err
	}
	inserted := saved.ID == row.ID
	if inserted {
		for _, event := range events {
			if event.MessageID != saved.ID || event.ChatID != saved.ChatID {
				return nil, false, errors.New("message outbox event does not match inserted message")
			}
			if err := enqueueMessageEvent(ctx, tx, event); err != nil {
				return nil, false, err
			}
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, false, err
	}
	return saved, inserted, nil
}

func (s *MessagesStore) UpdateMessageContentAndMentionsWithOutbox(ctx context.Context, chatID, spaceID, messageID, senderProfileID uuid.UUID, content string, mentionsJSON *string, events []messageevents.OutboxEvent) (*MessageRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("messages store: pool not configured")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var scope *uuid.UUID
	if spaceID != uuid.Nil {
		scope = &spaceID
	}
	if err := lockMessageMutation(ctx, tx, chatID, scope, &messageID); err != nil {
		return nil, err
	}
	query := `UPDATE messages SET content=$1,edited_at=now() WHERE id=$2 AND chat_id=$3 AND sender_profile_id=$4 AND deleted_at IS NULL AND NOT EXISTS(SELECT 1 FROM message_game_cards gc WHERE gc.message_id=messages.id) RETURNING ` + messageReturningCols
	args := []any{content, messageID, chatID, senderProfileID}
	if mentionsJSON != nil {
		query = `UPDATE messages SET content=$1,mentions=$2::jsonb,edited_at=now() WHERE id=$3 AND chat_id=$4 AND sender_profile_id=$5 AND deleted_at IS NULL AND NOT EXISTS(SELECT 1 FROM message_game_cards gc WHERE gc.message_id=messages.id) RETURNING ` + messageReturningCols
		args = []any{content, *mentionsJSON, messageID, chatID, senderProfileID}
	}
	updated, err := scanMessageRow(tx.QueryRow(ctx, query, args...))
	if err != nil {
		return nil, err
	}
	for _, event := range events {
		if event.MessageID != updated.ID || event.ChatID != updated.ChatID {
			return nil, errors.New("message outbox event does not match updated message")
		}
		if err := enqueueMessageEvent(ctx, tx, event); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *MessagesStore) SoftDeleteMessageWithOutbox(ctx context.Context, chatID, spaceID, messageID, senderProfileID uuid.UUID, event messageevents.OutboxEvent) error {
	if s == nil || s.Pool == nil {
		return errors.New("messages store: pool not configured")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
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
	ct, err := tx.Exec(ctx, `UPDATE messages SET deleted_at=now() WHERE id=$1 AND chat_id=$2 AND sender_profile_id=$3 AND deleted_at IS NULL`, messageID, chatID, senderProfileID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	if event.MessageID != messageID || event.ChatID != chatID {
		return errors.New("message outbox event does not match deleted message")
	}
	if err := enqueueMessageEvent(ctx, tx, event); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// UpsertReadStateAndOutbox commits private progress independently of the
// optional public receipt; only the public, currently visible cursor emits an event.
func (s *MessagesStore) UpsertReadStateAndOutbox(ctx context.Context, chatID, spaceID, profileID, messageID uuid.UUID, publishReceipt bool, event *messageevents.OutboxEvent) error {
	if s == nil || s.Pool == nil {
		return errors.New("messages store: pool not configured")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
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
	_, err = tx.Exec(ctx, `INSERT INTO read_positions(chat_id,profile_id,last_read_message_id,updated_at) VALUES($1,$2,$3,now())
ON CONFLICT(chat_id,profile_id) DO UPDATE SET last_read_message_id=CASE WHEN read_positions.last_read_message_id<EXCLUDED.last_read_message_id THEN EXCLUDED.last_read_message_id ELSE read_positions.last_read_message_id END,updated_at=CASE WHEN read_positions.last_read_message_id<EXCLUDED.last_read_message_id THEN now() ELSE read_positions.updated_at END`, chatID, profileID, messageID)
	if err != nil {
		return err
	}
	if publishReceipt {
		_, err = tx.Exec(ctx, `INSERT INTO read_receipts(chat_id,profile_id,last_read_message_id,last_delivered_message_id,updated_at) VALUES($1,$2,$3,$3,now())
ON CONFLICT(chat_id,profile_id) DO UPDATE SET last_read_message_id=CASE WHEN read_receipts.last_read_message_id IS NULL OR read_receipts.last_read_message_id<EXCLUDED.last_read_message_id THEN EXCLUDED.last_read_message_id ELSE read_receipts.last_read_message_id END,last_delivered_message_id=CASE WHEN read_receipts.last_delivered_message_id IS NULL OR read_receipts.last_delivered_message_id<EXCLUDED.last_read_message_id THEN EXCLUDED.last_read_message_id ELSE read_receipts.last_delivered_message_id END,updated_at=CASE WHEN read_receipts.last_read_message_id IS NULL OR read_receipts.last_read_message_id<EXCLUDED.last_read_message_id THEN now() ELSE read_receipts.updated_at END`, chatID, profileID, messageID)
		if err != nil {
			return err
		}
		if event == nil || event.MessageID != messageID || event.ChatID != chatID {
			return errors.New("public read receipt outbox event is missing or mismatched")
		}
		if err := enqueueMessageEvent(ctx, tx, *event); err != nil {
			return err
		}
	} else if event != nil {
		return errors.New("private read position cannot publish an outbox event")
	}
	return tx.Commit(ctx)
}

func (s *MessagesStore) ClearPublicReadReceiptsWithOutbox(ctx context.Context, profileID uuid.UUID, dmTargets map[uuid.UUID]uuid.UUID) ([]PublicReadReceipt, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("messages store: pool not configured")
	}
	if profileID == uuid.Nil {
		return nil, errors.New("receipt revocation profile is invalid")
	}
	if len(dmTargets) == 0 {
		return nil, nil
	}
	chatIDs := make([]uuid.UUID, 0, len(dmTargets))
	for chatID := range dmTargets {
		if chatID == uuid.Nil || dmTargets[chatID] == uuid.Nil {
			return nil, errors.New("receipt revocation DM scope is invalid")
		}
		chatIDs = append(chatIDs, chatID)
	}
	sort.Slice(chatIDs, func(i, j int) bool { return chatIDs[i].String() < chatIDs[j].String() })
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	for _, chatID := range chatIDs {
		if err := lockMessageMutation(ctx, tx, chatID, nil, nil); err != nil {
			return nil, err
		}
	}
	rows, err := tx.Query(ctx, `
WITH own_before AS (
  SELECT rr.chat_id, rr.profile_id, rr.last_read_message_id
  FROM read_receipts rr
  WHERE rr.profile_id = $1 AND rr.chat_id = ANY($2) AND rr.last_read_message_id IS NOT NULL
  FOR UPDATE
), cleared AS (
  UPDATE read_receipts rr SET last_read_message_id = NULL, updated_at = now()
  FROM own_before ob WHERE rr.chat_id = ob.chat_id AND rr.profile_id = ob.profile_id
), own_revocations AS (
  SELECT ob.chat_id, ob.profile_id, ob.last_read_message_id, $1::uuid AS recipient_profile_id FROM own_before ob
), peer_revocations AS (
  SELECT rr.chat_id, rr.profile_id, rr.last_read_message_id, $1::uuid AS recipient_profile_id
  FROM read_receipts rr
  WHERE rr.profile_id <> $1 AND rr.chat_id = ANY($2) AND rr.last_read_message_id IS NOT NULL
)
SELECT chat_id, profile_id, last_read_message_id, recipient_profile_id FROM own_revocations
UNION ALL
SELECT chat_id, profile_id, last_read_message_id, recipient_profile_id FROM peer_revocations`, profileID, chatIDs)
	if err != nil {
		return nil, err
	}
	var revoked []PublicReadReceipt
	for rows.Next() {
		var row PublicReadReceipt
		if err := rows.Scan(&row.ChatID, &row.ProfileID, &row.MessageID, &row.RecipientProfileID); err != nil {
			rows.Close()
			return nil, err
		}
		peerID, exists := dmTargets[row.ChatID]
		if !exists {
			rows.Close()
			return nil, errors.New("receipt revocation row is outside authorized DM scope")
		}
		if row.ProfileID == profileID {
			row.RecipientProfileID = peerID
		} else {
			row.RecipientProfileID = profileID
		}
		revoked = append(revoked, row)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	for _, row := range revoked {
		event, err := messageevents.NewReadReceiptRevokedOutbox(row.MessageID.String(), row.ChatID.String(), row.ProfileID.String(), row.RecipientProfileID.String())
		if err != nil {
			return nil, err
		}
		if err := enqueueMessageEvent(ctx, tx, event); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return revoked, nil
}

// lockMessageMutation serializes chat writes against purge snapshot creation
// and, for Space chats, against the lifecycle freeze fence. Every caller takes
// the chat lock first and Space lock second.
func lockMessageMutation(ctx context.Context, tx pgx.Tx, chatID uuid.UUID, spaceID *uuid.UUID, messageID *uuid.UUID) error {
	if chatID == uuid.Nil {
		return errors.New("message mutation chat ID is required")
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "voice.messaging.message-mutation.v1/"+chatID.String()); err != nil {
		return err
	}
	if spaceID != nil {
		if spaceID == nil || *spaceID == uuid.Nil {
			return errors.New("message mutation Space ID is invalid")
		}
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1::bigint)`, spacemutationlock.Key(*spaceID)); err != nil {
			return err
		}
		var frozen bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM messaging_space_lifecycle_fences WHERE space_id=$1 AND state<>'LIVE')`, *spaceID).Scan(&frozen); err != nil {
			return err
		}
		if frozen {
			return ErrSpaceLifecycleOrder
		}
	}
	if messageID != nil {
		var purging bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(
  SELECT 1 FROM managed_chat_purge_operations o
  JOIN managed_chat_purge_messages p ON p.operation_id=o.operation_id
  WHERE o.chat_id=$1 AND o.state='PENDING' AND p.message_id=$2
)`, chatID, *messageID).Scan(&purging); err != nil {
			return err
		}
		if purging {
			return ErrMessageMutationPurging
		}
	}
	return nil
}

func lockChatAndSpaceForPurge(ctx context.Context, tx pgx.Tx, chatID uuid.UUID) (*uuid.UUID, error) {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "voice.messaging.message-mutation.v1/"+chatID.String()); err != nil {
		return nil, err
	}
	spaceID, scoped := SpacePurgeScope(ctx)
	if !scoped {
		return nil, nil
	}
	if spaceID == uuid.Nil {
		return nil, ErrSpacePurgeReceiptBinding
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1::bigint)`, spacemutationlock.Key(spaceID)); err != nil {
		return nil, err
	}
	return &spaceID, nil
}

func (s *MessagesStore) ClaimMessageEventOutbox(ctx context.Context, limit int, lease time.Duration) ([]messageevents.OutboxEvent, error) {
	if s == nil || s.Pool == nil || limit < 1 || lease <= 0 {
		return nil, ErrMessageEventOutboxUnavailable
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	rows, err := tx.Query(ctx, `SELECT event_id,subject,message_id,chat_id,payload_bytes,headers::text
FROM message_event_outbox
WHERE pubacked_at IS NULL AND next_attempt_at<=clock_timestamp()
  AND (lease_expires_at IS NULL OR lease_expires_at<=clock_timestamp())
ORDER BY next_attempt_at,created_at,event_id
FOR UPDATE SKIP LOCKED LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	var events []messageevents.OutboxEvent
	for rows.Next() {
		var event messageevents.OutboxEvent
		var payload []byte
		var headersJSON string
		if err = rows.Scan(&event.EventID, &event.Subject, &event.MessageID, &event.ChatID, &payload, &headersJSON); err != nil {
			rows.Close()
			return nil, err
		}
		if len(payload) == 0 {
			rows.Close()
			return nil, fmt.Errorf("pending message event %s has no retry bytes", event.EventID)
		}
		if err = json.Unmarshal([]byte(headersJSON), &event.Headers); err != nil {
			rows.Close()
			return nil, err
		}
		event.Payload = append([]byte(nil), payload...)
		event.LeaseToken = uuid.New()
		events = append(events, event)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	for i := range events {
		if _, err = tx.Exec(ctx, `UPDATE message_event_outbox
SET lease_token=$2,lease_expires_at=clock_timestamp()+$3::interval,
    attempt_count=LEAST(attempt_count+1,2147483647)
WHERE event_id=$1 AND pubacked_at IS NULL`, events[i].EventID, events[i].LeaseToken, lease.String()); err != nil {
			return nil, err
		}
		if err = tx.QueryRow(ctx, `SELECT attempt_count FROM message_event_outbox WHERE event_id=$1`, events[i].EventID).Scan(&events[i].Attempts); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return events, nil
}

func (s *MessagesStore) RecordMessageEventPubAck(ctx context.Context, eventID, leaseToken uuid.UUID, sequence uint64) (bool, error) {
	if s == nil || s.Pool == nil || eventID == uuid.Nil || leaseToken == uuid.Nil || sequence == 0 {
		return false, ErrMessageEventOutboxUnavailable
	}
	ct, err := s.Pool.Exec(ctx, `UPDATE message_event_outbox
SET pubacked_at=clock_timestamp(),puback_sequence=$3,payload_bytes=NULL,headers='{}'::jsonb,
    payload_pruned_at=clock_timestamp(),lease_token=NULL,lease_expires_at=NULL
WHERE event_id=$1 AND lease_token=$2 AND pubacked_at IS NULL`, eventID, leaseToken, sequence)
	return ct.RowsAffected() == 1, err
}

func (s *MessagesStore) RetryMessageEventOutbox(ctx context.Context, eventID, leaseToken uuid.UUID, nextAttempt time.Time) (bool, error) {
	if s == nil || s.Pool == nil || eventID == uuid.Nil || leaseToken == uuid.Nil || nextAttempt.IsZero() {
		return false, ErrMessageEventOutboxUnavailable
	}
	ct, err := s.Pool.Exec(ctx, `UPDATE message_event_outbox
SET next_attempt_at=$3,lease_token=NULL,lease_expires_at=NULL
WHERE event_id=$1 AND lease_token=$2 AND pubacked_at IS NULL`, eventID, leaseToken, nextAttempt.UTC())
	return ct.RowsAffected() == 1, err
}

func messageEventSetHash(events []managedChatPurgeEvent) []byte {
	hash := sha256.New()
	_, _ = hash.Write([]byte("voice.messaging.managed-chat-purge-events.v1\x00"))
	var count [8]byte
	binary.BigEndian.PutUint64(count[:], uint64(len(events)))
	_, _ = hash.Write(count[:])
	for _, event := range events {
		_, _ = hash.Write(event.EventID[:])
		_, _ = hash.Write(event.MessageID[:])
		_, _ = hash.Write(event.SHA256)
	}
	return hash.Sum(nil)
}

var _ MessageEventOutboxClaimStore = (*MessagesStore)(nil)
