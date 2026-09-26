package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrSdkProfileMissing = errors.New("sdk target profile missing")
	ErrSdkProfileStale   = errors.New("sdk target profile ineligible or revision changed")
	ErrSdkAuthorConflict = errors.New("sdk author tombstone conflicts with committed receipt")
)

type SdkProfileEligibility struct {
	AccountID, ProfileID uuid.UUID
	Revision             uint64
	Deleted, Frozen      bool
}

func (s *ProfileStore) GetSdkProfileEligibility(ctx context.Context, accountID, profileID uuid.UUID) (SdkProfileEligibility, error) {
	var out SdkProfileEligibility
	var revision int64
	err := s.Pool().QueryRow(ctx, `SELECT account_id, id, sdk_eligibility_revision,
		deleted_at IS NOT NULL, frozen_at IS NOT NULL
		FROM profiles WHERE account_id = $1 AND id = $2`, accountID, profileID).
		Scan(&out.AccountID, &out.ProfileID, &revision, &out.Deleted, &out.Frozen)
	if errors.Is(err, pgx.ErrNoRows) {
		return SdkProfileEligibility{}, ErrSdkProfileMissing
	}
	if err != nil {
		return SdkProfileEligibility{}, err
	}
	if revision <= 0 {
		return SdkProfileEligibility{}, ErrSdkProfileStale
	}
	out.Revision = uint64(revision)
	return out, nil
}

type SdkAuthorTombstoneInput struct {
	OperationID, SourceAccountID, SourceActorID uuid.UUID
	TargetAccountID, TargetProfileID            uuid.UUID
	ExpectedProfileRevision                     uint64
	FrozenBindingID                             uuid.UUID
	FrozenAuthorityEpoch                        uint64
	FreezeReceiptID                             uuid.UUID
	RequestHash                                 string
}

type SdkAuthorTombstone struct {
	SdkAuthorTombstoneInput
	ReceiptID         uuid.UUID
	ProfileRevision   uint64
	TombstoneRevision uint64
	CommittedAt       time.Time
}

func scanSdkTombstone(row pgx.Row) (SdkAuthorTombstone, error) {
	var out SdkAuthorTombstone
	var profileRevision, tombstoneRevision, authorityEpoch int64
	err := row.Scan(&out.OperationID, &out.ReceiptID, &out.SourceAccountID,
		&out.SourceActorID, &out.TargetAccountID, &out.TargetProfileID,
		&profileRevision, &tombstoneRevision, &out.FrozenBindingID,
		&authorityEpoch, &out.FreezeReceiptID, &out.RequestHash, &out.CommittedAt)
	if err != nil {
		return SdkAuthorTombstone{}, err
	}
	if profileRevision <= 0 || tombstoneRevision <= 0 || authorityEpoch <= 0 {
		return SdkAuthorTombstone{}, ErrSdkAuthorConflict
	}
	out.ProfileRevision = uint64(profileRevision)
	out.TombstoneRevision = uint64(tombstoneRevision)
	out.FrozenAuthorityEpoch = uint64(authorityEpoch)
	out.ExpectedProfileRevision = out.ProfileRevision
	return out, nil
}

const sdkTombstoneColumns = `operation_id, receipt_id, source_account_id, source_actor_id,
	target_account_id, target_profile_id, profile_revision, tombstone_revision, frozen_binding_id,
	frozen_authority_epoch, freeze_receipt_id, request_hash, committed_at`

func sdkTombstoneMatches(existing SdkAuthorTombstone, in SdkAuthorTombstoneInput) bool {
	return existing.OperationID == in.OperationID &&
		existing.SourceAccountID == in.SourceAccountID &&
		existing.SourceActorID == in.SourceActorID &&
		existing.TargetAccountID == in.TargetAccountID &&
		existing.TargetProfileID == in.TargetProfileID &&
		existing.ProfileRevision == in.ExpectedProfileRevision &&
		existing.FrozenBindingID == in.FrozenBindingID &&
		existing.FrozenAuthorityEpoch == in.FrozenAuthorityEpoch &&
		existing.FreezeReceiptID == in.FreezeReceiptID &&
		existing.RequestHash == in.RequestHash
}

func (s *ProfileStore) RecordSdkAuthorTombstone(ctx context.Context, in SdkAuthorTombstoneInput) (SdkAuthorTombstone, error) {
	tx, err := s.Pool().Begin(ctx)
	if err != nil {
		return SdkAuthorTombstone{}, err
	}
	defer tx.Rollback(ctx)

	findExisting := func() (SdkAuthorTombstone, error) {
		return scanSdkTombstone(tx.QueryRow(ctx,
			"SELECT "+sdkTombstoneColumns+" FROM sdk_author_tombstones WHERE operation_id = $1", in.OperationID))
	}
	if existing, findErr := findExisting(); findErr == nil {
		if !sdkTombstoneMatches(existing, in) {
			return SdkAuthorTombstone{}, ErrSdkAuthorConflict
		}
		return existing, nil
	} else if !errors.Is(findErr, pgx.ErrNoRows) {
		return SdkAuthorTombstone{}, findErr
	}

	var revision int64
	var deleted, frozen bool
	err = tx.QueryRow(ctx, `SELECT sdk_eligibility_revision, deleted_at IS NOT NULL,
		frozen_at IS NOT NULL FROM profiles WHERE account_id = $1 AND id = $2 FOR UPDATE`,
		in.TargetAccountID, in.TargetProfileID).Scan(&revision, &deleted, &frozen)
	if errors.Is(err, pgx.ErrNoRows) {
		return SdkAuthorTombstone{}, ErrSdkProfileMissing
	}
	if err != nil {
		return SdkAuthorTombstone{}, err
	}
	if revision <= 0 || deleted || frozen || uint64(revision) != in.ExpectedProfileRevision {
		return SdkAuthorTombstone{}, ErrSdkProfileStale
	}

	receiptID := uuid.New()
	created, err := scanSdkTombstone(tx.QueryRow(ctx, `INSERT INTO sdk_author_tombstones
		(operation_id, receipt_id, source_account_id, source_actor_id, target_account_id,
		target_profile_id, profile_revision, frozen_binding_id, frozen_authority_epoch,
		freeze_receipt_id, request_hash)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		ON CONFLICT DO NOTHING RETURNING `+sdkTombstoneColumns,
		in.OperationID, receiptID, in.SourceAccountID, in.SourceActorID,
		in.TargetAccountID, in.TargetProfileID, revision, in.FrozenBindingID,
		int64(in.FrozenAuthorityEpoch), in.FreezeReceiptID, in.RequestHash))
	if errors.Is(err, pgx.ErrNoRows) {
		// A concurrent identical operation may already have committed. A different
		// operation claiming the same source actor is always a conflict.
		existing, findErr := findExisting()
		if findErr == nil && sdkTombstoneMatches(existing, in) {
			return existing, nil
		}
		if findErr != nil && !errors.Is(findErr, pgx.ErrNoRows) {
			return SdkAuthorTombstone{}, findErr
		}
		return SdkAuthorTombstone{}, ErrSdkAuthorConflict
	}
	if err != nil {
		return SdkAuthorTombstone{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return SdkAuthorTombstone{}, err
	}
	return created, nil
}
