package matchsquad

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	callsv1 "voice.app/voice/calls/v1"
	"voice/backend/voice/internal/livekit"
	"voice/backend/voice/internal/store"
)

func (s *MatchSquadMemberService) CheckSchema(ctx context.Context) error {
	if s == nil || s.Pool == nil {
		return errors.New("Voice MatchSquad member database unavailable")
	}
	queries := []string{
		`SELECT operation_id,request_bytes,response_bytes,state,failure_code,match_id,room_id,profile_id,account_id,session_epoch,media_epoch,method FROM voice_match_squad_member_operations LIMIT 0`,
		`SELECT operation_id,effect_kind,match_id,room_id,profile_id,media_epoch,state,confirmed_at FROM voice_match_squad_member_effects LIMIT 0`,
		`SELECT request_id,match_id,room_id,profile_id,account_id,session_epoch,media_epoch,issued_at,expires_at FROM voice_match_squad_member_grants LIMIT 0`,
		`SELECT profile_id,account_id,session_epoch,media_epoch,membership_state,latest_grant_expires_at FROM voice_room_memberships LIMIT 0`,
	}
	for _, query := range queries {
		rows, err := s.Pool.Query(ctx, query)
		if err != nil {
			return errors.New("Voice MatchSquad member schema unavailable")
		}
		rows.Close()
		if rows.Err() != nil {
			return errors.New("Voice MatchSquad member schema unavailable")
		}
	}
	return nil
}

