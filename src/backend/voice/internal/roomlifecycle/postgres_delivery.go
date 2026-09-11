package roomlifecycle

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const effectColumns = `effect_id,actor_profile_id,operation_id,ordinal,kind,schema_version,
target_voice_room_id,target_livekit_room_name,target_profile_id,target_participant_identity,target_media_epoch,
request_bytes,request_digest,state,attempt_count,last_error_class,last_error_at,next_attempt_at,
lease_owner,lease_until,lease_fence,applied_at,quarantine_class,quarantine_detail,quarantined_at,created_at,updated_at`

func prefixedColumns(alias, columns string) string {
	parts := strings.Split(strings.TrimSpace(columns), ",")
	for index := range parts {
		parts[index] = alias + "." + strings.TrimSpace(parts[index])
	}
	return strings.Join(parts, ",")
}

func scanEffect(row lifecycleScanner) (LifecycleEffect, error) {
	var effect LifecycleEffect
	var kind, state string
	var digest []byte
	if err := row.Scan(
		&effect.EffectID, &effect.ActorProfileID, &effect.OperationID, &effect.Ordinal, &kind, &effect.SchemaVersion,
		&effect.VoiceRoomID, &effect.LiveKitRoomName, &effect.TargetProfileID, &effect.ParticipantIdentity, &effect.MediaEpoch,
		&effect.RequestBytes, &digest, &state, &effect.AttemptCount, &effect.LastErrorClass, &effect.LastErrorAt, &effect.NextAttemptAt,
		&effect.LeaseOwner, &effect.LeaseUntil, &effect.LeaseFence, &effect.AppliedAt, &effect.QuarantineClass,
		&effect.QuarantineDetail, &effect.QuarantinedAt, &effect.CreatedAt, &effect.UpdatedAt,
	); err != nil {
		return LifecycleEffect{}, err
	}
	if anyNilUUID(effect.EffectID, effect.ActorProfileID, effect.OperationID, effect.VoiceRoomID) || effect.Ordinal < 0 || effect.SchemaVersion <= 0 ||
		!nonblank(effect.LiveKitRoomName) || effect.AttemptCount < 0 || effect.LeaseFence < 0 || len(effect.RequestBytes) == 0 {
		return LifecycleEffect{}, ErrInvariant
	}
	var err error
	effect.RequestDigest, err = digestFromBytes(digest)
	if err != nil {
		return LifecycleEffect{}, err
	}
	switch kind {
	case "livekit_ensure_room":
		effect.Kind = LifecycleEffectEnsureRoom
		if effect.TargetProfileID != nil || effect.ParticipantIdentity != nil || effect.MediaEpoch != nil {
			return LifecycleEffect{}, ErrInvariant
		}
	case "livekit_eject_participant":
		effect.Kind = LifecycleEffectEjectParticipant
		if effect.TargetProfileID == nil || effect.ParticipantIdentity == nil || effect.MediaEpoch == nil ||
			*effect.ParticipantIdentity != "profile:"+effect.TargetProfileID.String()+":media:"+effect.MediaEpoch.String() {
			return LifecycleEffect{}, ErrInvariant
		}
	default:
		return LifecycleEffect{}, ErrInvariant
	}
	expectedRequest, expectedDigest := encodeEffectRequest(effect.Kind, effect.VoiceRoomID, effect.LiveKitRoomName,
		effect.TargetProfileID, effect.ParticipantIdentity, effect.MediaEpoch)
	if !bytes.Equal(effect.RequestBytes, expectedRequest) || effect.RequestDigest != expectedDigest {
		return LifecycleEffect{}, ErrInvariant
	}
	switch state {
	case "ready":
		effect.State = LifecycleEffectStateReady
	case "applied":
		effect.State = LifecycleEffectStateApplied
	case "quarantined":
		effect.State = LifecycleEffectStateQuarantined
	default:
		return LifecycleEffect{}, ErrInvariant
	}
	effect.NextAttemptAt = effect.NextAttemptAt.UTC()
	effect.CreatedAt = effect.CreatedAt.UTC()
	effect.UpdatedAt = effect.UpdatedAt.UTC()
	normalizeUTC(effect.LastErrorAt)
	normalizeUTC(effect.LeaseUntil)
	normalizeUTC(effect.AppliedAt)
	normalizeUTC(effect.QuarantinedAt)
	return effect, nil
}

func (store *PostgresLifecycleStore) ListOperationEffects(ctx context.Context, actorProfileID, operationID uuid.UUID) ([]LifecycleEffect, error) {
	if store == nil || store.pool == nil || anyNilUUID(actorProfileID, operationID) {
		return nil, ErrUnavailable
	}
	rows, err := store.pool.Query(ctx, `SELECT `+effectColumns+` FROM voice_lifecycle_effects
WHERE actor_profile_id=$1 AND operation_id=$2 ORDER BY ordinal`, actorProfileID, operationID)
	if err != nil {
		return nil, ErrUnavailable
	}
	defer rows.Close()
	var effects []LifecycleEffect
	for rows.Next() {
		effect, scanErr := scanEffect(rows)
		if scanErr != nil {
			return nil, mapScanError(scanErr)
		}
		effects = append(effects, effect)
	}
	if err = rows.Err(); err != nil {
		return nil, ErrUnavailable
	}
	return effects, nil
}

func (store *PostgresLifecycleStore) ClaimNextEffect(ctx context.Context, workerID uuid.UUID, now time.Time, leaseDuration time.Duration) (LifecycleEffectClaim, bool, error) {
	if store == nil || store.pool == nil || workerID == uuid.Nil || now.IsZero() || leaseDuration <= 0 {
		return LifecycleEffectClaim{}, false, ErrUnavailable
	}
	leaseUntil := now.Add(leaseDuration)
	effect, err := scanEffect(store.pool.QueryRow(ctx, `
WITH candidate AS (
 SELECT effect_id FROM voice_lifecycle_effects
 WHERE state='ready' AND next_attempt_at <= $1 AND (lease_until IS NULL OR lease_until <= $1)
 ORDER BY next_attempt_at,created_at,effect_id
 FOR UPDATE SKIP LOCKED LIMIT 1
)
UPDATE voice_lifecycle_effects AS effect
SET lease_owner=$2,lease_until=$3,lease_fence=effect.lease_fence+1,updated_at=$1
FROM candidate WHERE effect.effect_id=candidate.effect_id
RETURNING `+prefixedColumns("effect", effectColumns), now, workerID, leaseUntil))
	if errors.Is(err, pgx.ErrNoRows) {
		return LifecycleEffectClaim{}, false, nil
	}
	if err != nil {
		return LifecycleEffectClaim{}, false, mapScanError(err)
	}
	return LifecycleEffectClaim{Effect: effect, WorkerID: workerID, Fence: effect.LeaseFence, LeaseUntil: leaseUntil}, true, nil
}

