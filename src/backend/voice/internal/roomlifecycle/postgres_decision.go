package roomlifecycle

import (
	"bytes"
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type lifecycleDecisionRows struct {
	source      *LifecycleRoomSnapshot
	destination *LifecycleRoomSnapshot
	membership  *LifecycleMembership
	mediaEpoch  *uuid.UUID
}

func (store *PostgresLifecycleStore) DecideOperation(ctx context.Context, decision LifecycleDecision) (_ LifecycleOperation, err error) {
	bindingBytes, fingerprint, err := encodeLifecycleDecision(decision)
	if err != nil {
		return LifecycleOperation{}, err
	}
	tx, err := beginLifecycleTx(ctx, store)
	if err != nil {
		return LifecycleOperation{}, err
	}
	defer finishLifecycleTx(ctx, tx, &err)
	if err = advisoryLock(ctx, tx, LifecycleOperationAdvisoryKey(decision.ActorProfileID, decision.OperationID)); err != nil {
		return LifecycleOperation{}, mapWriteError(err)
	}
	existing, found, err := loadOperation(ctx, tx, decision.ActorProfileID, decision.OperationID, "FOR UPDATE")
	if err != nil {
		return LifecycleOperation{}, err
	}
	if found {
		if existing.Fingerprint != fingerprint || !bytes.Equal(existing.BindingBytes, bindingBytes) {
			return LifecycleOperation{}, ErrOperationConflict
		}
		return existing, nil
	}
	if err = advisoryLock(ctx, tx, LifecycleSubjectAdvisoryKey(decision.SubjectProfileID)); err != nil {
		return LifecycleOperation{}, mapWriteError(err)
	}
	var subjectFenced bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(
 SELECT 1 FROM voice_lifecycle_operations WHERE subject_profile_id=$1 AND completed_at IS NULL
)`, decision.SubjectProfileID).Scan(&subjectFenced); err != nil {
		return LifecycleOperation{}, ErrUnavailable
	}
	if subjectFenced {
		return LifecycleOperation{}, ErrSubjectTransitionActive
	}
	for _, lock := range decisionLogicalLocks(decision) {
		if err = advisoryLock(ctx, tx, lock.Key); err != nil {
			return LifecycleOperation{}, mapWriteError(err)
		}
	}
	membership, membershipFound, err := loadMembership(ctx, tx, decision.SubjectProfileID, "FOR UPDATE")
	if err != nil {
		return LifecycleOperation{}, err
	}
	rows, err := resolveDecisionRows(ctx, tx, decision, membership, membershipFound)
	if err != nil {
		return LifecycleOperation{}, err
	}
	if err = lockRuntimeRooms(ctx, tx, rows); err != nil {
		return LifecycleOperation{}, err
	}
	if err = revalidateDecisionRows(ctx, tx, decision, &rows); err != nil {
		return LifecycleOperation{}, err
	}
	if err = insertDecisionOperation(ctx, tx, decision, rows, bindingBytes, fingerprint); err != nil {
		return LifecycleOperation{}, mapWriteError(err)
	}
	if err = insertDecisionEffects(ctx, tx, decision, rows); err != nil {
		return LifecycleOperation{}, mapWriteError(err)
	}
	if err = insertDecisionDenial(ctx, tx, decision, rows); err != nil {
		return LifecycleOperation{}, mapWriteError(err)
	}
	operation, found, err := loadOperation(ctx, tx, decision.ActorProfileID, decision.OperationID, "")
	if err != nil || !found {
		if err == nil {
			err = ErrInvariant
		}
		return LifecycleOperation{}, err
	}
	return operation, nil
}

func (store *PostgresLifecycleStore) CompleteNoOp(ctx context.Context, noOp LifecycleNoOpDecision) (_ LifecycleOperation, err error) {
	bindingBytes, fingerprint, err := encodeLifecycleDecision(noOp.Decision)
	if err != nil || (noOp.Decision.Method != LifecycleMethodJoin && noOp.Decision.Method != LifecycleMethodLeave) ||
		noOp.CompletedAt.IsZero() || noOp.ReplayUntil.Before(noOp.CompletedAt.Add(24*time.Hour)) {
		return LifecycleOperation{}, ErrInvariant
	}
	tx, err := beginLifecycleTx(ctx, store)
	if err != nil {
		return LifecycleOperation{}, err
	}
	defer finishLifecycleTx(ctx, tx, &err)
	decision := noOp.Decision
	if err = advisoryLock(ctx, tx, LifecycleOperationAdvisoryKey(decision.ActorProfileID, decision.OperationID)); err != nil {
		return LifecycleOperation{}, mapWriteError(err)
	}
	existing, found, err := loadOperation(ctx, tx, decision.ActorProfileID, decision.OperationID, "FOR UPDATE")
	if err != nil {
		return LifecycleOperation{}, err
	}
	if found {
		if existing.Fingerprint != fingerprint || !bytes.Equal(existing.BindingBytes, bindingBytes) ||
			existing.State != LifecycleOperationCompleted || existing.Receipt == nil || existing.Receipt.Outcome != LifecycleOutcomeNoOp {
			return LifecycleOperation{}, ErrOperationConflict
		}
		return existing, nil
	}
	if err = advisoryLock(ctx, tx, LifecycleSubjectAdvisoryKey(decision.SubjectProfileID)); err != nil {
		return LifecycleOperation{}, mapWriteError(err)
	}
	var subjectFenced bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(
 SELECT 1 FROM voice_lifecycle_operations WHERE subject_profile_id=$1 AND completed_at IS NULL
)`, decision.SubjectProfileID).Scan(&subjectFenced); err != nil {
		return LifecycleOperation{}, mapScanError(err)
	}
	if subjectFenced {
		return LifecycleOperation{}, ErrSubjectTransitionActive
	}
	logicalRoomID := decision.SourceVoiceRoomID
	if decision.Method == LifecycleMethodJoin {
		logicalRoomID = decision.DestinationVoiceRoomID
	}
	if err = advisoryLock(ctx, tx, LifecycleLogicalRoomAdvisoryKey(*logicalRoomID)); err != nil {
		return LifecycleOperation{}, mapWriteError(err)
	}
	membership, membershipFound, err := loadMembership(ctx, tx, decision.SubjectProfileID, "FOR UPDATE")
	if err != nil {
		return LifecycleOperation{}, err
	}
	var room *LifecycleRoomSnapshot
	var authority LifecycleAuthority
	receipt := LifecycleReceipt{
		OperationID: decision.OperationID, ActorProfileID: decision.ActorProfileID,
		SubjectProfileID: decision.SubjectProfileID, SpaceID: decision.SpaceID,
		Method: decision.Method, Outcome: LifecycleOutcomeNoOp,
	}
	if decision.Method == LifecycleMethodLeave {
		if membershipFound {
			return LifecycleOperation{}, ErrMembershipConflict
		}
	} else {
		if !membershipFound {
			return LifecycleOperation{}, ErrMembershipConflict
		}
		loadedRoom, roomFound, loadErr := loadRoomTx(ctx, tx, membership.RoomID, "FOR UPDATE")
		if loadErr != nil || !roomFound || !loadedRoom.Active || loadedRoom.SpaceID != decision.SpaceID || loadedRoom.VoiceRoomID != *decision.DestinationVoiceRoomID {
			return LifecycleOperation{}, ErrMembershipConflict
		}
		room = &loadedRoom
		authority = LifecycleAuthority{
			SpaceAccessEpoch: membership.SpaceAccessEpoch, SubjectRolePolicyEpoch: membership.RolePolicyEpoch,
			AuthorizationDigest: membership.AuthorizationDigest, SubjectGrants: membership.Grants,
		}
		receipt.DestinationVoiceRoomID = decision.DestinationVoiceRoomID
		receipt.RoomID = &loadedRoom.RoomID
		receipt.DestinationRosterVersion = &loadedRoom.RosterVersion
		receipt.MediaEpoch = &membership.MediaEpoch
		receipt.SpaceAccessEpoch = &membership.SpaceAccessEpoch
		receipt.RolePolicyEpoch = &membership.RolePolicyEpoch
		receipt.AuthorizationDigest = &membership.AuthorizationDigest
	}
	receiptBytes, receiptHash, err := EncodeLifecycleReceipt(receipt)
	if err != nil {
		return LifecycleOperation{}, err
	}
	if err = insertCompletedNoOp(ctx, tx, noOp, authority, room, bindingBytes, fingerprint, receipt, receiptBytes, receiptHash); err != nil {
		return LifecycleOperation{}, mapWriteError(err)
	}
	operation, found, err := loadOperation(ctx, tx, decision.ActorProfileID, decision.OperationID, "")
	if err != nil || !found {
		if err == nil {
			err = ErrInvariant
		}
		return LifecycleOperation{}, err
	}
	return operation, nil
}

