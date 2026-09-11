package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	commonv1 "voice.app/voice/common/v1"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/spacecore"
)

var lifecycleParticipants = [...]commonv1.ParticipantId{
	commonv1.ParticipantId_PARTICIPANT_ID_ROLE,
	commonv1.ParticipantId_PARTICIPANT_ID_CHAT,
	commonv1.ParticipantId_PARTICIPANT_ID_MESSAGING,
	commonv1.ParticipantId_PARTICIPANT_ID_FILE,
	commonv1.ParticipantId_PARTICIPANT_ID_VOICE,
	commonv1.ParticipantId_PARTICIPANT_ID_MATCHMAKING,
	commonv1.ParticipantId_PARTICIPANT_ID_SEARCH,
	commonv1.ParticipantId_PARTICIPANT_ID_SUBSCRIPTION,
	commonv1.ParticipantId_PARTICIPANT_ID_BOT,
	commonv1.ParticipantId_PARTICIPANT_ID_NOTIFICATION,
}

var lifecycleParticipantPackages = map[commonv1.ParticipantId]string{
	commonv1.ParticipantId_PARTICIPANT_ID_ROLE:         "voice.role.v1",
	commonv1.ParticipantId_PARTICIPANT_ID_CHAT:         "voice.chat.v1",
	commonv1.ParticipantId_PARTICIPANT_ID_MESSAGING:    "voice.messaging.v1",
	commonv1.ParticipantId_PARTICIPANT_ID_FILE:         "voice.file.v1",
	commonv1.ParticipantId_PARTICIPANT_ID_VOICE:        "voice.calls.v1",
	commonv1.ParticipantId_PARTICIPANT_ID_MATCHMAKING:  "voice.matchmaking.v1",
	commonv1.ParticipantId_PARTICIPANT_ID_SEARCH:       "voice.search.v1",
	commonv1.ParticipantId_PARTICIPANT_ID_SUBSCRIPTION: "voice.subscription.v1",
	commonv1.ParticipantId_PARTICIPANT_ID_BOT:          "voice.bot.v1",
	commonv1.ParticipantId_PARTICIPANT_ID_NOTIFICATION: "voice.notification.v1",
}

var (
	ErrLifecycleConflict        = errors.New("lifecycle binding conflict")
	ErrLifecycleStateTransition = errors.New("invalid lifecycle state transition")
	ErrLifecycleEvidenceInvalid = errors.New("invalid lifecycle evidence")
)

func (s *SpaceStore) ReserveLifecycleSchedule(ctx context.Context, accountID, actorProfileID uuid.UUID, sessionEpoch int64, request *spacev1.DeleteSpaceRequest) (*spacecore.LifecycleAggregate, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("space store: pool not configured")
	}
	if accountID == uuid.Nil || actorProfileID == uuid.Nil || sessionEpoch <= 0 || request == nil {
		return nil, ErrLifecycleEvidenceInvalid
	}
	spaceID, err := canonicalLifecycleUUID(request.GetSpaceId())
	if err != nil {
		return nil, err
	}
	operationID, err := canonicalLifecycleUUID(request.GetOperationId())
	if err != nil {
		return nil, err
	}
	if request.GetConfirmationName() == "" || request.GetProof() == "" || len(request.ProtoReflect().GetUnknown()) != 0 {
		return nil, ErrLifecycleEvidenceInvalid
	}
	requestBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(request)
	if err != nil {
		return nil, err
	}
	requestHash := lifecycleHash("voice.space.v1.DeleteSpaceRequest", requestBytes)
	confirmationHash := sha256.Sum256([]byte(request.GetConfirmationName()))
	proofDigest := sha256.Sum256([]byte(request.GetProof()))
	bindingHash := lifecycleOperationBindingHash(accountID, actorProfileID, spaceID, operationID, sessionEpoch, requestHash, confirmationHash, proofDigest)

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer rollbackLifecycleTx(ctx, tx)
	if err := lockLifecycleSpace(ctx, tx, spaceID); err != nil {
		return nil, err
	}
	var ownerID uuid.UUID
	var currentName string
	if err := tx.QueryRow(ctx, `SELECT owner_profile_id,name FROM spaces WHERE id=$1 FOR UPDATE`, spaceID).Scan(&ownerID, &currentName); err != nil {
		return nil, err
	}
	if ownerID != actorProfileID || currentName != request.GetConfirmationName() {
		return nil, ErrLifecycleConflict
	}

	command, err := tx.Exec(ctx, `INSERT INTO space_lifecycle_operations(
		operation_id,account_id,actor_profile_id,space_id,session_epoch,method,
		request_sha256,confirmation_name_sha256,proof_digest_sha256,binding_sha256,state)
		VALUES($1,$2,$3,$4,$5,'DELETE',$6,$7,$8,$9,'SCHEDULE_PENDING')
		ON CONFLICT (operation_id) DO NOTHING`, operationID, accountID, actorProfileID,
		spaceID, sessionEpoch, requestHash[:], confirmationHash[:], proofDigest[:], bindingHash[:])
	if err != nil {
		return nil, err
	}
	if command.RowsAffected() == 0 {
		if err := validateStoredLifecycleOperation(ctx, tx, operationID, accountID, actorProfileID, spaceID, sessionEpoch, requestHash, confirmationHash, proofDigest); err != nil {
			return nil, err
		}
	}
	aggregate, err := spacecore.NewLifecycleAggregate(spaceID.String(), operationID.String())
	if err != nil {
		return nil, err
	}
	if err := aggregate.BeginSchedule(1); err != nil {
		return nil, err
	}
	if err := persistLifecycleSnapshot(ctx, tx, aggregate.Snapshot()); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return aggregate, nil
}

