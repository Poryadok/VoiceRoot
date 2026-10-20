package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrDeletionClaimLost = errors.New("Bot deletion worker claim unavailable")
var ErrDeletionPending = errors.New("Bot deletion participant effects remain pending")

type BotDeletionEffect struct {
	EffectID                   uuid.UUID
	Target                     BotDeletionTarget
	Kind                       string
	ChatID                     *uuid.UUID
	RequestBytes, ReceiptBytes []byte
	State                      string
}

// Claims expire without losing any frozen request or received participant
// outcome. Each new claim monotonically fences every previous worker.
func (s *BotStore) ClaimBotDeletion(ctx context.Context, operation uuid.UUID) (*BotDeletion, error) {
	d, err := scanBotDeletion(s.Pool.QueryRow(ctx, `UPDATE bot_deletion_operations SET state='RUNNING',claim_generation=claim_generation+1,attempts=attempts+1,claimed_until=clock_timestamp()+interval '30 seconds' WHERE operation_id=$1 AND state<>'COMPLETE' AND (claimed_until IS NULL OR claimed_until<=clock_timestamp()) AND claim_generation<9223372036854775807 AND attempts<9223372036854775807 RETURNING `+deletionColumns, operation))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrDeletionClaimLost
	}
	return d, err
}

// ClaimNextBotDeletion atomically takes one due root without waiting for a peer
// worker's row lock. Expired claims retain the original child bytes and receipts.
func (s *BotStore) ClaimNextBotDeletion(ctx context.Context) (*BotDeletion, error) {
	d, err := scanBotDeletion(s.Pool.QueryRow(ctx, `UPDATE bot_deletion_operations SET state='RUNNING',claim_generation=claim_generation+1,attempts=attempts+1,claimed_until=clock_timestamp()+interval '30 seconds' WHERE operation_id=(SELECT operation_id FROM bot_deletion_operations WHERE state<>'COMPLETE' AND next_attempt_at<=clock_timestamp() AND (claimed_until IS NULL OR claimed_until<=clock_timestamp()) AND claim_generation<9223372036854775807 AND attempts<9223372036854775807 ORDER BY next_attempt_at,operation_id FOR UPDATE SKIP LOCKED LIMIT 1) RETURNING `+deletionColumns))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrDeletionClaimLost
	}
	return d, err
}

func (s *BotStore) RetryBotDeletion(ctx context.Context, claim *BotDeletion) error {
	tx, err := s.deletionClaimTx(ctx, claim)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	result, err := tx.Exec(ctx, lifecycleQuery(claim, `UPDATE bot_deletion_operations SET state='RETRY_WAIT',claimed_until=NULL,next_attempt_at=clock_timestamp()+interval '2 seconds',last_error='participant cleanup pending' WHERE operation_id=$1 AND claim_generation=$2 AND claimed_until>clock_timestamp()`), claim.OperationID, claim.ClaimGeneration)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return ErrDeletionClaimLost
	}
	return tx.Commit(ctx)
}

func (s *BotStore) BotDeletionEffects(ctx context.Context, operation uuid.UUID) ([]BotDeletionEffect, error) {
	return s.lifecycleEffects(ctx, nil, operation)
}

func (s *BotStore) LifecycleEffects(ctx context.Context, claim *BotDeletion) ([]BotDeletionEffect, error) {
	return s.lifecycleEffects(ctx, claim, claim.OperationID)
}

