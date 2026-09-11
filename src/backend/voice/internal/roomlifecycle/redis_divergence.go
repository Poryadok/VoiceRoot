package roomlifecycle

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrRedisDivergenceEvidenceOpen = &redisDivergenceEvidenceError{}
var ErrRedisMirrorQuarantined = ErrRedisDivergenceEvidenceOpen

type redisDivergenceEvidenceError struct{}

func (*redisDivergenceEvidenceError) Error() string {
	return "voice lifecycle redis divergence evidence is open"
}

type RedisDivergenceClassification string

const (
	RedisDivergenceOrphan      RedisDivergenceClassification = "orphan"
	RedisDivergenceDecided     RedisDivergenceClassification = "decided"
	RedisDivergenceCompleted   RedisDivergenceClassification = "completed"
	RedisDivergenceQuarantined RedisDivergenceClassification = "quarantined"
)

type RedisDivergenceClass string

const (
	RedisDivergenceLegacySchema       RedisDivergenceClass = "legacy_schema"
	RedisDivergenceMalformed          RedisDivergenceClass = "malformed"
	RedisDivergenceBindingMismatch    RedisDivergenceClass = "binding_mismatch"
	RedisDivergenceOwnerMismatch      RedisDivergenceClass = "owner_mismatch"
	RedisDivergenceStateOrderMismatch RedisDivergenceClass = "state_order_mismatch"
	RedisDivergenceReceiptMismatch    RedisDivergenceClass = "receipt_mismatch"
	RedisDivergenceDeadlineMismatch   RedisDivergenceClass = "deadline_mismatch"
	RedisDivergenceTTLInvalid         RedisDivergenceClass = "ttl_invalid"
)

type RedisDivergenceObservation struct {
	ActorProfileID     uuid.UUID
	OperationID        uuid.UUID
	RedisClass         RedisDivergenceClass
	RedisSchemaVersion *int16
	RedisState         *string
	ObservationDigest  LifecycleDigest
	ObservedAt         time.Time
}