func (s *SpaceStore) RecordLifecycleDeletionProofReceipt(ctx context.Context, actorProfileID, operationID uuid.UUID, receiptBytes []byte, receiptHash [sha256.Size]byte) (*spacecore.LifecycleAggregate, error) {
	if s == nil || s.Pool == nil || actorProfileID == uuid.Nil || operationID == uuid.Nil || len(receiptBytes) == 0 {
		return nil, ErrLifecycleEvidenceInvalid
	}
	if lifecycleHash("voice.auth.v1.ConsumeSpaceDeletionProofResponse", receiptBytes) != receiptHash {
		return nil, ErrLifecycleEvidenceInvalid
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer rollbackLifecycleTx(ctx, tx)
	operation, err := loadLifecycleOperation(ctx, tx, operationID)
	if err != nil {
		return nil, err
	}
	if operation.actorProfileID != actorProfileID {
		return nil, ErrLifecycleConflict
	}
	if err := lockLifecycleSpace(ctx, tx, operation.spaceID); err != nil {
		return nil, err
	}
	if err := operation.validate(); err != nil {
		return nil, err
	}
	if err := validateLifecycleAuthReceipt(receiptBytes, operation); err != nil {
		return nil, err
	}
	if operation.authReceiptBytes != nil {
		if !bytes.Equal(operation.authReceiptBytes, receiptBytes) || !bytes.Equal(operation.authReceiptHash, receiptHash[:]) {
			return nil, ErrLifecycleConflict
		}
	} else {
		command, updateErr := tx.Exec(ctx, `UPDATE space_lifecycle_operations
			SET auth_receipt_bytes=$2,auth_receipt_sha256=$3
			WHERE operation_id=$1 AND auth_receipt_bytes IS NULL`, operationID, receiptBytes, receiptHash[:])
		if updateErr != nil {
			return nil, updateErr
		}
		if command.RowsAffected() != 1 {
			return nil, ErrLifecycleConflict
		}
	}
	aggregate, err := loadLifecycle(ctx, tx, operation.spaceID, false)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return aggregate, nil
}

func (s *SpaceStore) PersistLifecycle(ctx context.Context, aggregate *spacecore.LifecycleAggregate) error {
	if aggregate == nil {
		return ErrLifecycleEvidenceInvalid
	}
	if s == nil || s.db() == nil {
		return errors.New("space store: pool not configured")
	}
	if s.tx != nil {
		return persistLifecycleSnapshot(ctx, s.tx, aggregate.Snapshot())
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer rollbackLifecycleTx(ctx, tx)
	if err := persistLifecycleSnapshot(ctx, tx, aggregate.Snapshot()); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *SpaceStore) LoadLifecycle(ctx context.Context, spaceID uuid.UUID) (*spacecore.LifecycleAggregate, error) {
	if spaceID == uuid.Nil {
		return nil, ErrLifecycleEvidenceInvalid
	}
	if s == nil || s.db() == nil {
		return nil, errors.New("space store: pool not configured")
	}
	return loadLifecycle(ctx, s.db(), spaceID, false)
}

func (s *SpaceStore) CompleteLifecycleSchedule(ctx context.Context, spaceID uuid.UUID) (*spacecore.LifecycleAggregate, spacecore.LifecycleOutboxRecord, error) {
	if s == nil || s.Pool == nil || spaceID == uuid.Nil {
		return nil, spacecore.LifecycleOutboxRecord{}, ErrLifecycleEvidenceInvalid
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, spacecore.LifecycleOutboxRecord{}, err
	}
	defer rollbackLifecycleTx(ctx, tx)
	if err := lockLifecycleSpace(ctx, tx, spaceID); err != nil {
		return nil, spacecore.LifecycleOutboxRecord{}, err
	}
	aggregate, err := loadLifecycle(ctx, tx, spaceID, true)
	if err != nil {
		return nil, spacecore.LifecycleOutboxRecord{}, err
	}
	var databaseTime time.Time
	if err := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&databaseTime); err != nil {
		return nil, spacecore.LifecycleOutboxRecord{}, err
	}
	event, err := aggregate.CompleteSchedule(databaseTime.UTC())
	if err != nil {
		return nil, spacecore.LifecycleOutboxRecord{}, err
	}
	if err := persistLifecycleSnapshot(ctx, tx, aggregate.Snapshot()); err != nil {
		return nil, spacecore.LifecycleOutboxRecord{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, spacecore.LifecycleOutboxRecord{}, err
	}
	return aggregate, event, nil
}

func (s *SpaceStore) DecideLifecycleRecovery(ctx context.Context, spaceID uuid.UUID, generation uint64) (*spacecore.LifecycleAggregate, error) {
	if s == nil || s.Pool == nil || spaceID == uuid.Nil || generation == 0 {
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
	var databaseTime time.Time
	if err := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&databaseTime); err != nil {
		return nil, err
	}
	if _, err := aggregate.DecideRecovery(databaseTime.UTC(), generation); err != nil {
		return nil, err
	}
	if err := persistLifecycleSnapshot(ctx, tx, aggregate.Snapshot()); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return aggregate, nil
}

func (s *SpaceStore) CompleteLifecyclePurge(ctx context.Context, aggregate *spacecore.LifecycleAggregate, ownerAccountHMAC, actorAccountHMAC []byte, keyVersion string) error {
	if s == nil || s.Pool == nil || aggregate == nil || len(ownerAccountHMAC) != sha256.Size || len(actorAccountHMAC) != sha256.Size || keyVersion == "" {
		return ErrLifecycleEvidenceInvalid
	}
	snapshot := aggregate.Snapshot()
	if snapshot.Phase != spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGED || snapshot.DeletedEvent == nil {
		return ErrLifecycleStateTransition
	}
	spaceID, err := canonicalLifecycleUUID(snapshot.SpaceID)
	if err != nil {
		return err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer rollbackLifecycleTx(ctx, tx)
	if err := lockLifecycleSpace(ctx, tx, spaceID); err != nil {
		return err
	}
	current, err := loadLifecycle(ctx, tx, spaceID, true)
	if err != nil {
		return err
	}
	currentSnapshot := current.Snapshot()
	if currentSnapshot.Phase == spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGED {
		return validateExistingLifecycleTombstone(ctx, tx, snapshot, ownerAccountHMAC, actorAccountHMAC, keyVersion)
	}
	if currentSnapshot.Phase != spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGING ||
		currentSnapshot.Generation != snapshot.Generation || currentSnapshot.DeletionOperationID != snapshot.DeletionOperationID {
		return ErrLifecycleStateTransition
	}
	_, err = tx.Exec(ctx, `INSERT INTO space_deletion_tombstones(
		space_id,owner_account_hmac,actor_account_hmac,key_version,reason,
		scheduled_at,purge_after,purge_decided_at,purged_at,retain_until)
		VALUES($1,$2,$3,$4,'OWNER_REQUESTED',$5::timestamptz AT TIME ZONE 'UTC',
			$6::timestamptz AT TIME ZONE 'UTC',$7::timestamptz AT TIME ZONE 'UTC',
			$8::timestamptz AT TIME ZONE 'UTC',($8::timestamptz AT TIME ZONE 'UTC')+interval '365 days')`,
		spaceID, ownerAccountHMAC, actorAccountHMAC, keyVersion, snapshot.ScheduledAt,
		snapshot.PurgeAfter, snapshot.PurgeDecidedAt, snapshot.DeletedEvent.OccurredAt)
	if err != nil {
		return err
	}
	if err := persistLifecycleSnapshot(ctx, tx, snapshot); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *SpaceStore) ReadReadyLifecycleOutbox(ctx context.Context, limit int, visit func(spacecore.LifecycleOutboxRecord) error) error {
	if limit < 1 || limit > 100 || visit == nil {
		return errors.New("invalid lifecycle outbox read")
	}
	if s == nil || s.Pool == nil {
		return errors.New("space store: pool not configured")
	}
	rows, err := s.Pool.Query(ctx, `SELECT event_id,space_id,deletion_operation_id,generation,event_type,occurred_at,event_bytes,event_sha256
		FROM space_lifecycle_outbox WHERE state='READY' ORDER BY created_at,event_id LIMIT $1`, limit)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		record, err := scanLifecycleOutbox(rows)
		if err != nil {
			return err
		}
		if err := visit(record); err != nil {
			return err
		}
	}
	return rows.Err()
}

type lifecycleOperation struct {
	operationID, accountID, actorProfileID, spaceID uuid.UUID
	sessionEpoch                                    int64
	method                                          string
	requestHash, confirmationHash, proofDigest      []byte
	bindingHash                                     []byte
	authReceiptBytes, authReceiptHash               []byte
}

func (o lifecycleOperation) validate() error {
	if o.operationID == uuid.Nil || o.accountID == uuid.Nil || o.actorProfileID == uuid.Nil || o.spaceID == uuid.Nil ||
		o.sessionEpoch <= 0 || o.method != "DELETE" || len(o.requestHash) != sha256.Size ||
		len(o.confirmationHash) != sha256.Size || len(o.proofDigest) != sha256.Size || len(o.bindingHash) != sha256.Size {
		return ErrLifecycleEvidenceInvalid
	}
	expected := lifecycleOperationBindingHash(o.accountID, o.actorProfileID, o.spaceID, o.operationID, o.sessionEpoch,
		toLifecycleHash(o.requestHash), toLifecycleHash(o.confirmationHash), toLifecycleHash(o.proofDigest))
	if !bytes.Equal(o.bindingHash, expected[:]) {
		return ErrLifecycleEvidenceInvalid
	}
	if (o.authReceiptBytes == nil) != (o.authReceiptHash == nil) {
		return ErrLifecycleEvidenceInvalid
	}
	if o.authReceiptBytes != nil {
		computed := lifecycleHash("voice.auth.v1.ConsumeSpaceDeletionProofResponse", o.authReceiptBytes)
		if !bytes.Equal(computed[:], o.authReceiptHash) || validateLifecycleAuthReceipt(o.authReceiptBytes, o) != nil {
			return ErrLifecycleEvidenceInvalid
		}
	}
	return nil
}

func loadLifecycleOperation(ctx context.Context, db spaceStoreDB, operationID uuid.UUID) (lifecycleOperation, error) {
	var operation lifecycleOperation
	err := db.QueryRow(ctx, `SELECT operation_id,account_id,actor_profile_id,space_id,session_epoch,method,
		request_sha256,confirmation_name_sha256,proof_digest_sha256,binding_sha256,auth_receipt_bytes,auth_receipt_sha256
		FROM space_lifecycle_operations WHERE operation_id=$1`, operationID).Scan(
		&operation.operationID, &operation.accountID, &operation.actorProfileID, &operation.spaceID,
		&operation.sessionEpoch, &operation.method, &operation.requestHash, &operation.confirmationHash,
		&operation.proofDigest, &operation.bindingHash, &operation.authReceiptBytes, &operation.authReceiptHash)
	return operation, err
}

func validateStoredLifecycleOperation(ctx context.Context, db spaceStoreDB, operationID, accountID, actorProfileID, spaceID uuid.UUID, sessionEpoch int64, requestHash, confirmationHash, proofDigest [sha256.Size]byte) error {
	stored, err := loadLifecycleOperation(ctx, db, operationID)
	if err != nil {
		return err
	}
	if err := stored.validate(); err != nil {
		return err
	}
	if stored.accountID != accountID || stored.actorProfileID != actorProfileID || stored.spaceID != spaceID ||
		stored.sessionEpoch != sessionEpoch || !bytes.Equal(stored.requestHash, requestHash[:]) ||
		!bytes.Equal(stored.confirmationHash, confirmationHash[:]) || !bytes.Equal(stored.proofDigest, proofDigest[:]) {
		return ErrLifecycleConflict
	}
	return nil
}

func lifecycleOperationBindingHash(accountID, actorProfileID, spaceID, operationID uuid.UUID, sessionEpoch int64, requestHash, confirmationHash, proofDigest [sha256.Size]byte) [sha256.Size]byte {
	var wire []byte
	wire = appendLifecycleBytes(wire, 1, accountID[:])
	wire = appendLifecycleBytes(wire, 2, actorProfileID[:])
	wire = protowire.AppendTag(wire, 3, protowire.VarintType)
	wire = protowire.AppendVarint(wire, uint64(sessionEpoch))
	wire = appendLifecycleBytes(wire, 4, spaceID[:])
	wire = appendLifecycleBytes(wire, 5, operationID[:])
	wire = appendLifecycleBytes(wire, 6, requestHash[:])
	wire = appendLifecycleBytes(wire, 7, confirmationHash[:])
	wire = appendLifecycleBytes(wire, 8, proofDigest[:])
	return lifecycleHash("voice.space.store.v1.LifecycleOperationBinding", wire)
}

func validateLifecycleAuthReceipt(wrapper []byte, operation lifecycleOperation) error {
	inner, err := lifecycleSingleBytesField(wrapper, 1)
	if err != nil {
		return ErrLifecycleEvidenceInvalid
	}
	fields, err := parseLifecycleAuthReceipt(inner)
	if err != nil || fields.version != 1 || fields.receiptID == "" || fields.operationID != operation.operationID.String() {
		return ErrLifecycleEvidenceInvalid
	}
	if !canonicalLifecycleAuthFactors(fields.factors) {
		return ErrLifecycleEvidenceInvalid
	}
	expectedBinding := lifecycleAuthBindingHash(operation, fields.factors)
	if !bytes.Equal(fields.bindingHash, expectedBinding[:]) || !bytes.Equal(fields.confirmationHash, operation.confirmationHash) || len(fields.consumedAt) == 0 || len(fields.factors) == 0 {
		return ErrLifecycleEvidenceInvalid
	}
	if _, err := uuid.Parse(fields.receiptID); err != nil {
		return ErrLifecycleEvidenceInvalid
	}
	var consumedAt timestamppb.Timestamp
	if err := proto.Unmarshal(fields.consumedAt, &consumedAt); err != nil || consumedAt.CheckValid() != nil {
		return ErrLifecycleEvidenceInvalid
	}
	return nil
}

type lifecycleAuthReceiptFields struct {
	version                       uint64
	receiptID, operationID        string
	bindingHash, confirmationHash []byte
	consumedAt                    []byte
	factors                       []uint64
	factorsSeen                   bool
}

func parseLifecycleAuthReceipt(raw []byte) (lifecycleAuthReceiptFields, error) {
	var result lifecycleAuthReceiptFields
	for len(raw) > 0 {
		number, typ, n := protowire.ConsumeTag(raw)
		if n < 0 {
			return result, protowire.ParseError(n)
		}
		raw = raw[n:]
		switch number {
		case 1:
			value, consumed := protowire.ConsumeVarint(raw)
			if typ != protowire.VarintType || consumed < 0 {
				return result, ErrLifecycleEvidenceInvalid
			}
			result.version = value
			raw = raw[consumed:]
		case 2, 3, 4, 5, 6, 7:
			value, consumed := protowire.ConsumeBytes(raw)
			if typ != protowire.BytesType || consumed < 0 {
				return result, ErrLifecycleEvidenceInvalid
			}
			switch number {
			case 2:
				result.receiptID = string(value)
			case 3:
				result.operationID = string(value)
			case 4:
				result.bindingHash = bytes.Clone(value)
			case 5:
				result.confirmationHash = bytes.Clone(value)
			case 6:
				result.consumedAt = bytes.Clone(value)
			case 7:
				if result.factorsSeen {
					return result, ErrLifecycleEvidenceInvalid
				}
				result.factorsSeen = true
				parsedFactors, parseErr := parseLifecycleAuthFactors(value)
				if parseErr != nil {
					return result, parseErr
				}
				result.factors = parsedFactors
			}
			raw = raw[consumed:]
		default:
			return result, ErrLifecycleEvidenceInvalid
		}
	}
	return result, nil
}

func lifecycleAuthBindingHash(operation lifecycleOperation, factors []uint64) [sha256.Size]byte {
	var wire []byte
	wire = protowire.AppendTag(wire, 1, protowire.VarintType)
	wire = protowire.AppendVarint(wire, 1)
	wire = appendLifecycleString(wire, 2, operation.accountID.String())
	wire = appendLifecycleString(wire, 3, operation.actorProfileID.String())
	wire = protowire.AppendTag(wire, 4, protowire.VarintType)
	wire = protowire.AppendVarint(wire, uint64(operation.sessionEpoch))
	wire = appendLifecycleString(wire, 5, operation.spaceID.String())
	wire = appendLifecycleString(wire, 6, operation.operationID.String())
	wire = appendLifecycleBytes(wire, 7, operation.confirmationHash)
	wire = appendLifecycleBytes(wire, 8, operation.proofDigest)
	wire = appendLifecycleBytes(wire, 9, marshalLifecycleAuthFactors(factors))
	wire = protowire.AppendTag(wire, 10, protowire.VarintType)
	wire = protowire.AppendVarint(wire, 1)
	return lifecycleHash("voice.auth.v1.SpaceDeletionProofBinding", wire)
}

func parseLifecycleAuthFactors(raw []byte) ([]uint64, error) {
	if len(raw) == 0 {
		return nil, ErrLifecycleEvidenceInvalid
	}
	factors := make([]uint64, 0, 2)
	for len(raw) > 0 {
		factor, consumed := protowire.ConsumeVarint(raw)
		if consumed < 0 {
			return nil, ErrLifecycleEvidenceInvalid
		}
		factors = append(factors, factor)
		raw = raw[consumed:]
	}
	return factors, nil
}

func marshalLifecycleAuthFactors(factors []uint64) []byte {
	var packed []byte
	for _, factor := range factors {
		packed = protowire.AppendVarint(packed, factor)
	}
	return packed
}

func canonicalLifecycleAuthFactors(factors []uint64) bool {
	if len(factors) == 1 {
		return factors[0] == 1
	}
	if len(factors) != 2 || factors[0] != 1 {
		return false
	}
	return factors[1] == 2 || factors[1] == 3
}

func persistLifecycleSnapshot(ctx context.Context, db spaceStoreDB, snapshot spacecore.LifecycleSnapshot) error {
	spaceID, err := canonicalLifecycleUUID(snapshot.SpaceID)
	if err != nil {
		return err
	}
	operationID, err := canonicalLifecycleUUID(snapshot.DeletionOperationID)
	if err != nil {
		return err
	}
	if snapshot.Generation == 0 {
		return ErrLifecycleEvidenceInvalid
	}
	if err := lockLifecycleSpace(ctx, db, spaceID); err != nil {
		return err
	}

	phase := lifecyclePhaseName(snapshot.Phase)
	if phase == "" {
		return ErrLifecycleEvidenceInvalid
	}
	var existingPhase string
	var existingGeneration int64
	err = db.QueryRow(ctx, `SELECT phase,generation FROM space_lifecycle_aggregates WHERE space_id=$1 FOR UPDATE`, spaceID).Scan(&existingPhase, &existingGeneration)
	if errors.Is(err, pgx.ErrNoRows) {
		if snapshot.Phase != spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_SCHEDULE_PENDING && snapshot.Phase != spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_FREEZE_PENDING {
			return ErrLifecycleStateTransition
		}
		_, err = db.Exec(ctx, `INSERT INTO space_lifecycle_aggregates(
			space_id,deletion_operation_id,phase,generation,manifest_id,manifest_sha256,manifest_item_count,
			scheduled_at,purge_after,purge_decided_at,completed_at,local_purge_completed)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, spaceID, operationID, phase,
			int64(snapshot.Generation), lifecycleManifestID(snapshot.Manifest), lifecycleManifestHash(snapshot.Manifest),
			lifecycleManifestCount(snapshot.Manifest), lifecycleNullTime(snapshot.ScheduledAt), lifecycleNullTime(snapshot.PurgeAfter),
			lifecycleNullTime(snapshot.PurgeDecidedAt), lifecycleSnapshotCompletedAt(snapshot), snapshot.LocalPurgeCompleted)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		if !allowedLifecycleTransition(existingPhase, existingGeneration, snapshot.Phase, snapshot.Generation) {
			return ErrLifecycleStateTransition
		}
		command, updateErr := db.Exec(ctx, `UPDATE space_lifecycle_aggregates SET
			deletion_operation_id=$2,phase=$3,generation=$4,manifest_id=$5,manifest_sha256=$6,manifest_item_count=$7,
			scheduled_at=$8,purge_after=$9,purge_decided_at=$10,completed_at=$11,local_purge_completed=$12,updated_at=clock_timestamp()
			WHERE space_id=$1 AND phase=$13 AND generation=$14`, spaceID, operationID, phase, int64(snapshot.Generation),
			lifecycleManifestID(snapshot.Manifest), lifecycleManifestHash(snapshot.Manifest), lifecycleManifestCount(snapshot.Manifest),
			lifecycleNullTime(snapshot.ScheduledAt), lifecycleNullTime(snapshot.PurgeAfter), lifecycleNullTime(snapshot.PurgeDecidedAt),
			lifecycleSnapshotCompletedAt(snapshot), snapshot.LocalPurgeCompleted, existingPhase, existingGeneration)
		if updateErr != nil {
			return updateErr
		}
		if command.RowsAffected() != 1 {
			return ErrLifecycleStateTransition
		}
	}
	if err := persistLifecycleParticipants(ctx, db, snapshot, spaceID, operationID); err != nil {
		return err
	}
	return persistLifecycleOutbox(ctx, db, snapshot, spaceID, operationID)
}

func persistLifecycleParticipants(ctx context.Context, db spaceStoreDB, snapshot spacecore.LifecycleSnapshot, spaceID, operationID uuid.UUID) error {
	state, fence := lifecycleFenceState(snapshot.Phase)
	if fence {
		for _, participant := range lifecycleParticipants {
			receipt := snapshot.FenceReceipts[participant]
			if receipt == nil {
				continue
			}
			request := &commonv1.SpaceLifecycleFenceRequest{ProtocolVersion: 1, SpaceId: snapshot.SpaceID,
				DeletionOperationId: snapshot.DeletionOperationID, Generation: snapshot.Generation,
				DesiredState: state, Manifest: proto.Clone(snapshot.Manifest).(*commonv1.ManifestBinding)}
			requestBytes, requestHash, err := lifecycleWrappedEvidence(lifecycleParticipantPackages[participant]+".ApplySpaceLifecycleFenceRequest", request)
			if err != nil {
				return err
			}
			var receiptBytes, receiptHash []byte
			var completedAt any
			progress := "NOT_STARTED"
			if receipt != nil {
				receiptBytes, err = proto.MarshalOptions{Deterministic: true}.Marshal(receipt)
				if err != nil {
					return err
				}
				hash := lifecycleHash("voice.common.v1.SpaceLifecycleFenceReceipt", receiptBytes)
				receiptHash = hash[:]
				completedAt = receipt.GetAppliedAt().AsTime().UTC()
				progress = "COMPLETE"
			}
			if err := upsertLifecycleParticipant(ctx, db, spaceID, operationID, snapshot.Generation, participant, "FENCE", progress,
				requestBytes, requestHash[:], receiptBytes, receiptHash, snapshot.Manifest, completedAt); err != nil {
				return err
			}
		}
	}
	if snapshot.Phase == spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGING || snapshot.Phase == spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGED {
		roleRequest := &rolev1.RetireSpaceRequest{ProtocolVersion: 1, SpaceId: snapshot.SpaceID,
			DeletionOperationId: snapshot.DeletionOperationID, Generation: snapshot.Generation,
			PurgeDecidedAt: timestamppb.New(snapshot.PurgeDecidedAt), Manifest: proto.Clone(snapshot.Manifest).(*commonv1.ManifestBinding)}
		requestBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(roleRequest)
		if err != nil {
			return err
		}
		requestHash := lifecycleHash("voice.role.v1.RetireSpaceRequest", requestBytes)
		var receiptBytes, receiptHash []byte
		var completedAt any
		progress := "NOT_STARTED"
		if snapshot.RoleReceipt != nil {
			receiptBytes, err = proto.MarshalOptions{Deterministic: true}.Marshal(snapshot.RoleReceipt)
			if err != nil {
				return err
			}
			hash := lifecycleHash("voice.role.v1.RetireSpaceReceipt", receiptBytes)
			receiptHash = hash[:]
			completedAt = snapshot.RoleReceipt.GetRetiredAt().AsTime().UTC()
			progress = "COMPLETE"
		}
		if err := upsertLifecycleParticipant(ctx, db, spaceID, operationID, snapshot.Generation,
			commonv1.ParticipantId_PARTICIPANT_ID_ROLE, "ROLE_RETIREMENT", progress, requestBytes,
			requestHash[:], receiptBytes, receiptHash, snapshot.Manifest, completedAt); err != nil {
			return err
		}

		for _, participant := range lifecycleParticipants[1:] {
			request := &commonv1.SpacePurgeRequest{ProtocolVersion: 1, SpaceId: snapshot.SpaceID,
				DeletionOperationId: snapshot.DeletionOperationID, Generation: snapshot.Generation,
				PurgeDecidedAt: timestamppb.New(snapshot.PurgeDecidedAt), ParticipantId: participant,
				Manifest: proto.Clone(snapshot.Manifest).(*commonv1.ManifestBinding)}
			requestBytes, requestHash, err := lifecycleWrappedEvidence(lifecycleParticipantPackages[participant]+".PurgeSpaceRequest", request)
			if err != nil {
				return err
			}
			var receiptBytes, receiptHash []byte
			var completedAt any
			progress := "NOT_STARTED"
			if receipt := snapshot.PurgeReceipts[participant]; receipt != nil {
				receiptBytes, err = proto.MarshalOptions{Deterministic: true}.Marshal(receipt)
				if err != nil {
					return err
				}
				hash := lifecycleHash("voice.common.v1.SpacePurgeReceipt", receiptBytes)
				receiptHash = hash[:]
				completedAt = receipt.GetCompletedAt().AsTime().UTC()
				progress = "COMPLETE"
			}
			if err := upsertLifecycleParticipant(ctx, db, spaceID, operationID, snapshot.Generation,
				participant, "PURGE", progress, requestBytes, requestHash[:], receiptBytes, receiptHash,
				snapshot.Manifest, completedAt); err != nil {
				return err
			}
		}
	}
	return nil
}

func upsertLifecycleParticipant(ctx context.Context, db spaceStoreDB, spaceID, operationID uuid.UUID, generation uint64,
	participant commonv1.ParticipantId, kind, progress string, requestBytes, requestHash, receiptBytes, receiptHash []byte,
	manifest *commonv1.ManifestBinding, completedAt any) error {
	command, err := db.Exec(ctx, `INSERT INTO space_lifecycle_participants(
		space_id,deletion_operation_id,generation,participant_id,request_kind,progress,
		request_bytes,request_sha256,receipt_bytes,receipt_sha256,manifest_id,manifest_sha256,manifest_item_count,completed_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		ON CONFLICT (space_id,generation,participant_id,request_kind) DO UPDATE SET
		progress=EXCLUDED.progress,receipt_bytes=EXCLUDED.receipt_bytes,
		receipt_sha256=EXCLUDED.receipt_sha256,completed_at=EXCLUDED.completed_at
		WHERE space_lifecycle_participants.deletion_operation_id=EXCLUDED.deletion_operation_id
		AND space_lifecycle_participants.request_bytes=EXCLUDED.request_bytes
		AND space_lifecycle_participants.request_sha256=EXCLUDED.request_sha256
		AND space_lifecycle_participants.manifest_id=EXCLUDED.manifest_id
		AND space_lifecycle_participants.manifest_sha256=EXCLUDED.manifest_sha256
		AND space_lifecycle_participants.manifest_item_count=EXCLUDED.manifest_item_count
		AND (space_lifecycle_participants.receipt_bytes IS NULL OR space_lifecycle_participants.receipt_bytes=EXCLUDED.receipt_bytes)
		AND (space_lifecycle_participants.receipt_sha256 IS NULL OR space_lifecycle_participants.receipt_sha256=EXCLUDED.receipt_sha256)`,
		spaceID, operationID, int64(generation), int16(participant), kind, progress, requestBytes, requestHash,
		receiptBytes, receiptHash, uuid.MustParse(manifest.GetManifestId()), manifest.GetManifestSha256(), int64(manifest.GetItemCount()), completedAt)
	if err != nil {
		return err
	}
	if command.RowsAffected() != 1 {
		return ErrLifecycleConflict
	}
	return nil
}

func persistLifecycleOutbox(ctx context.Context, db spaceStoreDB, snapshot spacecore.LifecycleSnapshot, spaceID, operationID uuid.UUID) error {
	items := make([]struct {
		kind  string
		event *spacecore.LifecycleOutboxRecord
	}, 0, 3)
	if snapshot.Phase != spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_SCHEDULE_PENDING {
		items = append(items, struct {
			kind  string
			event *spacecore.LifecycleOutboxRecord
		}{"space.deletion_scheduled", snapshot.ScheduleEvent})
	}
	if snapshot.Phase == spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_RESTORE_DECIDED || snapshot.RestoreEvent != nil {
		items = append(items, struct {
			kind  string
			event *spacecore.LifecycleOutboxRecord
		}{"space.restored", snapshot.RestoreEvent})
	}
	if snapshot.Phase == spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGE_DECIDED ||
		snapshot.Phase == spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGING || snapshot.DeletedEvent != nil {
		items = append(items, struct {
			kind  string
			event *spacecore.LifecycleOutboxRecord
		}{"space.deleted", snapshot.DeletedEvent})
	}
	for _, item := range items {
		projected := spacecore.LifecycleOutboxRecord{}
		var err error
		if item.event != nil {
			projected = *item.event
		} else {
			stub, stubErr := spacecore.RestoreLifecycleAggregate(snapshot)
			if stubErr != nil {
				return stubErr
			}
			projected, err = stub.ProjectedOutboxRecord(item.kind)
			if err != nil {
				return err
			}
		}
		state := "BLOCKED"
		var occurredAt any
		var eventBytes, eventHash []byte
		if item.event != nil {
			state = "READY"
			occurredAt = projected.OccurredAt
			eventBytes = marshalLifecycleOutbox(projected)
			hash := lifecycleHash("voice.space.store.v1.LifecycleOutboxRecord", eventBytes)
			eventHash = hash[:]
		}
		command, execErr := db.Exec(ctx, `INSERT INTO space_lifecycle_outbox(
			event_id,space_id,deletion_operation_id,generation,event_type,state,occurred_at,event_bytes,event_sha256)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)
			ON CONFLICT (event_id) DO UPDATE SET state=EXCLUDED.state,occurred_at=EXCLUDED.occurred_at,
			event_bytes=EXCLUDED.event_bytes,event_sha256=EXCLUDED.event_sha256
			WHERE space_lifecycle_outbox.space_id=EXCLUDED.space_id
			AND space_lifecycle_outbox.deletion_operation_id=EXCLUDED.deletion_operation_id
			AND space_lifecycle_outbox.generation=EXCLUDED.generation
			AND space_lifecycle_outbox.event_type=EXCLUDED.event_type
			AND space_lifecycle_outbox.state IN ('BLOCKED',EXCLUDED.state)
			AND (space_lifecycle_outbox.event_bytes IS NULL OR space_lifecycle_outbox.event_bytes=EXCLUDED.event_bytes)
			AND (space_lifecycle_outbox.event_sha256 IS NULL OR space_lifecycle_outbox.event_sha256=EXCLUDED.event_sha256)`,
			uuid.MustParse(projected.EventID), spaceID, operationID, int64(projected.Generation), item.kind,
			state, occurredAt, eventBytes, eventHash)
		if execErr != nil {
			return execErr
		}
		if command.RowsAffected() != 1 {
			return ErrLifecycleConflict
		}
	}
	return nil
}

func loadLifecycle(ctx context.Context, db spaceStoreDB, spaceID uuid.UUID, forUpdate bool) (*spacecore.LifecycleAggregate, error) {
	query := `SELECT deletion_operation_id,phase,generation,manifest_id,manifest_sha256,manifest_item_count,
		scheduled_at,purge_after,purge_decided_at,local_purge_completed
		FROM space_lifecycle_aggregates WHERE space_id=$1`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	var operationID uuid.UUID
	var phaseName string
	var generation int64
	var manifestID *uuid.UUID
	var manifestHash []byte
	var manifestCount *int64
	var scheduledAt, purgeAfter, purgeDecidedAt *time.Time
	var localPurgeCompleted bool
	if err := db.QueryRow(ctx, query, spaceID).Scan(&operationID, &phaseName, &generation, &manifestID, &manifestHash,
		&manifestCount, &scheduledAt, &purgeAfter, &purgeDecidedAt, &localPurgeCompleted); err != nil {
		return nil, err
	}
	phase, err := parseLifecyclePhase(phaseName)
	if err != nil {
		return nil, err
	}
	snapshot := spacecore.LifecycleSnapshot{SpaceID: spaceID.String(), DeletionOperationID: operationID.String(),
		Phase: phase, Generation: uint64(generation), LocalPurgeCompleted: localPurgeCompleted}
	if manifestID != nil && manifestCount != nil {
		snapshot.Manifest = &commonv1.ManifestBinding{ManifestId: manifestID.String(), ManifestSha256: bytes.Clone(manifestHash), ItemCount: uint64(*manifestCount)}
	}
	if scheduledAt != nil {
		snapshot.ScheduledAt = scheduledAt.UTC()
	}
	if purgeAfter != nil {
		snapshot.PurgeAfter = purgeAfter.UTC()
	}
	if purgeDecidedAt != nil {
		snapshot.PurgeDecidedAt = purgeDecidedAt.UTC()
	}

	if operation, loadErr := loadLifecycleOperation(ctx, db, operationID); loadErr == nil {
		if operation.spaceID != spaceID {
			return nil, ErrLifecycleConflict
		}
		if err := operation.validate(); err != nil {
			return nil, err
		}
	} else if !errors.Is(loadErr, pgx.ErrNoRows) {
		return nil, loadErr
	}

	if err := loadLifecycleParticipants(ctx, db, &snapshot, spaceID, operationID); err != nil {
		return nil, err
	}
	if err := loadLifecycleOutbox(ctx, db, &snapshot, spaceID, operationID); err != nil {
		return nil, err
	}
	return spacecore.RestoreLifecycleAggregate(snapshot)
}

func loadLifecycleParticipants(ctx context.Context, db spaceStoreDB, snapshot *spacecore.LifecycleSnapshot, spaceID, operationID uuid.UUID) error {
	rows, err := db.Query(ctx, `SELECT participant_id,request_kind,progress,request_bytes,request_sha256,
		receipt_bytes,receipt_sha256,manifest_id,manifest_sha256,manifest_item_count,completed_at
		FROM space_lifecycle_participants WHERE space_id=$1 AND deletion_operation_id=$2 AND generation=$3
		ORDER BY participant_id,request_kind`, spaceID, operationID, int64(snapshot.Generation))
	if err != nil {
		return err
	}
	defer rows.Close()
	snapshot.FenceReceipts = make(map[commonv1.ParticipantId]*commonv1.SpaceLifecycleFenceReceipt)
	snapshot.PurgeReceipts = make(map[commonv1.ParticipantId]*commonv1.SpacePurgeReceipt)
	for rows.Next() {
		var participantValue int16
		var kind, progress string
		var requestBytes, requestHash, receiptBytes, receiptHash, manifestHash []byte
		var manifestID uuid.UUID
		var manifestCount int64
		var completedAt *time.Time
		if err := rows.Scan(&participantValue, &kind, &progress, &requestBytes, &requestHash, &receiptBytes, &receiptHash,
			&manifestID, &manifestHash, &manifestCount, &completedAt); err != nil {
			return err
		}
		participant := commonv1.ParticipantId(participantValue)
		if snapshot.Manifest == nil || manifestID.String() != snapshot.Manifest.GetManifestId() ||
			!bytes.Equal(manifestHash, snapshot.Manifest.GetManifestSha256()) || uint64(manifestCount) != snapshot.Manifest.GetItemCount() {
			return ErrLifecycleEvidenceInvalid
		}
		if err := validateLifecycleParticipantEvidence(snapshot, participant, kind, progress, requestBytes, requestHash,
			receiptBytes, receiptHash, completedAt); err != nil {
			return err
		}
		switch kind {
		case "FENCE":
			if receiptBytes != nil {
				receipt := new(commonv1.SpaceLifecycleFenceReceipt)
				if err := proto.Unmarshal(receiptBytes, receipt); err != nil {
					return err
				}
				snapshot.FenceReceipts[participant] = receipt
			}
		case "PURGE":
			if receiptBytes != nil {
				receipt := new(commonv1.SpacePurgeReceipt)
				if err := proto.Unmarshal(receiptBytes, receipt); err != nil {
					return err
				}
				snapshot.PurgeReceipts[participant] = receipt
			}
		case "ROLE_RETIREMENT":
			if receiptBytes != nil {
				receipt := new(rolev1.RetireSpaceReceipt)
				if err := proto.Unmarshal(receiptBytes, receipt); err != nil {
					return err
				}
				snapshot.RoleReceipt = receipt
			}
		default:
			return ErrLifecycleEvidenceInvalid
		}
	}
	return rows.Err()
}

func validateLifecycleParticipantEvidence(snapshot *spacecore.LifecycleSnapshot, participant commonv1.ParticipantId, kind, progress string,
	requestBytes, requestHash, receiptBytes, receiptHash []byte, completedAt *time.Time) error {
	if !lifecycleParticipantValid(participant) || len(requestHash) != sha256.Size ||
		(receiptBytes == nil) != (receiptHash == nil) || (receiptBytes == nil) != (completedAt == nil) ||
		(progress == "COMPLETE") != (receiptBytes != nil) {
		return ErrLifecycleEvidenceInvalid
	}
	var requestFQN, receiptFQN string
	switch kind {
	case "FENCE":
		requestFQN = lifecycleParticipantPackages[participant] + ".ApplySpaceLifecycleFenceRequest"
		receiptFQN = "voice.common.v1.SpaceLifecycleFenceReceipt"
	case "PURGE":
		if participant == commonv1.ParticipantId_PARTICIPANT_ID_ROLE {
			return ErrLifecycleEvidenceInvalid
		}
		requestFQN = lifecycleParticipantPackages[participant] + ".PurgeSpaceRequest"
		receiptFQN = "voice.common.v1.SpacePurgeReceipt"
	case "ROLE_RETIREMENT":
		if participant != commonv1.ParticipantId_PARTICIPANT_ID_ROLE {
			return ErrLifecycleEvidenceInvalid
		}
		requestFQN = "voice.role.v1.RetireSpaceRequest"
		receiptFQN = "voice.role.v1.RetireSpaceReceipt"
	default:
		return ErrLifecycleEvidenceInvalid
	}
	computedRequest := lifecycleHash(requestFQN, requestBytes)
	if !bytes.Equal(computedRequest[:], requestHash) {
		return ErrLifecycleEvidenceInvalid
	}
	if err := validateLifecycleRequestBytes(snapshot, participant, kind, requestBytes); err != nil {
		return err
	}
	if receiptBytes != nil {
		computedReceipt := lifecycleHash(receiptFQN, receiptBytes)
		if !bytes.Equal(computedReceipt[:], receiptHash) {
			return ErrLifecycleEvidenceInvalid
		}
	}
	return nil
}

func validateLifecycleRequestBytes(snapshot *spacecore.LifecycleSnapshot, participant commonv1.ParticipantId, kind string, raw []byte) error {
	requestRaw := raw
	if kind != "ROLE_RETIREMENT" {
		var err error
		requestRaw, err = lifecycleSingleBytesField(raw, 1)
		if err != nil {
			return err
		}
	}
	switch kind {
	case "FENCE":
		request := new(commonv1.SpaceLifecycleFenceRequest)
		if err := proto.Unmarshal(requestRaw, request); err != nil {
			return err
		}
		state, ok := lifecyclePersistedFenceState(snapshot.Phase)
		if !ok || request.GetProtocolVersion() != 1 || request.GetSpaceId() != snapshot.SpaceID || request.GetDeletionOperationId() != snapshot.DeletionOperationID || request.GetGeneration() != snapshot.Generation || request.GetDesiredState() != state || !proto.Equal(request.GetManifest(), snapshot.Manifest) {
			return ErrLifecycleEvidenceInvalid
		}
	case "PURGE":
		request := new(commonv1.SpacePurgeRequest)
		if err := proto.Unmarshal(requestRaw, request); err != nil {
			return err
		}
		if request.GetProtocolVersion() != 1 || request.GetSpaceId() != snapshot.SpaceID || request.GetDeletionOperationId() != snapshot.DeletionOperationID || request.GetGeneration() != snapshot.Generation || request.GetParticipantId() != participant || !request.GetPurgeDecidedAt().AsTime().Equal(snapshot.PurgeDecidedAt) || !proto.Equal(request.GetManifest(), snapshot.Manifest) {
			return ErrLifecycleEvidenceInvalid
		}
	case "ROLE_RETIREMENT":
		request := new(rolev1.RetireSpaceRequest)
		if err := proto.Unmarshal(requestRaw, request); err != nil {
			return err
		}
		if request.GetProtocolVersion() != 1 || request.GetSpaceId() != snapshot.SpaceID || request.GetDeletionOperationId() != snapshot.DeletionOperationID || request.GetGeneration() != snapshot.Generation || !request.GetPurgeDecidedAt().AsTime().Equal(snapshot.PurgeDecidedAt) || !proto.Equal(request.GetManifest(), snapshot.Manifest) {
			return ErrLifecycleEvidenceInvalid
		}
	}
	return nil
}

func loadLifecycleOutbox(ctx context.Context, db spaceStoreDB, snapshot *spacecore.LifecycleSnapshot, spaceID, operationID uuid.UUID) error {
	rows, err := db.Query(ctx, `SELECT event_id,space_id,deletion_operation_id,generation,event_type,occurred_at,event_bytes,event_sha256,state
		FROM space_lifecycle_outbox WHERE space_id=$1 AND deletion_operation_id=$2 ORDER BY created_at,event_id`, spaceID, operationID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var eventID, rowSpaceID, rowOperationID uuid.UUID
		var generation int64
		var eventType, state string
		var occurredAt *time.Time
		var eventBytes, eventHash []byte
		if err := rows.Scan(&eventID, &rowSpaceID, &rowOperationID, &generation, &eventType, &occurredAt, &eventBytes, &eventHash, &state); err != nil {
			return err
		}
		if state == "BLOCKED" {
			if occurredAt != nil || eventBytes != nil || eventHash != nil {
				return ErrLifecycleEvidenceInvalid
			}
			continue
		}
		if state != "READY" && state != "DELIVERED" {
			return ErrLifecycleEvidenceInvalid
		}
		if occurredAt == nil {
			return ErrLifecycleEvidenceInvalid
		}
		record := spacecore.LifecycleOutboxRecord{EventID: eventID.String(), EventType: eventType, SpaceID: rowSpaceID.String(), DeletionOperationID: rowOperationID.String(), Generation: uint64(generation), OccurredAt: occurredAt.UTC()}
		computedBytes := marshalLifecycleOutbox(record)
		computedHash := lifecycleHash("voice.space.store.v1.LifecycleOutboxRecord", computedBytes)
		if !bytes.Equal(computedBytes, eventBytes) || !bytes.Equal(computedHash[:], eventHash) {
			return ErrLifecycleEvidenceInvalid
		}
		switch eventType {
		case "space.deletion_scheduled":
			snapshot.ScheduleEvent = &record
		case "space.restored":
			snapshot.RestoreEvent = &record
		case "space.deleted":
			snapshot.DeletedEvent = &record
		default:
			return ErrLifecycleEvidenceInvalid
		}
	}
	return rows.Err()
}

func scanLifecycleOutbox(row interface{ Scan(...any) error }) (spacecore.LifecycleOutboxRecord, error) {
	var eventID, spaceID, operationID uuid.UUID
	var generation int64
	var eventType string
	var occurredAt time.Time
	var eventBytes, eventHash []byte
	if err := row.Scan(&eventID, &spaceID, &operationID, &generation, &eventType, &occurredAt, &eventBytes, &eventHash); err != nil {
		return spacecore.LifecycleOutboxRecord{}, err
	}
	record := spacecore.LifecycleOutboxRecord{EventID: eventID.String(), EventType: eventType, SpaceID: spaceID.String(), DeletionOperationID: operationID.String(), Generation: uint64(generation), OccurredAt: occurredAt.UTC()}
	encoded := marshalLifecycleOutbox(record)
	hash := lifecycleHash("voice.space.store.v1.LifecycleOutboxRecord", encoded)
	if !bytes.Equal(encoded, eventBytes) || !bytes.Equal(hash[:], eventHash) {
		return spacecore.LifecycleOutboxRecord{}, ErrLifecycleEvidenceInvalid
	}
	return record, nil
}

func validateExistingLifecycleTombstone(ctx context.Context, db spaceStoreDB, snapshot spacecore.LifecycleSnapshot, ownerHMAC, actorHMAC []byte, keyVersion string) error {
	var gotOwner, gotActor []byte
	var gotKey string
	var purgedAt time.Time
	err := db.QueryRow(ctx, `SELECT owner_account_hmac,actor_account_hmac,key_version,purged_at FROM space_deletion_tombstones WHERE space_id=$1`, uuid.MustParse(snapshot.SpaceID)).Scan(&gotOwner, &gotActor, &gotKey, &purgedAt)
	if err != nil {
		return err
	}
	if !bytes.Equal(gotOwner, ownerHMAC) || !bytes.Equal(gotActor, actorHMAC) || gotKey != keyVersion || snapshot.DeletedEvent == nil || !purgedAt.Equal(snapshot.DeletedEvent.OccurredAt) {
		return ErrLifecycleConflict
	}
	return nil
}

func allowedLifecycleTransition(existing string, generation int64, target spacev1.SpaceDeletionPhase, targetGeneration uint64) bool {
	targetName := lifecyclePhaseName(target)
	tg := int64(targetGeneration)
	if existing == targetName && generation == tg {
		return true
	}
	switch target {
	case spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_FREEZE_PENDING:
		return existing == "SCHEDULE_PENDING" && generation == tg
	case spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_SCHEDULED:
		return existing == "FREEZE_PENDING" && generation == tg
	case spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_RESTORE_DECIDED, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGE_DECIDED:
		return existing == "SCHEDULED" && generation < tg
	case spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGING:
		return existing == "PURGE_DECIDED" && generation == tg
	case spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_LIVE:
		return existing == "RESTORE_DECIDED" && generation == tg
	case spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGED:
		return existing == "PURGING" && generation == tg
	default:
		return false
	}
}

func lifecycleFenceState(phase spacev1.SpaceDeletionPhase) (commonv1.LifecycleFenceState, bool) {
	switch phase {
	case spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_FREEZE_PENDING:
		return commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, true
	case spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_RESTORE_DECIDED:
		return commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE, true
	case spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGE_DECIDED:
		return commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED, true
	default:
		return 0, false
	}
}

func lifecyclePersistedFenceState(phase spacev1.SpaceDeletionPhase) (commonv1.LifecycleFenceState, bool) {
	switch phase {
	case spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_FREEZE_PENDING,
		spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_SCHEDULED:
		return commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, true
	case spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_RESTORE_DECIDED,
		spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_LIVE:
		return commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE, true
	case spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGE_DECIDED,
		spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGING,
		spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGED:
		return commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED, true
	default:
		return 0, false
	}
}

func lifecyclePhaseName(phase spacev1.SpaceDeletionPhase) string {
	const prefix = "SPACE_DELETION_PHASE_"
	value := phase.String()
	if len(value) <= len(prefix) || value[:len(prefix)] != prefix {
		return ""
	}
	return value[len(prefix):]
}

func parseLifecyclePhase(value string) (spacev1.SpaceDeletionPhase, error) {
	number, ok := spacev1.SpaceDeletionPhase_value["SPACE_DELETION_PHASE_"+value]
	if !ok {
		return 0, ErrLifecycleEvidenceInvalid
	}
	return spacev1.SpaceDeletionPhase(number), nil
}

func lockLifecycleSpace(ctx context.Context, db spaceStoreDB, spaceID uuid.UUID) error {
	_, err := db.Exec(ctx, `SELECT pg_advisory_xact_lock($1::bigint)`, spaceMutationAdvisoryKey(spaceID))
	return err
}

func rollbackLifecycleTx(ctx context.Context, tx pgx.Tx) {
	cleanup, cancel := BoundedCleanupContext(ctx)
	defer cancel()
	_ = tx.Rollback(cleanup)
}

func lifecycleWrappedEvidence(fqn string, message proto.Message) ([]byte, [sha256.Size]byte, error) {
	inner, err := proto.MarshalOptions{Deterministic: true}.Marshal(message)
	if err != nil {
		return nil, [sha256.Size]byte{}, err
	}
	wrapper := appendLifecycleBytes(nil, 1, inner)
	return wrapper, lifecycleHash(fqn, wrapper), nil
}

func lifecycleHash(fqn string, raw []byte) [sha256.Size]byte {
	payload := make([]byte, 0, len(fqn)+1+len(raw))
	payload = append(payload, fqn...)
	payload = append(payload, 0)
	payload = append(payload, raw...)
	return sha256.Sum256(payload)
}

func appendLifecycleBytes(target []byte, field protowire.Number, value []byte) []byte {
	target = protowire.AppendTag(target, field, protowire.BytesType)
	return protowire.AppendBytes(target, value)
}
func appendLifecycleString(target []byte, field protowire.Number, value string) []byte {
	target = protowire.AppendTag(target, field, protowire.BytesType)
	return protowire.AppendString(target, value)
}

func lifecycleSingleBytesField(raw []byte, field protowire.Number) ([]byte, error) {
	number, typ, n := protowire.ConsumeTag(raw)
	if n < 0 || number != field || typ != protowire.BytesType {
		return nil, ErrLifecycleEvidenceInvalid
	}
	value, m := protowire.ConsumeBytes(raw[n:])
	if m < 0 || n+m != len(raw) {
		return nil, ErrLifecycleEvidenceInvalid
	}
	return bytes.Clone(value), nil
}

func canonicalLifecycleUUID(value string) (uuid.UUID, error) {
	parsed, err := uuid.Parse(value)
	if err != nil || parsed == uuid.Nil || parsed.String() != value {
		return uuid.Nil, ErrLifecycleEvidenceInvalid
	}
	return parsed, nil
}
func toLifecycleHash(value []byte) [sha256.Size]byte {
	var result [sha256.Size]byte
	copy(result[:], value)
	return result
}
func lifecycleParticipantValid(value commonv1.ParticipantId) bool {
	return value >= commonv1.ParticipantId_PARTICIPANT_ID_ROLE && value <= commonv1.ParticipantId_PARTICIPANT_ID_NOTIFICATION
}
func lifecycleManifestID(value *commonv1.ManifestBinding) any {
	if value == nil {
		return nil
	}
	return uuid.MustParse(value.GetManifestId())
}
func lifecycleManifestHash(value *commonv1.ManifestBinding) any {
	if value == nil {
		return nil
	}
	return value.GetManifestSha256()
}
func lifecycleManifestCount(value *commonv1.ManifestBinding) any {
	if value == nil {
		return nil
	}
	return int64(value.GetItemCount())
}
func lifecycleNullTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value.UTC()
}
func lifecycleSnapshotCompletedAt(value spacecore.LifecycleSnapshot) any {
	if value.DeletedEvent != nil {
		return value.DeletedEvent.OccurredAt.UTC()
	}
	if value.RestoreEvent != nil {
		return value.RestoreEvent.OccurredAt.UTC()
	}
	return nil
}

func marshalLifecycleOutbox(record spacecore.LifecycleOutboxRecord) []byte {
	var wire []byte
	wire = appendLifecycleString(wire, 1, record.EventID)
	wire = appendLifecycleString(wire, 2, record.EventType)
	wire = appendLifecycleString(wire, 3, record.SpaceID)
	wire = appendLifecycleString(wire, 4, record.DeletionOperationID)
	wire = protowire.AppendTag(wire, 5, protowire.VarintType)
	wire = protowire.AppendVarint(wire, record.Generation)
	timestamp, _ := proto.MarshalOptions{Deterministic: true}.Marshal(timestamppb.New(record.OccurredAt))
	wire = appendLifecycleBytes(wire, 6, timestamp)
	return wire
}