func insertCompletedNoOp(ctx context.Context, tx pgx.Tx, noOp LifecycleNoOpDecision, authority LifecycleAuthority,
	room *LifecycleRoomSnapshot, bindingBytes []byte, fingerprint LifecycleDigest, receipt LifecycleReceipt,
	receiptBytes []byte, receiptHash LifecycleDigest) error {
	decision := noOp.Decision
	method, _ := methodName(decision.Method)
	storedDecision := decision
	storedDecision.Authority = authority
	spaceEpoch, roleEpoch, actorSourceEpoch, actorDestinationEpoch, authDigest,
		canJoin, canAudio, canVideo, canScreen, canSubscribe, canMute, canDeafen, canMove, canPTT, priority := nullableAuthority(storedDecision)
	var sourceRoom, destinationRoom, sourceMedia, destinationMedia any
	if room != nil {
		destinationRoom = room.RoomID
		destinationMedia = *receipt.MediaEpoch
	}
	_, err := tx.Exec(ctx, `
INSERT INTO voice_lifecycle_operations(
 actor_profile_id,operation_id,schema_version,method,fingerprint,binding_bytes,binding_hash,
 actor_account_id,subject_profile_id,space_id,source_voice_room_id,destination_voice_room_id,
 source_room_id,destination_room_id,source_media_epoch,destination_media_epoch,
 space_access_epoch,subject_role_policy_epoch,actor_source_role_policy_epoch,actor_destination_role_policy_epoch,
 authorization_digest,can_join,can_publish_audio,can_publish_video,can_publish_screen_share,can_subscribe,
 can_mute_others,can_deafen_others,can_move_others,can_use_ptt,priority_speaker,
 actor_can_move_source,actor_can_move_destination,decided_at,created_at,updated_at,redis_owner_token,state,lease_fence,
 receipt_outcome,receipt_source_voice_room_id,receipt_destination_voice_room_id,receipt_room_id,
 receipt_source_roster_version,receipt_destination_roster_version,receipt_media_epoch,
 receipt_space_access_epoch,receipt_role_policy_epoch,receipt_authorization_digest,
 receipt_bytes,receipt_hash,completed_at,replay_until)
VALUES(
 @actor,@operation,1,@method,@fingerprint,@binding,@binding_hash,@account,@subject,@space,@source_voice,@destination_voice,
 @source_room,@destination_room,@source_media,@destination_media,@space_epoch,@role_epoch,@actor_source_epoch,@actor_destination_epoch,
 @auth_digest,@g0,@g1,@g2,@g3,@g4,@g5,@g6,@g7,@g8,@g9,@actor_can_source,@actor_can_destination,
 @decided_at,@decided_at,@completed_at,@owner_token,'completed',0,'no_op',
 @receipt_source_voice,@receipt_destination_voice,@receipt_room,@receipt_source_roster,@receipt_destination_roster,
 @receipt_media,@receipt_space_epoch,@receipt_role_epoch,@receipt_digest,@receipt_bytes,@receipt_hash,@completed_at,@replay_until)`, pgx.NamedArgs{
		"actor": decision.ActorProfileID, "operation": decision.OperationID, "method": method,
		"fingerprint": fingerprint[:], "binding": bindingBytes, "binding_hash": fingerprint[:],
		"account": decision.ActorAccountID, "subject": decision.SubjectProfileID, "space": decision.SpaceID,
		"source_voice": decision.SourceVoiceRoomID, "destination_voice": decision.DestinationVoiceRoomID,
		"source_room": sourceRoom, "destination_room": destinationRoom, "source_media": sourceMedia, "destination_media": destinationMedia,
		"space_epoch": spaceEpoch, "role_epoch": roleEpoch, "actor_source_epoch": actorSourceEpoch, "actor_destination_epoch": actorDestinationEpoch,
		"auth_digest": authDigest, "g0": canJoin, "g1": canAudio, "g2": canVideo, "g3": canScreen, "g4": canSubscribe,
		"g5": canMute, "g6": canDeafen, "g7": canMove, "g8": canPTT, "g9": priority,
		"actor_can_source": decision.Authority.ActorCanMoveSource, "actor_can_destination": decision.Authority.ActorCanMoveDestination,
		"decided_at": decision.DecidedAt, "completed_at": noOp.CompletedAt, "owner_token": decision.RedisOwnerToken[:],
		"receipt_source_voice": receipt.SourceVoiceRoomID, "receipt_destination_voice": receipt.DestinationVoiceRoomID,
		"receipt_room": receipt.RoomID, "receipt_source_roster": receipt.SourceRosterVersion,
		"receipt_destination_roster": receipt.DestinationRosterVersion, "receipt_media": receipt.MediaEpoch,
		"receipt_space_epoch": receipt.SpaceAccessEpoch, "receipt_role_epoch": receipt.RolePolicyEpoch,
		"receipt_digest": digestBytes(receipt.AuthorizationDigest), "receipt_bytes": receiptBytes, "receipt_hash": receiptHash[:],
		"replay_until": noOp.ReplayUntil,
	})
	return err
}

