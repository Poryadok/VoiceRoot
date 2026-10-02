package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrManagedChatPurgeNotDue            = errors.New("managed chat purge cutoff has not elapsed")
	ErrManagedChatPurgeOperationConflict = errors.New("managed chat purge operation conflicts with saved request")
)

type ManagedChatPurgeMessage struct {
	ID              uuid.UUID
	AttachmentsJSON string
	FileIDs         []uuid.UUID
}

type ManagedChatPurgeWork struct {
	OperationID         uuid.UUID
	ChatID              uuid.UUID
	PurgeAfter          time.Time
	RequestSHA256       []byte
	State               string
	FileReceiptSHA256   []byte
	SearchReceiptSHA256 []byte
	CompletedAt         *time.Time
	MessageIDs          []uuid.UUID
	MessageAttachments  []ManagedChatPurgeMessage
}

// CompleteManagedChatPurge removes the immutable work set only after callers
// have verified both owner receipts. The hashes are stored with the terminal
// receipt in the same transaction as the payload deletion.
func (s *MessagesStore) CompleteManagedChatPurge(ctx context.Context, operationID uuid.UUID, fileReceiptSHA256, searchReceiptSHA256 []byte) (*ManagedChatPurgeWork, error) {
	if s == nil || s.Pool == nil || operationID == uuid.Nil || len(fileReceiptSHA256) != 32 || len(searchReceiptSHA256) != 32 {
		return nil, errors.New("managed chat purge completion evidence is invalid")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var chatID uuid.UUID
	var cutoff time.Time
	var requestHash, savedFileHash, savedSearchHash []byte
	var state string
	var completedAt *time.Time
	err = tx.QueryRow(ctx, `SELECT chat_id,purge_after,request_sha256,state,file_receipt_sha256,search_receipt_sha256,completed_at FROM managed_chat_purge_operations WHERE operation_id=$1 FOR UPDATE`, operationID).Scan(&chatID, &cutoff, &requestHash, &state, &savedFileHash, &savedSearchHash, &completedAt)
	if err != nil {
		return nil, err
	}
	if state == "COMPLETED" {
		if string(savedFileHash) != string(fileReceiptSHA256) || string(savedSearchHash) != string(searchReceiptSHA256) {
			return nil, ErrManagedChatPurgeOperationConflict
		}
		work, err := loadManagedChatPurgeWork(ctx, tx, operationID, chatID, cutoff, requestHash, state)
		if err != nil {
			return nil, err
		}
		work.FileReceiptSHA256, work.SearchReceiptSHA256, work.CompletedAt = savedFileHash, savedSearchHash, completedAt
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return work, nil
	}
	if state != "PENDING" {
		return nil, ErrManagedChatPurgeOperationConflict
	}
	if err := authorizeSpaceManagedPurge(ctx, tx, chatID, operationID); err != nil {
		return nil, err
	}
	// Keep every payload and side row in the same transaction as completion.
	// Explicit deletes precede messages so the Space mutation guards can still
	// resolve each message's chat; several side tables have no cascading FK.
	for _, table := range []string{"reactions", "message_hides", "pins", "game_message_revisions", "game_message_operation_receipts", "game_message_tombstone_actions"} {
		if _, err := tx.Exec(ctx, "DELETE FROM "+table+" WHERE message_id IN (SELECT message_id FROM managed_chat_purge_messages WHERE operation_id=$1)", operationID); err != nil {
			return nil, err
		}
	}
	for _, table := range []string{"read_positions", "read_receipts"} {
		condition := "last_read_message_id IN (SELECT message_id FROM managed_chat_purge_messages WHERE operation_id=$1)"
		if table == "read_receipts" {
			condition += " OR last_delivered_message_id IN (SELECT message_id FROM managed_chat_purge_messages WHERE operation_id=$1)"
		}
		if _, scoped := SpacePurgeScope(ctx); scoped {
			if _, err := tx.Exec(ctx, "DELETE FROM "+table+" WHERE chat_id=$1", chatID); err != nil {
				return nil, err
			}
			continue
		}
		if _, err := tx.Exec(ctx, "DELETE FROM "+table+" WHERE chat_id=$2 AND ("+condition+")", operationID, chatID); err != nil {
			return nil, err
		}
	}
	if _, err := tx.Exec(ctx, `DELETE FROM scheduled_messages WHERE chat_id=$1 AND created_at<=$2`, chatID, cutoff); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM messages WHERE id IN (SELECT message_id FROM managed_chat_purge_messages WHERE operation_id=$1)`, operationID); err != nil {
		return nil, err
	}
	var remaining int64
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM managed_chat_purge_messages p JOIN messages m ON m.id=p.message_id WHERE p.operation_id=$1`, operationID).Scan(&remaining); err != nil {
		return nil, err
	}
	if remaining != 0 {
		return nil, fmt.Errorf("managed chat purge left %d frozen message payloads", remaining)
	}
	if err := tx.QueryRow(ctx, `UPDATE managed_chat_purge_operations SET state='COMPLETED',file_receipt_sha256=$2,search_receipt_sha256=$3,completed_at=clock_timestamp() WHERE operation_id=$1 RETURNING completed_at`, operationID, fileReceiptSHA256, searchReceiptSHA256).Scan(&completedAt); err != nil {
		return nil, err
	}
	work, err := loadManagedChatPurgeWork(ctx, tx, operationID, chatID, cutoff, requestHash, "COMPLETED")
	if err != nil {
		return nil, err
	}
	work.FileReceiptSHA256, work.SearchReceiptSHA256, work.CompletedAt = append([]byte(nil), fileReceiptSHA256...), append([]byte(nil), searchReceiptSHA256...), completedAt
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return work, nil
}

