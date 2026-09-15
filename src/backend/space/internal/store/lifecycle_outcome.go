package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/spacecore"
)

var ErrLifecycleOutcomeExpired = errors.New("lifecycle outcome replay expired")

const lifecycleScheduleOutcomeFQN = "voice.space.v1.DeleteSpaceResponse"

// ReplayLifecycleScheduleOutcome returns the saved successful response, or nil
// when the operation is missing or pending. The caller must supply an already
// authenticated principal. This lookup never authorizes a new deletion.
func (s *SpaceStore) ReplayLifecycleScheduleOutcome(ctx context.Context, accountID, actorProfileID uuid.UUID, sessionEpoch int64, request *spacev1.DeleteSpaceRequest) (*spacev1.DeleteSpaceResponse, error) {
	if s == nil || s.Pool == nil || s.tx != nil {
		return nil, ErrLifecycleEvidenceInvalid
	}
	binding, err := lifecycleScheduleBinding(accountID, actorProfileID, sessionEpoch, request)
	if err != nil {
		return nil, err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer rollbackLifecycleTx(ctx, tx)
	if err := lockLifecycleSpace(ctx, tx, binding.spaceID); err != nil {
		return nil, err
	}
	operation, err := loadLifecycleOperation(ctx, tx, binding.operationID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := validateStoredLifecycleOperation(ctx, tx, binding.operationID, accountID, actorProfileID, binding.spaceID, sessionEpoch,
		toLifecycleHash(binding.requestHash), toLifecycleHash(binding.confirmationHash), toLifecycleHash(binding.proofDigest)); err != nil {
		return nil, err
	}
	if err := checkLifecycleOutcomeExpiry(ctx, tx, operation); err != nil {
		return nil, err
	}
	if operation.completedAt == nil {
		return nil, nil
	}
	response := new(spacev1.DeleteSpaceResponse)
	if err := proto.Unmarshal(operation.outcomeBytes, response); err != nil {
		return nil, ErrLifecycleEvidenceInvalid
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return response, nil
}

func lifecycleScheduleBinding(accountID, actorProfileID uuid.UUID, sessionEpoch int64, request *spacev1.DeleteSpaceRequest) (lifecycleOperation, error) {
	if accountID == uuid.Nil || actorProfileID == uuid.Nil || sessionEpoch <= 0 || request == nil ||
		request.GetConfirmationName() == "" || request.GetProof() == "" || len(request.ProtoReflect().GetUnknown()) != 0 {
		return lifecycleOperation{}, ErrLifecycleEvidenceInvalid
	}
	spaceID, err := canonicalLifecycleUUID(request.GetSpaceId())
	if err != nil {
		return lifecycleOperation{}, err
	}
	operationID, err := canonicalLifecycleUUID(request.GetOperationId())
	if err != nil {
		return lifecycleOperation{}, err
	}
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(request)
	if err != nil {
		return lifecycleOperation{}, err
	}
	requestHash := lifecycleHash("voice.space.v1.DeleteSpaceRequest", raw)
	confirmationHash := sha256.Sum256([]byte(request.GetConfirmationName()))
	proofDigest := sha256.Sum256([]byte(request.GetProof()))
	bindingHash := lifecycleOperationBindingHash(accountID, actorProfileID, spaceID, operationID, sessionEpoch, requestHash, confirmationHash, proofDigest)
	return lifecycleOperation{operationID: operationID, accountID: accountID, actorProfileID: actorProfileID,
		spaceID: spaceID, sessionEpoch: sessionEpoch, method: "DELETE", state: "SCHEDULE_PENDING",
		requestHash: requestHash[:], confirmationHash: confirmationHash[:], proofDigest: proofDigest[:], bindingHash: bindingHash[:]}, nil
}

func validateLifecycleOutcome(operation lifecycleOperation) error {
	if operation.state == "SCHEDULE_PENDING" {
		if operation.completedAt != nil || operation.outcomeBytes != nil || operation.outcomeHash != nil {
			return ErrLifecycleEvidenceInvalid
		}
		return nil
	}
	if operation.state != "COMPLETED" || operation.completedAt == nil || operation.completedAt.IsZero() ||
		operation.authReceiptBytes == nil || operation.outcomeBytes == nil || len(operation.outcomeHash) != sha256.Size {
		return ErrLifecycleEvidenceInvalid
	}
	// DELETE has precisely the empty success projection. Unknown payload bytes,
	// even with a matching hash, cannot substitute a different saved outcome.
	if len(operation.outcomeBytes) != 0 {
		return ErrLifecycleEvidenceInvalid
	}
	expectedHash := lifecycleHash(lifecycleScheduleOutcomeFQN, operation.outcomeBytes)
	if !bytes.Equal(expectedHash[:], operation.outcomeHash) {
		return ErrLifecycleEvidenceInvalid
	}
	return nil
}

func checkLifecycleOutcomeExpiry(ctx context.Context, db spaceStoreDB, operation lifecycleOperation) error {
	if operation.completedAt == nil {
		return nil
	}
	var expired bool
	// UTC duration avoids session-timezone/DST changes to the thirty-day window.
	expiresAt := operation.completedAt.UTC().Add(30 * 24 * time.Hour)
	if err := db.QueryRow(ctx, `SELECT clock_timestamp() >= $1::timestamptz`, expiresAt).Scan(&expired); err != nil {
		return err
	}
	if expired {
		return ErrLifecycleOutcomeExpired
	}
	return nil
}

// Called only by CompleteLifecycleSchedule after its locked database-time
// barrier. Generic snapshot persistence cannot create an admitted outcome.
func persistLifecycleScheduleOutcome(ctx context.Context, db spaceStoreDB, snapshot spacecore.LifecycleSnapshot, completing bool) error {
	if snapshot.Phase != spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_SCHEDULED {
		return nil
	}
	operation, err := loadLifecycleOperation(ctx, db, uuid.MustParse(snapshot.DeletionOperationID))
	if errors.Is(err, pgx.ErrNoRows) {
		// Legacy aggregate-only fixtures do not represent an admitted operation
		// and cannot produce a replayable DELETE result.
		return nil
	}
	if err != nil {
		return err
	}
	if err := operation.validate(); err != nil {
		return err
	}
	if operation.spaceID.String() != snapshot.SpaceID || operation.authReceiptBytes == nil || snapshot.ScheduledAt.IsZero() {
		return ErrLifecycleEvidenceInvalid
	}
	if operation.completedAt != nil {
		if !operation.completedAt.Equal(snapshot.ScheduledAt) {
			return ErrLifecycleConflict
		}
		return nil
	}
	if !completing {
		return ErrLifecycleStateTransition
	}
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(&spacev1.DeleteSpaceResponse{})
	if err != nil {
		return err
	}
	hash := lifecycleHash(lifecycleScheduleOutcomeFQN, raw)
	command, err := db.Exec(ctx, `UPDATE space_lifecycle_operations
		SET state='COMPLETED',outcome_bytes=$2,outcome_sha256=$3,completed_at=$4
		WHERE operation_id=$1 AND state='SCHEDULE_PENDING' AND completed_at IS NULL`,
		operation.operationID, raw, hash[:], snapshot.ScheduledAt.UTC())
	if err != nil {
		return err
	}
	if command.RowsAffected() != 1 {
		return ErrLifecycleConflict
	}
	return nil
}
