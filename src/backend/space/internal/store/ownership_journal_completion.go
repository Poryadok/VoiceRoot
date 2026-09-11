package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	rolev1 "voice.app/voice/role/v1"
)

type ownershipTerminalReceiptEvidence struct {
	bytes       []byte
	hash        [32]byte
	intentBytes []byte
}

func validateOwnershipTerminalReceipt(binding OwnershipBinding, receipt *rolev1.OwnershipTransferReceipt, wantState rolev1.OwnershipTransferState, wantOwner uuid.UUID) (*ownershipTerminalReceiptEvidence, error) {
	if receipt == nil || receipt.GetIntent() == nil || receipt.GetIntent().GetProtocolVersion() != 2 {
		return nil, ErrOwnershipRoleReceiptInvalid
	}
	intent := receipt.GetIntent()
	spaceID, err := uuid.Parse(intent.GetSpaceId())
	if err != nil || spaceID == uuid.Nil {
		return nil, ErrOwnershipRoleReceiptInvalid
	}
	oldOwnerID, err := uuid.Parse(intent.GetOldOwnerProfileId())
	if err != nil || oldOwnerID == uuid.Nil {
		return nil, ErrOwnershipRoleReceiptInvalid
	}
	newOwnerID, err := uuid.Parse(intent.GetNewOwnerProfileId())
	if err != nil || newOwnerID == uuid.Nil {
		return nil, ErrOwnershipRoleReceiptInvalid
	}
	operationID, err := uuid.Parse(intent.GetOperationId())
	if err != nil || operationID == uuid.Nil {
		return nil, ErrOwnershipRoleReceiptInvalid
	}
	if spaceID != binding.SpaceID || oldOwnerID != binding.ActorProfileID || newOwnerID != binding.NewOwnerProfileID || operationID != binding.OperationID {
		return nil, ErrOwnershipConflict
	}
	if receipt.GetState() != wantState {
		return nil, ErrOwnershipRoleReceiptInvalid
	}
	currentOwnerID, err := uuid.Parse(receipt.GetCurrentOwnerProfileId())
	if err != nil || currentOwnerID == uuid.Nil || currentOwnerID != wantOwner {
		return nil, ErrOwnershipRoleReceiptInvalid
	}
	options := proto.MarshalOptions{Deterministic: true}
	intentBytes, err := options.Marshal(intent)
	if err != nil {
		return nil, ErrOwnershipRoleReceiptInvalid
	}
	receiptBytes, err := options.Marshal(receipt)
	if err != nil {
		return nil, ErrOwnershipRoleReceiptInvalid
	}
	return &ownershipTerminalReceiptEvidence{bytes: receiptBytes, hash: sha256.Sum256(receiptBytes), intentBytes: intentBytes}, nil
}

func canonicalOwnershipIntentBytes(binding OwnershipBinding) ([]byte, error) {
	return proto.MarshalOptions{Deterministic: true}.Marshal(&rolev1.OwnershipTransferIntent{
		ProtocolVersion:   binding.ProtocolVersion,
		SpaceId:           binding.SpaceID.String(),
		OldOwnerProfileId: binding.ActorProfileID.String(),
		NewOwnerProfileId: binding.NewOwnerProfileID.String(),
		OperationId:       binding.OperationID.String(),
	})
}

func (s *SpaceStore) CompleteOwnershipCommit(ctx context.Context, binding OwnershipBinding, receipt *rolev1.OwnershipTransferReceipt) (*OwnershipJournal, error) {
	return s.completeOwnership(ctx, binding, receipt, true)
}

func (s *SpaceStore) CompleteOwnershipAbort(ctx context.Context, binding OwnershipBinding, receipt *rolev1.OwnershipTransferReceipt) (*OwnershipJournal, error) {
	return s.completeOwnership(ctx, binding, receipt, false)
}

