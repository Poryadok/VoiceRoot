package roomlifecycle

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresLifecycleStore struct {
	pool *pgxpool.Pool
}

func NewPostgresLifecycleStore(pool *pgxpool.Pool) *PostgresLifecycleStore {
	return &PostgresLifecycleStore{pool: pool}
}

func (store *PostgresLifecycleStore) CheckSchema(ctx context.Context) error {
	if store == nil || store.pool == nil {
		return ErrUnavailable
	}
	var ready bool
	err := store.pool.QueryRow(ctx, `
SELECT bool_and(to_regclass(table_name) IS NOT NULL)
FROM unnest(ARRAY[
  'voice_room_instances','voice_room_memberships','voice_lifecycle_operations',
  'voice_lifecycle_effects','voice_media_epoch_denials','voice_event_outbox'
]) AS table_name`).Scan(&ready)
	if err != nil || !ready {
		return ErrUnavailable
	}
	return nil
}

type lifecycleScanner interface {
	Scan(...any) error
}

type lifecycleQueryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func advisoryLock(ctx context.Context, tx pgx.Tx, key LifecycleAdvisoryLockKey) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1::integer,$2::integer)`, key.Namespace, key.Key)
	return err
}

func beginLifecycleTx(ctx context.Context, store *PostgresLifecycleStore) (pgx.Tx, error) {
	if store == nil || store.pool == nil {
		return nil, ErrUnavailable
	}
	tx, err := store.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, ErrUnavailable
	}
	return tx, nil
}

func finishLifecycleTx(ctx context.Context, tx pgx.Tx, errp *error) {
	if *errp != nil {
		rollbackCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		deferredCancel := time.AfterFunc(5*time.Second, cancel)
		defer deferredCancel.Stop()
		_ = tx.Rollback(rollbackCtx)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		*errp = mapWriteError(err)
	}
}

func unavailableSQLState(code string) bool {
	if len(code) < 2 {
		return false
	}
	switch code[:2] {
	case "08", "40", "53", "54", "55", "57", "58":
		return true
	default:
		return false
	}
}

func mapWriteError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return ErrUnavailable
	}
	if errors.Is(err, ErrInvariant) {
		return ErrInvariant
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.ConstraintName {
		case "voice_lifecycle_operations_pkey":
			return ErrOperationConflict
		case "voice_lifecycle_one_nonterminal_subject":
			return ErrSubjectTransitionActive
		case "voice_room_memberships_pkey", "voice_room_memberships_media_epoch_key":
			return ErrMembershipConflict
		}
		if len(pgErr.Code) >= 2 && pgErr.Code[:2] == "23" {
			return ErrInvariant
		}
		if unavailableSQLState(pgErr.Code) {
			return ErrUnavailable
		}
		return ErrInvariant
	}
	return ErrUnavailable
}

func mapScanError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrInvariant) {
		return ErrInvariant
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return ErrUnavailable
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if unavailableSQLState(pgErr.Code) {
			return ErrUnavailable
		}
		return ErrInvariant
	}
	return ErrUnavailable
}

func digestFromBytes(raw []byte) (LifecycleDigest, error) {
	if len(raw) != len(LifecycleDigest{}) {
		return LifecycleDigest{}, ErrInvariant
	}
	var digest LifecycleDigest
	copy(digest[:], raw)
	return digest, nil
}

func normalizeUTC(value *time.Time) {
	if value != nil {
		*value = value.UTC()
	}
}

func methodName(method LifecycleMethod) (string, error) {
	switch method {
	case LifecycleMethodJoin:
		return "join", nil
	case LifecycleMethodLeave:
		return "leave", nil
	case LifecycleMethodSelfMove:
		return "self_move", nil
	case LifecycleMethodModeratorMove:
		return "moderator_move", nil
	default:
		return "", ErrInvariant
	}
}

func parseMethod(value string) (LifecycleMethod, error) {
	switch value {
	case "join":
		return LifecycleMethodJoin, nil
	case "leave":
		return LifecycleMethodLeave, nil
	case "self_move":
		return LifecycleMethodSelfMove, nil
	case "moderator_move":
		return LifecycleMethodModeratorMove, nil
	default:
		return 0, ErrInvariant
	}
}

func outcomeName(outcome LifecycleOutcome) (string, error) {
	switch outcome {
	case LifecycleOutcomeJoined:
		return "joined", nil
	case LifecycleOutcomeLeft:
		return "left", nil
	case LifecycleOutcomeMoved:
		return "moved", nil
	case LifecycleOutcomeNoOp:
		return "no_op", nil
	default:
		return "", ErrInvariant
	}
}

func parseOutcome(value string) (LifecycleOutcome, error) {
	switch value {
	case "joined":
		return LifecycleOutcomeJoined, nil
	case "left":
		return LifecycleOutcomeLeft, nil
	case "moved":
		return LifecycleOutcomeMoved, nil
	case "no_op":
		return LifecycleOutcomeNoOp, nil
	default:
		return 0, ErrInvariant
	}
}

const operationColumns = `
actor_profile_id,operation_id,method,fingerprint,binding_bytes,binding_hash,
actor_account_id,subject_profile_id,space_id,source_voice_room_id,destination_voice_room_id,
source_room_id,destination_room_id,source_media_epoch,destination_media_epoch,
space_access_epoch,subject_role_policy_epoch,actor_source_role_policy_epoch,actor_destination_role_policy_epoch,
authorization_digest,can_join,can_publish_audio,can_publish_video,can_publish_screen_share,can_subscribe,
can_mute_others,can_deafen_others,can_move_others,can_use_ptt,priority_speaker,
actor_can_move_source,actor_can_move_destination,redis_owner_token,state,lease_owner,lease_until,lease_fence,
receipt_outcome,receipt_source_voice_room_id,receipt_destination_voice_room_id,receipt_room_id,
receipt_source_roster_version,receipt_destination_roster_version,receipt_media_epoch,
receipt_space_access_epoch,receipt_role_policy_epoch,receipt_authorization_digest,
receipt_bytes,receipt_hash,completed_at,replay_until,quarantine_class,quarantine_detail,quarantined_at,
decided_at,created_at,updated_at`

func scanOperation(row lifecycleScanner) (LifecycleOperation, error) {
	var operation LifecycleOperation
	var method, state string
	var fingerprint, bindingHash, authorizationDigest, ownerToken []byte
	var receiptOutcome *string
	var receiptAuthorizationDigest, receiptHash []byte
	var receipt LifecycleReceipt
	var spaceEpoch, roleEpoch, actorSourceEpoch, actorDestinationEpoch *int64
	var canJoin, canPublishAudio, canPublishVideo, canPublishScreen, canSubscribe *bool
	var canMute, canDeafen, canMove, canPTT, priority, actorCanSource, actorCanDestination *bool
	err := row.Scan(
		&operation.ActorProfileID, &operation.OperationID, &method, &fingerprint, &operation.BindingBytes, &bindingHash,
		&operation.ActorAccountID, &operation.SubjectProfileID, &operation.SpaceID, &operation.SourceVoiceRoomID, &operation.DestinationVoiceRoomID,
		&operation.SourceRoomID, &operation.DestinationRoomID, &operation.SourceMediaEpoch, &operation.DestinationMediaEpoch,
		&spaceEpoch, &roleEpoch, &actorSourceEpoch, &actorDestinationEpoch,
		&authorizationDigest, &canJoin, &canPublishAudio, &canPublishVideo, &canPublishScreen, &canSubscribe,
		&canMute, &canDeafen, &canMove, &canPTT, &priority, &actorCanSource, &actorCanDestination,
		&ownerToken, &state, &operation.LeaseOwner, &operation.LeaseUntil, &operation.LeaseFence,
		&receiptOutcome, &receipt.SourceVoiceRoomID, &receipt.DestinationVoiceRoomID, &receipt.RoomID,
		&receipt.SourceRosterVersion, &receipt.DestinationRosterVersion, &receipt.MediaEpoch,
		&receipt.SpaceAccessEpoch, &receipt.RolePolicyEpoch, &receiptAuthorizationDigest,
		&operation.ReceiptBytes, &receiptHash, &operation.CompletedAt, &operation.ReplayUntil,
		&operation.QuarantineClass, &operation.QuarantineDetail, &operation.QuarantinedAt,
		&operation.DecidedAt, &operation.CreatedAt, &operation.UpdatedAt,
	)
	if err != nil {
		return LifecycleOperation{}, err
	}
	parsedMethod, err := parseMethod(method)
	if err != nil {
		return LifecycleOperation{}, err
	}
	operation.Method = parsedMethod
	operation.Fingerprint, err = digestFromBytes(fingerprint)
	if err != nil {
		return LifecycleOperation{}, err
	}
	storedBindingHash, err := digestFromBytes(bindingHash)
	if err != nil {
		return LifecycleOperation{}, err
	}
	computedBindingHash := LifecycleDigest(sha256.Sum256(operation.BindingBytes))
	if operation.Fingerprint != computedBindingHash || storedBindingHash != computedBindingHash {
		return LifecycleOperation{}, ErrInvariant
	}
	operation.RedisOwnerToken, err = digestFromBytes(ownerToken)
	if err != nil {
		return LifecycleOperation{}, err
	}
	operation.Authority = LifecycleAuthority{
		ActorSourceRolePolicyEpoch: actorSourceEpoch, ActorDestinationRolePolicyEpoch: actorDestinationEpoch,
		ActorCanMoveSource: actorCanSource, ActorCanMoveDestination: actorCanDestination,
	}
	if spaceEpoch != nil {
		operation.Authority.SpaceAccessEpoch = *spaceEpoch
	}
	if roleEpoch != nil {
		operation.Authority.SubjectRolePolicyEpoch = *roleEpoch
	}
	if authorizationDigest != nil {
		operation.Authority.AuthorizationDigest, err = digestFromBytes(authorizationDigest)
		if err != nil {
			return LifecycleOperation{}, err
		}
	}
	grants := []*bool{canJoin, canPublishAudio, canPublishVideo, canPublishScreen, canSubscribe, canMute, canDeafen, canMove, canPTT, priority}
	allNil := true
	for _, grant := range grants {
		allNil = allNil && grant == nil
	}
	if !allNil {
		for _, grant := range grants {
			if grant == nil {
				return LifecycleOperation{}, ErrInvariant
			}
		}
		operation.Authority.SubjectGrants = LifecycleGrants{
			CanJoin: *canJoin, CanPublishAudio: *canPublishAudio, CanPublishVideo: *canPublishVideo,
			CanPublishScreenShare: *canPublishScreen, CanSubscribe: *canSubscribe, CanMuteOthers: *canMute,
			CanDeafenOthers: *canDeafen, CanMoveOthers: *canMove, CanUsePTT: *canPTT, PrioritySpeaker: *priority,
		}
	}
	switch state {
	case "decided":
		operation.State = LifecycleOperationDecided
	case "completed":
		operation.State = LifecycleOperationCompleted
	case "quarantined":
		operation.State = LifecycleOperationQuarantined
	default:
		return LifecycleOperation{}, ErrInvariant
	}
	if operation.LeaseFence < 0 {
		return LifecycleOperation{}, ErrInvariant
	}
	if receiptOutcome != nil {
		parsedOutcome, parseErr := parseOutcome(*receiptOutcome)
		if parseErr != nil {
			return LifecycleOperation{}, parseErr
		}
		receipt.OperationID = operation.OperationID
		receipt.ActorProfileID = operation.ActorProfileID
		receipt.SubjectProfileID = operation.SubjectProfileID
		receipt.SpaceID = operation.SpaceID
		receipt.Method = operation.Method
		receipt.Outcome = parsedOutcome
		if receiptAuthorizationDigest != nil {
			digest, digestErr := digestFromBytes(receiptAuthorizationDigest)
			if digestErr != nil {
				return LifecycleOperation{}, digestErr
			}
			receipt.AuthorizationDigest = &digest
		}
		receiptDigest, digestErr := digestFromBytes(receiptHash)
		if digestErr != nil {
			return LifecycleOperation{}, digestErr
		}
		canonicalBytes, canonicalHash, encodeErr := EncodeLifecycleReceipt(receipt)
		if encodeErr != nil || !bytes.Equal(operation.ReceiptBytes, canonicalBytes) || receiptDigest != canonicalHash {
			return LifecycleOperation{}, ErrInvariant
		}
		operation.ReceiptHash = &receiptDigest
		operation.Receipt = &receipt
	} else {
		if len(operation.ReceiptBytes) != 0 || len(receiptHash) != 0 {
			return LifecycleOperation{}, ErrInvariant
		}
		operation.Receipt = nil
	}
	operation.DecidedAt = operation.DecidedAt.UTC()
	operation.CreatedAt = operation.CreatedAt.UTC()
	operation.UpdatedAt = operation.UpdatedAt.UTC()
	normalizeUTC(operation.LeaseUntil)
	normalizeUTC(operation.CompletedAt)
	normalizeUTC(operation.ReplayUntil)
	normalizeUTC(operation.QuarantinedAt)
	return operation, nil
}

func loadOperation(ctx context.Context, queryer lifecycleQueryer, actorProfileID, operationID uuid.UUID, suffix string) (LifecycleOperation, bool, error) {
	operation, err := scanOperation(queryer.QueryRow(ctx, `SELECT `+operationColumns+` FROM voice_lifecycle_operations WHERE actor_profile_id=$1 AND operation_id=$2 `+suffix, actorProfileID, operationID))
	if errors.Is(err, pgx.ErrNoRows) {
		return LifecycleOperation{}, false, nil
	}
	if err != nil {
		return LifecycleOperation{}, false, mapScanError(err)
	}
	return operation, true, nil
}

func (store *PostgresLifecycleStore) LoadOperation(ctx context.Context, actorProfileID, operationID uuid.UUID) (LifecycleOperation, bool, error) {
	if store == nil || store.pool == nil || anyNilUUID(actorProfileID, operationID) {
		return LifecycleOperation{}, false, ErrUnavailable
	}
	return loadOperation(ctx, store.pool, actorProfileID, operationID, "")
}

func scanRoom(row lifecycleScanner) (LifecycleRoomSnapshot, error) {
	var room LifecycleRoomSnapshot
	var state string
	err := row.Scan(&room.RoomID, &room.SpaceID, &room.VoiceRoomID, &room.LiveKitRoomName, &state,
		&room.RosterVersion, &room.CreatedAt, &room.UpdatedAt, &room.ClosedAt)
	if err != nil {
		return LifecycleRoomSnapshot{}, err
	}
	if room.RosterVersion < 0 || !nonblank(room.LiveKitRoomName) || (state != "active" && state != "closed") {
		return LifecycleRoomSnapshot{}, ErrInvariant
	}
	room.Active = state == "active"
	room.CreatedAt = room.CreatedAt.UTC()
	room.UpdatedAt = room.UpdatedAt.UTC()
	normalizeUTC(room.ClosedAt)
	return room, nil
}

func (store *PostgresLifecycleStore) LoadRoomSnapshot(ctx context.Context, roomID uuid.UUID) (LifecycleRoomSnapshot, bool, error) {
	if store == nil || store.pool == nil || roomID == uuid.Nil {
		return LifecycleRoomSnapshot{}, false, ErrUnavailable
	}
	room, err := scanRoom(store.pool.QueryRow(ctx, `