func (store *PostgresLifecycleStore) RenewEffectLease(ctx context.Context, lease LifecycleEffectLease) error {
	if store == nil || store.pool == nil || anyNilUUID(lease.EffectID, lease.WorkerID) || lease.ExpectedFence <= 0 || lease.LeaseUntil.IsZero() {
		return ErrUnavailable
	}
	tag, err := store.pool.Exec(ctx, `UPDATE voice_lifecycle_effects SET lease_until=$1,updated_at=$1
WHERE effect_id=$2 AND state='ready' AND lease_owner=$3 AND lease_fence=$4`,
		lease.LeaseUntil, lease.EffectID, lease.WorkerID, lease.ExpectedFence)
	if err != nil {
		return mapWriteError(err)
	}
	if tag.RowsAffected() != 1 {
		return ErrStaleLease
	}
	return nil
}

func sameEffectObservation(want, got LifecycleEffect) bool {
	return want.EffectID == got.EffectID && want.ActorProfileID == got.ActorProfileID && want.OperationID == got.OperationID &&
		want.Ordinal == got.Ordinal && want.Kind == got.Kind && want.SchemaVersion == got.SchemaVersion &&
		want.VoiceRoomID == got.VoiceRoomID && want.LiveKitRoomName == got.LiveKitRoomName &&
		equalUUIDPointer(want.TargetProfileID, got.TargetProfileID) && equalStringPointer(want.ParticipantIdentity, got.ParticipantIdentity) &&
		equalUUIDPointer(want.MediaEpoch, got.MediaEpoch) && bytes.Equal(want.RequestBytes, got.RequestBytes) &&
		want.RequestDigest == got.RequestDigest && want.CreatedAt.Equal(got.CreatedAt)
}

func equalUUIDPointer(left, right *uuid.UUID) bool {
	return (left == nil && right == nil) || (left != nil && right != nil && *left == *right)
}

func equalStringPointer(left, right *string) bool {
	return (left == nil && right == nil) || (left != nil && right != nil && *left == *right)
}

func loadEffectForUpdate(ctx context.Context, tx pgx.Tx, effectID uuid.UUID) (LifecycleEffect, bool, error) {
	effect, err := scanEffect(tx.QueryRow(ctx, `SELECT `+effectColumns+` FROM voice_lifecycle_effects WHERE effect_id=$1 FOR UPDATE`, effectID))
	if errors.Is(err, pgx.ErrNoRows) {
		return LifecycleEffect{}, false, nil
	}
	if err != nil {
		return LifecycleEffect{}, false, mapScanError(err)
	}
	return effect, true, nil
}

func (store *PostgresLifecycleStore) MarkEffectApplied(ctx context.Context, applied LifecycleEffectApplied) (err error) {
	if store == nil || store.pool == nil || anyNilUUID(applied.EffectID, applied.WorkerID) || applied.ExpectedFence <= 0 || applied.AppliedAt.IsZero() {
		return ErrUnavailable
	}
	tx, err := beginLifecycleTx(ctx, store)
	if err != nil {
		return err
	}
	defer finishLifecycleTx(ctx, tx, &err)
	effect, found, err := loadEffectForUpdate(ctx, tx, applied.EffectID)
	if err != nil {
		return err
	}
	if !found || effect.LeaseFence != applied.ExpectedFence || !sameEffectObservation(applied.Observed, effect) {
		if found && effect.LeaseFence == applied.ExpectedFence && !sameEffectObservation(applied.Observed, effect) {
			return ErrInvariant
		}
		return ErrStaleLease
	}
	if effect.State == LifecycleEffectStateApplied {
		if effect.AppliedAt != nil && effect.AppliedAt.Equal(applied.AppliedAt) {
			return nil
		}
		return ErrStaleLease
	}
	if effect.State != LifecycleEffectStateReady || effect.LeaseOwner == nil || *effect.LeaseOwner != applied.WorkerID {
		return ErrStaleLease
	}
	tag, err := tx.Exec(ctx, `UPDATE voice_lifecycle_effects
SET state='applied',lease_owner=NULL,lease_until=NULL,applied_at=$1,updated_at=$1
WHERE effect_id=$2 AND state='ready' AND lease_owner=$3 AND lease_fence=$4`,
		applied.AppliedAt, applied.EffectID, applied.WorkerID, applied.ExpectedFence)
	if err != nil {
		return mapWriteError(err)
	}
	if tag.RowsAffected() != 1 {
		return ErrStaleLease
	}
	reloaded, found, err := loadEffectForUpdate(ctx, tx, applied.EffectID)
	if err != nil {
		return err
	}
	if !found || reloaded.State != LifecycleEffectStateApplied || reloaded.AppliedAt == nil ||
		!reloaded.AppliedAt.Equal(applied.AppliedAt) || reloaded.LeaseOwner != nil || reloaded.LeaseUntil != nil ||
		reloaded.LeaseFence != applied.ExpectedFence || !sameEffectObservation(applied.Observed, reloaded) {
		return ErrInvariant
	}
	return nil
}

func (store *PostgresLifecycleStore) MarkEffectRetry(ctx context.Context, retry LifecycleEffectRetry) error {
	if store == nil || store.pool == nil || anyNilUUID(retry.EffectID, retry.Retry.WorkerID) || retry.Retry.ExpectedFence <= 0 ||
		!nonblank(retry.Retry.ErrorClass) || retry.Retry.ErrorAt.IsZero() || retry.Retry.NextAttemptAt.IsZero() {
		return ErrUnavailable
	}
	tag, err := store.pool.Exec(ctx, `UPDATE voice_lifecycle_effects
SET attempt_count=attempt_count+1,last_error_class=$1,last_error_at=$2,next_attempt_at=$3,
    lease_owner=NULL,lease_until=NULL,updated_at=$2
WHERE effect_id=$4 AND state='ready' AND lease_owner=$5 AND lease_fence=$6`,
		retry.Retry.ErrorClass, retry.Retry.ErrorAt, retry.Retry.NextAttemptAt, retry.EffectID, retry.Retry.WorkerID, retry.Retry.ExpectedFence)
	if err != nil {
		return mapWriteError(err)
	}
	if tag.RowsAffected() != 1 {
		return ErrStaleLease
	}
	return nil
}

