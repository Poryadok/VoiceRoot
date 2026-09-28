package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"voice/backend/messaging/internal/gameprotocol"
)

var (
	ErrGameOperationConflict         = errors.New("game message operation id reused with different bytes")
	ErrGameRevisionConflict          = errors.New("game message revision does not extend current revision")
	ErrGameMessageTerminal           = errors.New("game message is deleted")
	ErrGameAuthorityExpired          = errors.New("game message authority expired before commit")
	ErrGameAuthorityRevisionConflict = errors.New("game device authority revision is stale or inconsistent")
)

type GameMessagePermitCompletion struct {
	PermitID      uuid.UUID
	GISPermitID   uuid.UUID
	OperationID   uuid.UUID
	RequestSHA256 string
	Outcome       string
}

// AppendGameMessageTombstone appends one signed terminal moderator revision.
// The signer runs after action receipt and message locks, and before any writes,
// so retries return the original envelope and concurrent actions cannot fork.
func (s *MessagesStore) AppendGameMessageTombstone(ctx context.Context, tombstone gameprotocol.GameMessageTombstone, sign func(gameprotocol.GameMessageTombstone) (string, error)) (*MessageRow, string, error) {
	if s == nil || s.Pool == nil || sign == nil || tombstone.ActionID == uuid.Nil || tombstone.ApplicationID == uuid.Nil || tombstone.EnvironmentID == uuid.Nil || tombstone.ChatID == uuid.Nil || tombstone.MessageID == uuid.Nil ||
		(tombstone.ReasonClass != "moderation" && tombstone.ReasonClass != "system_retention") {
		return nil, "", errors.New("game tombstone: invalid action or store")
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, "", err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "game-tombstone-action/"+tombstone.ActionID.String()); err != nil {
		return nil, "", err
	}
	var priorApp, priorEnv, priorChat, priorMessage uuid.UUID
	var priorReason, priorCompact string
	err = tx.QueryRow(ctx, `SELECT application_id,environment_id,chat_id,message_id,reason_class,compact_jws FROM game_message_tombstone_actions WHERE action_id=$1`, tombstone.ActionID).
		Scan(&priorApp, &priorEnv, &priorChat, &priorMessage, &priorReason, &priorCompact)
	if err == nil {
		if priorApp != tombstone.ApplicationID || priorEnv != tombstone.EnvironmentID || priorChat != tombstone.ChatID || priorMessage != tombstone.MessageID || priorReason != tombstone.ReasonClass {
			return nil, "", ErrGameOperationConflict
		}
		row, err := scanMessageRow(tx.QueryRow(ctx, messageSelectSQL+`FROM messages WHERE chat_id=$1 AND id=$2`, tombstone.ChatID, tombstone.MessageID))
		if err != nil {
			return nil, "", err
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, "", err
		}
		return row, priorCompact, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, "", err
	}
	lockKey := fmt.Sprintf("%s/%s", tombstone.ChatID, tombstone.MessageID)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, lockKey); err != nil {
		return nil, "", err
	}
	var currentRevision int64
	var previousCompact string
	var terminal bool
	err = tx.QueryRow(ctx, `SELECT r.revision,r.compact_jws,m.deleted_at IS NOT NULL FROM game_message_revisions r JOIN messages m ON m.chat_id=r.chat_id AND m.id=r.message_id WHERE r.chat_id=$1 AND r.message_id=$2 ORDER BY r.revision DESC LIMIT 1 FOR UPDATE OF m`, tombstone.ChatID, tombstone.MessageID).Scan(&currentRevision, &previousCompact, &terminal)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", ErrGameRevisionConflict
	}
	if err != nil {
		return nil, "", err
	}
	if terminal {
		return nil, "", ErrGameMessageTerminal
	}
	tombstone.Revision = currentRevision + 1
	tombstone.PreviousRevisionHash = hashCompact(previousCompact)
	compact, err := sign(tombstone)
	if err != nil {
		return nil, "", err
	}
	if compact == "" {
		return nil, "", errors.New("game tombstone signer returned an empty envelope")
	}
	if _, err := tx.Exec(ctx, `INSERT INTO game_message_revisions (chat_id,message_id,revision,operation,operation_id,previous_revision_hash,compact_jws,content_sha256) VALUES ($1,$2,$3,'moderator_delete',$4,$5,$6,$7)`, tombstone.ChatID, tombstone.MessageID, tombstone.Revision, tombstone.ActionID, tombstone.PreviousRevisionHash, compact, hashCompact(compact)); err != nil {
		return nil, "", err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO game_message_tombstone_actions (action_id,application_id,environment_id,chat_id,message_id,reason_class,revision,compact_jws) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`, tombstone.ActionID, tombstone.ApplicationID, tombstone.EnvironmentID, tombstone.ChatID, tombstone.MessageID, tombstone.ReasonClass, tombstone.Revision, compact); err != nil {
		return nil, "", err
	}
	if _, err := tx.Exec(ctx, `UPDATE messages SET deleted_at=now() WHERE chat_id=$1 AND id=$2 AND deleted_at IS NULL`, tombstone.ChatID, tombstone.MessageID); err != nil {
		return nil, "", err
	}
	row, err := scanMessageRow(tx.QueryRow(ctx, messageSelectSQL+`FROM messages WHERE chat_id=$1 AND id=$2`, tombstone.ChatID, tombstone.MessageID))
	if err != nil {
		return nil, "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, "", err
	}
	return row, compact, nil
}

// LookupGameMessageTombstoneReceipt returns an exact action retry before key
// availability or current key validity is consulted.
func (s *MessagesStore) LookupGameMessageTombstoneReceipt(ctx context.Context, tombstone gameprotocol.GameMessageTombstone) (string, bool, error) {
	if s == nil || s.Pool == nil || tombstone.ActionID == uuid.Nil {
		return "", false, errors.New("game tombstone: invalid receipt lookup")
	}
	var app, env, chat, message uuid.UUID
	var reason, compact string
	err := s.Pool.QueryRow(ctx, `SELECT application_id,environment_id,chat_id,message_id,reason_class,compact_jws FROM game_message_tombstone_actions WHERE action_id=$1`, tombstone.ActionID).Scan(&app, &env, &chat, &message, &reason, &compact)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if app != tombstone.ApplicationID || env != tombstone.EnvironmentID || chat != tombstone.ChatID || message != tombstone.MessageID || reason != tombstone.ReasonClass {
		return "", false, ErrGameOperationConflict
	}
	return compact, true, nil
}

// ApplyGameMessage atomically writes a verified signed operation, the ordinary
// Messaging message row, its immutable revision, and its retry receipt. The
// caller must verify the JWS and resolve senderProfileID from trusted binding
// authority before calling this method.
func (s *MessagesStore) ApplyGameMessage(ctx context.Context, message gameprotocol.Message, senderProfileID uuid.UUID) (*MessageRow, error) {
	return s.ApplyGameMessageWithDeadline(ctx, message, senderProfileID, time.Time{})
}

// ApplyGameMessageWithDeadline repeats the Auth monotonic deadline check after
// all SQL work and immediately before commit. A zero deadline is reserved for
// trusted store tests and non-network internal callers.
func (s *MessagesStore) ApplyGameMessageWithDeadline(ctx context.Context, message gameprotocol.Message, senderProfileID uuid.UUID, deadline time.Time) (*MessageRow, error) {
	return s.applyGameMessage(ctx, message, senderProfileID, deadline, nil)
}

// ApplyGameMessageWithExecutionPermit commits the game mutation, durable
// receipt, and Auth/GIS completion outbox entry in one PostgreSQL transaction.
func (s *MessagesStore) ApplyGameMessageWithExecutionPermit(ctx context.Context, message gameprotocol.Message, senderProfileID uuid.UUID, deadline time.Time, permit GameMessagePermitCompletion) (*MessageRow, error) {
	if permit.Outcome != "committed" || permit.OperationID != message.OperationID || permit.PermitID == uuid.Nil || permit.GISPermitID == uuid.Nil || len(permit.RequestSHA256) != 64 {
		return nil, errors.New("game message: invalid execution permit completion receipt")
	}
	return s.applyGameMessage(ctx, message, senderProfileID, deadline, &permit)
}

func (s *MessagesStore) applyGameMessage(ctx context.Context, message gameprotocol.Message, senderProfileID uuid.UUID, deadline time.Time, permit *GameMessagePermitCompletion) (*MessageRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("messages store: pool not configured")
	}
	if message.Compact == "" || message.KeyID == uuid.Nil || message.DeviceID == uuid.Nil || message.AuthorityRevision <= 0 || message.OperationID == uuid.Nil || message.ApplicationID == uuid.Nil || message.EnvironmentID == uuid.Nil || message.ChatID == uuid.Nil || message.MessageID == uuid.Nil || senderProfileID == uuid.Nil {
		return nil, errors.New("game message: missing verified operation identity")
	}
	if hashContent(message.Content) != message.ContentSHA256 {
		return nil, errors.New("game message: content digest mismatch")
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// Receipt lookup is first so an exact retry remains idempotent after the
	// envelope expires or its device authority is later revoked.
	replay, found, err := gameReceipt(ctx, tx, message)
	if err != nil || found {
		if err != nil {
			return nil, err
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return replay, nil
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, operationLockKey(message)); err != nil {
		return nil, err
	}
	// A concurrent identical request may have committed while this caller
	// waited for the operation lock.
	replay, found, err = gameReceipt(ctx, tx, message)
	if err != nil || found {
		if err != nil {
			return nil, err
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return replay, nil
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, messageLockKey(message)); err != nil {
		return nil, err
	}
	if err := advanceGameDeviceAuthority(ctx, tx, message); err != nil {
		return nil, err
	}

	var result *MessageRow
	if message.Operation == "create" {
		if message.Revision != 1 || message.PreviousRevisionHash != "" {
			return nil, ErrGameRevisionConflict
		}
		created, err := s.insertGameMessage(ctx, tx, message, senderProfileID)
		if err != nil {
			return nil, err
		}
		result = created
	} else {
		result, err = s.applyGameMutation(ctx, tx, message, senderProfileID)
		if err != nil {
			return nil, err
		}
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO game_message_revisions
  (chat_id, message_id, revision, operation, operation_id, previous_revision_hash, compact_jws, content_sha256)
VALUES ($1,$2,$3,$4,$5,NULLIF($6,''),$7,$8)
`, message.ChatID, message.MessageID, message.Revision, message.Operation, message.OperationID,
		message.PreviousRevisionHash, message.Compact, message.ContentSHA256); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO game_message_operation_receipts
  (application_id, environment_id, operation_id, compact_jws, chat_id, message_id, result_content, result_deleted)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