func digestBytes(digest *LifecycleDigest) any {
	if digest == nil {
		return nil
	}
	return digest[:]
}

func decisionLogicalLocks(decision LifecycleDecision) []LifecycleLogicalRoomLock {
	var ids []uuid.UUID
	if decision.SourceVoiceRoomID != nil {
		ids = append(ids, *decision.SourceVoiceRoomID)
	}
	if decision.DestinationVoiceRoomID != nil {
		ids = append(ids, *decision.DestinationVoiceRoomID)
	}
	return SortLifecycleLogicalRoomLocks(ids)
}

func resolveDecisionRows(ctx context.Context, tx pgx.Tx, decision LifecycleDecision, membership LifecycleMembership, found bool) (lifecycleDecisionRows, error) {
	rows := lifecycleDecisionRows{}
	if found {
		rows.membership = &membership
	}
	switch decision.Method {
	case LifecycleMethodJoin:
		if found {
			return rows, ErrMembershipConflict
		}
	case LifecycleMethodLeave, LifecycleMethodSelfMove, LifecycleMethodModeratorMove:
		if !found {
			return rows, ErrMembershipConflict
		}
		source, ok, err := loadRoomTx(ctx, tx, membership.RoomID, "")
		if err != nil || !ok {
			return rows, ErrInvariant
		}
		if !source.Active || source.SpaceID != decision.SpaceID || source.VoiceRoomID != *decision.SourceVoiceRoomID {
			return rows, ErrMembershipConflict
		}
		rows.source = &source
	}
	if decision.DestinationVoiceRoomID != nil {
		destination, err := createOrReloadDestination(ctx, tx, decision.SpaceID, *decision.DestinationVoiceRoomID, decision.DecidedAt)
		if err != nil {
			return rows, err
		}
		rows.destination = &destination
		mediaEpoch := uuid.New()
		rows.mediaEpoch = &mediaEpoch
	}
	return rows, nil
}