// RepairExpiredLeaves advances only generations whose durable leave effects
// are confirmed and whose latest issued bearer has expired according to the
// database clock. A failed fence release is retried for already-LEFT rows.
func (s *MatchSquadMemberService) RepairExpiredLeaves(ctx context.Context, limit int) error {
	if s == nil || s.Pool == nil || s.Fences == nil {
		return status.Error(codes.Unavailable, "Voice MatchSquad member authority unavailable")
	}
	if limit <= 0 || limit > 256 {
		limit = 64
	}
	if err := s.RepairPendingJoins(ctx, limit); err != nil {
		return err
	}
	if err := s.RepairPendingLeaves(ctx, limit); err != nil {
		return err
	}
	rows, err := s.Pool.Query(ctx, `SELECT o.match_id,m.profile_id,m.account_id,m.room_id,m.media_epoch
FROM voice_room_memberships m
JOIN voice_room_instances r ON r.room_id=m.room_id AND r.purpose='MATCH_SQUAD' AND r.room_type='group_voice'
JOIN voice_match_squad_operations o ON o.room_id=r.room_id AND o.match_id=r.owner_id
WHERE (m.membership_state='LEAVING'
       AND (m.latest_grant_expires_at IS NULL OR m.latest_grant_expires_at<=clock_timestamp())
       AND EXISTS (SELECT 1 FROM voice_match_squad_member_effects e WHERE e.match_id=o.match_id AND e.room_id=o.room_id AND e.profile_id=m.profile_id AND e.media_epoch=m.media_epoch AND e.effect_kind IN ('leave_projection','leave_livekit'))
       AND NOT EXISTS (SELECT 1 FROM voice_match_squad_member_effects e WHERE e.match_id=o.match_id AND e.room_id=o.room_id AND e.profile_id=m.profile_id AND e.media_epoch=m.media_epoch AND e.effect_kind IN ('leave_projection','leave_livekit') AND e.state<>'confirmed'))
	   OR (m.membership_state IN ('LEFT','EJECTED') AND EXISTS (SELECT 1 FROM voice_account_voice_fences f WHERE f.account_id=m.account_id AND f.profile_id=m.profile_id AND f.room_id=m.room_id::text AND f.state='active'))
ORDER BY m.updated_at,m.profile_id LIMIT $1`, limit)
	if err != nil {
		return status.Error(codes.Unavailable, "Voice MatchSquad leave repair unavailable")
	}
	type candidate struct{ match, profile, account, room, epoch uuid.UUID }
	var candidates []candidate
	for rows.Next() {
		var c candidate
		if err := rows.Scan(&c.match, &c.profile, &c.account, &c.room, &c.epoch); err != nil {
			rows.Close()
			return status.Error(codes.Unavailable, "Voice MatchSquad leave repair unavailable")
		}
		candidates = append(candidates, c)
	}
	rows.Close()
	if rows.Err() != nil {
		return status.Error(codes.Unavailable, "Voice MatchSquad leave repair unavailable")
	}
	for _, c := range candidates {
		tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return status.Error(codes.Unavailable, "Voice MatchSquad leave repair unavailable")
		}
		var state string
		err = tx.QueryRow(ctx, `SELECT m.membership_state FROM voice_room_memberships m
JOIN voice_room_instances r ON r.room_id=m.room_id AND r.purpose='MATCH_SQUAD' AND r.room_type='group_voice'
JOIN voice_match_squad_operations o ON o.room_id=r.room_id AND o.match_id=r.owner_id
WHERE o.match_id=$1 AND m.profile_id=$2 AND m.account_id=$3 AND m.room_id=$4 AND m.media_epoch=$5
AND ((m.membership_state='LEAVING' AND (m.latest_grant_expires_at IS NULL OR m.latest_grant_expires_at<=clock_timestamp())
 AND EXISTS (SELECT 1 FROM voice_match_squad_member_effects e WHERE e.match_id=o.match_id AND e.room_id=o.room_id AND e.profile_id=m.profile_id AND e.media_epoch=m.media_epoch AND e.effect_kind IN ('leave_projection','leave_livekit'))
 AND NOT EXISTS (SELECT 1 FROM voice_match_squad_member_effects e WHERE e.match_id=o.match_id AND e.room_id=o.room_id AND e.profile_id=m.profile_id AND e.media_epoch=m.media_epoch AND e.effect_kind IN ('leave_projection','leave_livekit') AND e.state<>'confirmed'))
		OR m.membership_state IN ('LEFT','EJECTED')) FOR UPDATE OF m`, c.match, c.profile, c.account, c.room, c.epoch).Scan(&state)
		if errors.Is(err, pgx.ErrNoRows) {
			_ = tx.Rollback(ctx)
			continue
		}
		if err != nil {
			_ = tx.Rollback(ctx)
			return status.Error(codes.Unavailable, "Voice MatchSquad leave repair unavailable")
		}
		if state == "LEAVING" {
			_, err = tx.Exec(ctx, `UPDATE voice_room_memberships m SET membership_state='LEFT',updated_at=clock_timestamp() WHERE m.profile_id=$1 AND m.room_id=$2 AND m.media_epoch=$3 AND m.membership_state='LEAVING' AND EXISTS (SELECT 1 FROM voice_room_instances r JOIN voice_match_squad_operations o ON o.room_id=r.room_id AND o.match_id=r.owner_id WHERE r.room_id=m.room_id AND r.owner_id=$4 AND r.purpose='MATCH_SQUAD' AND r.room_type='group_voice')`, c.profile, c.room, c.epoch, c.match)
			if err != nil {
				_ = tx.Rollback(ctx)
				return status.Error(codes.Unavailable, "Voice MatchSquad leave repair unavailable")
			}
		}
		if err := releaseTerminalFenceInTx(ctx, tx, c.match, c.room, c.account, c.profile, c.epoch); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return status.Error(codes.Unavailable, "Voice MatchSquad leave repair unavailable")
		}
	}
	return nil
}