type RedisDivergenceRecord struct {
	DivergenceID         uuid.UUID
	ActorProfileID       uuid.UUID
	OperationID          uuid.UUID
	Classification       RedisDivergenceClassification
	RedisClass           RedisDivergenceClass
	RedisSchemaVersion   *int16
	RedisState           *string
	ObservationDigest    LifecycleDigest
	SubjectProfileID     *uuid.UUID
	ClassifiedPGState    *LifecycleOperationState
	ClassifiedLeaseFence *int64
	FirstObservedAt      time.Time
	LastObservedAt       time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

func validRedisDivergenceObservation(observation RedisDivergenceObservation) bool {
	if observation.ActorProfileID == uuid.Nil || observation.OperationID == uuid.Nil || observation.ObservedAt.IsZero() {
		return false
	}
	switch observation.RedisClass {
	case RedisDivergenceLegacySchema, RedisDivergenceMalformed, RedisDivergenceBindingMismatch,
		RedisDivergenceOwnerMismatch, RedisDivergenceStateOrderMismatch, RedisDivergenceReceiptMismatch,
		RedisDivergenceDeadlineMismatch, RedisDivergenceTTLInvalid:
	default:
		return false
	}
	if observation.RedisSchemaVersion != nil && *observation.RedisSchemaVersion <= 0 {
		return false
	}
	if observation.RedisState != nil && *observation.RedisState != "pending" && *observation.RedisState != "completed" {
		return false
	}
	return true
}

func (store *PostgresLifecycleStore) LoadRedisDivergence(ctx context.Context, actorProfileID, operationID uuid.UUID) (RedisDivergenceRecord, bool, error) {
	if store == nil || store.pool == nil || actorProfileID == uuid.Nil || operationID == uuid.Nil {
		return RedisDivergenceRecord{}, false, ErrUnavailable
	}
	return loadRedisDivergence(ctx, store.pool, actorProfileID, operationID, "")
}

func loadRedisDivergence(ctx context.Context, queryer lifecycleQueryer, actorProfileID, operationID uuid.UUID, suffix string) (RedisDivergenceRecord, bool, error) {
	row := queryer.QueryRow(ctx, `SELECT divergence_id,actor_profile_id,operation_id,classification,redis_class,redis_schema_version,redis_state,observation_digest,subject_profile_id,classified_pg_state,classified_lease_fence,first_observed_at,last_observed_at,created_at,updated_at FROM voice_lifecycle_redis_divergences WHERE actor_profile_id=$1 AND operation_id=$2 AND resolved_at IS NULL `+suffix, actorProfileID, operationID)
	var record RedisDivergenceRecord
	var classification, redisClass string
	var pgState *string
	var digest []byte
	err := row.Scan(&record.DivergenceID, &record.ActorProfileID, &record.OperationID, &classification, &redisClass, &record.RedisSchemaVersion, &record.RedisState, &digest, &record.SubjectProfileID, &pgState, &record.ClassifiedLeaseFence, &record.FirstObservedAt, &record.LastObservedAt, &record.CreatedAt, &record.UpdatedAt)
	if err == pgx.ErrNoRows {
		return RedisDivergenceRecord{}, false, nil
	}
	if err != nil {
		return RedisDivergenceRecord{}, false, mapScanError(err)
	}
	record.Classification = RedisDivergenceClassification(classification)
	record.RedisClass = RedisDivergenceClass(redisClass)
	parsed, err := digestFromBytes(digest)
	if err != nil {
		return RedisDivergenceRecord{}, false, err
	}
	record.ObservationDigest = parsed
	if pgState != nil {
		state, err := parseOperationState(*pgState)
		if err != nil {
			return RedisDivergenceRecord{}, false, err
		}
		record.ClassifiedPGState = &state
	}
	normalizeUTC(&record.FirstObservedAt)
	normalizeUTC(&record.LastObservedAt)
	normalizeUTC(&record.CreatedAt)
	normalizeUTC(&record.UpdatedAt)
	return record, true, nil
}

func loadRedisDivergenceForAdmission(ctx context.Context, tx pgx.Tx, actorProfileID, operationID uuid.UUID) (RedisDivergenceRecord, bool, error) {
	var available bool
	if err := tx.QueryRow(ctx, `SELECT to_regclass('voice_lifecycle_redis_divergences') IS NOT NULL`).Scan(&available); err != nil {
		return RedisDivergenceRecord{}, false, mapScanError(err)
	}
	if !available {
		return RedisDivergenceRecord{}, false, nil
	}
	return loadRedisDivergence(ctx, tx, actorProfileID, operationID, "FOR UPDATE")
}

func parseOperationState(value string) (LifecycleOperationState, error) {
	switch value {
	case "decided":
		return LifecycleOperationDecided, nil
	case "completed":
		return LifecycleOperationCompleted, nil
	case "quarantined":
		return LifecycleOperationQuarantined, nil
	default:
		return 0, ErrInvariant
	}
}

func (store *PostgresLifecycleStore) ClassifyAndRecordRedisDivergence(ctx context.Context, observation RedisDivergenceObservation) (_ RedisDivergenceRecord, err error) {
	if !validRedisDivergenceObservation(observation) {
		return RedisDivergenceRecord{}, ErrInvariant
	}
	observation.ObservedAt = observation.ObservedAt.UTC()
	tx, err := beginLifecycleTx(ctx, store)
	if err != nil {
		return RedisDivergenceRecord{}, err
	}
	defer finishLifecycleTx(ctx, tx, &err)
	if err = advisoryLock(ctx, tx, LifecycleOperationAdvisoryKey(observation.ActorProfileID, observation.OperationID)); err != nil {
		return RedisDivergenceRecord{}, mapWriteError(err)
	}
	existing, found, err := loadRedisDivergence(ctx, tx, observation.ActorProfileID, observation.OperationID, "FOR UPDATE")
	if err != nil {
		return RedisDivergenceRecord{}, err
	}
	if found {
		if existing.RedisClass != observation.RedisClass || existing.ObservationDigest != observation.ObservationDigest || !optionalInt16Equal(existing.RedisSchemaVersion, observation.RedisSchemaVersion) || !optionalStringEqual(existing.RedisState, observation.RedisState) {
			return RedisDivergenceRecord{}, ErrInvariant
		}
		_, err = tx.Exec(ctx, `UPDATE voice_lifecycle_redis_divergences SET last_observed_at=GREATEST(last_observed_at,$1),updated_at=GREATEST(updated_at,$1) WHERE divergence_id=$2`, observation.ObservedAt, existing.DivergenceID)
		if err != nil {
			return RedisDivergenceRecord{}, mapWriteError(err)
		}
		updated, ok, err := loadRedisDivergence(ctx, tx, observation.ActorProfileID, observation.OperationID, "")
		if err != nil || !ok {
			if err == nil {
				err = ErrInvariant
			}
			return RedisDivergenceRecord{}, err
		}
		return updated, nil
	}
	operation, operationFound, err := loadOperation(ctx, tx, observation.ActorProfileID, observation.OperationID, "FOR UPDATE")
	if err != nil {
		return RedisDivergenceRecord{}, err
	}
	classification := RedisDivergenceOrphan
	var subject *uuid.UUID
	var state *LifecycleOperationState
	var fence *int64
	if operationFound {
		subjectValue := operation.SubjectProfileID
		subject = &subjectValue
		classification = RedisDivergenceClassification(operationStateName(operation.State))
		stateValue := operation.State
		state = &stateValue
		fenceValue := operation.LeaseFence
		if operation.State == LifecycleOperationDecided {
			if err = advisoryLock(ctx, tx, LifecycleSubjectAdvisoryKey(operation.SubjectProfileID)); err != nil {
				return RedisDivergenceRecord{}, mapWriteError(err)
			}
			fenceValue++
			tag, execErr := tx.Exec(ctx, `UPDATE voice_lifecycle_operations SET state='quarantined',lease_owner=NULL,lease_until=NULL,lease_fence=$1,quarantine_class='redis_divergence',quarantined_at=$2,updated_at=GREATEST(updated_at,$2) WHERE actor_profile_id=$3 AND operation_id=$4 AND state='decided'`, fenceValue, observation.ObservedAt, observation.ActorProfileID, observation.OperationID)
			if execErr != nil {
				return RedisDivergenceRecord{}, mapWriteError(execErr)
			}
			if tag.RowsAffected() != 1 {
				return RedisDivergenceRecord{}, ErrInvariant
			}
		}
		fence = &fenceValue
	}
	incidentID := uuid.New()
	_, err = tx.Exec(ctx, `INSERT INTO voice_lifecycle_redis_divergences(divergence_id,actor_profile_id,operation_id,classification,redis_class,redis_schema_version,redis_state,observation_digest,subject_profile_id,classified_pg_state,classified_lease_fence,first_observed_at,last_observed_at,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$12,$12,$12)`, incidentID, observation.ActorProfileID, observation.OperationID, string(classification), string(observation.RedisClass), observation.RedisSchemaVersion, observation.RedisState, observation.ObservationDigest[:], subject, operationStatePointerName(state), fence, observation.ObservedAt)
	if err != nil {
		return RedisDivergenceRecord{}, mapWriteError(err)
	}
	record, ok, err := loadRedisDivergence(ctx, tx, observation.ActorProfileID, observation.OperationID, "")
	if err != nil || !ok {
		if err == nil {
			err = ErrInvariant
		}
		return RedisDivergenceRecord{}, err
	}
	return record, nil
}

func optionalInt16Equal(left, right *int16) bool {
	return (left == nil && right == nil) || (left != nil && right != nil && *left == *right)
}
func optionalStringEqual(left, right *string) bool {
	return (left == nil && right == nil) || (left != nil && right != nil && *left == *right)
}
func operationStateName(state LifecycleOperationState) string {
	switch state {
	case LifecycleOperationDecided:
		return "decided"
	case LifecycleOperationCompleted:
		return "completed"
	case LifecycleOperationQuarantined:
		return "quarantined"
	}
	return ""
}
func operationStatePointerName(state *LifecycleOperationState) any {
	if state == nil {
		return nil
	}
	return operationStateName(*state)
}