func loadRoomTx(ctx context.Context, tx pgx.Tx, roomID uuid.UUID, suffix string) (LifecycleRoomSnapshot, bool, error) {
	room, err := scanRoom(tx.QueryRow(ctx, `
SELECT room_id,space_id,voice_room_id,livekit_room_name,state,roster_version,created_at,updated_at,closed_at
FROM voice_room_instances WHERE room_id=$1 `+suffix, roomID))
	if errors.Is(err, pgx.ErrNoRows) {
		return LifecycleRoomSnapshot{}, false, nil
	}
	if err != nil {
		return LifecycleRoomSnapshot{}, false, mapScanError(err)
	}
	return room, true, nil
}

func createOrReloadDestination(ctx context.Context, tx pgx.Tx, spaceID, voiceRoomID uuid.UUID, at time.Time) (LifecycleRoomSnapshot, error) {
	load := func() (LifecycleRoomSnapshot, bool, error) {
		room, err := scanRoom(tx.QueryRow(ctx, `
SELECT room_id,space_id,voice_room_id,livekit_room_name,state,roster_version,created_at,updated_at,closed_at
FROM voice_room_instances WHERE voice_room_id=$1 AND state='active' FOR UPDATE`, voiceRoomID))
		if errors.Is(err, pgx.ErrNoRows) {
			return LifecycleRoomSnapshot{}, false, nil
		}
		if err != nil {
			return LifecycleRoomSnapshot{}, false, mapScanError(err)
		}
		return room, true, nil
	}
	room, found, err := load()
	if err != nil {
		return LifecycleRoomSnapshot{}, err
	}
	wantName := deterministicLiveKitRoomName(voiceRoomID)
	if !found {
		_, err = tx.Exec(ctx, `
INSERT INTO voice_room_instances(room_id,space_id,voice_room_id,livekit_room_name,state,roster_version,created_at,updated_at,closed_at)
VALUES($1,$2,$3,$4,'active',0,$5,$5,NULL)
ON CONFLICT (voice_room_id) WHERE state='active' DO NOTHING`, uuid.New(), spaceID, voiceRoomID, wantName, at)
		if err != nil {
			return LifecycleRoomSnapshot{}, mapWriteError(err)
		}
		room, found, err = load()
		if err != nil || !found {
			return LifecycleRoomSnapshot{}, ErrInvariant
		}
	}
	if room.SpaceID != spaceID || room.VoiceRoomID != voiceRoomID || !room.Active || room.LiveKitRoomName != wantName {
		return LifecycleRoomSnapshot{}, ErrInvariant
	}
	return room, nil
}