// RepairPendingJoins permanently rejects a partially applied, unacknowledged
// Join before cleaning its projections. The operation row is locked first so
// a concurrent handler cannot commit a JOINED receipt after this rollback.
func (s *MatchSquadMemberService) RepairPendingJoins(ctx context.Context, limit int) error {
	if s == nil || s.Pool == nil || s.Calls == nil || s.RoomEffect == nil || s.Fences == nil {
		return status.Error(codes.Unavailable, "Voice MatchSquad member authority unavailable")
	}
	if limit <= 0 || limit > 256 {
		limit = 64
	}
	rows, err := s.Pool.Query(ctx, `SELECT o.operation_id,o.match_id,o.room_id,o.profile_id,o.account_id,o.session_epoch,o.media_epoch,r.livekit_room_name
FROM voice_match_squad_member_operations o
JOIN voice_room_instances r ON r.room_id=o.room_id AND r.owner_id=o.match_id AND r.purpose='MATCH_SQUAD' AND r.room_type='group_voice'
JOIN voice_room_memberships m ON m.room_id=o.room_id AND m.profile_id=o.profile_id AND m.account_id=o.account_id AND m.session_epoch=o.session_epoch AND m.media_epoch=o.media_epoch
WHERE o.method='join' AND o.state IN ('pending','rejected') AND m.membership_state IN ('JOINING','EJECTED')
AND EXISTS (SELECT 1 FROM voice_match_squad_member_effects e WHERE e.operation_id=o.operation_id AND e.effect_kind='join_projection' AND e.state='pending')
ORDER BY o.updated_at,o.operation_id LIMIT $1`, limit)
	if err != nil {
		return status.Error(codes.Unavailable, "Voice MatchSquad join repair unavailable")
	}
	type pendingJoin struct {
		operation, match, room, profile, account, epoch uuid.UUID
		session                                         int64
		livekitRoom                                     string
	}
	var pending []pendingJoin
	for rows.Next() {
		var item pendingJoin
		if err := rows.Scan(&item.operation, &item.match, &item.room, &item.profile, &item.account, &item.session, &item.epoch, &item.livekitRoom); err != nil {
			rows.Close()
			return status.Error(codes.Unavailable, "Voice MatchSquad join repair unavailable")
		}
		pending = append(pending, item)
	}
	rows.Close()
	if rows.Err() != nil {
		return status.Error(codes.Unavailable, "Voice MatchSquad join repair unavailable")
	}
	for _, item := range pending {
		tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return status.Error(codes.Unavailable, "Voice MatchSquad join repair unavailable")
		}
		var state string
		err = tx.QueryRow(ctx, `SELECT state FROM voice_match_squad_member_operations
WHERE operation_id=$1 AND match_id=$2 AND room_id=$3 AND profile_id=$4 AND account_id=$5 AND session_epoch=$6 AND media_epoch=$7 AND method='join' FOR UPDATE`, item.operation, item.match, item.room, item.profile, item.account, item.session, item.epoch).Scan(&state)
		if err != nil || state == "complete" {
			_ = tx.Rollback(ctx)
			if errors.Is(err, pgx.ErrNoRows) || state == "complete" {
				continue
			}
			return status.Error(codes.Unavailable, "Voice MatchSquad join repair unavailable")
		}
		if state == "pending" {
			command, updateErr := tx.Exec(ctx, `UPDATE voice_room_memberships SET membership_state='EJECTED',updated_at=clock_timestamp()
WHERE profile_id=$1 AND account_id=$2 AND session_epoch=$3 AND room_id=$4 AND media_epoch=$5 AND membership_state='JOINING'
AND EXISTS (SELECT 1 FROM voice_room_instances r JOIN voice_match_squad_operations o ON o.room_id=r.room_id AND o.match_id=r.owner_id WHERE r.room_id=$4 AND r.owner_id=$6 AND r.purpose='MATCH_SQUAD' AND r.room_type='group_voice')`, item.profile, item.account, item.session, item.room, item.epoch, item.match)
			if updateErr != nil || command.RowsAffected() != 1 {
				_ = tx.Rollback(ctx)
				return status.Error(codes.FailedPrecondition, "MatchSquad joining generation is no longer current")
			}
			if _, err := tx.Exec(ctx, `UPDATE voice_match_squad_member_operations SET state='rejected',failure_code='join_repair_aborted',updated_at=clock_timestamp() WHERE operation_id=$1 AND state='pending'`, item.operation); err != nil {
				_ = tx.Rollback(ctx)
				return status.Error(codes.Unavailable, "Voice MatchSquad join repair unavailable")
			}
		} else if state != "rejected" {
			_ = tx.Rollback(ctx)
			return status.Error(codes.FailedPrecondition, "MatchSquad join operation state is invalid")
		}
		if err := tx.Commit(ctx); err != nil {
			return status.Error(codes.Unavailable, "Voice MatchSquad join repair unavailable")
		}
		if err := s.removeProjectionMember(ctx, item.room, item.match, item.profile, item.epoch); err != nil {
			return err
		}
		identity, err := livekit.MatchSquadIdentity(item.profile.String(), item.epoch.String())
		if err != nil {
			return status.Error(codes.FailedPrecondition, "MatchSquad media generation is invalid")
		}
		if err := s.RoomEffect.RemoveParticipant(ctx, item.livekitRoom, identity); err != nil {
			return status.Error(codes.Unavailable, "MatchSquad join cleanup is not confirmed")
		}
		tx, err = s.Pool.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return status.Error(codes.Unavailable, "Voice MatchSquad join repair unavailable")
		}
		if _, err := tx.Exec(ctx, `UPDATE voice_match_squad_member_effects SET state='confirmed',confirmed_at=clock_timestamp(),updated_at=clock_timestamp() WHERE operation_id=$1 AND effect_kind='join_projection' AND match_id=$2 AND room_id=$3 AND profile_id=$4 AND media_epoch=$5 AND state='pending'`, item.operation, item.match, item.room, item.profile, item.epoch); err != nil {
			_ = tx.Rollback(ctx)
			return status.Error(codes.Unavailable, "could not persist repaired MatchSquad join effect")
		}
		if err := releaseTerminalFenceInTx(ctx, tx, item.match, item.room, item.account, item.profile, item.epoch); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return status.Error(codes.Unavailable, "could not persist repaired MatchSquad join effect")
		}
	}
	return nil
}

