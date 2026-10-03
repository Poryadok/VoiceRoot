package store

import (
	"bytes"
	"context"
	"errors"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	commonv1 "voice.app/voice/common/v1"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/spacecore"
)

// LifecycleDeletionProofRecorded reports whether Auth's exact deletion-proof
// receipt is durable before Space starts any participant freeze side effect.
func (s *SpaceStore) LifecycleDeletionProofRecorded(ctx context.Context, spaceID uuid.UUID) (bool, error) {
	if s == nil || s.Pool == nil || spaceID == uuid.Nil {
		return false, ErrLifecycleEvidenceInvalid
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer rollbackLifecycleTx(ctx, tx)
	if err := lockLifecycleSpace(ctx, tx, spaceID); err != nil {
		return false, err
	}
	aggregate, err := loadLifecycle(ctx, tx, spaceID, true)
	if err != nil {
		return false, err
	}
	operationID, err := canonicalLifecycleUUID(aggregate.Snapshot().DeletionOperationID)
	if err != nil {
		return false, err
	}
	operation, err := loadLifecycleOperation(ctx, tx, operationID)
	if err != nil {
		return false, err
	}
	if operation.method != "DELETE" || operation.spaceID != spaceID {
		return false, ErrLifecycleConflict
	}
	if err := operation.validate(); err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return len(operation.authReceiptBytes) > 0, nil
}

// BeginLifecycleFreeze persists Chat's immutable manifest only after the exact
// Auth proof receipt has been saved. Exact retries retain the same generation
// and manifest even after participant progress advances.
func (s *SpaceStore) BeginLifecycleFreeze(ctx context.Context, spaceID uuid.UUID, manifest *commonv1.ManifestBinding) (*spacecore.LifecycleAggregate, error) {
	if spaceID == uuid.Nil || manifest == nil {
		return nil, ErrLifecycleEvidenceInvalid
	}
	return s.withLockedLifecycle(ctx, spaceID, func(tx pgx.Tx, aggregate *spacecore.LifecycleAggregate) (bool, error) {
		snapshot := aggregate.Snapshot()
		if snapshot.Phase != spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_SCHEDULE_PENDING {
			if proto.Equal(snapshot.Manifest, manifest) {
				return false, nil
			}
			return false, ErrLifecycleConflict
		}
		operationID, err := canonicalLifecycleUUID(snapshot.DeletionOperationID)
		if err != nil {
			return false, ErrLifecycleEvidenceInvalid
		}
		operation, err := loadLifecycleOperation(ctx, tx, operationID)
		if err != nil {
			return false, err
		}
		if operation.method != "DELETE" || operation.spaceID != spaceID || len(operation.authReceiptBytes) == 0 {
			return false, ErrLifecycleConflict
		}
		if err := aggregate.BeginFreeze(manifest); err != nil {
			return false, ErrLifecycleEvidenceInvalid
		}
		return true, nil
	})
}

// RecordLifecycleFenceReceipt commits one authoritative acknowledgement against
// fresh state. The caller must authenticate the participant before this boundary.
func (s *SpaceStore) RecordLifecycleFenceReceipt(ctx context.Context, receipt *commonv1.SpaceLifecycleFenceReceipt) (*spacecore.LifecycleAggregate, error) {
	if receipt == nil {
		return nil, ErrLifecycleEvidenceInvalid
	}
	return s.recordLifecycleReceipt(ctx, receipt.SpaceId, receipt.DeletionOperationId, receipt.Generation,
		receipt.ParticipantId, "FENCE", receipt, func(aggregate *spacecore.LifecycleAggregate) error {
			return aggregate.RecordFenceReceipt(receipt)
		})
}

// RecordLifecycleRoleRetirementReceipt retains Role's stricter typed evidence;
// a generic participant purge receipt cannot replace retirement authority.
func (s *SpaceStore) RecordLifecycleRoleRetirementReceipt(ctx context.Context, receipt *rolev1.RetireSpaceReceipt) (*spacecore.LifecycleAggregate, error) {
	if receipt == nil {
		return nil, ErrLifecycleEvidenceInvalid
	}
	return s.recordLifecycleReceipt(ctx, receipt.SpaceId, receipt.DeletionOperationId, receipt.Generation,
		commonv1.ParticipantId_PARTICIPANT_ID_ROLE, "ROLE_RETIREMENT", receipt, func(aggregate *spacecore.LifecycleAggregate) error {
			return aggregate.RecordRoleRetirementReceipt(receipt)
		})
}

func (s *SpaceStore) RecordLifecyclePurgeReceipt(ctx context.Context, receipt *commonv1.SpacePurgeReceipt) (*spacecore.LifecycleAggregate, error) {
	if receipt == nil || receipt.ParticipantId == commonv1.ParticipantId_PARTICIPANT_ID_ROLE {
		return nil, ErrLifecycleEvidenceInvalid
	}
	return s.recordLifecycleReceipt(ctx, receipt.SpaceId, receipt.DeletionOperationId, receipt.Generation,
		receipt.ParticipantId, "PURGE", receipt, func(aggregate *spacecore.LifecycleAggregate) error {
			return aggregate.RecordPurgeReceipt(receipt)
		})
}

func (s *SpaceStore) recordLifecycleReceipt(ctx context.Context, space, operation string, generation uint64,
	participant commonv1.ParticipantId, kind string, receipt proto.Message,
	apply func(*spacecore.LifecycleAggregate) error) (*spacecore.LifecycleAggregate, error) {
	spaceID, err := canonicalLifecycleUUID(space)
	if err != nil {
		return nil, err
	}
	operationID, err := canonicalLifecycleUUID(operation)
	if err != nil {
		return nil, err
	}
	if generation == 0 || generation > math.MaxInt64 || !lifecycleParticipantValid(participant) {
		return nil, ErrLifecycleEvidenceInvalid
	}
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(receipt)
	if err != nil {
		return nil, err
	}
	hash := lifecycleHash(string(receipt.ProtoReflect().Descriptor().FullName()), raw)
	return s.withLockedLifecycle(ctx, spaceID, func(tx pgx.Tx, aggregate *spacecore.LifecycleAggregate) (bool, error) {
		// The ledger outlives a phase and generation. Check exact accepted bytes
		// before current-phase validation so delayed responses remain retryable.
		var storedBytes, storedHash []byte
		var progress string
		err := tx.QueryRow(ctx, `SELECT progress,receipt_bytes,receipt_sha256
			FROM space_lifecycle_participants
			WHERE space_id=$1 AND deletion_operation_id=$2 AND generation=$3
			AND participant_id=$4 AND request_kind=$5`, spaceID, operationID, int64(generation), int16(participant), kind).
			Scan(&progress, &storedBytes, &storedHash)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return false, err
		}
		if err == nil && storedBytes != nil {
			if progress != "COMPLETE" || !bytes.Equal(storedBytes, raw) || !bytes.Equal(storedHash, hash[:]) {
				return false, ErrLifecycleConflict
			}
			return false, nil
		}
		if err := apply(aggregate); err != nil {
			return false, err
		}
		return true, nil
	})
}