func lockRuntimeRooms(ctx context.Context, tx pgx.Tx, rows lifecycleDecisionRows) error {
	var ids []uuid.UUID
	if rows.source != nil {
		ids = append(ids, rows.source.RoomID)
	}
	if rows.destination != nil {
		ids = append(ids, rows.destination.RoomID)
	}
	sort.Slice(ids, func(i, j int) bool { return bytes.Compare(ids[i][:], ids[j][:]) < 0 })
	for _, id := range ids {
		if _, err := tx.Exec(ctx, `SELECT 1 FROM voice_room_instances WHERE room_id=$1 FOR UPDATE`, id); err != nil {
			return mapWriteError(err)
		}
	}
	return nil
}

func revalidateDecisionRows(ctx context.Context, tx pgx.Tx, decision LifecycleDecision, rows *lifecycleDecisionRows) error {
	if rows.source != nil {
		room, found, err := loadRoomTx(ctx, tx, rows.source.RoomID, "")
		if err != nil {
			return err
		}
		if !found || !room.Active || room.RoomID != rows.source.RoomID || room.SpaceID != decision.SpaceID ||
			decision.SourceVoiceRoomID == nil || room.VoiceRoomID != *decision.SourceVoiceRoomID {
			return ErrMembershipConflict
		}
		rows.source = &room
	}
	if rows.destination != nil {
		room, found, err := loadRoomTx(ctx, tx, rows.destination.RoomID, "")
		if err != nil {
			return err
		}
		if !found || !room.Active || room.RoomID != rows.destination.RoomID || room.SpaceID != decision.SpaceID ||
			decision.DestinationVoiceRoomID == nil || room.VoiceRoomID != *decision.DestinationVoiceRoomID ||
			room.LiveKitRoomName != deterministicLiveKitRoomName(*decision.DestinationVoiceRoomID) {
			return ErrInvariant
		}
		rows.destination = &room
	}
	return nil
}