func (s *BotStore) lifecycleEffects(ctx context.Context, claim *BotDeletion, operation uuid.UUID) ([]BotDeletionEffect, error) {
	rows, err := s.Pool.Query(ctx, lifecycleQuery(claim, `SELECT e.effect_id,t.scope_bytes,t.scope_sha256,e.kind,e.chat_id,e.request_bytes,e.request_sha256,e.receipt_bytes,e.receipt_sha256,e.state FROM bot_deletion_installations t JOIN bot_deletion_effects e ON e.target_id=t.target_id WHERE t.operation_id=$1 AND t.disposition='ACTIVE' ORDER BY t.space_id,e.kind,e.chat_id NULLS FIRST`), operation)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []BotDeletionEffect{}
	for rows.Next() {
		var effect BotDeletionEffect
		var scope, digest, requestDigest, receiptDigest []byte
		if err = rows.Scan(&effect.EffectID, &scope, &digest, &effect.Kind, &effect.ChatID, &effect.RequestBytes, &requestDigest, &effect.ReceiptBytes, &receiptDigest, &effect.State); err != nil {
			return nil, err
		}
		for _, retained := range []struct{ body, digest []byte }{{effect.RequestBytes, requestDigest}, {effect.ReceiptBytes, receiptDigest}} {
			if retained.body == nil && retained.digest == nil {
				continue
			}
			sum := sha256.Sum256(retained.body)
			if len(retained.body) == 0 || !bytes.Equal(sum[:], retained.digest) {
				return nil, ErrInstallationConflict
			}
		}
		if (effect.State == "COMPLETE") != (effect.ReceiptBytes != nil) || (effect.ReceiptBytes != nil && effect.RequestBytes == nil) {
			return nil, ErrInstallationConflict
		}
		actual := sha256.Sum256(scope)
		if !bytes.Equal(actual[:], digest) {
			return nil, ErrInstallationConflict
		}
		if err = json.Unmarshal(scope, &effect.Target); err != nil {
			return nil, err
		}
		if effect.Target.OperationID != operation {
			return nil, ErrInstallationConflict
		}
		effect.Target.ScopeBytes = scope
		effect.Target.ScopeSHA256 = digest
		out = append(out, effect)
	}
	return out, rows.Err()
}