// RepairPendingLeaves retries only exact, durable self-leave effects. It never
// needs the expired caller credential to finish cleanup already authorized
// and persisted by the handler.
func (s *MatchSquadMemberService) RepairPendingLeaves(ctx context.Context, limit int) error {
	if s == nil || s.Pool == nil || s.Calls == nil || s.RoomEffect == nil || s.Fences == nil {
		return status.Error(codes.Unavailable, "Voice MatchSquad member authority unavailable")
	}
	if limit <= 0 || limit > 256 {
		limit = 64
	}
	rows, err := s.Pool.Query(ctx, `SELECT o.operation_id,o.match_id,o.room_id,o.profile_id,o.account_id,o.session_epoch,o.media_epoch,o.request_sha256,
 r.chat_id,r.livekit_room_name,r.created_at
FROM voice_match_squad_member_operations o
JOIN voice_room_instances r ON r.room_id=o.room_id AND r.owner_id=o.match_id AND r.purpose='MATCH_SQUAD' AND r.room_type='group_voice'
JOIN voice_room_memberships m ON m.room_id=o.room_id AND m.profile_id=o.profile_id AND m.account_id=o.account_id AND m.session_epoch=o.session_epoch AND m.media_epoch=o.media_epoch AND m.membership_state='LEAVING'
WHERE o.method='leave' AND o.state='pending'
AND EXISTS (SELECT 1 FROM voice_match_squad_member_effects e WHERE e.operation_id=o.operation_id AND e.effect_kind IN ('leave_projection','leave_livekit') AND e.state='pending')
ORDER BY o.updated_at,o.operation_id LIMIT $1`, limit)
	if err != nil {
		return status.Error(codes.Unavailable, "Voice MatchSquad member effect repair unavailable")
	}
	type pendingLeave struct {
		operation, match, room, profile, account, epoch uuid.UUID
		session                                         int64
		hash                                            []byte
		chatID, livekitRoom                             string
		createdAt                                       time.Time
	}
	var pending []pendingLeave
	for rows.Next() {
		var item pendingLeave
		if err := rows.Scan(&item.operation, &item.match, &item.room, &item.profile, &item.account, &item.session, &item.epoch, &item.hash, &item.chatID, &item.livekitRoom, &item.createdAt); err != nil {
			rows.Close()
			return status.Error(codes.Unavailable, "Voice MatchSquad member effect repair unavailable")
		}
		pending = append(pending, item)
	}
	rows.Close()
	if rows.Err() != nil {
		return status.Error(codes.Unavailable, "Voice MatchSquad member effect repair unavailable")
	}
	for _, item := range pending {
		if err := s.removeProjectionMember(ctx, item.room, item.match, item.profile, item.epoch); err != nil {
			return err
		}
		identity, err := livekit.MatchSquadIdentity(item.profile.String(), item.epoch.String())
		if err != nil {
			return status.Error(codes.FailedPrecondition, "MatchSquad media generation is invalid")
		}
		if err := s.RoomEffect.RemoveParticipant(ctx, item.livekitRoom, identity); err != nil {
			return status.Error(codes.Unavailable, "MatchSquad media leave is not confirmed")
		}
		tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return status.Error(codes.Unavailable, "Voice MatchSquad member effect repair unavailable")
		}
		var state string
		var expired bool
		err = tx.QueryRow(ctx, `SELECT m.membership_state,m.membership_state='LEFT' OR m.latest_grant_expires_at IS NULL OR m.latest_grant_expires_at<=clock_timestamp() FROM voice_room_memberships m
JOIN voice_room_instances r ON r.room_id=m.room_id AND r.owner_id=$2 AND r.purpose='MATCH_SQUAD' AND r.room_type='group_voice'
JOIN voice_match_squad_operations o ON o.room_id=r.room_id AND o.match_id=r.owner_id
WHERE m.profile_id=$3 AND m.account_id=$4 AND m.session_epoch=$5 AND m.media_epoch=$6 AND m.room_id=$1 FOR UPDATE OF m`, item.room, item.match, item.profile, item.account, item.session, item.epoch).Scan(&state, &expired)
		if err != nil || (state != "LEAVING" && state != "LEFT") {
			_ = tx.Rollback(ctx)
			return status.Error(codes.FailedPrecondition, "MatchSquad leave generation is no longer current")
		}
		left := expired
		membership := callsv1.MatchSquadMembershipState_MATCH_SQUAD_MEMBERSHIP_STATE_LEAVING
		if left {
			membership = callsv1.MatchSquadMembershipState_MATCH_SQUAD_MEMBERSHIP_STATE_LEFT
			if _, err := tx.Exec(ctx, `UPDATE voice_room_memberships SET membership_state='LEFT',updated_at=clock_timestamp() WHERE profile_id=$1 AND room_id=$2 AND media_epoch=$3 AND membership_state='LEAVING'`, item.profile, item.room, item.epoch); err != nil {
				_ = tx.Rollback(ctx)
				return status.Error(codes.Unavailable, "Voice MatchSquad member effect repair unavailable")
			}
		}
		response := &callsv1.LeaveMatchSquadRoomResponse{
			CallSession: memberCallSession(store.Call{RoomID: item.room.String(), LivekitRoomName: item.livekitRoom, ChatID: item.chatID, MatchSquadMatchID: item.match.String(), SessionKind: callsv1.VoiceSessionKind_VOICE_SESSION_KIND_GROUP_VOICE, InitiatorProfileID: item.profile.String(), MediaKind: callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO, Status: callsv1.CallStatus_CALL_STATUS_ACTIVE, StartedAt: item.createdAt}, item.profile.String()),
			MediaEpoch:  item.epoch.String(), MembershipState: membership,
		}
		responseBytes, err := marshal(response)
		if err != nil {
			_ = tx.Rollback(ctx)
			return status.Error(codes.Internal, "could not encode repaired MatchSquad leave receipt")
		}
		for _, kind := range []string{"leave_projection", "leave_livekit"} {
			if _, err := tx.Exec(ctx, `UPDATE voice_match_squad_member_effects SET state='confirmed',confirmed_at=clock_timestamp(),updated_at=clock_timestamp() WHERE operation_id=$1 AND effect_kind=$2 AND match_id=$3 AND room_id=$4 AND profile_id=$5 AND media_epoch=$6 AND state='pending'`, item.operation, kind, item.match, item.room, item.profile, item.epoch); err != nil {
				_ = tx.Rollback(ctx)
				return status.Error(codes.Unavailable, "could not persist repaired MatchSquad leave effect")
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE voice_match_squad_member_operations SET state='complete',response_bytes=$2,updated_at=clock_timestamp() WHERE operation_id=$1 AND request_sha256=$3 AND method='leave' AND state='pending'`, item.operation, responseBytes, item.hash); err != nil {
			_ = tx.Rollback(ctx)
			return status.Error(codes.Unavailable, "could not persist repaired MatchSquad leave receipt")
		}
		if left {
			if err := releaseTerminalFenceInTx(ctx, tx, item.match, item.room, item.account, item.profile, item.epoch); err != nil {
				_ = tx.Rollback(ctx)
				return err
			}
		}
		if err := tx.Commit(ctx); err != nil {
			return status.Error(codes.Unavailable, "could not persist repaired MatchSquad leave receipt")
		}
	}
	return nil
}

// ReadyForTeardown prevents room closure while any issued bearer can still
// authorize media or any generation-bound effect remains unconfirmed.
func (s *MatchSquadMemberService) ReadyForTeardown(ctx context.Context, matchID, roomID uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return status.Error(codes.Unavailable, "Voice MatchSquad member authority unavailable")
	}
	var ready bool
	err := s.Pool.QueryRow(ctx, `SELECT EXISTS (
 SELECT 1 FROM voice_room_instances r JOIN voice_match_squad_operations o USING(room_id)
 WHERE r.room_id=$1 AND r.owner_id=$2 AND o.match_id=$2 AND r.purpose='MATCH_SQUAD'
   AND r.room_type='group_voice' AND r.state IN ('closing','closed')
) AND NOT EXISTS (
 SELECT 1 FROM voice_room_memberships m
 JOIN voice_room_instances r ON r.room_id=m.room_id AND r.owner_id=$2 AND r.purpose='MATCH_SQUAD' AND r.room_type='group_voice'
 WHERE m.room_id=$1 AND m.latest_grant_expires_at>clock_timestamp()
) AND NOT EXISTS (
 SELECT 1 FROM voice_match_squad_member_effects e WHERE e.match_id=$2 AND e.room_id=$1 AND e.state<>'confirmed'
)`, roomID, matchID).Scan(&ready)
	if err != nil {
		return status.Error(codes.Unavailable, "Voice MatchSquad teardown drain unavailable")
	}
	if !ready {
		return status.Error(codes.Unavailable, "MatchSquad member grants or effects are still draining")
	}
	return nil
}

// PrepareForTeardown drains the exact MatchSquad Redis roster while its call
// document is still active. The provider can mark the projection ended only
// after these member indexes have been safely removed.
func (s *MatchSquadMemberService) PrepareForTeardown(ctx context.Context, matchID, roomID uuid.UUID) error {
	if s == nil || s.Calls == nil {
		return status.Error(codes.Unavailable, "Voice MatchSquad member projection unavailable")
	}
	call, err := s.Calls.GetCall(ctx, roomID.String())
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil || call.RoomID != roomID.String() || call.MatchSquadMatchID != matchID.String() || call.LivekitRoomName != "match-squad-"+roomID.String() {
		return status.Error(codes.FailedPrecondition, "current MatchSquad Redis projection diverged")
	}
	for profile := range call.States {
		profileID, parseErr := uuid.Parse(profile)
		if parseErr != nil || profileID == uuid.Nil || profileID.String() != profile {
			return status.Error(codes.FailedPrecondition, "current MatchSquad Redis roster contains an invalid profile id")
		}
		epoch, ok := call.MatchSquadMemberEpochs[profile]
		parsedEpoch, epochErr := uuid.Parse(epoch)
		if !ok || epochErr != nil || parsedEpoch == uuid.Nil || parsedEpoch.String() != epoch {
			return status.Error(codes.FailedPrecondition, "current MatchSquad roster lacks an exact media generation")
		}
		if err := s.removeProjectionMember(ctx, roomID, matchID, profileID, parsedEpoch); err != nil {
			return err
		}
	}
	return nil
}

// FinalizeAfterRoomAbsent is called only after the LiveKit close operation
// confirms absence. It terminates remaining media generations and releases
// their account fences, while completed self-leaves remain LEFT.
func (s *MatchSquadMemberService) FinalizeAfterRoomAbsent(ctx context.Context, matchID, roomID uuid.UUID) error {
	if s == nil || s.Pool == nil || s.Calls == nil || s.Fences == nil {
		return status.Error(codes.Unavailable, "Voice MatchSquad member authority unavailable")
	}
	rows, err := s.Pool.Query(ctx, `SELECT m.profile_id,m.account_id,m.media_epoch
FROM voice_room_memberships m JOIN voice_room_instances r ON r.room_id=m.room_id
JOIN voice_match_squad_operations o ON o.room_id=r.room_id AND o.match_id=r.owner_id
WHERE m.room_id=$1 AND r.owner_id=$2 AND r.purpose='MATCH_SQUAD' AND r.room_type='group_voice'
 AND (m.membership_state IN ('JOINING','JOINED','LEAVING') OR (m.membership_state='LEFT' AND EXISTS (
  SELECT 1 FROM voice_account_voice_fences f WHERE f.account_id=m.account_id AND f.profile_id=m.profile_id AND f.room_id=m.room_id::text AND f.state='active')))
 AND NOT EXISTS (SELECT 1 FROM voice_match_squad_member_effects e WHERE e.match_id=$2 AND e.room_id=$1 AND e.profile_id=m.profile_id AND e.media_epoch=m.media_epoch AND e.state<>'confirmed')`, roomID, matchID)
	if err != nil {
		return status.Error(codes.Unavailable, "Voice MatchSquad terminal member drain unavailable")
	}
	type member struct{ profile, account, epoch uuid.UUID }
	var members []member
	for rows.Next() {
		var m member
		if err := rows.Scan(&m.profile, &m.account, &m.epoch); err != nil {
			rows.Close()
			return status.Error(codes.Unavailable, "Voice MatchSquad terminal member drain unavailable")
		}
		members = append(members, m)
	}
	rows.Close()
	if rows.Err() != nil {
		return status.Error(codes.Unavailable, "Voice MatchSquad terminal member drain unavailable")
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return status.Error(codes.Unavailable, "Voice MatchSquad terminal member drain unavailable")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var absent bool
	if err := tx.QueryRow(ctx, `SELECT r.state='closing' AND r.owner_id=$2 AND r.purpose='MATCH_SQUAD' AND r.room_type='group_voice'
FROM voice_room_instances r JOIN voice_match_squad_operations o ON o.room_id=r.room_id AND o.match_id=r.owner_id
WHERE r.room_id=$1 FOR UPDATE OF r,o`, roomID, matchID).Scan(&absent); err != nil || !absent {
		return status.Error(codes.FailedPrecondition, "MatchSquad room is no longer current for terminal member drain")
	}
	var pending bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM voice_match_squad_member_effects WHERE match_id=$2 AND room_id=$1 AND state<>'confirmed')`, roomID, matchID).Scan(&pending); err != nil || pending {
		return status.Error(codes.Unavailable, "MatchSquad member effects are still draining")
	}
	if _, err := tx.Exec(ctx, `UPDATE voice_room_memberships m SET membership_state='EJECTED',updated_at=clock_timestamp()
WHERE m.room_id=$1 AND m.membership_state IN ('JOINING','JOINED','LEAVING')
AND EXISTS (SELECT 1 FROM voice_room_instances r JOIN voice_match_squad_operations o ON o.room_id=r.room_id AND o.match_id=r.owner_id WHERE r.room_id=m.room_id AND r.owner_id=$2 AND r.purpose='MATCH_SQUAD' AND r.room_type='group_voice')`, roomID, matchID); err != nil {
		return status.Error(codes.Unavailable, "could not persist MatchSquad terminal member state")
	}
	for _, m := range members {
		if err := releaseTerminalFenceInTx(ctx, tx, matchID, roomID, m.account, m.profile, m.epoch); err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return status.Error(codes.Unavailable, "could not persist MatchSquad terminal member state")
	}
	return nil
}