func nullableAuthority(decision LifecycleDecision) (any, any, any, any, any, any, any, any, any, any, any, any, any, any, any) {
	if decision.Method == LifecycleMethodLeave {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil
	}
	authority := decision.Authority
	return authority.SpaceAccessEpoch, authority.SubjectRolePolicyEpoch,
		authority.ActorSourceRolePolicyEpoch, authority.ActorDestinationRolePolicyEpoch, authority.AuthorizationDigest[:],
		authority.SubjectGrants.CanJoin, authority.SubjectGrants.CanPublishAudio, authority.SubjectGrants.CanPublishVideo,
		authority.SubjectGrants.CanPublishScreenShare, authority.SubjectGrants.CanSubscribe, authority.SubjectGrants.CanMuteOthers,
		authority.SubjectGrants.CanDeafenOthers, authority.SubjectGrants.CanMoveOthers, authority.SubjectGrants.CanUsePTT,
		authority.SubjectGrants.PrioritySpeaker
}

func insertDecisionOperation(ctx context.Context, tx pgx.Tx, decision LifecycleDecision, rows lifecycleDecisionRows, bindingBytes []byte, fingerprint LifecycleDigest) error {
	method, _ := methodName(decision.Method)
	spaceEpoch, roleEpoch, actorSourceEpoch, actorDestinationEpoch, authDigest,
		canJoin, canAudio, canVideo, canScreen, canSubscribe, canMute, canDeafen, canMove, canPTT, priority := nullableAuthority(decision)
	var sourceRoomID, destinationRoomID, sourceMediaEpoch, destinationMediaEpoch any
	if rows.source != nil {
		sourceRoomID = rows.source.RoomID
		sourceMediaEpoch = rows.membership.MediaEpoch
	}
	if rows.destination != nil {
		destinationRoomID = rows.destination.RoomID
		destinationMediaEpoch = *rows.mediaEpoch
	}
	bindingHash := fingerprint
	_, err := tx.Exec(ctx, `
INSERT INTO voice_lifecycle_operations(
 actor_profile_id,operation_id,schema_version,method,fingerprint,binding_bytes,binding_hash,
 actor_account_id,subject_profile_id,space_id,source_voice_room_id,destination_voice_room_id,
 source_room_id,destination_room_id,source_media_epoch,destination_media_epoch,
 space_access_epoch,subject_role_policy_epoch,actor_source_role_policy_epoch,actor_destination_role_policy_epoch,
 authorization_digest,can_join,can_publish_audio,can_publish_video,can_publish_screen_share,can_subscribe,
 can_mute_others,can_deafen_others,can_move_others,can_use_ptt,priority_speaker,
 actor_can_move_source,actor_can_move_destination,decided_at,created_at,updated_at,redis_owner_token,state,lease_fence)
VALUES(
 $1,$2,1,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,
 $16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28,$29,$30,$31,$32,$33,$33,$33,$34,'decided',0)`,
		decision.ActorProfileID, decision.OperationID, method, fingerprint[:], bindingBytes, bindingHash[:],
		decision.ActorAccountID, decision.SubjectProfileID, decision.SpaceID, decision.SourceVoiceRoomID, decision.DestinationVoiceRoomID,
		sourceRoomID, destinationRoomID, sourceMediaEpoch, destinationMediaEpoch,
		spaceEpoch, roleEpoch, actorSourceEpoch, actorDestinationEpoch, authDigest,
		canJoin, canAudio, canVideo, canScreen, canSubscribe, canMute, canDeafen, canMove, canPTT, priority,
		decision.Authority.ActorCanMoveSource, decision.Authority.ActorCanMoveDestination,
		decision.DecidedAt, decision.RedisOwnerToken[:])
	return err
}