func (s *SpaceStore) completeOwnership(ctx context.Context, binding OwnershipBinding, receipt *rolev1.OwnershipTransferReceipt, commit bool) (*OwnershipJournal, error) {
	encoded, _, err := EncodeOwnershipBinding(binding)
	if err != nil {
		return nil, err
	}
	wantRoleState := rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_ABORTED
	wantOwner := binding.ActorProfileID
	wantDecision, wantTerminal := "abort_decided", "aborted"
	if commit {
		wantRoleState = rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_FINALIZED
		wantOwner = binding.NewOwnerProfileID
		wantDecision, wantTerminal = "commit_decided", "completed"
	}
	evidence, err := validateOwnershipTerminalReceipt(binding, receipt, wantRoleState, wantOwner)
	if err != nil {
		return nil, err
	}
	if s == nil || s.Pool == nil {
		return nil, errors.New("space store: pool not configured")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		cleanupCtx, cancel := BoundedCleanupContext(ctx)
		defer cancel()
		_ = tx.Rollback(cleanupCtx)
	}()
	if err := lockOwnershipTransaction(ctx, tx, binding.OperationID, binding.SpaceID); err != nil {
		return nil, err
	}
	existing, err := loadOwnershipJournal(ctx, tx, binding.OperationID)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(existing.BindingBytes, encoded) {
		return nil, ErrOwnershipConflict
	}
	if existing.State == wantTerminal {
		storedBytes, storedHash, ok := existing.OwnershipTerminalReceiptEvidence()
		if !ok || !bytes.Equal(storedBytes, evidence.bytes) || storedHash != evidence.hash {
			return nil, ErrOwnershipConflict
		}
		return existing, nil
	}
	if existing.State != wantDecision {
		return nil, ErrOwnershipStateTransition
	}
	if existing.RoleReceipt != nil {
		if !bytes.Equal(existing.RoleReceipt.IntentBytes, evidence.intentBytes) {
			return nil, ErrOwnershipConflict
		}
	} else {
		if commit {
			return nil, ErrOwnershipStateTransition
		}
		canonicalIntent, marshalErr := canonicalOwnershipIntentBytes(binding)
		if marshalErr != nil || !bytes.Equal(canonicalIntent, evidence.intentBytes) {
			return nil, ErrOwnershipConflict
		}
	}

	var owner uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT owner_profile_id FROM spaces WHERE id=$1 FOR UPDATE`, binding.SpaceID).Scan(&owner); err != nil {
		return nil, err
	}
	if owner != wantOwner {
		return nil, ErrNotSpaceOwner
	}

	if commit {
		var ready bool
		if err := tx.QueryRow(ctx, `SELECT ready FROM ownership_outbox
			WHERE event_id=$1 AND operation_id=$2 AND space_id=$3
			AND previous_owner_profile_id=$4 AND new_owner_profile_id=$5 AND event_type='space.updated'
			FOR UPDATE`, existing.EventID, binding.OperationID, binding.SpaceID, binding.ActorProfileID, binding.NewOwnerProfileID).Scan(&ready); err != nil {
			return nil, err
		}
		if ready || existing.PendingAudit == nil {
			return nil, ErrOwnershipStateTransition
		}
		_, err = tx.Exec(ctx, `INSERT INTO audit_log(id,space_id,actor_profile_id,action,target_type,target_id,details)
			VALUES($1,$2,$3,$4,$5,$6,$7::jsonb)`, existing.AuditID, binding.SpaceID, binding.ActorProfileID,
			existing.PendingAudit.Action, existing.PendingAudit.TargetType, existing.PendingAudit.TargetID, existing.PendingAudit.DetailsJSON)
		if err != nil {
			return nil, err
		}
		command, updateErr := tx.Exec(ctx, `UPDATE ownership_outbox SET ready=true WHERE event_id=$1 AND operation_id=$2 AND NOT ready`, existing.EventID, binding.OperationID)
		if updateErr != nil {
			return nil, updateErr
		}
		if command.RowsAffected() != 1 {
			return nil, ErrOwnershipStateTransition
		}
	}

	completed, err := scanOwnershipJournalTerminal(tx.QueryRow(ctx, `UPDATE ownership_journal SET
		state=$2,role_terminal_receipt_bytes=$3,role_terminal_receipt_hash=$4,updated_at=now()
		WHERE operation_id=$1 AND state=$5 RETURNING `+ownershipJournalTerminalColumns,
		binding.OperationID, wantTerminal, evidence.bytes, evidence.hash[:], wantDecision))
	if errors.Is(err, ErrOwnershipMissing) {
		return nil, ErrOwnershipStateTransition
	}
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		recovered, loadErr := s.LoadOwnership(context.WithoutCancel(ctx), binding.OperationID)
		if loadErr == nil && recovered.State == wantTerminal {
			storedBytes, storedHash, ok := recovered.OwnershipTerminalReceiptEvidence()
			if ok && bytes.Equal(storedBytes, evidence.bytes) && storedHash == evidence.hash {
				return recovered, nil
			}
		}
		return nil, fmt.Errorf("ownership terminal commit outcome is ambiguous: %w", err)
	}
	return completed, nil
}