func (store *PostgresLifecycleStore) QuarantineEffect(ctx context.Context, quarantine LifecycleEffectQuarantine) error {
	if store == nil || store.pool == nil || anyNilUUID(quarantine.EffectID, quarantine.WorkerID) || quarantine.ExpectedFence <= 0 ||
		!nonblank(quarantine.Class) || quarantine.At.IsZero() {
		return ErrUnavailable
	}
	var detail any
	if quarantine.Detail != "" {
		if !nonblank(quarantine.Detail) {
			return ErrInvariant
		}
		detail = quarantine.Detail
	}
	tag, err := store.pool.Exec(ctx, `UPDATE voice_lifecycle_effects
SET state='quarantined',lease_owner=NULL,lease_until=NULL,quarantine_class=$1,quarantine_detail=$2,
    quarantined_at=$3,updated_at=$3
WHERE effect_id=$4 AND state='ready' AND lease_owner=$5 AND lease_fence=$6`, quarantine.Class, detail, quarantine.At,
		quarantine.EffectID, quarantine.WorkerID, quarantine.ExpectedFence)
	if err != nil {
		return mapWriteError(err)
	}
	if tag.RowsAffected() != 1 {
		return ErrStaleLease
	}
	return nil
}

func (store *PostgresLifecycleStore) CheckGrantEligibility(ctx context.Context, profileID, mediaEpoch uuid.UUID) (_ LifecycleGrantEligibility, err error) {
	if store == nil || store.pool == nil || anyNilUUID(profileID, mediaEpoch) {
		return LifecycleGrantEligibility{}, ErrUnavailable
	}
	tx, err := beginLifecycleTx(ctx, store)
	if err != nil {
		return LifecycleGrantEligibility{}, err
	}
	defer finishLifecycleTx(ctx, tx, &err)
	if err = advisoryLock(ctx, tx, LifecycleSubjectAdvisoryKey(profileID)); err != nil {
		return LifecycleGrantEligibility{}, mapWriteError(err)
	}
	membership, found, err := loadMembership(ctx, tx, profileID, "FOR UPDATE")
	if err != nil {
		return LifecycleGrantEligibility{}, err
	}
	if !found || membership.MediaEpoch != mediaEpoch {
		return LifecycleGrantEligibility{Membership: membership, Eligible: false}, nil
	}
	var fenced bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM voice_lifecycle_operations WHERE subject_profile_id=$1 AND completed_at IS NULL)`, profileID).Scan(&fenced); err != nil {
		return LifecycleGrantEligibility{}, ErrUnavailable
	}
	return LifecycleGrantEligibility{Membership: membership, Eligible: !fenced}, nil
}

func (store *PostgresLifecycleStore) AdvanceLatestGrantExpiry(ctx context.Context, profileID, mediaEpoch uuid.UUID, expiresAt time.Time) (_ LifecycleMembership, err error) {
	if store == nil || store.pool == nil || anyNilUUID(profileID, mediaEpoch) || expiresAt.IsZero() {
		return LifecycleMembership{}, ErrUnavailable
	}
	tx, err := beginLifecycleTx(ctx, store)
	if err != nil {
		return LifecycleMembership{}, err
	}
	defer finishLifecycleTx(ctx, tx, &err)
	if err = advisoryLock(ctx, tx, LifecycleSubjectAdvisoryKey(profileID)); err != nil {
		return LifecycleMembership{}, mapWriteError(err)
	}
	var fenced bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM voice_lifecycle_operations WHERE subject_profile_id=$1 AND completed_at IS NULL)`, profileID).Scan(&fenced); err != nil {
		return LifecycleMembership{}, ErrUnavailable
	}
	if fenced {
		return LifecycleMembership{}, ErrSubjectTransitionActive
	}
	membership, found, err := loadMembership(ctx, tx, profileID, "FOR UPDATE")
	if err != nil {
		return LifecycleMembership{}, err
	}
	if !found || membership.MediaEpoch != mediaEpoch {
		return LifecycleMembership{}, ErrMembershipConflict
	}
	if _, err = tx.Exec(ctx, `UPDATE voice_room_memberships
SET latest_grant_expires_at=CASE WHEN latest_grant_expires_at IS NULL OR latest_grant_expires_at < $1 THEN $1 ELSE latest_grant_expires_at END,
    updated_at=GREATEST(updated_at,clock_timestamp()) WHERE profile_id=$2`, expiresAt, profileID); err != nil {
		return LifecycleMembership{}, mapWriteError(err)
	}
	membership, found, err = loadMembership(ctx, tx, profileID, "")
	if err != nil || !found {
		if err == nil {
			err = ErrInvariant
		}
		return LifecycleMembership{}, err
	}
	return membership, nil
}

func (store *PostgresLifecycleStore) LookupMediaEpochDenial(ctx context.Context, mediaEpoch uuid.UUID) (LifecycleMediaEpochDenial, bool, error) {
	if store == nil || store.pool == nil || mediaEpoch == uuid.Nil {
		return LifecycleMediaEpochDenial{}, false, ErrUnavailable
	}
	var denial LifecycleMediaEpochDenial
	var skewSeconds float64
	err := store.pool.QueryRow(ctx, `
SELECT media_epoch,profile_id,room_id,livekit_room_name,participant_identity,grant_expires_at,
       EXTRACT(EPOCH FROM accepted_clock_skew)::double precision,deny_until,reason,
       operation_actor_profile_id,operation_id,absence_observed_at,created_at,updated_at
FROM voice_media_epoch_denials WHERE media_epoch=$1`, mediaEpoch).Scan(
		&denial.MediaEpoch, &denial.ProfileID, &denial.RoomID, &denial.LiveKitRoomName, &denial.ParticipantIdentity,
		&denial.GrantExpiresAt, &skewSeconds, &denial.DenyUntil, &denial.Reason, &denial.ActorProfileID,
		&denial.OperationID, &denial.AbsenceObservedAt, &denial.CreatedAt, &denial.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return LifecycleMediaEpochDenial{}, false, nil
	}
	if err != nil {
		return LifecycleMediaEpochDenial{}, false, ErrUnavailable
	}
	if anyNilUUID(denial.MediaEpoch, denial.ProfileID, denial.RoomID) || !nonblank(denial.LiveKitRoomName) ||
		!nonblank(denial.ParticipantIdentity) || !nonblank(denial.Reason) || skewSeconds < 0 {
		return LifecycleMediaEpochDenial{}, false, ErrInvariant
	}
	denial.AcceptedClockSkew = time.Duration(skewSeconds * float64(time.Second))
	denial.GrantExpiresAt = denial.GrantExpiresAt.UTC()
	denial.DenyUntil = denial.DenyUntil.UTC()
	denial.CreatedAt = denial.CreatedAt.UTC()
	denial.UpdatedAt = denial.UpdatedAt.UTC()
	normalizeUTC(denial.AbsenceObservedAt)
	return denial, true, nil
}

