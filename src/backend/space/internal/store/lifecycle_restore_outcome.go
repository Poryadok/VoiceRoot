package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/spacecore"
)

const lifecycleRestoreOutcomeFQN = "voice.space.v1.RestoreSpaceResponse"

// ReserveLifecycleRestore admits a request from an already authenticated
// account/profile/session principal. Network authentication belongs to the
// coordinator; this storage boundary does not activate a public restore route.
func (s *SpaceStore) ReserveLifecycleRestore(ctx context.Context, accountID, actorProfileID uuid.UUID, sessionEpoch int64, request *spacev1.RestoreSpaceRequest) (*spacecore.LifecycleAggregate, error) {
	if s == nil || s.Pool == nil || s.tx != nil {
		return nil, ErrLifecycleEvidenceInvalid
	}
	binding, err := lifecycleRestoreBinding(accountID, actorProfileID, sessionEpoch, request)
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
	if stored, err := loadLifecycleOperation(ctx, tx, binding.operationID); err == nil {
		if err := validateStoredLifecycleRestore(stored, binding); err != nil {
			return nil, err
		}
		if err := checkLifecycleOutcomeExpiry(ctx, tx, stored); err != nil {
			return nil, err
		}
		aggregate, err := loadLifecycle(ctx, tx, binding.spaceID, true)
		if err != nil {
			return nil, err
		}
		if aggregate.Snapshot().DeletionOperationID != stored.deletionOperationID.String() || aggregate.Generation() != uint64(*stored.generation) {
			return nil, ErrLifecycleConflict
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return aggregate, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	var owner uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT owner_profile_id FROM spaces WHERE id=$1 FOR UPDATE`, binding.spaceID).Scan(&owner); err != nil {
		return nil, err
	}
	if owner != actorProfileID {
		return nil, ErrNotSpaceOwner
	}
	if err := checkOwnershipJournalAvailable(ctx, tx, []uuid.UUID{binding.spaceID}); err != nil {
		return nil, err
	}
	aggregate, err := loadLifecycle(ctx, tx, binding.spaceID, true)
	if err != nil {
		return nil, err
	}
	if aggregate.Phase() != spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_SCHEDULED || aggregate.Generation() >= math.MaxInt64 {
		return nil, ErrLifecycleStateTransition
	}
	var databaseTime time.Time
	if err := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&databaseTime); err != nil {
		return nil, err
	}
	phase, err := aggregate.DecideRecovery(databaseTime.UTC(), aggregate.Generation()+1)
	if err != nil {
		return nil, err
	}
	if phase == spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGE_DECIDED {
		if err := persistLifecycleSnapshot(ctx, tx, aggregate.Snapshot()); err != nil {
			return nil, err
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return nil, ErrLifecycleStateTransition
	}
	deletionID := uuid.MustParse(aggregate.Snapshot().DeletionOperationID)
	generation := int64(aggregate.Generation())
	if deletionID == binding.operationID {
		return nil, ErrLifecycleConflict
	}
	binding.deletionOperationID, binding.generation = &deletionID, &generation
	hash := lifecycleRestoreBindingHash(binding)
	binding.bindingHash = hash[:]
	command, err := tx.Exec(ctx, `INSERT INTO space_lifecycle_operations(
		operation_id,account_id,actor_profile_id,space_id,session_epoch,method,request_sha256,binding_sha256,state,deletion_operation_id,generation)
		VALUES($1,$2,$3,$4,$5,'RESTORE',$6,$7,'RESTORE_PENDING',$8,$9) ON CONFLICT (operation_id) DO NOTHING`,
		binding.operationID, accountID, actorProfileID, binding.spaceID, sessionEpoch, binding.requestHash, binding.bindingHash, deletionID, generation)
	if err != nil {
		return nil, err
	}
	if command.RowsAffected() != 1 {
		return nil, ErrLifecycleConflict
	}
	if err := persistLifecycleSnapshot(ctx, tx, aggregate.Snapshot()); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return aggregate, nil
}

// ReplayLifecycleRestoreOutcome validates the original authenticated binding
// before returning saved data. It does not depend on retained aggregate rows.
func (s *SpaceStore) ReplayLifecycleRestoreOutcome(ctx context.Context, accountID, actorProfileID uuid.UUID, sessionEpoch int64, request *spacev1.RestoreSpaceRequest) (*spacev1.RestoreSpaceResponse, error) {
	if s == nil || s.Pool == nil || s.tx != nil {
		return nil, ErrLifecycleEvidenceInvalid
	}
	binding, err := lifecycleRestoreBinding(accountID, actorProfileID, sessionEpoch, request)
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
	stored, err := loadLifecycleOperation(ctx, tx, binding.operationID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := validateStoredLifecycleRestore(stored, binding); err != nil {
		return nil, err
	}
	if err := checkLifecycleOutcomeExpiry(ctx, tx, stored); err != nil {
		return nil, err
	}
	if stored.completedAt == nil {
		return nil, nil
	}
	response := new(spacev1.RestoreSpaceResponse)
	if err := proto.Unmarshal(stored.outcomeBytes, response); err != nil {
		return nil, ErrLifecycleEvidenceInvalid
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return response, nil
}

func lifecycleRestoreBinding(accountID, actorProfileID uuid.UUID, sessionEpoch int64, request *spacev1.RestoreSpaceRequest) (lifecycleOperation, error) {
	if accountID == uuid.Nil || actorProfileID == uuid.Nil || sessionEpoch <= 0 || request == nil || len(request.ProtoReflect().GetUnknown()) != 0 {
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
	hash := lifecycleHash("voice.space.v1.RestoreSpaceRequest", raw)
	return lifecycleOperation{operationID: operationID, accountID: accountID, actorProfileID: actorProfileID, spaceID: spaceID, sessionEpoch: sessionEpoch,
		method: "RESTORE", state: "RESTORE_PENDING", requestHash: hash[:]}, nil
}

func lifecycleRestoreBindingHash(operation lifecycleOperation) [sha256.Size]byte {
	var wire []byte
	wire = appendLifecycleBytes(wire, 1, operation.accountID[:])
	wire = appendLifecycleBytes(wire, 2, operation.actorProfileID[:])
	wire = protowire.AppendVarint(protowire.AppendTag(wire, 3, protowire.VarintType), uint64(operation.sessionEpoch))
	wire = appendLifecycleBytes(wire, 4, operation.spaceID[:])
	wire = appendLifecycleBytes(wire, 5, operation.operationID[:])
	wire = appendLifecycleBytes(wire, 6, operation.requestHash)
	wire = appendLifecycleBytes(wire, 7, operation.deletionOperationID[:])
	wire = protowire.AppendVarint(protowire.AppendTag(wire, 8, protowire.VarintType), uint64(*operation.generation))
	return lifecycleHash("voice.space.store.v1.RestoreOperationBinding", wire)
}

func validateStoredLifecycleRestore(stored, binding lifecycleOperation) error {
	if err := stored.validate(); err != nil {
		return err
	}
	if stored.method != "RESTORE" || stored.operationID != binding.operationID || stored.accountID != binding.accountID ||
		stored.actorProfileID != binding.actorProfileID || stored.sessionEpoch != binding.sessionEpoch || stored.spaceID != binding.spaceID || !bytes.Equal(stored.requestHash, binding.requestHash) {
		return ErrLifecycleConflict
	}
	return nil
}

func validateLifecycleRestoreOperation(operation lifecycleOperation) error {
	if operation.method != "RESTORE" || operation.deletionOperationID == nil || *operation.deletionOperationID == uuid.Nil ||
		*operation.deletionOperationID == operation.operationID || operation.generation == nil || *operation.generation <= 0 ||
		operation.confirmationHash != nil || operation.proofDigest != nil || operation.authReceiptBytes != nil || operation.authReceiptHash != nil {
		return ErrLifecycleEvidenceInvalid
	}
	binding, err := lifecycleRestoreBinding(operation.accountID, operation.actorProfileID, operation.sessionEpoch,
		&spacev1.RestoreSpaceRequest{SpaceId: operation.spaceID.String(), OperationId: operation.operationID.String()})
	if err != nil || !bytes.Equal(binding.requestHash, operation.requestHash) {
		return ErrLifecycleEvidenceInvalid
	}
	hash := lifecycleRestoreBindingHash(operation)
	if !bytes.Equal(hash[:], operation.bindingHash) {
		return ErrLifecycleEvidenceInvalid
	}
	if operation.state == "RESTORE_PENDING" {
		if operation.completedAt != nil || operation.outcomeBytes != nil || operation.outcomeHash != nil {
			return ErrLifecycleEvidenceInvalid
		}
		return nil
	}
	if operation.state != "COMPLETED" || operation.completedAt == nil || operation.completedAt.IsZero() || operation.outcomeBytes == nil {
		return ErrLifecycleEvidenceInvalid
	}
	hash = lifecycleHash(lifecycleRestoreOutcomeFQN, operation.outcomeBytes)
	if !bytes.Equal(hash[:], operation.outcomeHash) {
		return ErrLifecycleEvidenceInvalid
	}
	response := new(spacev1.RestoreSpaceResponse)
	if err := proto.Unmarshal(operation.outcomeBytes, response); err != nil {
		return ErrLifecycleEvidenceInvalid
	}
	space := response.GetSpace()
	if space == nil || space.GetId() != operation.spaceID.String() || space.GetName() == "" ||
		space.GetCreatedAt() == nil || space.GetCreatedAt().CheckValid() != nil || space.GetUpdatedAt() == nil || space.GetUpdatedAt().CheckValid() != nil ||
		space.GetDeletionScheduledAt() != nil || space.GetPurgeAfter() != nil {
		return ErrLifecycleEvidenceInvalid
	}
	if _, err := canonicalLifecycleUUID(space.GetOwnerProfileId()); err != nil {
		return err
	}
	return nil
}

func loadRestoreForSpace(ctx context.Context, db spaceStoreDB, spaceID uuid.UUID) (*lifecycleOperation, error) {
	// One active deletion cycle per Space exists in the current foundation.
	// Query by Space first so a forged deletion ID cannot hide an admission.
	var operationID uuid.UUID
	err := db.QueryRow(ctx, `SELECT operation_id FROM space_lifecycle_operations WHERE space_id=$1 AND method='RESTORE' ORDER BY generation DESC LIMIT 1`, spaceID).Scan(&operationID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	operation, err := loadLifecycleOperation(ctx, db, operationID)
	if err != nil {
		return nil, err
	}
	if err := operation.validate(); err != nil {
		return nil, err
	}
	return &operation, nil
}

func guardAdmittedLifecycleRestoreSnapshot(ctx context.Context, db spaceStoreDB, snapshot spacecore.LifecycleSnapshot) error {
	operation, err := loadRestoreForSpace(ctx, db, uuid.MustParse(snapshot.SpaceID))
	if err != nil || operation == nil {
		return err
	}
	if operation.deletionOperationID.String() != snapshot.DeletionOperationID || uint64(*operation.generation) != snapshot.Generation {
		return ErrLifecycleConflict
	}
	if operation.state == "RESTORE_PENDING" && snapshot.Phase != spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_RESTORE_DECIDED {
		return ErrLifecycleStateTransition
	}
	if operation.state == "COMPLETED" && (snapshot.Phase != spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_LIVE || snapshot.RestoreEvent == nil || !operation.completedAt.Equal(snapshot.RestoreEvent.OccurredAt)) {
		return ErrLifecycleConflict
	}
	return nil
}

func persistLifecycleRestoreOutcome(ctx context.Context, db spaceStoreDB, snapshot spacecore.LifecycleSnapshot, completing bool) error {
	operation, err := loadRestoreForSpace(ctx, db, uuid.MustParse(snapshot.SpaceID))
	if err != nil || operation == nil {
		return err
	} // aggregate-only storage fixtures are not admitted API calls
	if operation.deletionOperationID.String() != snapshot.DeletionOperationID || uint64(*operation.generation) != snapshot.Generation || snapshot.RestoreEvent == nil {
		return ErrLifecycleConflict
	}
	if operation.completedAt != nil {
		if !operation.completedAt.Equal(snapshot.RestoreEvent.OccurredAt) {
			return ErrLifecycleConflict
		}
		return nil
	}
	if !completing || snapshot.Phase != spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_LIVE {
		return ErrLifecycleStateTransition
	}
	row, err := scanSpaceRow(db.QueryRow(ctx, `SELECT `+spaceSelectColumns+` FROM spaces WHERE id=$1`, operation.spaceID))
	if err != nil {
		return err
	}
	response := &spacev1.RestoreSpaceResponse{Space: row.ToProto()}
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(response)
	if err != nil {
		return err
	}
	hash := lifecycleHash(lifecycleRestoreOutcomeFQN, raw)
	completedAt := snapshot.RestoreEvent.OccurredAt.UTC()
	operation.state, operation.outcomeBytes, operation.outcomeHash, operation.completedAt = "COMPLETED", raw, hash[:], &completedAt
	if err := operation.validate(); err != nil {
		return err
	}
	command, err := db.Exec(ctx, `UPDATE space_lifecycle_operations SET state='COMPLETED',outcome_bytes=$2,outcome_sha256=$3,completed_at=$4
		WHERE operation_id=$1 AND method='RESTORE' AND state='RESTORE_PENDING' AND completed_at IS NULL`, operation.operationID, raw, hash[:], completedAt)
	if err != nil {
		return err
	}
	if command.RowsAffected() != 1 {
		return ErrLifecycleConflict
	}
	return nil
}

// ToProto is the canonical Space row projection used by live reads and saved
// restore outcomes. Lifecycle scheduling fields are absent for a restored Space.
func (r *SpaceRow) ToProto() *spacev1.Space {
	if r == nil {
		return nil
	}
	out := &spacev1.Space{Id: r.ID.String(), Name: r.Name, Description: r.Description, Visibility: r.Visibility,
		OwnerProfileId: r.OwnerProfileID.String(), MemberCount: r.MemberCount, IsVerified: r.IsVerified, VerificationType: r.VerificationType,
		EntryRequirement: r.EntryRequirement, AllowGuests: r.AllowGuests, CreatedAt: timestamppb.New(r.CreatedAt), UpdatedAt: timestamppb.New(r.UpdatedAt)}
	if r.IconURL != nil {
		value := *r.IconURL
		out.IconUrl = &value
	}
	if r.BannerURL != nil {
		value := *r.BannerURL
		out.BannerUrl = &value
	}
	if r.EntryQuestionsJSON != nil {
		out.EntryQuestionsJson = *r.EntryQuestionsJSON
	}
	if r.MMConfigJSON != nil {
		out.MmConfigJson = *r.MMConfigJSON
	}
	return out
}