// StartManagedChatPurge checks the frozen cutoff against the Messaging DB
// clock and atomically persists the exact rows and attachment IDs to process.
// Operation replay returns the stored set even if live rows have since changed.
func (s *MessagesStore) StartManagedChatPurge(ctx context.Context, operationID, chatID uuid.UUID, purgeAfter time.Time, requestSHA256 []byte) (*ManagedChatPurgeWork, error) {
	if s == nil || s.Pool == nil || operationID == uuid.Nil || chatID == uuid.Nil || purgeAfter.IsZero() || len(requestSHA256) != 32 {
		return nil, errors.New("managed chat purge has invalid identity or request hash")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var savedChat uuid.UUID
	var savedCutoff time.Time
	var savedHash []byte
	var savedFileHash, savedSearchHash []byte
	var completedAt *time.Time
	var state string
	err = tx.QueryRow(ctx, `SELECT chat_id,purge_after,request_sha256,state,file_receipt_sha256,search_receipt_sha256,completed_at FROM managed_chat_purge_operations WHERE operation_id=$1 FOR UPDATE`, operationID).Scan(&savedChat, &savedCutoff, &savedHash, &state, &savedFileHash, &savedSearchHash, &completedAt)
	if err == nil {
		// PostgreSQL represents timestamps in microseconds. The full signed
		// request hash still distinguishes every protobuf nanosecond.
		if savedChat != chatID || !savedCutoff.Equal(purgeAfter.Truncate(time.Microsecond)) || string(savedHash) != string(requestSHA256) {
			return nil, ErrManagedChatPurgeOperationConflict
		}
		work, err := loadManagedChatPurgeWork(ctx, tx, operationID, savedChat, savedCutoff, savedHash, state)
		if err != nil {
			return nil, err
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		work.FileReceiptSHA256 = append([]byte(nil), savedFileHash...)
		work.SearchReceiptSHA256 = append([]byte(nil), savedSearchHash...)
		work.CompletedAt = completedAt
		work.PurgeAfter = purgeAfter
		return work, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	// Evidence expiry cannot turn a deleted Space chat into a new empty success.
	// The permanent compact membership/fence survives the private child ledger.
	var purgedSpaceChat bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM messaging_space_chat_manifest_items i JOIN messaging_space_lifecycle_fences f ON f.space_id=i.space_id WHERE i.chat_id=$1 AND f.state='PURGED')`, chatID).Scan(&purgedSpaceChat); err != nil {
		return nil, err
	}
	if purgedSpaceChat {
		return nil, ErrManagedChatPurgeOperationConflict
	}
	var due bool
	if err := tx.QueryRow(ctx, `SELECT clock_timestamp() >= $1::timestamptz`, purgeAfter).Scan(&due); err != nil {
		return nil, err
	}
	if !due {
		return nil, ErrManagedChatPurgeNotDue
	}
	if _, err := tx.Exec(ctx, `INSERT INTO managed_chat_purge_operations(operation_id,chat_id,purge_after,request_sha256,state) VALUES($1,$2,$3,$4,'PENDING')`, operationID, chatID, purgeAfter.Truncate(time.Microsecond), requestSHA256); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT id,attachments::text FROM messages WHERE chat_id=$1 AND created_at <= $2 ORDER BY id FOR UPDATE`, chatID, purgeAfter)
	if err != nil {
		return nil, err
	}
	type snapshot struct {
		id          uuid.UUID
		attachments string
	}
	var snapshots []snapshot
	for rows.Next() {
		var item snapshot
		if err := rows.Scan(&item.id, &item.attachments); err != nil {
			rows.Close()
			return nil, err
		}
		snapshots = append(snapshots, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	var fileReferenceCount int64
	for _, item := range snapshots {
		message, err := managedChatPurgeMessage(item.id, item.attachments)
		if err != nil {
			return nil, err
		}
		fileReferenceCount += int64(len(message.FileIDs))
		if _, err := tx.Exec(ctx, `INSERT INTO managed_chat_purge_messages(operation_id,message_id,attachments) VALUES($1,$2,$3::jsonb)`, operationID, item.id, item.attachments); err != nil {
			return nil, err
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE managed_chat_purge_operations SET message_count=$2,file_reference_count=$3 WHERE operation_id=$1`, operationID, len(snapshots), fileReferenceCount); err != nil {
		return nil, err
	}
	work := &ManagedChatPurgeWork{OperationID: operationID, ChatID: chatID, PurgeAfter: purgeAfter, RequestSHA256: append([]byte(nil), requestSHA256...), State: "PENDING"}
	for _, item := range snapshots {
		message, err := managedChatPurgeMessage(item.id, item.attachments)
		if err != nil {
			return nil, err
		}
		work.MessageIDs = append(work.MessageIDs, item.id)
		work.MessageAttachments = append(work.MessageAttachments, message)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return work, nil
}

type managedChatPurgeRows interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func loadManagedChatPurgeWork(ctx context.Context, db managedChatPurgeRows, operationID, chatID uuid.UUID, cutoff time.Time, requestHash []byte, state string) (*ManagedChatPurgeWork, error) {
	rows, err := db.Query(ctx, `SELECT message_id,attachments::text FROM managed_chat_purge_messages WHERE operation_id=$1 ORDER BY message_id`, operationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	work := &ManagedChatPurgeWork{OperationID: operationID, ChatID: chatID, PurgeAfter: cutoff, RequestSHA256: append([]byte(nil), requestHash...), State: state}
	for rows.Next() {
		var id uuid.UUID
		var attachments string
		if err := rows.Scan(&id, &attachments); err != nil {
			return nil, err
		}
		message, err := managedChatPurgeMessage(id, attachments)
		if err != nil {
			return nil, err
		}
		work.MessageIDs = append(work.MessageIDs, id)
		work.MessageAttachments = append(work.MessageAttachments, message)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return work, nil
}

func managedChatPurgeMessage(id uuid.UUID, attachments string) (ManagedChatPurgeMessage, error) {
	var parsed []struct {
		FileID string `json:"file_id"`
	}
	if err := json.Unmarshal([]byte(attachments), &parsed); err != nil {
		return ManagedChatPurgeMessage{}, fmt.Errorf("decode message %s attachments for purge: %w", id, err)
	}
	message := ManagedChatPurgeMessage{ID: id, AttachmentsJSON: attachments}
	seen := make(map[uuid.UUID]struct{}, len(parsed))
	for _, attachment := range parsed {
		if attachment.FileID == "" {
			continue
		}
		fileID, err := uuid.Parse(attachment.FileID)
		if err != nil || fileID == uuid.Nil {
			return ManagedChatPurgeMessage{}, fmt.Errorf("message %s contains invalid attachment file_id", id)
		}
		if _, exists := seen[fileID]; !exists {
			seen[fileID] = struct{}{}
			message.FileIDs = append(message.FileIDs, fileID)
		}
	}
	return message, nil
}
