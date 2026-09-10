package store

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"time"

	"github.com/google/uuid"
)

var (
	ErrOwnershipReceiptInvalid  = errors.New("invalid ownership auth receipt")
	ErrOwnershipStateTransition = errors.New("invalid ownership journal state transition")
)

// OwnershipAuthReceipt is the exact committed receipt returned by Auth.
type OwnershipAuthReceipt struct {
	ReceiptID         uuid.UUID
	AccountID         uuid.UUID
	ProfileID         uuid.UUID
	SpaceID           uuid.UUID
	NewOwnerProfileID uuid.UUID
	OperationID       uuid.UUID
	SessionEpoch      int64
	ConsumedAt        time.Time
	VerifiedFactors   []string
}

func validateOwnershipAuthReceiptShape(receipt OwnershipAuthReceipt) error {
	_, offset := receipt.ConsumedAt.Zone()
	if receipt.ReceiptID == uuid.Nil || receipt.AccountID == uuid.Nil || receipt.ProfileID == uuid.Nil ||
		receipt.SpaceID == uuid.Nil || receipt.NewOwnerProfileID == uuid.Nil || receipt.OperationID == uuid.Nil ||
		receipt.SessionEpoch <= 0 || receipt.ConsumedAt.IsZero() || offset != 0 || receipt.ConsumedAt.Nanosecond()%1000 != 0 {
		return ErrOwnershipReceiptInvalid
	}
	return nil
}

func ownershipAuthReceiptMatchesBinding(receipt OwnershipAuthReceipt, binding OwnershipBinding) bool {
	return receipt.AccountID == binding.AccountID &&
		receipt.ProfileID == binding.ActorProfileID &&
		receipt.SpaceID == binding.SpaceID &&
		receipt.NewOwnerProfileID == binding.NewOwnerProfileID &&
		receipt.OperationID == binding.OperationID &&
		receipt.SessionEpoch == binding.SessionEpoch
}

func ownershipAuthReceiptsEqual(left, right OwnershipAuthReceipt) bool {
	return left.ReceiptID == right.ReceiptID &&
		left.AccountID == right.AccountID &&
		left.ProfileID == right.ProfileID &&
		left.SpaceID == right.SpaceID &&
		left.NewOwnerProfileID == right.NewOwnerProfileID &&
		left.OperationID == right.OperationID &&
		left.SessionEpoch == right.SessionEpoch &&
		left.ConsumedAt.Equal(right.ConsumedAt) &&
		slices.Equal(left.VerifiedFactors, right.VerifiedFactors)
}

func (s *SpaceStore) ConfirmOwnershipProof(ctx context.Context, binding OwnershipBinding, receipt OwnershipAuthReceipt) (*OwnershipJournal, error) {
	encoded, _, err := EncodeOwnershipBinding(binding)
	if err != nil {
		return nil, err
	}
	if err := validateOwnershipAuthReceiptShape(receipt); err != nil {
		return nil, err
	}
	if !ownershipAuthReceiptMatchesBinding(receipt, binding) {
		return nil, ErrOwnershipConflict
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
	existing, err := scanOwnershipJournal(tx.QueryRow(ctx,
		`SELECT `+ownershipJournalColumns+` FROM ownership_journal WHERE operation_id=$1`, binding.OperationID))
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(existing.BindingBytes, encoded) {
		return nil, ErrOwnershipConflict
	}
	switch existing.State {
	case "proof_confirmed":
		if existing.AuthReceipt == nil || !ownershipAuthReceiptsEqual(*existing.AuthReceipt, receipt) {
			return nil, ErrOwnershipConflict
		}
		return existing, nil
	case "reserved":
		confirmed, err := scanOwnershipJournal(tx.QueryRow(ctx, `UPDATE ownership_journal SET
			state='proof_confirmed',auth_receipt_id=$2,auth_account_id=$3,auth_profile_id=$4,
			auth_space_id=$5,auth_new_owner_profile_id=$6,auth_operation_id=$7,
			auth_session_epoch=$8,auth_consumed_at=$9,auth_verified_factors=$10,updated_at=now()
			WHERE operation_id=$1 AND state='reserved' RETURNING `+ownershipJournalColumns,
			binding.OperationID, receipt.ReceiptID, receipt.AccountID, receipt.ProfileID,
			receipt.SpaceID, receipt.NewOwnerProfileID, receipt.OperationID, receipt.SessionEpoch,
			receipt.ConsumedAt.UTC(), receipt.VerifiedFactors))
		if errors.Is(err, ErrOwnershipMissing) {
			return nil, ErrOwnershipStateTransition
		}
		if err != nil {
			return nil, err
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return confirmed, nil
	default:
		return nil, ErrOwnershipStateTransition
	}
}

func (s *SpaceStore) DecideOwnershipAbort(ctx context.Context, binding OwnershipBinding) (*OwnershipJournal, error) {
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
	existing, err := scanOwnershipJournal(tx.QueryRow(ctx,
		`SELECT `+ownershipJournalColumns+` FROM ownership_journal WHERE operation_id=$1`, binding.OperationID))
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(existing.BindingBytes, encoded) {
		return nil, ErrOwnershipConflict
	}
	switch existing.State {
	case "abort_decided":
		return existing, nil
	case "reserved", "proof_confirmed":
		aborted, err := scanOwnershipJournal(tx.QueryRow(ctx, `UPDATE ownership_journal
			SET state='abort_decided',updated_at=now()
			WHERE operation_id=$1 AND state IN ('reserved','proof_confirmed')
			RETURNING `+ownershipJournalColumns, binding.OperationID))
		if errors.Is(err, ErrOwnershipMissing) {
			return nil, ErrOwnershipStateTransition
		}
		if err != nil {
			return nil, err
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return aborted, nil
	default:
		return nil, ErrOwnershipStateTransition
	}
}