`, message.ApplicationID, message.EnvironmentID, message.OperationID, message.Compact, message.ChatID, message.MessageID, result.Content, result.DeletedAt != nil); err != nil {
		return nil, err
	}
	if permit != nil {
		if _, err := tx.Exec(ctx, `INSERT INTO game_message_execution_permit_completions
(permit_id,gis_permit_id,operation_id,request_sha256,outcome,status)
VALUES ($1,$2,$3,$4,'committed','pending')`, permit.PermitID, permit.GISPermitID, permit.OperationID, permit.RequestSHA256); err != nil {
			return nil, err
		}
	}
	if !deadline.IsZero() && !time.Now().Before(deadline) {
		return nil, ErrGameAuthorityExpired
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return result, nil
}

// RecordAbortedGameMessagePermit persists a known pre-commit denial so exact
// retries cannot reuse an already-aborted one-use permit.
func (s *MessagesStore) RecordAbortedGameMessagePermit(ctx context.Context, permit GameMessagePermitCompletion) error {
	if s == nil || s.Pool == nil || permit.PermitID == uuid.Nil || permit.GISPermitID == uuid.Nil || permit.OperationID == uuid.Nil || permit.Outcome != "aborted" || len(permit.RequestSHA256) != 64 {
		return errors.New("game message: invalid aborted permit completion")
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "game-permit-operation/"+permit.OperationID.String()); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO game_message_execution_permit_completions
(permit_id,gis_permit_id,operation_id,request_sha256,outcome,status)
VALUES ($1,$2,$3,$4,'aborted','pending') ON CONFLICT (operation_id) DO NOTHING`,
		permit.PermitID, permit.GISPermitID, permit.OperationID, permit.RequestSHA256); err != nil {
		return err
	}
	var priorPermit, priorGIS, requestHash, outcome string
	if err := tx.QueryRow(ctx, `SELECT permit_id::text,gis_permit_id::text,request_sha256,outcome
FROM game_message_execution_permit_completions WHERE operation_id=$1`, permit.OperationID).
		Scan(&priorPermit, &priorGIS, &requestHash, &outcome); err != nil {
		return err
	}
	if priorPermit != permit.PermitID.String() || priorGIS != permit.GISPermitID.String() || requestHash != permit.RequestSHA256 || outcome != "aborted" {
		return ErrGameOperationConflict
	}
	return tx.Commit(ctx)
}