func (store *PostgresLifecycleStore) MarkDenialAbsenceObserved(ctx context.Context, mediaEpoch uuid.UUID, observedAt time.Time) error {
	if store == nil || store.pool == nil || mediaEpoch == uuid.Nil || observedAt.IsZero() {
		return ErrUnavailable
	}
	tag, err := store.pool.Exec(ctx, `UPDATE voice_media_epoch_denials
SET absence_observed_at=CASE WHEN absence_observed_at IS NULL OR absence_observed_at < $1 THEN $1 ELSE absence_observed_at END,
    updated_at=$1 WHERE media_epoch=$2`, observedAt, mediaEpoch)
	if err != nil {
		return mapWriteError(err)
	}
	if tag.RowsAffected() != 1 {
		return ErrMembershipConflict
	}
	return nil
}

func (store *PostgresLifecycleStore) DeleteExpiredObservedDenials(ctx context.Context, now time.Time, limit int) (int64, error) {
	if store == nil || store.pool == nil || now.IsZero() || limit <= 0 {
		return 0, ErrUnavailable
	}
	tag, err := store.pool.Exec(ctx, `
WITH expired AS (
 SELECT media_epoch FROM voice_media_epoch_denials
 WHERE deny_until <= $1 AND absence_observed_at IS NOT NULL
 ORDER BY deny_until,media_epoch FOR UPDATE SKIP LOCKED LIMIT $2
)
DELETE FROM voice_media_epoch_denials AS denial USING expired WHERE denial.media_epoch=expired.media_epoch`, now, limit)
	if err != nil {
		return 0, mapWriteError(err)
	}
	return tag.RowsAffected(), nil
}

const outboxColumns = `event_id,actor_profile_id,operation_id,ordinal,subject,schema_version,payload_bytes,payload_hash,
subject_profile_id,room_id,voice_room_id,space_id,roster_version,state,attempt_count,last_error_class,last_error_at,
next_attempt_at,lease_owner,lease_until,lease_fence,delivered_at,quarantine_class,quarantine_detail,quarantined_at,created_at,updated_at`

func scanOutbox(row lifecycleScanner) (LifecycleOutbox, error) {
	var outbox LifecycleOutbox
	var state string
	var digest []byte
	if err := row.Scan(&outbox.EventID, &outbox.ActorProfileID, &outbox.OperationID, &outbox.Ordinal, &outbox.Subject,
		&outbox.SchemaVersion, &outbox.PayloadBytes, &digest, &outbox.SubjectProfileID, &outbox.RoomID, &outbox.VoiceRoomID,
		&outbox.SpaceID, &outbox.RosterVersion, &state, &outbox.AttemptCount, &outbox.LastErrorClass, &outbox.LastErrorAt,
		&outbox.NextAttemptAt, &outbox.LeaseOwner, &outbox.LeaseUntil, &outbox.LeaseFence, &outbox.DeliveredAt,
		&outbox.QuarantineClass, &outbox.QuarantineDetail, &outbox.QuarantinedAt, &outbox.CreatedAt, &outbox.UpdatedAt); err != nil {
		return LifecycleOutbox{}, err
	}
	if anyNilUUID(outbox.EventID, outbox.ActorProfileID, outbox.OperationID, outbox.SubjectProfileID, outbox.RoomID, outbox.VoiceRoomID, outbox.SpaceID) ||
		outbox.Ordinal < 0 || outbox.SchemaVersion <= 0 || outbox.RosterVersion < 0 || outbox.AttemptCount < 0 || outbox.LeaseFence < 0 ||
		!strings.HasPrefix(outbox.Subject, "voice.") || len(outbox.PayloadBytes) == 0 {
		return LifecycleOutbox{}, ErrInvariant
	}
	var err error
	outbox.PayloadHash, err = digestFromBytes(digest)
	if err != nil {
		return LifecycleOutbox{}, err
	}
	if LifecycleDigest(sha256.Sum256(outbox.PayloadBytes)) != outbox.PayloadHash {
		return LifecycleOutbox{}, ErrInvariant
	}
	switch state {
	case "ready":
		outbox.State = LifecycleOutboxStateReady
	case "delivered":
		outbox.State = LifecycleOutboxStateDelivered
	case "quarantined":
		outbox.State = LifecycleOutboxStateQuarantined
	default:
		return LifecycleOutbox{}, ErrInvariant
	}
	outbox.NextAttemptAt = outbox.NextAttemptAt.UTC()
	outbox.CreatedAt = outbox.CreatedAt.UTC()
	outbox.UpdatedAt = outbox.UpdatedAt.UTC()
	normalizeUTC(outbox.LastErrorAt)
	normalizeUTC(outbox.LeaseUntil)
	normalizeUTC(outbox.DeliveredAt)
	normalizeUTC(outbox.QuarantinedAt)
	return outbox, nil
}

func (store *PostgresLifecycleStore) ClaimNextOutbox(ctx context.Context, workerID uuid.UUID, now time.Time, leaseDuration time.Duration) (LifecycleOutboxClaim, bool, error) {
	if store == nil || store.pool == nil || workerID == uuid.Nil || now.IsZero() || leaseDuration <= 0 {
		return LifecycleOutboxClaim{}, false, ErrUnavailable
	}
	leaseUntil := now.Add(leaseDuration)
	outbox, err := scanOutbox(store.pool.QueryRow(ctx, `
WITH candidate AS (
 SELECT event_id FROM voice_event_outbox WHERE state='ready' AND next_attempt_at <= $1
   AND (lease_until IS NULL OR lease_until <= $1)
 ORDER BY next_attempt_at,created_at,event_id FOR UPDATE SKIP LOCKED LIMIT 1
)
UPDATE voice_event_outbox AS outbox SET lease_owner=$2,lease_until=$3,lease_fence=outbox.lease_fence+1,updated_at=$1
FROM candidate WHERE outbox.event_id=candidate.event_id RETURNING `+prefixedColumns("outbox", outboxColumns), now, workerID, leaseUntil))
	if errors.Is(err, pgx.ErrNoRows) {
		return LifecycleOutboxClaim{}, false, nil
	}
	if err != nil {
		return LifecycleOutboxClaim{}, false, mapScanError(err)
	}
	return LifecycleOutboxClaim{Outbox: outbox, WorkerID: workerID, Fence: outbox.LeaseFence, LeaseUntil: leaseUntil}, true, nil
}

