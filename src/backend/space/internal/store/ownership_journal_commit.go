package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	rolev1 "voice.app/voice/role/v1"
)

var (
	ErrOwnershipRoleReceiptInvalid = errors.New("invalid ownership role receipt")
	ErrOwnershipCommitAmbiguous    = errors.New("ownership commit outcome is ambiguous")
)

type OwnershipRolePreparedReceipt struct {
	ReceiptBytes []byte
	ReceiptHash  [32]byte
	IntentBytes  []byte
	IntentHash   [32]byte
}

type OwnershipPendingAudit struct {
	ID             uuid.UUID
	SpaceID        uuid.UUID
	ActorProfileID uuid.UUID
	Action         string
	TargetType     string
	TargetID       uuid.UUID
	DetailsJSON    string
}

func validateOwnershipRolePreparedReceipt(binding OwnershipBinding, receipt *rolev1.OwnershipTransferReceipt) (*OwnershipRolePreparedReceipt, error) {
	if receipt == nil || receipt.GetIntent() == nil || receipt.GetIntent().GetProtocolVersion() != 2 ||
		receipt.GetState() != rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_PREPARED ||
		receipt.GetCurrentOwnerProfileId() != "" {
		return nil, ErrOwnershipRoleReceiptInvalid
	}
	intent := receipt.GetIntent()
	spaceID, err := uuid.Parse(intent.GetSpaceId())
	if err != nil {
		return nil, ErrOwnershipRoleReceiptInvalid
	}
	oldOwnerID, err := uuid.Parse(intent.GetOldOwnerProfileId())
	if err != nil {
		return nil, ErrOwnershipRoleReceiptInvalid
	}
	newOwnerID, err := uuid.Parse(intent.GetNewOwnerProfileId())
	if err != nil {
		return nil, ErrOwnershipRoleReceiptInvalid
	}
	operationID, err := uuid.Parse(intent.GetOperationId())
	if err != nil {
		return nil, ErrOwnershipRoleReceiptInvalid
	}
	if spaceID == uuid.Nil || oldOwnerID == uuid.Nil || newOwnerID == uuid.Nil || operationID == uuid.Nil {
		return nil, ErrOwnershipRoleReceiptInvalid
	}
	if spaceID != binding.SpaceID || oldOwnerID != binding.ActorProfileID ||
		newOwnerID != binding.NewOwnerProfileID || operationID != binding.OperationID {
		return nil, ErrOwnershipConflict
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
	return &OwnershipRolePreparedReceipt{
		ReceiptBytes: receiptBytes,
		ReceiptHash:  sha256.Sum256(receiptBytes),
		IntentBytes:  intentBytes,
		IntentHash:   sha256.Sum256(intentBytes),
	}, nil
}

func ownershipRolePreparedReceiptsEqual(left, right *OwnershipRolePreparedReceipt) bool {
	return left != nil && right != nil &&
		bytes.Equal(left.ReceiptBytes, right.ReceiptBytes) && left.ReceiptHash == right.ReceiptHash &&
		bytes.Equal(left.IntentBytes, right.IntentBytes) && left.IntentHash == right.IntentHash
}

func decodeOwnershipRolePreparedReceipt(binding OwnershipBinding, receiptBytes []byte) (*OwnershipRolePreparedReceipt, error) {
	var receipt rolev1.OwnershipTransferReceipt
	if err := proto.Unmarshal(receiptBytes, &receipt); err != nil {
		return nil, ErrOwnershipRoleReceiptInvalid
	}
	return validateOwnershipRolePreparedReceipt(binding, &receipt)
}

func (s *SpaceStore) MarkOwnershipPrepared(ctx context.Context, binding OwnershipBinding, receipt *rolev1.OwnershipTransferReceipt) (*OwnershipJournal, error) {
	encoded, _, err := EncodeOwnershipBinding(binding)
	if err != nil {
		return nil, err
	}
	evidence, err := validateOwnershipRolePreparedReceipt(binding, receipt)
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
	switch existing.State {
	case "prepared":
		if !ownershipRolePreparedReceiptsEqual(existing.RoleReceipt, evidence) {
			return nil, ErrOwnershipConflict
		}
		return existing, nil
	case "proof_confirmed":
		if existing.AuthReceipt == nil || existing.RoleReceipt != nil {
			return nil, ErrOwnershipStateTransition
		}
		prepared, err := scanOwnershipJournalCommit(tx.QueryRow(ctx, `UPDATE ownership_journal SET
			state='prepared',role_receipt_bytes=$2,role_receipt_hash=$3,
			role_intent_bytes=$4,role_intent_hash=$5,updated_at=now()
			WHERE operation_id=$1 AND state='proof_confirmed' RETURNING `+ownershipJournalCommitColumns,
			binding.OperationID, evidence.ReceiptBytes, evidence.ReceiptHash[:], evidence.IntentBytes, evidence.IntentHash[:]))
		if errors.Is(err, ErrOwnershipMissing) {
			return nil, ErrOwnershipStateTransition
		}
		if err != nil {
			return nil, err
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return prepared, nil
	default:
		return nil, ErrOwnershipStateTransition
	}
}

func (s *SpaceStore) DecideOwnershipCommit(ctx context.Context, binding OwnershipBinding) (*OwnershipJournal, error) {
	encoded, _, err := EncodeOwnershipBinding(binding)
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
	if existing.State == "commit_decided" {
		return existing, nil
	}
	if existing.State != "prepared" || existing.AuthReceipt == nil || existing.RoleReceipt == nil || existing.PendingAudit != nil {
		return nil, ErrOwnershipStateTransition
	}
	var owner uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT owner_profile_id FROM spaces WHERE id=$1 FOR UPDATE`, binding.SpaceID).Scan(&owner); err != nil {
		return nil, err
	}
	if owner != binding.ActorProfileID {
		return nil, ErrNotSpaceOwner
	}
	var member uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT profile_id FROM space_members WHERE space_id=$1 AND profile_id=$2 FOR KEY SHARE`, binding.SpaceID, binding.NewOwnerProfileID).Scan(&member); errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrMemberNotFound
	} else if err != nil {
		return nil, err
	}
	command, err := tx.Exec(ctx, `UPDATE spaces SET owner_profile_id=$2,updated_at=now() WHERE id=$1 AND owner_profile_id=$3`, binding.SpaceID, binding.NewOwnerProfileID, binding.ActorProfileID)
	if err != nil {
		return nil, err
	}
	if command.RowsAffected() != 1 {
		return nil, ErrNotSpaceOwner
	}
	committed, err := scanOwnershipJournalCommit(tx.QueryRow(ctx, `UPDATE ownership_journal SET
		state='commit_decided',pending_audit_action='ownership_transferred',
		pending_audit_target_type='profile',pending_audit_target_id=new_owner_profile_id,
		pending_audit_details='{}'::jsonb,updated_at=now()
		WHERE operation_id=$1 AND state='prepared' RETURNING `+ownershipJournalCommitColumns, binding.OperationID))
	if errors.Is(err, ErrOwnershipMissing) {
		return nil, ErrOwnershipStateTransition
	}
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO ownership_outbox
		(event_id,operation_id,space_id,previous_owner_profile_id,new_owner_profile_id,event_type,ready)
		VALUES($1,$2,$3,$4,$5,'space.updated',false)`, existing.EventID, binding.OperationID,
		binding.SpaceID, binding.ActorProfileID, binding.NewOwnerProfileID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrOwnershipCommitAmbiguous, err)
	}
	return committed, nil
}