// CompleteLifecycleRestore atomically publishes the local LIVE decision and
// outbox visibility only after all ten authoritative LIVE acknowledgements.
func (s *SpaceStore) CompleteLifecycleRestore(ctx context.Context, spaceID uuid.UUID) (*spacecore.LifecycleAggregate, spacecore.LifecycleOutboxRecord, error) {
	var event spacecore.LifecycleOutboxRecord
	aggregate, err := s.withLockedLifecycle(ctx, spaceID, func(tx pgx.Tx, aggregate *spacecore.LifecycleAggregate) (bool, error) {
		completed := aggregate.Snapshot().RestoreEvent != nil
		var databaseTime time.Time
		if err := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&databaseTime); err != nil {
			return false, err
		}
		var err error
		event, err = aggregate.CompleteRestore(databaseTime.UTC())
		if err != nil {
			return false, err
		}
		if err := persistLifecycleRestoreOutcome(ctx, tx, aggregate.Snapshot(), !completed); err != nil {
			return false, err
		}
		return !completed, nil
	})
	if err != nil {
		return nil, spacecore.LifecycleOutboxRecord{}, err
	}
	return aggregate, event, nil
}

// All read/validate/write work shares one transaction and the same advisory lock
// as lifecycle decisions. Returning persist=false makes exact replay read-only.
func (s *SpaceStore) withLockedLifecycle(ctx context.Context, spaceID uuid.UUID,
	action func(pgx.Tx, *spacecore.LifecycleAggregate) (bool, error)) (*spacecore.LifecycleAggregate, error) {
	if s == nil || s.Pool == nil || spaceID == uuid.Nil || s.tx != nil {
		return nil, ErrLifecycleEvidenceInvalid
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer rollbackLifecycleTx(ctx, tx)
	if err := lockLifecycleSpace(ctx, tx, spaceID); err != nil {
		return nil, err
	}
	aggregate, err := loadLifecycle(ctx, tx, spaceID, true)
	if err != nil {
		return nil, err
	}
	persist, err := action(tx, aggregate)
	if err != nil {
		return nil, err
	}
	if persist {
		if err := persistLifecycleSnapshot(ctx, tx, aggregate.Snapshot()); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return aggregate, nil
}