SELECT room_id,space_id,voice_room_id,livekit_room_name,state,roster_version,created_at,updated_at,closed_at
FROM voice_room_instances WHERE room_id=$1`, roomID))
	if errors.Is(err, pgx.ErrNoRows) {
		return LifecycleRoomSnapshot{}, false, nil
	}
	if err != nil {
		return LifecycleRoomSnapshot{}, false, mapScanError(err)
	}
	return room, true, nil
}

const membershipColumns = `profile_id,room_id,media_epoch,space_access_epoch,role_policy_epoch,
authorization_digest,can_join,can_publish_audio,can_publish_video,can_publish_screen_share,
can_subscribe,can_mute_others,can_deafen_others,can_move_others,can_use_ptt,priority_speaker,
latest_grant_expires_at,joined_at,updated_at`

func scanMembership(row lifecycleScanner) (LifecycleMembership, error) {
	var membership LifecycleMembership
	var digest []byte
	err := row.Scan(
		&membership.ProfileID, &membership.RoomID, &membership.MediaEpoch,
		&membership.SpaceAccessEpoch, &membership.RolePolicyEpoch, &digest,
		&membership.Grants.CanJoin, &membership.Grants.CanPublishAudio, &membership.Grants.CanPublishVideo,
		&membership.Grants.CanPublishScreenShare, &membership.Grants.CanSubscribe, &membership.Grants.CanMuteOthers,
		&membership.Grants.CanDeafenOthers, &membership.Grants.CanMoveOthers, &membership.Grants.CanUsePTT,
		&membership.Grants.PrioritySpeaker, &membership.LatestGrantExpiresAt, &membership.JoinedAt, &membership.UpdatedAt,
	)
	if err != nil {
		return LifecycleMembership{}, err
	}
	if anyNilUUID(membership.ProfileID, membership.RoomID, membership.MediaEpoch) || membership.SpaceAccessEpoch <= 0 || membership.RolePolicyEpoch <= 0 {
		return LifecycleMembership{}, ErrInvariant
	}
	membership.AuthorizationDigest, err = digestFromBytes(digest)
	if err != nil {
		return LifecycleMembership{}, err
	}
	membership.JoinedAt = membership.JoinedAt.UTC()
	membership.UpdatedAt = membership.UpdatedAt.UTC()
	normalizeUTC(membership.LatestGrantExpiresAt)
	return membership, nil
}

func loadMembership(ctx context.Context, queryer lifecycleQueryer, profileID uuid.UUID, suffix string) (LifecycleMembership, bool, error) {
	membership, err := scanMembership(queryer.QueryRow(ctx, `SELECT `+membershipColumns+` FROM voice_room_memberships WHERE profile_id=$1 `+suffix, profileID))
	if errors.Is(err, pgx.ErrNoRows) {
		return LifecycleMembership{}, false, nil
	}
	if err != nil {
		return LifecycleMembership{}, false, mapScanError(err)
	}
	return membership, true, nil
}

func (store *PostgresLifecycleStore) LoadMembership(ctx context.Context, profileID uuid.UUID) (LifecycleMembership, bool, error) {
	if store == nil || store.pool == nil || profileID == uuid.Nil {
		return LifecycleMembership{}, false, ErrUnavailable
	}
	return loadMembership(ctx, store.pool, profileID, "")
}

func deterministicLiveKitRoomName(voiceRoomID uuid.UUID) string {
	return "voice-" + voiceRoomID.String()
}