func (store *PostgresLifecycleStore) RenewOutboxLease(ctx context.Context, lease LifecycleOutboxLease) error {
	if store == nil || store.pool == nil || anyNilUUID(lease.EventID, lease.WorkerID) || lease.ExpectedFence <= 0 || lease.LeaseUntil.IsZero() {
		return ErrUnavailable
	}
	tag, err := store.pool.Exec(ctx, `UPDATE voice_event_outbox SET lease_until=$1,updated_at=$1
WHERE event_id=$2 AND state='ready' AND lease_owner=$3 AND lease_fence=$4`, lease.LeaseUntil, lease.EventID, lease.WorkerID, lease.ExpectedFence)
	if err != nil {
		return mapWriteError(err)
	}
	if tag.RowsAffected() != 1 {
		return ErrStaleLease
	}
	return nil
}

func (store *PostgresLifecycleStore) MarkOutboxDelivered(ctx context.Context, delivered LifecycleOutboxDelivered) (err error) {
	if store == nil || store.pool == nil || anyNilUUID(delivered.EventID, delivered.WorkerID) || delivered.ExpectedFence <= 0 || delivered.DeliveredAt.IsZero() {
		return ErrUnavailable
	}
	tx, err := beginLifecycleTx(ctx, store)
	if err != nil {
		return err
	}
	defer finishLifecycleTx(ctx, tx, &err)
	outbox, err := scanOutbox(tx.QueryRow(ctx, `SELECT `+outboxColumns+` FROM voice_event_outbox WHERE event_id=$1 FOR UPDATE`, delivered.EventID))
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrStaleLease
	}
	if err != nil {
		return mapScanError(err)
	}
	if outbox.LeaseFence != delivered.ExpectedFence {
		return ErrStaleLease
	}
	if outbox.State == LifecycleOutboxStateDelivered && outbox.DeliveredAt != nil && outbox.DeliveredAt.Equal(delivered.DeliveredAt) {
		return nil
	}
	if outbox.State != LifecycleOutboxStateReady || outbox.LeaseOwner == nil || *outbox.LeaseOwner != delivered.WorkerID {
		return ErrStaleLease
	}
	tag, err := tx.Exec(ctx, `UPDATE voice_event_outbox
SET state='delivered',lease_owner=NULL,lease_until=NULL,delivered_at=$1,updated_at=$1
WHERE event_id=$2 AND state='ready' AND lease_owner=$3 AND lease_fence=$4`,
		delivered.DeliveredAt, delivered.EventID, delivered.WorkerID, delivered.ExpectedFence)
	if err != nil {
		return mapWriteError(err)
	}
	if tag.RowsAffected() != 1 {
		return ErrStaleLease
	}
	reloaded, err := scanOutbox(tx.QueryRow(ctx, `SELECT `+outboxColumns+` FROM voice_event_outbox WHERE event_id=$1 FOR UPDATE`, delivered.EventID))
	if err != nil {
		return mapScanError(err)
	}
	if reloaded.State != LifecycleOutboxStateDelivered || reloaded.DeliveredAt == nil ||
		!reloaded.DeliveredAt.Equal(delivered.DeliveredAt) || reloaded.LeaseOwner != nil || reloaded.LeaseUntil != nil ||
		reloaded.LeaseFence != delivered.ExpectedFence {
		return ErrInvariant
	}
	return nil
}

func (store *PostgresLifecycleStore) MarkOutboxRetry(ctx context.Context, retry LifecycleOutboxRetry) error {
	if store == nil || store.pool == nil || anyNilUUID(retry.EventID, retry.Retry.WorkerID) || retry.Retry.ExpectedFence <= 0 ||
		!nonblank(retry.Retry.ErrorClass) || retry.Retry.ErrorAt.IsZero() || retry.Retry.NextAttemptAt.IsZero() {
		return ErrUnavailable
	}
	tag, err := store.pool.Exec(ctx, `UPDATE voice_event_outbox
SET attempt_count=attempt_count+1,last_error_class=$1,last_error_at=$2,next_attempt_at=$3,lease_owner=NULL,lease_until=NULL,updated_at=$2
WHERE event_id=$4 AND state='ready' AND lease_owner=$5 AND lease_fence=$6`, retry.Retry.ErrorClass, retry.Retry.ErrorAt,
		retry.Retry.NextAttemptAt, retry.EventID, retry.Retry.WorkerID, retry.Retry.ExpectedFence)
	if err != nil {
		return mapWriteError(err)
	}
	if tag.RowsAffected() != 1 {
		return ErrStaleLease
	}
	return nil
}

func (store *PostgresLifecycleStore) QuarantineOutbox(ctx context.Context, quarantine LifecycleOutboxQuarantine) error {
	if store == nil || store.pool == nil || anyNilUUID(quarantine.EventID, quarantine.WorkerID) || quarantine.ExpectedFence <= 0 ||
		!nonblank(quarantine.Class) || quarantine.At.IsZero() {
		return ErrUnavailable
	}
	var detail any
	if quarantine.Detail != "" {
		if !nonblank(quarantine.Detail) {
			return ErrInvariant
		}
		detail = quarantine.Detail
	}
	tag, err := store.pool.Exec(ctx, `UPDATE voice_event_outbox
SET state='quarantined',lease_owner=NULL,lease_until=NULL,quarantine_class=$1,quarantine_detail=$2,quarantined_at=$3,updated_at=$3
WHERE event_id=$4 AND state='ready' AND lease_owner=$5 AND lease_fence=$6`, quarantine.Class, detail, quarantine.At,
		quarantine.EventID, quarantine.WorkerID, quarantine.ExpectedFence)
	if err != nil {
		return mapWriteError(err)
	}
	if tag.RowsAffected() != 1 {
		return ErrStaleLease
	}
	return nil
}

