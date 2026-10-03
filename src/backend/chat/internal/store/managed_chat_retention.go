package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type ManagedChatRetention struct {
	ApplicationID uuid.UUID
	EnvironmentID uuid.UUID
	OperationID   uuid.UUID
	ExternalKey   string
	RequestHash   string
	PurgeAfter    time.Time
}

type ManagedChatRetentionResult struct {
	ChatID      uuid.UUID
	ReceiptID   uuid.UUID
	RequestHash string
	PurgeAfter  time.Time
	CompletedAt time.Time
	Replayed    bool
}

type managedChatRetentionReceipt struct {
	ChatID      string    `json:"chat_id"`
	ReceiptID   string    `json:"receipt_id"`
	RequestHash string    `json:"request_hash"`
	PurgeAfter  time.Time `json:"purge_after"`
	CompletedAt time.Time `json:"completed_at"`
}

// SetManagedChatRetention installs an immutable absolute cutoff for a GIS
// managed chat. The request and receipt share managed_chat_operations so an
// exact retry remains stable across process restarts.
func (s *DMStore) SetManagedChatRetention(ctx context.Context, request ManagedChatRetention) (ManagedChatRetentionResult, error) {
	if s == nil || s.Pool == nil {
		return ManagedChatRetentionResult{}, errors.New("dm store: pool not configured")
	}
	request.ExternalKey = strings.TrimSpace(request.ExternalKey)
	request.PurgeAfter = request.PurgeAfter.UTC()
	if request.ApplicationID == uuid.Nil || request.EnvironmentID == uuid.Nil || request.OperationID == uuid.Nil ||
		request.ExternalKey == "" || len(request.ExternalKey) > 512 || request.PurgeAfter.IsZero() ||
		!managedRequestHashPattern.MatchString(request.RequestHash) {
		return ManagedChatRetentionResult{}, errors.New("invalid managed chat retention request")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return ManagedChatRetentionResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockManagedOperation(ctx, tx, request.ApplicationID, request.EnvironmentID, request.OperationID); err != nil {
		return ManagedChatRetentionResult{}, err
	}
	if saved, exists, err := readManagedOperation(ctx, tx, request.ApplicationID, request.EnvironmentID, request.OperationID, "retention", request.RequestHash); err != nil {
		return ManagedChatRetentionResult{}, err
	} else if exists {
		var receipt managedChatRetentionReceipt
		if err := json.Unmarshal(saved.Bytes, &receipt); err != nil {
			return ManagedChatRetentionResult{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return ManagedChatRetentionResult{}, err
		}
		return ManagedChatRetentionResult{ChatID: saved.ChatID, ReceiptID: request.OperationID,
			RequestHash: saved.RequestHash, PurgeAfter: receipt.PurgeAfter.UTC(),
			CompletedAt: receipt.CompletedAt.UTC(), Replayed: true}, nil
	}
	resourceLock := request.ApplicationID.String() + "/" + request.EnvironmentID.String() + "/" + request.ExternalKey
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 31421))`, resourceLock); err != nil {
		return ManagedChatRetentionResult{}, err
	}
	chat, err := managedChatByExternalKey(ctx, tx, request.ApplicationID, request.EnvironmentID, request.ExternalKey)
	if errors.Is(err, ErrManagedChatNotFound) || chat == nil {
		return ManagedChatRetentionResult{}, ErrManagedChatNotFound
	}
	if err != nil {
		return ManagedChatRetentionResult{}, err
	}
	var savedCutoff time.Time
	err = tx.QueryRow(ctx, `SELECT purge_after FROM managed_chat_retention WHERE chat_id=$1 FOR UPDATE`, chat.ID).Scan(&savedCutoff)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return ManagedChatRetentionResult{}, err
	}
	if err == nil && !savedCutoff.Equal(request.PurgeAfter) {
		return ManagedChatRetentionResult{}, ErrManagedResourceConflict
	}
	if errors.Is(err, pgx.ErrNoRows) {
		if _, err := tx.Exec(ctx, `INSERT INTO managed_chat_retention(chat_id,application_id,environment_id,purge_after) VALUES($1,$2,$3,$4)`,
			chat.ID, request.ApplicationID, request.EnvironmentID, request.PurgeAfter); err != nil {
			return ManagedChatRetentionResult{}, err
		}
	}
	result := ManagedChatRetentionResult{ChatID: chat.ID, ReceiptID: request.OperationID,
		RequestHash: request.RequestHash, PurgeAfter: request.PurgeAfter, CompletedAt: time.Now().UTC()}
	encoded, err := json.Marshal(managedChatRetentionReceipt{ChatID: result.ChatID.String(), ReceiptID: result.ReceiptID.String(),
		RequestHash: result.RequestHash, PurgeAfter: result.PurgeAfter, CompletedAt: result.CompletedAt})
	if err != nil {
		return ManagedChatRetentionResult{}, err
	}
	if err := saveManagedOperation(ctx, tx, request.ApplicationID, request.EnvironmentID, request.OperationID,
		"retention", request.RequestHash, chat.ID, encoded); err != nil {
		return ManagedChatRetentionResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ManagedChatRetentionResult{}, err
	}
	return result, nil
}