func (s *BotStore) deletionClaimTx(ctx context.Context, claim *BotDeletion) (pgx.Tx, error) {
	if claim == nil || claim.OperationID == uuid.Nil || claim.ClaimGeneration <= 0 {
		return nil, ErrDeletionClaimLost
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	if err = s.lockLifecycleClaim(ctx, tx, claim); err != nil {
		_ = tx.Rollback(context.Background())
		return nil, err
	}
	return tx, nil
}

func (s *BotStore) lockLifecycleClaim(ctx context.Context, tx pgx.Tx, claim *BotDeletion) error {
	var generation int64
	var state string
	err := tx.QueryRow(ctx, lifecycleQuery(claim, `SELECT claim_generation,state FROM bot_deletion_operations WHERE operation_id=$1 FOR UPDATE`), claim.OperationID).Scan(&generation, &state)
	if err != nil || generation != claim.ClaimGeneration || state != "RUNNING" {
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		return ErrDeletionClaimLost
	}
	// Evaluate expiry after acquiring the row lock, including any lock wait.
	var current bool
	err = tx.QueryRow(ctx, lifecycleQuery(claim, `SELECT claimed_until>clock_timestamp() FROM bot_deletion_operations WHERE operation_id=$1`), claim.OperationID).Scan(&current)
	if err != nil || !current {
		if err != nil {
			return err
		}
		return ErrDeletionClaimLost
	}
	return nil
}

func (s *BotStore) SealBotDeletionRequest(ctx context.Context, claim *BotDeletion, effect uuid.UUID, request []byte) error {
	if len(request) == 0 || len(request) > 1024*1024 {
		return ErrInstallationConflict
	}
	tx, err := s.deletionClaimTx(ctx, claim)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var saved []byte
	err = tx.QueryRow(ctx, lifecycleQuery(claim, `SELECT e.request_bytes FROM bot_deletion_effects e JOIN bot_deletion_installations t ON t.target_id=e.target_id WHERE e.effect_id=$1 AND t.operation_id=$2 FOR UPDATE OF e`), effect, claim.OperationID).Scan(&saved)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrInstallationConflict
	}
	if err != nil {
		return err
	}
	if saved != nil {
		if !bytes.Equal(saved, request) {
			return ErrInstallationConflict
		}
		return tx.Commit(ctx)
	}
	digest := sha256.Sum256(request)
	_, err = tx.Exec(ctx, lifecycleQuery(claim, `UPDATE bot_deletion_effects SET request_bytes=$2,request_sha256=$3 WHERE effect_id=$1`), effect, request, digest[:])
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// The caller validates the exact typed participant receipt before this durable
// CAS. Persistence alone never establishes that a remote effect completed.
func (s *BotStore) SaveBotDeletionReceipt(ctx context.Context, claim *BotDeletion, effect uuid.UUID, request, receipt []byte) error {
	if len(receipt) == 0 || len(receipt) > 1024*1024 {
		return ErrInstallationConflict
	}
	tx, err := s.deletionClaimTx(ctx, claim)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var savedRequest, savedReceipt []byte
	err = tx.QueryRow(ctx, lifecycleQuery(claim, `SELECT e.request_bytes,e.receipt_bytes FROM bot_deletion_effects e JOIN bot_deletion_installations t ON t.target_id=e.target_id WHERE e.effect_id=$1 AND t.operation_id=$2 FOR UPDATE OF e`), effect, claim.OperationID).Scan(&savedRequest, &savedReceipt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrInstallationConflict
	}
	if err != nil {
		return err
	}
	if savedRequest == nil || !bytes.Equal(savedRequest, request) {
		return ErrInstallationConflict
	}
	if savedReceipt != nil {
		if !bytes.Equal(savedReceipt, receipt) {
			return ErrInstallationConflict
		}
		return tx.Commit(ctx)
	}
	digest := sha256.Sum256(receipt)
	_, err = tx.Exec(ctx, lifecycleQuery(claim, `UPDATE bot_deletion_effects SET receipt_bytes=$2,receipt_sha256=$3,state='COMPLETE' WHERE effect_id=$1`), effect, receipt, digest[:])
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *BotStore) CompleteBotDeletion(ctx context.Context, claim *BotDeletion) error {
	if claim != nil && claim.Purpose == "space_uninstall" {
		return s.completeUninstallCleanup(ctx, claim)
	}
	if claim == nil || claim.OperationID == uuid.Nil || claim.ClaimGeneration <= 0 {
		return ErrDeletionClaimLost
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	// Serialize with Space->Bot purge and owner admission using the actual root's
	// Bot before locking the root. No Space lock is acquired below this lock.
	var retiredUnresolved bool
	var botStatus string
	err = tx.QueryRow(ctx, `SELECT b.retired_cleanup_unresolved,b.status FROM bots b JOIN bot_deletion_operations o ON o.bot_id=b.id WHERE o.operation_id=$1 FOR UPDATE OF b`, claim.OperationID).Scan(&retiredUnresolved, &botStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrDeletionClaimLost
	}
	if err != nil {
		return err
	}
	if err = s.lockLifecycleClaim(ctx, tx, claim); err != nil {
		return err
	}
	if botStatus != "disabled" {
		return ErrInstallationConflict
	}
	if retiredUnresolved {
		return ErrDeletionPending
	}
	accounted, _, err := lifecycleAccountingTx(ctx, tx, claim)
	if err != nil {
		return err
	}
	if !accounted {
		return ErrDeletionPending
	}
	var terminals int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM bot_deletion_terminal_effects WHERE operation_id=$1 AND state='COMPLETE'`, claim.OperationID).Scan(&terminals); err != nil {
		return err
	}
	if terminals != 2 {
		return ErrDeletionPending
	}
	var pending bool
	err = tx.QueryRow(ctx, lifecycleQuery(claim, `SELECT EXISTS(SELECT 1 FROM bot_deletion_effects e JOIN bot_deletion_installations t ON t.target_id=e.target_id WHERE t.operation_id=$1 AND e.state<>'COMPLETE')`), claim.OperationID).Scan(&pending)
	if err != nil {
		return err
	}
	if pending {
		return ErrDeletionPending
	}
	result, err := tx.Exec(ctx, lifecycleQuery(claim, `UPDATE bot_deletion_operations SET state='COMPLETE',completed_at=clock_timestamp(),claimed_until=NULL,last_error='' WHERE operation_id=$1 AND claim_generation=$2 AND claimed_until>clock_timestamp()`), claim.OperationID, claim.ClaimGeneration)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return ErrDeletionClaimLost
	}
	return tx.Commit(ctx)
}