func validateCompletion(completion LifecycleCompletion) ([]byte, LifecycleDigest, error) {
	if anyNilUUID(completion.ActorProfileID, completion.OperationID, completion.WorkerID) || completion.ExpectedFence <= 0 ||
		completion.CompletedAt.IsZero() || completion.ReplayUntil.Before(completion.CompletedAt.Add(24*time.Hour)) {
		return nil, LifecycleDigest{}, ErrInvariant
	}
	receiptBytes, receiptHash, err := EncodeLifecycleReceipt(completion.Receipt)
	if err != nil {
		return nil, LifecycleDigest{}, err
	}
	for index, record := range completion.Outbox {
		if record.EventID == uuid.Nil || record.Ordinal != int16(index) || !strings.HasPrefix(record.Subject, "voice.") ||
			strings.TrimSpace(record.Subject) != record.Subject || record.SchemaVersion <= 0 || len(record.PayloadBytes) == 0 ||
			anyNilUUID(record.SubjectProfileID, record.RoomID, record.VoiceRoomID, record.SpaceID) || record.RosterVersion < 0 || record.NextAttemptAt.IsZero() {
			return nil, LifecycleDigest{}, ErrInvariant
		}
	}
	return receiptBytes, receiptHash, nil
}

func (store *PostgresLifecycleStore) CompleteOperation(ctx context.Context, completion LifecycleCompletion) (_ LifecycleOperation, err error) {
	receiptBytes, receiptHash, err := validateCompletion(completion)
	if err != nil {
		return LifecycleOperation{}, err
	}
	tx, err := beginLifecycleTx(ctx, store)
	if err != nil {
		return LifecycleOperation{}, err
	}
	defer finishLifecycleTx(ctx, tx, &err)
	if err = advisoryLock(ctx, tx, LifecycleOperationAdvisoryKey(completion.ActorProfileID, completion.OperationID)); err != nil {
		return LifecycleOperation{}, mapWriteError(err)
	}
	operation, found, err := loadOperation(ctx, tx, completion.ActorProfileID, completion.OperationID, "FOR UPDATE")
	if err != nil {
		return LifecycleOperation{}, err
	}
	if !found {
		return LifecycleOperation{}, ErrStaleLease
	}
	if operation.State == LifecycleOperationCompleted {
		if bytes.Equal(operation.ReceiptBytes, receiptBytes) && operation.ReceiptHash != nil && *operation.ReceiptHash == receiptHash {
			return operation, nil
		}
		return LifecycleOperation{}, ErrOperationConflict
	}
	if operation.State != LifecycleOperationDecided || operation.LeaseOwner == nil || *operation.LeaseOwner != completion.WorkerID || operation.LeaseFence != completion.ExpectedFence {
		return LifecycleOperation{}, ErrStaleLease
	}
	if err = advisoryLock(ctx, tx, LifecycleSubjectAdvisoryKey(operation.SubjectProfileID)); err != nil {
		return LifecycleOperation{}, mapWriteError(err)
	}
	for _, lock := range SortLifecycleLogicalRoomLocks(operationLogicalRoomIDs(operation)) {
		if err = advisoryLock(ctx, tx, lock.Key); err != nil {
			return LifecycleOperation{}, mapWriteError(err)
		}
	}
	membership, membershipFound, err := loadMembership(ctx, tx, operation.SubjectProfileID, "FOR UPDATE")
	if err != nil {
		return LifecycleOperation{}, err
	}
	runtimeRows, err := loadAndLockCompletionRooms(ctx, tx, operation)
	if err != nil {
		return LifecycleOperation{}, err
	}
	effects, err := loadAndValidateCompletionEffects(ctx, tx, operation)
	if err != nil {
		return LifecycleOperation{}, err
	}
	for _, effect := range effects {
		if effect.State != LifecycleEffectStateApplied {
			return LifecycleOperation{}, ErrOperationNotCompletable
		}
	}
	expectedReceipt, err := expectedCompletionReceipt(operation, membership, membershipFound, runtimeRows)
	if err != nil {
		return LifecycleOperation{}, err
	}
	expectedBytes, expectedHash, err := EncodeLifecycleReceipt(expectedReceipt)
	if err != nil || !bytes.Equal(expectedBytes, receiptBytes) || expectedHash != receiptHash {
		return LifecycleOperation{}, ErrInvariant
	}
	if err = applyCompletionMembership(ctx, tx, operation, membership, membershipFound, runtimeRows, completion.CompletedAt); err != nil {
		return LifecycleOperation{}, err
	}
	if err = insertCompletionOutbox(ctx, tx, operation, completion); err != nil {
		return LifecycleOperation{}, err
	}
	outcome, _ := outcomeName(completion.Receipt.Outcome)
	tag, err := tx.Exec(ctx, `UPDATE voice_lifecycle_operations SET
 state='completed',lease_owner=NULL,lease_until=NULL,receipt_outcome=$1,
 receipt_source_voice_room_id=$2,receipt_destination_voice_room_id=$3,receipt_room_id=$4,
 receipt_source_roster_version=$5,receipt_destination_roster_version=$6,receipt_media_epoch=$7,
 receipt_space_access_epoch=$8,receipt_role_policy_epoch=$9,receipt_authorization_digest=$10,
 receipt_bytes=$11,receipt_hash=$12,completed_at=$13,replay_until=$14,updated_at=$13
WHERE actor_profile_id=$15 AND operation_id=$16 AND state='decided' AND lease_owner=$17 AND lease_fence=$18`, outcome, completion.Receipt.SourceVoiceRoomID,
		completion.Receipt.DestinationVoiceRoomID, completion.Receipt.RoomID, completion.Receipt.SourceRosterVersion,
		completion.Receipt.DestinationRosterVersion, completion.Receipt.MediaEpoch, completion.Receipt.SpaceAccessEpoch,
		completion.Receipt.RolePolicyEpoch, digestBytes(completion.Receipt.AuthorizationDigest), receiptBytes, receiptHash[:],
		completion.CompletedAt, completion.ReplayUntil, completion.ActorProfileID, completion.OperationID,
		completion.WorkerID, completion.ExpectedFence)
	if err != nil {
		return LifecycleOperation{}, mapWriteError(err)
	}
	if tag.RowsAffected() != 1 {
		return LifecycleOperation{}, ErrStaleLease
	}
	completed, found, err := loadOperation(ctx, tx, completion.ActorProfileID, completion.OperationID, "")
	if err != nil || !found {
		if err == nil {
			err = ErrInvariant
		}
		return LifecycleOperation{}, err
	}
	return completed, nil
}