func (s *MessagesStore) GameMessagePermitCompletionByOperation(ctx context.Context, operationID uuid.UUID) (GameMessagePermitCompletion, string, bool, error) {
	var completion GameMessagePermitCompletion
	var status string
	if s == nil || s.Pool == nil || operationID == uuid.Nil {
		return completion, "", false, errors.New("messages store: invalid execution permit lookup")
	}
	err := s.Pool.QueryRow(ctx, `SELECT permit_id,gis_permit_id,operation_id,request_sha256,outcome,status
FROM game_message_execution_permit_completions WHERE operation_id=$1`, operationID).
		Scan(&completion.PermitID, &completion.GISPermitID, &completion.OperationID, &completion.RequestSHA256, &completion.Outcome, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return completion, "", false, nil
	}
	return completion, status, err == nil, err
}

func (s *MessagesStore) PendingGameMessagePermitCompletions(ctx context.Context, limit int) ([]GameMessagePermitCompletion, error) {
	if s == nil || s.Pool == nil || limit <= 0 || limit > 1000 {
		return nil, errors.New("messages store: invalid execution permit outbox query")
	}
	rows, err := s.Pool.Query(ctx, `SELECT permit_id,gis_permit_id,operation_id,request_sha256,outcome
FROM game_message_execution_permit_completions WHERE status='pending' ORDER BY created_at,permit_id LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []GameMessagePermitCompletion
	for rows.Next() {
		var completion GameMessagePermitCompletion
		if err := rows.Scan(&completion.PermitID, &completion.GISPermitID, &completion.OperationID, &completion.RequestSHA256, &completion.Outcome); err != nil {
			return nil, err
		}
		result = append(result, completion)
	}
	return result, rows.Err()
}

func (s *MessagesStore) MarkGameMessagePermitCompletion(ctx context.Context, completion GameMessagePermitCompletion) error {
	if s == nil || s.Pool == nil || completion.PermitID == uuid.Nil || completion.OperationID == uuid.Nil || (completion.Outcome != "committed" && completion.Outcome != "aborted") {
		return errors.New("messages store: invalid execution permit completion")
	}
	result, err := s.Pool.Exec(ctx, `UPDATE game_message_execution_permit_completions SET status='completed',completed_at=now()
WHERE permit_id=$1 AND operation_id=$2 AND outcome=$3 AND status='pending'`, completion.PermitID, completion.OperationID, completion.Outcome)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		var status string
		err := s.Pool.QueryRow(ctx, `SELECT status FROM game_message_execution_permit_completions
WHERE permit_id=$1 AND operation_id=$2 AND outcome=$3`, completion.PermitID, completion.OperationID, completion.Outcome).Scan(&status)
		if err != nil || status != "completed" {
			return ErrGameOperationConflict
		}
	}
	return nil
}

func advanceGameDeviceAuthority(ctx context.Context, tx pgx.Tx, message gameprotocol.Message) error {
	lockKey := fmt.Sprintf("%s/%s/%s", message.ApplicationID, message.EnvironmentID, message.DeviceID)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, lockKey); err != nil {
		return err
	}
	var currentRevision int64
	var currentKey uuid.UUID
	var currentStatus string
	err := tx.QueryRow(ctx, `
SELECT authority_revision,key_id,status FROM game_message_device_authorities
WHERE application_id=$1 AND environment_id=$2 AND device_id=$3 FOR UPDATE
`, message.ApplicationID, message.EnvironmentID, message.DeviceID).Scan(&currentRevision, &currentKey, &currentStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		_, err = tx.Exec(ctx, `INSERT INTO game_message_device_authorities
(application_id,environment_id,device_id,authority_revision,key_id,status)
VALUES ($1,$2,$3,$4,$5,'active')`, message.ApplicationID, message.EnvironmentID, message.DeviceID, message.AuthorityRevision, message.KeyID)
		return err
	}
	if err != nil {
		return err
	}
	if message.AuthorityRevision < currentRevision ||
		(message.AuthorityRevision == currentRevision && (message.KeyID != currentKey || currentStatus != "active")) {
		return ErrGameAuthorityRevisionConflict
	}
	if message.AuthorityRevision > currentRevision {
		_, err = tx.Exec(ctx, `UPDATE game_message_device_authorities
SET authority_revision=$1,key_id=$2,status='active',updated_at=now()
WHERE application_id=$3 AND environment_id=$4 AND device_id=$5`, message.AuthorityRevision, message.KeyID, message.ApplicationID, message.EnvironmentID, message.DeviceID)
	}
	return err
}

// LookupGameMessageReceipt implements the read-only receipt-first stage. The
// caller supplies only fields extracted by strict structural parsing; this
// method intentionally performs no signature, expiry, or authority check.
func (s *MessagesStore) LookupGameMessageReceipt(ctx context.Context, key gameprotocol.ReceiptKey) (*MessageRow, bool, error) {
	if s == nil || s.Pool == nil {
		return nil, false, errors.New("messages store: pool not configured")
	}
	if key.ApplicationID == uuid.Nil || key.EnvironmentID == uuid.Nil || key.OperationID == uuid.Nil || key.ChatID == uuid.Nil || key.MessageID == uuid.Nil || key.Revision <= 0 || key.Compact == "" {
		return nil, false, errors.New("game message: invalid receipt key")
	}
	var receiptBytes string
	var chatID, messageID uuid.UUID
	var resultContent string
	var resultDeleted bool
	err := s.Pool.QueryRow(ctx, `
SELECT compact_jws, chat_id, message_id, result_content, result_deleted
FROM game_message_operation_receipts
WHERE application_id=$1 AND environment_id=$2 AND operation_id=$3
`, key.ApplicationID, key.EnvironmentID, key.OperationID).Scan(&receiptBytes, &chatID, &messageID, &resultContent, &resultDeleted)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, false, err
	}
	if err == nil {
		if receiptBytes != key.Compact || chatID != key.ChatID || messageID != key.MessageID {
			return nil, false, ErrGameOperationConflict
		}
		return s.loadGameReceiptResult(ctx, chatID, messageID, resultContent, resultDeleted)
	}
	var revisionBytes string
	err = s.Pool.QueryRow(ctx, `
SELECT compact_jws FROM game_message_revisions
WHERE chat_id=$1 AND message_id=$2 AND revision=$3
	`, key.ChatID, key.MessageID, key.Revision).Scan(&revisionBytes)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if revisionBytes != key.Compact {
		return nil, false, ErrGameOperationConflict
	}
	return nil, false, errors.New("game message: revision exists without operation receipt")
}

func (s *MessagesStore) loadGameReceiptResult(ctx context.Context, chatID, messageID uuid.UUID, content string, deleted bool) (*MessageRow, bool, error) {
	row, err := scanMessageRow(s.Pool.QueryRow(ctx, messageSelectSQL+`FROM messages WHERE chat_id=$1 AND id=$2`, chatID, messageID))
	if err != nil {
		return nil, false, err
	}
	row.Content = content
	if deleted {
		row.DeletedAt = &row.CreatedAt
	}
	return row, true, nil
}

func gameReceipt(ctx context.Context, tx pgx.Tx, message gameprotocol.Message) (*MessageRow, bool, error) {
	var compact string
	var chatID, messageID uuid.UUID
	var content string
	var deleted bool
	err := tx.QueryRow(ctx, `
SELECT compact_jws, chat_id, message_id, result_content, result_deleted
FROM game_message_operation_receipts
WHERE application_id = $1 AND environment_id = $2 AND operation_id = $3
	`, message.ApplicationID, message.EnvironmentID, message.OperationID).Scan(&compact, &chatID, &messageID, &content, &deleted)
	if errors.Is(err, pgx.ErrNoRows) {
		var revisionBytes string
		revisionErr := tx.QueryRow(ctx, `
SELECT compact_jws FROM game_message_revisions
WHERE chat_id=$1 AND message_id=$2 AND revision=$3
`, message.ChatID, message.MessageID, message.Revision).Scan(&revisionBytes)
		if errors.Is(revisionErr, pgx.ErrNoRows) {
			return nil, false, nil
		}
		if revisionErr != nil {
			return nil, false, revisionErr
		}
		if revisionBytes != message.Compact {
			return nil, false, ErrGameOperationConflict
		}
		return nil, false, errors.New("game message: revision exists without operation receipt")
	}
	if err != nil {
		return nil, false, err
	}
	if compact != message.Compact {
		return nil, false, ErrGameOperationConflict
	}
	row, err := scanMessageRow(tx.QueryRow(ctx, messageSelectSQL+`FROM messages WHERE chat_id=$1 AND id=$2`, chatID, messageID))
	if err != nil {
		return nil, false, err
	}
	row.Content = content
	if deleted {
		row.DeletedAt = &row.CreatedAt
	}
	return row, true, nil
}

func (s *MessagesStore) insertGameMessage(ctx context.Context, tx pgx.Tx, message gameprotocol.Message, senderProfileID uuid.UUID) (*MessageRow, error) {
	if message.Content == nil || len(message.Content) == 0 {
		return nil, errors.New("game message: create requires non-empty content")
	}
	attachments, err := gameAttachmentsJSON(message.Attachments)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `
INSERT INTO messages (id, chat_id, chat_type, sender_profile_id, content, type, attachments, mentions)
VALUES ($1,$2,'dm',$3,$4,'regular',$5::jsonb,'[]'::jsonb)
`, message.MessageID, message.ChatID, senderProfileID, string(message.Content), attachments)
	if err != nil {
		return nil, err
	}
	return scanMessageRow(tx.QueryRow(ctx, messageSelectSQL+`FROM messages WHERE chat_id=$1 AND id=$2`, message.ChatID, message.MessageID))
}

func (s *MessagesStore) applyGameMutation(ctx context.Context, tx pgx.Tx, message gameprotocol.Message, senderProfileID uuid.UUID) (*MessageRow, error) {
	var currentRevision int64
	var previousCompact string
	var terminal bool
	err := tx.QueryRow(ctx, `
SELECT r.revision, r.compact_jws, m.deleted_at IS NOT NULL
FROM game_message_revisions r
JOIN messages m ON m.chat_id=r.chat_id AND m.id=r.message_id
WHERE r.chat_id=$1 AND r.message_id=$2
ORDER BY r.revision DESC LIMIT 1 FOR UPDATE OF m
`, message.ChatID, message.MessageID).Scan(&currentRevision, &previousCompact, &terminal)
	if errors.Is(err, pgx.ErrNoRows) || currentRevision+1 != message.Revision {
		return nil, ErrGameRevisionConflict
	}
	if err != nil {
		return nil, err
	}
	if terminal {
		return nil, ErrGameMessageTerminal
	}
	if message.PreviousRevisionHash != hashCompact(previousCompact) {
		return nil, ErrGameRevisionConflict
	}
	if message.Operation == "edit" {
		attachments, err := gameAttachmentsJSON(message.Attachments)
		if err != nil {
			return nil, err
		}
		updated, err := scanMessageRow(tx.QueryRow(ctx, `
UPDATE messages SET content=$1, attachments=$2::jsonb, edited_at=now()
WHERE chat_id=$3 AND id=$4 AND sender_profile_id=$5 AND deleted_at IS NULL
RETURNING `+messageReturningCols, string(message.Content), attachments, message.ChatID, message.MessageID, senderProfileID))
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrGameRevisionConflict
		}
		return updated, err
	}
	if message.Operation == "delete" {
		if _, err := tx.Exec(ctx, `UPDATE messages SET deleted_at=now() WHERE chat_id=$1 AND id=$2 AND sender_profile_id=$3 AND deleted_at IS NULL`, message.ChatID, message.MessageID, senderProfileID); err != nil {
			return nil, err
		}
		return scanMessageRow(tx.QueryRow(ctx, messageSelectSQL+`FROM messages WHERE chat_id=$1 AND id=$2`, message.ChatID, message.MessageID))
	}
	return nil, fmt.Errorf("game message: unsupported operation %q", message.Operation)
}

func operationLockKey(message gameprotocol.Message) string {
	return fmt.Sprintf("%s/%s/%s", message.ApplicationID, message.EnvironmentID, message.OperationID)
}

func messageLockKey(message gameprotocol.Message) string {
	return fmt.Sprintf("%s/%s", message.ChatID, message.MessageID)
}

func hashContent(content []byte) string {
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:])
}

func hashCompact(compact string) string {
	digest := sha256.Sum256([]byte(compact))
	return hex.EncodeToString(digest[:])
}

func gameAttachmentsJSON(attachments []gameprotocol.Attachment) (string, error) {
	if len(attachments) == 0 {
		return "[]", nil
	}
	type attachmentJSON struct {
		FileID         string `json:"file_id"`
		ObjectRevision int64  `json:"object_revision"`
		ByteLength     int64  `json:"byte_length"`
		ContentSHA256  string `json:"content_sha256"`
		MediaType      string `json:"media_type"`
	}
	serialized := make([]attachmentJSON, 0, len(attachments))
	for _, attachment := range attachments {
		serialized = append(serialized, attachmentJSON{
			FileID: attachment.FileID.String(), ObjectRevision: attachment.ObjectRevision,
			ByteLength: attachment.ByteLength, ContentSHA256: attachment.ContentSHA256,
			MediaType: attachment.MediaType,
		})
	}
	data, err := json.Marshal(serialized)
	if err != nil {
		return "", fmt.Errorf("encode game attachment manifest: %w", err)
	}
	return string(data), nil
}