func insertDecisionEffects(ctx context.Context, tx pgx.Tx, decision LifecycleDecision, rows lifecycleDecisionRows) error {
	for _, plan := range decision.Effects {
		var targetRoom *LifecycleRoomSnapshot
		var profileID *uuid.UUID
		var identity *string
		var mediaEpoch *uuid.UUID
		kind := "livekit_ensure_room"
		if plan.Kind == LifecycleEffectEjectParticipant {
			kind = "livekit_eject_participant"
			targetRoom = rows.source
			profile := decision.SubjectProfileID
			profileID = &profile
			participant := "profile:" + profile.String() + ":media:" + rows.membership.MediaEpoch.String()
			identity = &participant
			media := rows.membership.MediaEpoch
			mediaEpoch = &media
		} else {
			targetRoom = rows.destination
		}
		requestBytes, requestDigest := encodeEffectRequest(plan.Kind, targetRoom.VoiceRoomID, targetRoom.LiveKitRoomName, profileID, identity, mediaEpoch)
		_, err := tx.Exec(ctx, `
INSERT INTO voice_lifecycle_effects(
 effect_id,actor_profile_id,operation_id,ordinal,kind,schema_version,target_voice_room_id,target_livekit_room_name,
 target_profile_id,target_participant_identity,target_media_epoch,request_bytes,request_digest,state,attempt_count,
 next_attempt_at,lease_fence,created_at,updated_at)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,'ready',0,$14,0,$14,$14)`,
			plan.EffectID, decision.ActorProfileID, decision.OperationID, plan.Ordinal, kind, plan.SchemaVersion,
			targetRoom.VoiceRoomID, targetRoom.LiveKitRoomName, profileID, identity, mediaEpoch, requestBytes, requestDigest[:], decision.DecidedAt)
		if err != nil {
			return err
		}
	}
	return nil
}

func insertDecisionDenial(ctx context.Context, tx pgx.Tx, decision LifecycleDecision, rows lifecycleDecisionRows) error {
	if rows.membership == nil || rows.membership.LatestGrantExpiresAt == nil || rows.source == nil {
		return nil
	}
	participant := "profile:" + decision.SubjectProfileID.String() + ":media:" + rows.membership.MediaEpoch.String()
	method, _ := methodName(decision.Method)
	_, err := tx.Exec(ctx, `
INSERT INTO voice_media_epoch_denials(
 media_epoch,profile_id,room_id,livekit_room_name,participant_identity,grant_expires_at,
 accepted_clock_skew,deny_until,reason,operation_actor_profile_id,operation_id,created_at,updated_at)
VALUES($1,$2,$3,$4,$5,$6,$7,$6::timestamptz+$7::interval,$8,$9,$10,$11,$11)`,
		rows.membership.MediaEpoch, decision.SubjectProfileID, rows.source.RoomID, rows.source.LiveKitRoomName, participant,
		*rows.membership.LatestGrantExpiresAt, decision.AcceptedClockSkew, method, decision.ActorProfileID, decision.OperationID, decision.DecidedAt)
	return err
}