func loadAndValidateCompletionEffects(ctx context.Context, tx pgx.Tx, operation LifecycleOperation) ([]LifecycleEffect, error) {
	rows, err := tx.Query(ctx, `SELECT `+effectColumns+` FROM voice_lifecycle_effects
WHERE actor_profile_id=$1 AND operation_id=$2 ORDER BY ordinal FOR UPDATE`, operation.ActorProfileID, operation.OperationID)
	if err != nil {
		return nil, mapScanError(err)
	}
	defer rows.Close()
	var effects []LifecycleEffect
	for rows.Next() {
		effect, scanErr := scanEffect(rows)
		if scanErr != nil {
			return nil, mapScanError(scanErr)
		}
		effects = append(effects, effect)
	}
	if err = rows.Err(); err != nil {
		return nil, mapScanError(err)
	}
	expectedKinds := []LifecycleEffectKind{LifecycleEffectEnsureRoom}
	switch operation.Method {
	case LifecycleMethodLeave:
		expectedKinds = []LifecycleEffectKind{LifecycleEffectEjectParticipant}
	case LifecycleMethodSelfMove, LifecycleMethodModeratorMove:
		expectedKinds = []LifecycleEffectKind{LifecycleEffectEjectParticipant, LifecycleEffectEnsureRoom}
	case LifecycleMethodJoin:
	default:
		return nil, ErrInvariant
	}
	if len(effects) != len(expectedKinds) {
		return nil, ErrInvariant
	}
	for index, effect := range effects {
		if effect.Ordinal != int16(index) || effect.Kind != expectedKinds[index] {
			return nil, ErrInvariant
		}
	}
	return effects, nil
}

func operationLogicalRoomIDs(operation LifecycleOperation) []uuid.UUID {
	var ids []uuid.UUID
	if operation.SourceVoiceRoomID != nil {
		ids = append(ids, *operation.SourceVoiceRoomID)
	}
	if operation.DestinationVoiceRoomID != nil {
		ids = append(ids, *operation.DestinationVoiceRoomID)
	}
	return ids
}

func loadAndLockCompletionRooms(ctx context.Context, tx pgx.Tx, operation LifecycleOperation) (lifecycleDecisionRows, error) {
	rows := lifecycleDecisionRows{}
	if operation.SourceRoomID != nil {
		room, found, err := loadRoomTx(ctx, tx, *operation.SourceRoomID, "")
		if err != nil || !found {
			return rows, ErrInvariant
		}
		rows.source = &room
	}
	if operation.DestinationRoomID != nil {
		room, found, err := loadRoomTx(ctx, tx, *operation.DestinationRoomID, "")
		if err != nil || !found {
			return rows, ErrInvariant
		}
		rows.destination = &room
	}
	if err := lockRuntimeRooms(ctx, tx, rows); err != nil {
		return rows, err
	}
	if rows.source != nil {
		room, found, err := loadRoomTx(ctx, tx, rows.source.RoomID, "")
		if err != nil {
			return rows, err
		}
		if !found || operation.SourceRoomID == nil || operation.SourceVoiceRoomID == nil || room.RoomID != *operation.SourceRoomID ||
			room.SpaceID != operation.SpaceID || room.VoiceRoomID != *operation.SourceVoiceRoomID || !room.Active {
			return rows, ErrMembershipConflict
		}
		rows.source = &room
	}
	if rows.destination != nil {
		room, found, err := loadRoomTx(ctx, tx, rows.destination.RoomID, "")
		if err != nil {
			return rows, err
		}
		if !found || operation.DestinationRoomID == nil || operation.DestinationVoiceRoomID == nil ||
			room.RoomID != *operation.DestinationRoomID || room.SpaceID != operation.SpaceID ||
			room.VoiceRoomID != *operation.DestinationVoiceRoomID || !room.Active ||
			room.LiveKitRoomName != deterministicLiveKitRoomName(*operation.DestinationVoiceRoomID) {
			return rows, ErrInvariant
		}
		rows.destination = &room
	}
	return rows, nil
}

func expectedCompletionReceipt(operation LifecycleOperation, membership LifecycleMembership, membershipFound bool, rows lifecycleDecisionRows) (LifecycleReceipt, error) {
	receipt := LifecycleReceipt{OperationID: operation.OperationID, ActorProfileID: operation.ActorProfileID,
		SubjectProfileID: operation.SubjectProfileID, SpaceID: operation.SpaceID, Method: operation.Method}
	switch operation.Method {
	case LifecycleMethodJoin:
		if membershipFound || rows.destination == nil || !rows.destination.Active || operation.DestinationMediaEpoch == nil {
			return LifecycleReceipt{}, ErrMembershipConflict
		}
		destinationRoster := rows.destination.RosterVersion + 1
		receipt.Outcome = LifecycleOutcomeJoined
		receipt.DestinationVoiceRoomID = operation.DestinationVoiceRoomID
		receipt.RoomID = operation.DestinationRoomID
		receipt.DestinationRosterVersion = &destinationRoster
		receipt.MediaEpoch = operation.DestinationMediaEpoch
		receipt.SpaceAccessEpoch = &operation.Authority.SpaceAccessEpoch
		receipt.RolePolicyEpoch = &operation.Authority.SubjectRolePolicyEpoch
		receipt.AuthorizationDigest = &operation.Authority.AuthorizationDigest
	case LifecycleMethodLeave:
		if !membershipFound || rows.source == nil || membership.RoomID != *operation.SourceRoomID || membership.MediaEpoch != *operation.SourceMediaEpoch {
			return LifecycleReceipt{}, ErrMembershipConflict
		}
		sourceRoster := rows.source.RosterVersion + 1
		receipt.Outcome = LifecycleOutcomeLeft
		receipt.SourceVoiceRoomID = operation.SourceVoiceRoomID
		receipt.RoomID = operation.SourceRoomID
		receipt.SourceRosterVersion = &sourceRoster
	case LifecycleMethodSelfMove, LifecycleMethodModeratorMove:
		if !membershipFound || rows.source == nil || rows.destination == nil || membership.RoomID != *operation.SourceRoomID ||
			membership.MediaEpoch != *operation.SourceMediaEpoch || operation.DestinationMediaEpoch == nil {
			return LifecycleReceipt{}, ErrMembershipConflict
		}
		sourceRoster, destinationRoster := rows.source.RosterVersion+1, rows.destination.RosterVersion+1
		receipt.Outcome = LifecycleOutcomeMoved
		receipt.SourceVoiceRoomID = operation.SourceVoiceRoomID
		receipt.DestinationVoiceRoomID = operation.DestinationVoiceRoomID
		receipt.RoomID = operation.DestinationRoomID
		receipt.SourceRosterVersion = &sourceRoster
		receipt.DestinationRosterVersion = &destinationRoster
		receipt.MediaEpoch = operation.DestinationMediaEpoch
		receipt.SpaceAccessEpoch = &operation.Authority.SpaceAccessEpoch
		receipt.RolePolicyEpoch = &operation.Authority.SubjectRolePolicyEpoch
		receipt.AuthorizationDigest = &operation.Authority.AuthorizationDigest
	default:
		return LifecycleReceipt{}, ErrInvariant
	}
	return receipt, nil
}