func (store *PostgresLifecycleStore) ClaimNextOperation(ctx context.Context, workerID uuid.UUID, now time.Time, leaseDuration time.Duration) (LifecycleOperationClaim, bool, error) {
	if store == nil || store.pool == nil || workerID == uuid.Nil || now.IsZero() || leaseDuration <= 0 {
		return LifecycleOperationClaim{}, false, ErrUnavailable
	}
	leaseUntil := now.Add(leaseDuration)
	operation, err := scanOperation(store.pool.QueryRow(ctx, `
WITH candidate AS (
 SELECT actor_profile_id,operation_id
 FROM voice_lifecycle_operations
 WHERE state='decided' AND (lease_until IS NULL OR lease_until <= $1)
 ORDER BY decided_at,actor_profile_id,operation_id
 FOR UPDATE SKIP LOCKED
 LIMIT 1
)
UPDATE voice_lifecycle_operations AS operation
SET lease_owner=$2,lease_until=$3,lease_fence=operation.lease_fence+1,updated_at=$1
FROM candidate
WHERE operation.actor_profile_id=candidate.actor_profile_id AND operation.operation_id=candidate.operation_id
RETURNING `+prefixedOperationColumns("operation"), now, workerID, leaseUntil))
	if errors.Is(err, pgx.ErrNoRows) {
		return LifecycleOperationClaim{}, false, nil
	}
	if err != nil {
		return LifecycleOperationClaim{}, false, mapScanError(err)
	}
	return LifecycleOperationClaim{Operation: operation, WorkerID: workerID, Fence: operation.LeaseFence, LeaseUntil: leaseUntil}, true, nil
}

func prefixedOperationColumns(alias string) string {
	columns := strings.Split(strings.TrimSpace(operationColumns), ",")
	for index := range columns {
		columns[index] = alias + "." + strings.TrimSpace(columns[index])
	}
	return strings.Join(columns, ",")
}

func (store *PostgresLifecycleStore) RenewOperationLease(ctx context.Context, lease LifecycleOperationLease) error {
	if store == nil || store.pool == nil || anyNilUUID(lease.ActorProfileID, lease.OperationID, lease.WorkerID) || lease.ExpectedFence <= 0 || lease.LeaseUntil.IsZero() {
		return ErrUnavailable
	}
	tag, err := store.pool.Exec(ctx, `
UPDATE voice_lifecycle_operations
SET lease_until=$1,updated_at=$1
WHERE actor_profile_id=$2 AND operation_id=$3 AND state='decided'
  AND lease_owner=$4 AND lease_fence=$5`, lease.LeaseUntil, lease.ActorProfileID, lease.OperationID, lease.WorkerID, lease.ExpectedFence)
	if err != nil {
		return mapWriteError(err)
	}
	if tag.RowsAffected() != 1 {
		return ErrStaleLease
	}
	return nil
}

func (store *PostgresLifecycleStore) QuarantineOperation(ctx context.Context, quarantine LifecycleOperationQuarantine) error {
	if store == nil || store.pool == nil || anyNilUUID(quarantine.ActorProfileID, quarantine.OperationID, quarantine.WorkerID) || quarantine.ExpectedFence <= 0 || !nonblank(quarantine.Class) || quarantine.At.IsZero() {
		return ErrUnavailable
	}
	var detail any
	if quarantine.Detail != "" {
		if !nonblank(quarantine.Detail) {
			return ErrInvariant
		}
		detail = quarantine.Detail
	}
	tag, err := store.pool.Exec(ctx, `
UPDATE voice_lifecycle_operations
SET state='quarantined',lease_owner=NULL,lease_until=NULL,
    quarantine_class=$1,quarantine_detail=$2,quarantined_at=$3,updated_at=$3
WHERE actor_profile_id=$4 AND operation_id=$5 AND state='decided'
  AND lease_owner=$6 AND lease_fence=$7`, quarantine.Class, detail, quarantine.At,
		quarantine.ActorProfileID, quarantine.OperationID, quarantine.WorkerID, quarantine.ExpectedFence)
	if err != nil {
		return mapWriteError(err)
	}
	if tag.RowsAffected() != 1 {
		return ErrStaleLease
	}
	return nil
}