func applyCompletionMembership(ctx context.Context, tx pgx.Tx, operation LifecycleOperation, membership LifecycleMembership,
	membershipFound bool, rows lifecycleDecisionRows, completedAt time.Time) error {
	switch operation.Method {
	case LifecycleMethodJoin:
		_, err := tx.Exec(ctx, `INSERT INTO voice_room_memberships(
 profile_id,room_id,media_epoch,space_access_epoch,role_policy_epoch,authorization_digest,
 can_join,can_publish_audio,can_publish_video,can_publish_screen_share,can_subscribe,can_mute_others,can_deafen_others,
 can_move_others,can_use_ptt,priority_speaker,latest_grant_expires_at,joined_at,updated_at)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,NULL,$17,$17)`,
			operation.SubjectProfileID, *operation.DestinationRoomID, *operation.DestinationMediaEpoch,
			operation.Authority.SpaceAccessEpoch, operation.Authority.SubjectRolePolicyEpoch, operation.Authority.AuthorizationDigest[:],
			operation.Authority.SubjectGrants.CanJoin, operation.Authority.SubjectGrants.CanPublishAudio,
			operation.Authority.SubjectGrants.CanPublishVideo, operation.Authority.SubjectGrants.CanPublishScreenShare,
			operation.Authority.SubjectGrants.CanSubscribe, operation.Authority.SubjectGrants.CanMuteOthers,
			operation.Authority.SubjectGrants.CanDeafenOthers, operation.Authority.SubjectGrants.CanMoveOthers,
			operation.Authority.SubjectGrants.CanUsePTT, operation.Authority.SubjectGrants.PrioritySpeaker, completedAt)
		if err != nil {
			return mapWriteError(err)
		}
	case LifecycleMethodLeave:
		if !membershipFound {
			return ErrMembershipConflict
		}
		tag, err := tx.Exec(ctx, `DELETE FROM voice_room_memberships WHERE profile_id=$1 AND room_id=$2 AND media_epoch=$3`,
			operation.SubjectProfileID, membership.RoomID, membership.MediaEpoch)
		if err != nil {
			return mapWriteError(err)
		}
		if tag.RowsAffected() != 1 {
			return ErrMembershipConflict
		}
	case LifecycleMethodSelfMove, LifecycleMethodModeratorMove:
		if !membershipFound {
			return ErrMembershipConflict
		}
		tag, err := tx.Exec(ctx, `UPDATE voice_room_memberships SET
 room_id=$1,media_epoch=$2,space_access_epoch=$3,role_policy_epoch=$4,authorization_digest=$5,
 can_join=$6,can_publish_audio=$7,can_publish_video=$8,can_publish_screen_share=$9,can_subscribe=$10,
 can_mute_others=$11,can_deafen_others=$12,can_move_others=$13,can_use_ptt=$14,priority_speaker=$15,
 latest_grant_expires_at=NULL,joined_at=$16,updated_at=$16 WHERE profile_id=$17 AND room_id=$18 AND media_epoch=$19`,
			*operation.DestinationRoomID, *operation.DestinationMediaEpoch, operation.Authority.SpaceAccessEpoch,
			operation.Authority.SubjectRolePolicyEpoch, operation.Authority.AuthorizationDigest[:],
			operation.Authority.SubjectGrants.CanJoin, operation.Authority.SubjectGrants.CanPublishAudio,
			operation.Authority.SubjectGrants.CanPublishVideo, operation.Authority.SubjectGrants.CanPublishScreenShare,
			operation.Authority.SubjectGrants.CanSubscribe, operation.Authority.SubjectGrants.CanMuteOthers,
			operation.Authority.SubjectGrants.CanDeafenOthers, operation.Authority.SubjectGrants.CanMoveOthers,
			operation.Authority.SubjectGrants.CanUsePTT, operation.Authority.SubjectGrants.PrioritySpeaker, completedAt,
			operation.SubjectProfileID, membership.RoomID, membership.MediaEpoch)
		if err != nil {
			return mapWriteError(err)
		}
		if tag.RowsAffected() != 1 {
			return ErrMembershipConflict
		}
	}
	if rows.source != nil {
		if _, err := tx.Exec(ctx, `UPDATE voice_room_instances SET roster_version=roster_version+1,updated_at=$1 WHERE room_id=$2`, completedAt, rows.source.RoomID); err != nil {
			return mapWriteError(err)
		}
	}
	if rows.destination != nil {
		if _, err := tx.Exec(ctx, `UPDATE voice_room_instances SET roster_version=roster_version+1,updated_at=$1 WHERE room_id=$2`, completedAt, rows.destination.RoomID); err != nil {
			return mapWriteError(err)
		}
	}
	return nil
}

func insertCompletionOutbox(ctx context.Context, tx pgx.Tx, operation LifecycleOperation, completion LifecycleCompletion) error {
	for _, record := range completion.Outbox {
		if record.SubjectProfileID != operation.SubjectProfileID || record.SpaceID != operation.SpaceID {
			return ErrInvariant
		}
		hash := sha256.Sum256(record.PayloadBytes)
		_, err := tx.Exec(ctx, `INSERT INTO voice_event_outbox(
 event_id,actor_profile_id,operation_id,ordinal,subject,schema_version,payload_bytes,payload_hash,
 subject_profile_id,room_id,voice_room_id,space_id,roster_version,state,attempt_count,next_attempt_at,
 lease_fence,created_at,updated_at)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,'ready',0,$14,0,$15,$15)`,
			record.EventID, operation.ActorProfileID, operation.OperationID, record.Ordinal, record.Subject,
			record.SchemaVersion, record.PayloadBytes, hash[:], record.SubjectProfileID, record.RoomID,
			record.VoiceRoomID, record.SpaceID, record.RosterVersion, record.NextAttemptAt, completion.CompletedAt)
		if err != nil {
			return mapWriteError(err)
		}
	}
	return nil
}
