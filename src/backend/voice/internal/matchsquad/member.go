package matchsquad

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	callsv1 "voice.app/voice/calls/v1"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/pkg/principal"
	"voice/backend/voice/internal/gameprovision"
	"voice/backend/voice/internal/store"
)

type MemberProfileAccountResolver interface {
	AccountIDByProfileID(context.Context, uuid.UUID) (uuid.UUID, error)
}

type MemberPrincipalCurrent interface {
	CheckCurrent(context.Context, principal.Principal, time.Time) error
}

type MatchSquadTokenIssuer interface {
	MatchSquadJoinToken(string, string, *bool, time.Time, time.Time) (string, time.Time, error)
	LivekitURL() string
}

type MatchSquadMemberService struct {
	Pool       *pgxpool.Pool
	Calls      store.CallStore
	Fences     gameprovision.AccountVoiceFenceStore
	Profiles   MemberProfileAccountResolver
	Principal  MemberPrincipalCurrent
	Tokens     MatchSquadTokenIssuer
	RoomEffect interface {
		RemoveParticipant(context.Context, string, string) error
	}
	Now func() time.Time
}

const matchSquadMaxParticipants = 64

type memberRoom struct {
	chatID, livekitRoom, resourceState, operationState string
	roomType, purpose                                  string
	owner, rowOwner                                    uuid.UUID
	manifest, createRequest                            []byte
	createdAt                                          time.Time
}

func (s *MatchSquadMemberService) Join(ctx context.Context, req *callsv1.JoinMatchSquadRoomRequest) (*callsv1.JoinMatchSquadRoomResponse, error) {
	if s == nil || s.Pool == nil || s.Calls == nil || s.Fences == nil || s.Profiles == nil || s.Principal == nil || s.Tokens == nil || s.RoomEffect == nil {
		return nil, status.Error(codes.Unavailable, "Voice MatchSquad member authority unavailable")
	}
	actor, operation, matchID, roomID, encoded, requestHash, err := s.joinInput(ctx, req)
	if err != nil {
		return nil, err
	}
	if err := s.Principal.CheckCurrent(ctx, actor, s.now()); err != nil {
		return nil, err
	}
	profileID, _ := uuid.Parse(actor.ProfileID)
	accountID, err := s.Profiles.AccountIDByProfileID(ctx, profileID)
	if err != nil {
		return nil, status.Error(codes.Unavailable, "Voice profile account authority unavailable")
	}
	claimedAccount, _ := uuid.Parse(actor.AccountID)
	if accountID != claimedAccount {
		return nil, status.Error(codes.PermissionDenied, "verified account does not own this Voice profile")
	}
	if err := s.Principal.CheckCurrent(ctx, actor, s.now()); err != nil {
		return nil, err
	}

	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, status.Error(codes.Unavailable, "Voice MatchSquad database unavailable")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var priorRequest, priorResponse []byte
	var priorState string
	var priorFailure *string
	var priorMatch, priorRoom, priorProfile, priorAccount uuid.UUID
	var priorSession int64
	var priorMethod string
	err = tx.QueryRow(ctx, `SELECT request_bytes,response_bytes,state,failure_code,match_id,room_id,profile_id,account_id,session_epoch,method FROM voice_match_squad_member_operations WHERE operation_id=$1 FOR UPDATE`, operation).Scan(&priorRequest, &priorResponse, &priorState, &priorFailure, &priorMatch, &priorRoom, &priorProfile, &priorAccount, &priorSession, &priorMethod)
	if err == nil {
		if !bytes.Equal(priorRequest, encoded) || priorMatch != matchID || priorRoom != roomID || priorProfile.String() != actor.ProfileID || priorAccount.String() != actor.AccountID || priorSession != actor.SessionEpoch || priorMethod != "join" {
			return nil, status.Error(codes.AlreadyExists, "MatchSquad member operation conflicts with another request")
		}
		if priorState == "rejected" {
			return nil, status.Error(codes.FailedPrecondition, "MatchSquad join operation was durably rejected")
		}
		if priorState == "complete" {
			response := new(callsv1.JoinMatchSquadRoomResponse)
			if proto.Unmarshal(priorResponse, response) != nil {
				return nil, status.Error(codes.Internal, "stored MatchSquad member response is invalid")
			}
			if err := tx.Commit(ctx); err != nil {
				return nil, status.Error(codes.Unavailable, "Voice MatchSquad database unavailable")
			}
			return response, nil
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, status.Error(codes.Unavailable, "Voice MatchSquad database unavailable")
	}
	room, err := lockMemberRoom(ctx, tx, matchID, roomID)
	if err != nil {
		return nil, err
	}
	if room.owner != matchID || room.rowOwner != matchID || room.roomType != "group_voice" || room.purpose != "MATCH_SQUAD" || room.resourceState != "active" || room.operationState != "active" {
		return nil, status.Error(codes.FailedPrecondition, "MatchSquad room is not current and active")
	}
	if !manifestContains(room.createRequest, matchID, room.manifest, profileID) {
		return nil, status.Error(codes.PermissionDenied, "verified profile is not entitled to this MatchSquad room")
	}
	newFence, err := s.Fences.Reserve(ctx, accountID, profileID, roomID.String())
	if err != nil {
		return nil, fenceStatus(err)
	}
	fenceCommitted := false
	defer func() {
		if newFence && !fenceCommitted {
			_ = s.Fences.Release(ctx, accountID, profileID, roomID.String())
		}
	}()
	if err := s.Principal.CheckCurrent(ctx, actor, s.now()); err != nil {
		return nil, err
	}

	mediaEpoch, _, err := s.reserveGeneration(ctx, tx, actor, matchID, roomID, profileID, accountID, operation, requestHash, encoded)
	if err != nil {
		return nil, err
	}
	fenceCommitted = true // preserve the reservation if Commit has an ambiguous outcome
	if err := tx.Commit(ctx); err != nil {
		return nil, status.Error(codes.Unavailable, "Voice MatchSquad database unavailable")
	}
	if err := s.Fences.Commit(ctx, accountID, profileID, roomID.String()); err != nil {
		return nil, status.Error(codes.Unavailable, "Voice account fence commit unavailable")
	}
	if err := s.Principal.CheckCurrent(ctx, actor, s.now()); err != nil {
		return nil, err
	}
	call, err := s.ensureProjectionMember(ctx, roomID, matchID, profileID)
	if err != nil {
		return nil, err
	}
	if err := s.finishJoin(ctx, actor, operation, matchID, roomID, profileID, accountID, mediaEpoch, requestHash, call); err != nil {
		if status.Code(err) == codes.FailedPrecondition || status.Code(err) == codes.Unauthenticated {
			if cleanupErr := s.abortJoin(ctx, operation, roomID, matchID, profileID, mediaEpoch); cleanupErr != nil {
				return nil, status.Error(codes.Unavailable, "MatchSquad join cleanup remains pending")
			}
		}
		return nil, err
	}
	return s.loadJoinResponse(ctx, operation)
}

func (s *MatchSquadMemberService) GetJoinToken(ctx context.Context, req *callsv1.GetMatchSquadJoinTokenRequest) (*callsv1.GetMatchSquadJoinTokenResponse, error) {
	if s == nil || s.Pool == nil || s.Calls == nil || s.Principal == nil || s.Tokens == nil || s.Profiles == nil || s.Fences == nil {
		return nil, status.Error(codes.Unavailable, "Voice MatchSquad member authority unavailable")
	}
	actor, requestID, matchID, roomID, mediaEpoch, err := s.tokenInput(ctx, req)
	if err != nil {
		return nil, err
	}
	profileID, _ := uuid.Parse(actor.ProfileID)
	accountID, _ := uuid.Parse(actor.AccountID)
	if err := s.Principal.CheckCurrent(ctx, actor, s.now()); err != nil {
		return nil, err
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, status.Error(codes.Unavailable, "Voice MatchSquad database unavailable")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	room, err := lockMemberRoom(ctx, tx, matchID, roomID)
	if err != nil {
		return nil, err
	}
	if room.owner != matchID || room.rowOwner != matchID || room.roomType != "group_voice" || room.purpose != "MATCH_SQUAD" || room.resourceState != "active" || room.operationState != "active" || !manifestContains(room.createRequest, matchID, room.manifest, profileID) {
		return nil, status.Error(codes.FailedPrecondition, "MatchSquad room is not current and active")
	}
	var storedRoom, storedAccount uuid.UUID
	var storedSession int64
	var storedEpoch uuid.UUID
	var state string
	var latest pgtype.Timestamptz
	err = tx.QueryRow(ctx, `SELECT room_id,account_id,session_epoch,media_epoch,membership_state,latest_grant_expires_at FROM voice_room_memberships WHERE profile_id=$1 FOR UPDATE`, profileID).Scan(&storedRoom, &storedAccount, &storedSession, &storedEpoch, &state, &latest)
	if err != nil {
		return nil, status.Error(codes.FailedPrecondition, "MatchSquad member is not currently joined")
	}
	if storedRoom != roomID || storedAccount != accountID || storedSession != actor.SessionEpoch || storedEpoch != mediaEpoch || state != "JOINED" {
		return nil, status.Error(codes.FailedPrecondition, "MatchSquad membership generation is not current")
	}
	var fenceReady bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM voice_profile_account_mappings WHERE profile_id=$1 AND account_id=$2) AND EXISTS(SELECT 1 FROM voice_account_voice_fences WHERE account_id=$2 AND profile_id=$1 AND room_id=$3 AND state='active')`, profileID, accountID, roomID.String()).Scan(&fenceReady); err != nil || !fenceReady {
		return nil, status.Error(codes.FailedPrecondition, "MatchSquad account and profile fence is not current")
	}
	if err := s.Principal.CheckCurrent(ctx, actor, s.now()); err != nil {
		return nil, err
	}
	now := s.now()
	canPublish := true
	jwt, expiresAt, err := s.Tokens.MatchSquadJoinToken(actor.ProfileID, room.livekitRoom, &canPublish, now, actor.ExpiresAt)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "delegated user credential expires too soon")
	}
	response := &callsv1.GetMatchSquadJoinTokenResponse{
		Token:      &callsv1.GetJoinTokenResponse{Jwt: jwt, ExpiresAt: timestamppb.New(expiresAt), LivekitUrl: s.Tokens.LivekitURL(), MediaEpoch: mediaEpoch.String()},
		MediaEpoch: mediaEpoch.String(),
	}
	grantExpiry := time.Unix(expiresAt.Unix(), 0).UTC()
	if latest.Valid && latest.Time.After(grantExpiry) {
		grantExpiry = latest.Time
	}
	if _, err := tx.Exec(ctx, `INSERT INTO voice_match_squad_member_grants(request_id,match_id,room_id,profile_id,account_id,session_epoch,media_epoch,issued_at,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, requestID, matchID, roomID, profileID, accountID, actor.SessionEpoch, mediaEpoch, now, expiresAt); err != nil {
		return nil, status.Error(codes.AlreadyExists, "MatchSquad token request was already used")
	}
	if _, err := tx.Exec(ctx, `UPDATE voice_room_memberships SET latest_grant_expires_at=$2,updated_at=$3 WHERE profile_id=$1 AND room_id=$4 AND media_epoch=$5 AND membership_state='JOINED'`, profileID, grantExpiry, now, roomID, mediaEpoch); err != nil {
		return nil, status.Error(codes.Unavailable, "could not record MatchSquad bearer expiry")
	}
	if err := s.Principal.CheckCurrent(ctx, actor, s.now()); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, status.Error(codes.Unavailable, "could not persist MatchSquad bearer expiry")
	}
	return response, nil
}

func (s *MatchSquadMemberService) Leave(ctx context.Context, req *callsv1.LeaveMatchSquadRoomRequest) (*callsv1.LeaveMatchSquadRoomResponse, error) {
	if s == nil || s.Pool == nil || s.Calls == nil || s.Fences == nil || s.Profiles == nil || s.Principal == nil || s.RoomEffect == nil {
		return nil, status.Error(codes.Unavailable, "Voice MatchSquad member authority unavailable")
	}
	actor, operation, matchID, roomID, expectedEpoch, encoded, requestHash, err := s.leaveInput(ctx, req)
	if err != nil {
		return nil, err
	}
	if err := s.Principal.CheckCurrent(ctx, actor, s.now()); err != nil {
		return nil, err
	}
	profileID, _ := uuid.Parse(actor.ProfileID)
	accountID, _ := uuid.Parse(actor.AccountID)
	resolved, err := s.Profiles.AccountIDByProfileID(ctx, profileID)
	if err != nil || resolved != accountID {
		return nil, status.Error(codes.PermissionDenied, "verified account does not own this Voice profile")
	}
	if err := s.Principal.CheckCurrent(ctx, actor, s.now()); err != nil {
		return nil, err
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, status.Error(codes.Unavailable, "Voice MatchSquad database unavailable")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var priorRequest, priorResponse []byte
	var priorState string
	var priorMatch, priorRoom, priorProfile, priorAccount uuid.UUID
	var priorSession int64
	var priorMethod string
	err = tx.QueryRow(ctx, `SELECT request_bytes,response_bytes,state,match_id,room_id,profile_id,account_id,session_epoch,method FROM voice_match_squad_member_operations WHERE operation_id=$1 FOR UPDATE`, operation).Scan(&priorRequest, &priorResponse, &priorState, &priorMatch, &priorRoom, &priorProfile, &priorAccount, &priorSession, &priorMethod)
	if err == nil {
		if !bytes.Equal(priorRequest, encoded) || priorMatch != matchID || priorRoom != roomID || priorProfile.String() != actor.ProfileID || priorAccount.String() != actor.AccountID || priorSession != actor.SessionEpoch || priorMethod != "leave" {
			return nil, status.Error(codes.AlreadyExists, "MatchSquad member operation conflicts with another request")
		}
		if priorState == "complete" {
			response := new(callsv1.LeaveMatchSquadRoomResponse)
			if proto.Unmarshal(priorResponse, response) != nil {
				return nil, status.Error(codes.Internal, "stored MatchSquad member response is invalid")
			}
			if err := tx.Commit(ctx); err != nil {
				return nil, status.Error(codes.Unavailable, "Voice MatchSquad database unavailable")
			}
			return response, nil
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, status.Error(codes.Unavailable, "Voice MatchSquad database unavailable")
	}
	room, err := lockMemberRoom(ctx, tx, matchID, roomID)
	if err != nil {
		return nil, err
	}
	if room.owner != matchID || room.rowOwner != matchID || room.roomType != "group_voice" || room.purpose != "MATCH_SQUAD" || !manifestContains(room.createRequest, matchID, room.manifest, profileID) {
		return nil, status.Error(codes.FailedPrecondition, "MatchSquad room binding is not current")
	}
	var storedRoom, storedAccount, mediaEpoch uuid.UUID
	var storedSession int64
	var memberState string
	err = tx.QueryRow(ctx, `SELECT room_id,account_id,session_epoch,media_epoch,membership_state FROM voice_room_memberships WHERE profile_id=$1 FOR UPDATE`, profileID).Scan(&storedRoom, &storedAccount, &storedSession, &mediaEpoch, &memberState)
	if err != nil || storedRoom != roomID || storedAccount != accountID || storedSession != actor.SessionEpoch || mediaEpoch != expectedEpoch {
		return nil, status.Error(codes.FailedPrecondition, "MatchSquad membership generation is not current")
	}
	if memberState != "JOINED" && memberState != "LEAVING" {
		return nil, status.Error(codes.FailedPrecondition, "MatchSquad membership is not active")
	}
	now := s.now()
	if memberState == "JOINED" {
		if _, err := tx.Exec(ctx, `UPDATE voice_room_memberships SET membership_state='LEAVING',updated_at=$2 WHERE profile_id=$1 AND media_epoch=$3 AND membership_state='JOINED'`, profileID, now, mediaEpoch); err != nil {
			return nil, status.Error(codes.Unavailable, "could not begin MatchSquad leave")
		}
	}
	if err := insertMemberOperation(ctx, tx, operation, matchID, roomID, profileID, accountID, actor.SessionEpoch, mediaEpoch, "leave", requestHash, encoded, now); err != nil {
		return nil, err
	}
	if err := insertMemberEffects(ctx, tx, operation, matchID, roomID, profileID, mediaEpoch, now, "leave_projection", "leave_livekit"); err != nil {
		return nil, err
	}
	if err := s.Principal.CheckCurrent(ctx, actor, s.now()); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, status.Error(codes.Unavailable, "Voice MatchSquad database unavailable")
	}
	if err := s.removeProjectionMember(ctx, roomID, matchID, profileID); err != nil {
		return nil, err
	}
	if err := s.RoomEffect.RemoveParticipant(ctx, room.livekitRoom, profileID.String()); err != nil {
		return nil, status.Error(codes.Unavailable, "MatchSquad media leave is not confirmed")
	}
	if err := s.finishLeave(ctx, actor, operation, matchID, roomID, profileID, accountID, mediaEpoch, requestHash, room); err != nil {
		return nil, err
	}
	return s.loadLeaveResponse(ctx, operation)
}

func (s *MatchSquadMemberService) now() time.Time {
	if s != nil && s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func (s *MatchSquadMemberService) joinInput(ctx context.Context, req *callsv1.JoinMatchSquadRoomRequest) (principal.Principal, uuid.UUID, uuid.UUID, uuid.UUID, []byte, [32]byte, error) {
	var zero [32]byte
	if req == nil {
		return principal.Principal{}, uuid.Nil, uuid.Nil, uuid.Nil, nil, zero, status.Error(codes.InvalidArgument, "invalid MatchSquad join request")
	}
	actor, encoded, hash, err := memberActor(ctx, req, callsv1.MatchSquadMemberService_JoinMatchSquadRoom_FullMethodName, req.GetOperationId())
	if err != nil {
		return principal.Principal{}, uuid.Nil, uuid.Nil, uuid.Nil, nil, zero, err
	}
	ids, err := parseUUIDs(req.GetOperationId(), req.GetMatchId(), req.GetRoomId())
	if err != nil || req.GetProtocolVersion() != 1 {
		return principal.Principal{}, uuid.Nil, uuid.Nil, uuid.Nil, nil, zero, status.Error(codes.InvalidArgument, "invalid MatchSquad join request")
	}
	return actor, ids[0], ids[1], ids[2], encoded, hash, nil
}

func (s *MatchSquadMemberService) tokenInput(ctx context.Context, req *callsv1.GetMatchSquadJoinTokenRequest) (principal.Principal, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID, error) {
	if req == nil {
		return principal.Principal{}, uuid.Nil, uuid.Nil, uuid.Nil, uuid.Nil, status.Error(codes.InvalidArgument, "invalid MatchSquad token request")
	}
	actor, _, _, err := memberActor(ctx, req, callsv1.MatchSquadMemberService_GetMatchSquadJoinToken_FullMethodName, "")
	if err != nil {
		return principal.Principal{}, uuid.Nil, uuid.Nil, uuid.Nil, uuid.Nil, err
	}
	transport, err := principal.IncomingMetadata(ctx)
	if err != nil {
		return principal.Principal{}, uuid.Nil, uuid.Nil, uuid.Nil, uuid.Nil, status.Error(codes.Unauthenticated, "invalid delegated user principal")
	}
	ids, err := parseUUIDs(transport.RequestID, req.GetMatchId(), req.GetRoomId(), req.GetMediaEpoch())
	if err != nil || req.GetProtocolVersion() != 1 {
		return principal.Principal{}, uuid.Nil, uuid.Nil, uuid.Nil, uuid.Nil, status.Error(codes.InvalidArgument, "invalid MatchSquad token request")
	}
	if actor.RequestID != transport.RequestID {
		return principal.Principal{}, uuid.Nil, uuid.Nil, uuid.Nil, uuid.Nil, status.Error(codes.Unauthenticated, "delegated user request id mismatch")
	}
	return actor, ids[0], ids[1], ids[2], ids[3], nil
}

func (s *MatchSquadMemberService) leaveInput(ctx context.Context, req *callsv1.LeaveMatchSquadRoomRequest) (principal.Principal, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID, []byte, [32]byte, error) {
	var zero [32]byte
	if req == nil {
		return principal.Principal{}, uuid.Nil, uuid.Nil, uuid.Nil, uuid.Nil, nil, zero, status.Error(codes.InvalidArgument, "invalid MatchSquad leave request")
	}
	actor, encoded, hash, err := memberActor(ctx, req, callsv1.MatchSquadMemberService_LeaveMatchSquadRoom_FullMethodName, req.GetOperationId())
	if err != nil {
		return principal.Principal{}, uuid.Nil, uuid.Nil, uuid.Nil, uuid.Nil, nil, zero, err
	}
	ids, err := parseUUIDs(req.GetOperationId(), req.GetMatchId(), req.GetRoomId(), req.GetExpectedMediaEpoch())
	if err != nil || req.GetProtocolVersion() != 1 {
		return principal.Principal{}, uuid.Nil, uuid.Nil, uuid.Nil, uuid.Nil, nil, zero, status.Error(codes.InvalidArgument, "invalid MatchSquad leave request")
	}
	return actor, ids[0], ids[1], ids[2], ids[3], encoded, hash, nil
}

func memberActor(ctx context.Context, message proto.Message, method, operation string) (principal.Principal, []byte, [32]byte, error) {
	var empty [32]byte
	if message == nil || len(message.ProtoReflect().GetUnknown()) != 0 {
		return principal.Principal{}, nil, empty, status.Error(codes.InvalidArgument, "invalid MatchSquad member request")
	}
	actor, ok := principal.FromContext(ctx)
	if !ok || actor.Kind != "delegated_user" || actor.Issuer != "gateway" || actor.Audience != "voice" || actor.RPC != method || actor.SessionEpoch <= 0 || actor.ExpiresAt.IsZero() || !memberCanonicalUUID(actor.AccountID) || actor.Subject != actor.AccountID || !memberCanonicalUUID(actor.ProfileID) {
		return principal.Principal{}, nil, empty, status.Error(codes.Unauthenticated, "verified delegated user required")
	}
	if operation != "" && actor.RequestID != operation {
		return principal.Principal{}, nil, empty, status.Error(codes.Unauthenticated, "delegated user operation binding mismatch")
	}
	hash, err := principal.RequestHash(message)
	if err != nil || actor.RequestHash != hash {
		return principal.Principal{}, nil, empty, status.Error(codes.Unauthenticated, "delegated user request binding mismatch")
	}
	bytes, err := marshal(message)
	if err != nil {
		return principal.Principal{}, nil, empty, status.Error(codes.InvalidArgument, "invalid MatchSquad member request")
	}
	digest := sha256.Sum256(bytes)
	return actor, bytes, digest, nil
}

func memberCanonicalUUID(value string) bool {
	parsed, err := uuid.Parse(value)
	return err == nil && parsed != uuid.Nil && parsed.String() == value
}

func lockMemberRoom(ctx context.Context, tx pgx.Tx, matchID, roomID uuid.UUID) (memberRoom, error) {
	var room memberRoom
	err := tx.QueryRow(ctx, `SELECT o.owner_id,r.owner_id,o.participant_manifest_sha256,o.create_request_bytes,o.state,r.state,r.chat_id,r.livekit_room_name,r.room_type,r.purpose,r.created_at FROM voice_match_squad_operations o JOIN voice_room_instances r USING(room_id) WHERE o.match_id=$1 AND o.room_id=$2 FOR UPDATE OF o,r`, matchID, roomID).Scan(&room.owner, &room.rowOwner, &room.manifest, &room.createRequest, &room.operationState, &room.resourceState, &room.chatID, &room.livekitRoom, &room.roomType, &room.purpose, &room.createdAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return memberRoom{}, status.Error(codes.NotFound, "MatchSquad room not found")
	}
	if err != nil {
		return memberRoom{}, status.Error(codes.Unavailable, "Voice MatchSquad database unavailable")
	}
	return room, nil
}

func manifestContains(createBytes []byte, match uuid.UUID, manifest []byte, profile uuid.UUID) bool {
	request := new(callsv1.CreateMatchSquadRoomRequest)
	if proto.Unmarshal(createBytes, request) != nil {
		return false
	}
	_, storedMatch, participants, storedManifest, _, _, err := validateCreate(request)
	if err != nil || storedMatch != match || !bytes.Equal(storedManifest[:], manifest) {
		return false
	}
	for _, participant := range participants {
		if participant == profile.String() {
			return true
		}
	}
	return false
}

func fenceStatus(err error) error {
	switch {
	case errors.Is(err, gameprovision.ErrAccountProfileMappingConflict):
		return status.Error(codes.PermissionDenied, "Voice profile account mapping conflicts")
	case errors.Is(err, gameprovision.ErrActiveAccountVoiceSession):
		return status.Error(codes.FailedPrecondition, "account already has an active Voice session")
	default:
		return status.Error(codes.Unavailable, "Voice account fence unavailable")
	}
}

func insertMemberOperation(ctx context.Context, tx pgx.Tx, operation, match, room, profile, account uuid.UUID, session int64, epoch uuid.UUID, method string, requestHash [32]byte, request []byte, now time.Time) error {
	_, err := tx.Exec(ctx, `INSERT INTO voice_match_squad_member_operations(operation_id,match_id,room_id,profile_id,account_id,session_epoch,media_epoch,method,request_sha256,request_bytes,state,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'pending',$11,$11) ON CONFLICT(operation_id) DO NOTHING`, operation, match, room, profile, account, session, epoch, method, requestHash[:], request, now)
	if err != nil {
		return status.Error(codes.Unavailable, "could not persist MatchSquad member operation")
	}
	var prior []byte
	if err := tx.QueryRow(ctx, `SELECT request_bytes FROM voice_match_squad_member_operations WHERE operation_id=$1 FOR UPDATE`, operation).Scan(&prior); err != nil || !bytes.Equal(prior, request) {
		return status.Error(codes.AlreadyExists, "MatchSquad member operation conflicts")
	}
	return nil
}

func insertMemberEffects(ctx context.Context, tx pgx.Tx, operation, match, room, profile, epoch uuid.UUID, now time.Time, kinds ...string) error {
	for _, kind := range kinds {
		if _, err := tx.Exec(ctx, `INSERT INTO voice_match_squad_member_effects(operation_id,effect_kind,match_id,room_id,profile_id,media_epoch,state,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,'pending',$7,$7) ON CONFLICT(operation_id,effect_kind) DO NOTHING`, operation, kind, match, room, profile, epoch, now.UTC()); err != nil {
			return status.Error(codes.Unavailable, "could not persist MatchSquad member effect")
		}
	}
	return nil
}

func (s *MatchSquadMemberService) reserveGeneration(ctx context.Context, tx pgx.Tx, actor principal.Principal, match, room, profile, account, operation uuid.UUID, requestHash [32]byte, request []byte) (uuid.UUID, string, error) {
	var storedRoom, mediaEpoch uuid.UUID
	var storedAccount pgtype.UUID
	var session pgtype.Int8
	var state pgtype.Text
	var latest pgtype.Timestamptz
	var grantsExpired bool
	err := tx.QueryRow(ctx, `SELECT room_id,account_id,session_epoch,media_epoch,membership_state,latest_grant_expires_at,latest_grant_expires_at IS NULL OR latest_grant_expires_at<=clock_timestamp() FROM voice_room_memberships WHERE profile_id=$1 FOR UPDATE`, profile).Scan(&storedRoom, &storedAccount, &session, &mediaEpoch, &state, &latest, &grantsExpired)
	now := s.now()
	if errors.Is(err, pgx.ErrNoRows) {
		mediaEpoch = uuid.New()
		if _, err := tx.Exec(ctx, `INSERT INTO voice_room_memberships(profile_id,room_id,media_epoch,space_access_epoch,role_policy_epoch,authorization_digest,can_join,can_publish_audio,can_publish_video,can_publish_screen_share,can_subscribe,can_mute_others,can_deafen_others,can_move_others,can_use_ptt,priority_speaker,joined_at,updated_at,account_id,session_epoch,membership_state) VALUES($1,$2,$3,NULL,NULL,$4,true,true,false,false,true,false,false,false,false,false,$5,$5,$6,$7,'JOINING')`, profile, room, mediaEpoch, requestHash[:], now, account, actor.SessionEpoch); err != nil {
			return uuid.Nil, "", status.Error(codes.FailedPrecondition, "MatchSquad profile already has a current Voice membership")
		}
		state = pgtype.Text{String: "JOINING", Valid: true}
	} else if err != nil {
		return uuid.Nil, "", status.Error(codes.Unavailable, "Voice membership database unavailable")
	} else {
		if !storedAccount.Valid || !session.Valid || !state.Valid {
			return uuid.Nil, "", status.Error(codes.FailedPrecondition, "profile has an incomplete existing Voice membership")
		}
		if storedAccount.Bytes != account {
			return uuid.Nil, "", status.Error(codes.PermissionDenied, "profile account mapping conflicts")
		}
		if storedRoom == room && session.Int64 == actor.SessionEpoch && (state.String == "JOINED" || state.String == "JOINING") {
			// Same current generation: replay a separate mutation as a repair, never rotate it.
		} else {
			if state.String != "LEFT" && state.String != "EJECTED" {
				return uuid.Nil, "", status.Error(codes.FailedPrecondition, "profile already has a current Voice membership")
			}
			var pendingEffects bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM voice_match_squad_member_effects WHERE profile_id=$1 AND media_epoch=$2 AND state='pending')`, profile, mediaEpoch).Scan(&pendingEffects); err != nil {
				return uuid.Nil, "", status.Error(codes.Unavailable, "MatchSquad member effects unavailable")
			}
			if pendingEffects {
				return uuid.Nil, "", status.Error(codes.FailedPrecondition, "previous MatchSquad member effects are still draining")
			}
			if !grantsExpired {
				return uuid.Nil, "", status.Error(codes.FailedPrecondition, "previous MatchSquad bearer grants are still valid")
			}
			mediaEpoch = uuid.New()
			if _, err := tx.Exec(ctx, `UPDATE voice_room_memberships SET room_id=$2,media_epoch=$3,authorization_digest=$4,can_join=true,can_publish_audio=true,can_publish_video=false,can_publish_screen_share=false,can_subscribe=true,membership_state='JOINING',session_epoch=$5,latest_grant_expires_at=NULL,joined_at=$6,updated_at=$6 WHERE profile_id=$1`, profile, room, mediaEpoch, requestHash[:], actor.SessionEpoch, now); err != nil {
				return uuid.Nil, "", status.Error(codes.FailedPrecondition, "profile membership generation cannot be reopened")
			}
			state = pgtype.Text{String: "JOINING", Valid: true}
		}
	}
	if err := insertMemberOperation(ctx, tx, operation, match, room, profile, account, actor.SessionEpoch, mediaEpoch, "join", requestHash, request, now); err != nil {
		return uuid.Nil, "", err
	}
	if err := insertMemberEffects(ctx, tx, operation, match, room, profile, mediaEpoch, now, "join_projection"); err != nil {
		return uuid.Nil, "", err
	}
	return mediaEpoch, state.String, nil
}

func (s *MatchSquadMemberService) ensureProjectionMember(ctx context.Context, room, match, profile uuid.UUID) (store.Call, error) {
	call, err := s.Calls.GetCall(ctx, room.String())
	if err != nil || call.MatchSquadMatchID != match.String() || call.RoomID != room.String() || call.LivekitRoomName != "match-squad-"+room.String() || call.Status != callsv1.CallStatus_CALL_STATUS_ACTIVE {
		return store.Call{}, status.Error(codes.Unavailable, "current MatchSquad Redis projection unavailable")
	}
	if _, err := s.Calls.AddParticipant(ctx, room.String(), profile.String(), matchSquadMaxParticipants); err != nil {
		return store.Call{}, status.Error(codes.FailedPrecondition, "Voice participant projection could not be admitted")
	}
	call, err = s.Calls.GetCall(ctx, room.String())
	if err != nil || call.MatchSquadMatchID != match.String() || !call.IsParticipant(profile.String()) {
		return store.Call{}, status.Error(codes.Unavailable, "MatchSquad participant projection was not confirmed")
	}
	return call, nil
}

func (s *MatchSquadMemberService) finishJoin(ctx context.Context, actor principal.Principal, operation, match, room, profile, account, epoch uuid.UUID, requestHash [32]byte, call store.Call) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return status.Error(codes.Unavailable, "Voice MatchSquad database unavailable")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var owner uuid.UUID
	var resourceState, operationState string
	if err := tx.QueryRow(ctx, `SELECT o.owner_id,r.state,o.state FROM voice_match_squad_operations o JOIN voice_room_instances r USING(room_id) WHERE o.match_id=$1 AND o.room_id=$2 AND r.owner_id=$1 AND r.room_type='group_voice' AND r.purpose='MATCH_SQUAD' FOR UPDATE OF o,r`, match, room).Scan(&owner, &resourceState, &operationState); err != nil || owner != match || resourceState != "active" || operationState != "active" {
		return status.Error(codes.FailedPrecondition, "MatchSquad room began closing during join")
	}
	var memberOperationState string
	if err := tx.QueryRow(ctx, `SELECT state FROM voice_match_squad_member_operations WHERE operation_id=$1 AND match_id=$2 AND room_id=$3 AND profile_id=$4 AND account_id=$5 AND session_epoch=$6 AND media_epoch=$7 AND method='join' FOR UPDATE`, operation, match, room, profile, account, actor.SessionEpoch, epoch).Scan(&memberOperationState); err != nil {
		return status.Error(codes.FailedPrecondition, "MatchSquad join operation is no longer current")
	}
	if memberOperationState == "complete" {
		return tx.Commit(ctx)
	}
	if memberOperationState != "pending" {
		return status.Error(codes.FailedPrecondition, "MatchSquad join operation was durably rejected")
	}
	if err := s.Principal.CheckCurrent(ctx, actor, s.now()); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `UPDATE voice_room_memberships SET membership_state='JOINED',updated_at=$2 WHERE profile_id=$1 AND room_id=$3 AND account_id=$4 AND session_epoch=$5 AND media_epoch=$6 AND membership_state IN ('JOINING','JOINED')`, profile, s.now(), room, account, actor.SessionEpoch, epoch)
	if err != nil || tag.RowsAffected() != 1 {
		return status.Error(codes.Unavailable, "could not commit MatchSquad membership")
	}
	response := &callsv1.JoinMatchSquadRoomResponse{CallSession: memberCallSession(call, actor.ProfileID), MediaEpoch: epoch.String(), MembershipState: callsv1.MatchSquadMembershipState_MATCH_SQUAD_MEMBERSHIP_STATE_JOINED}
	responseBytes, err := marshal(response)
	if err != nil {
		return status.Error(codes.Internal, "could not encode MatchSquad join response")
	}
	tag, err = tx.Exec(ctx, `UPDATE voice_match_squad_member_effects SET state='confirmed',confirmed_at=$3,updated_at=$3 WHERE operation_id=$1 AND effect_kind='join_projection' AND media_epoch=$2 AND state='pending'`, operation, epoch, s.now())
	if err != nil || tag.RowsAffected() != 1 {
		return status.Error(codes.Unavailable, "could not confirm MatchSquad join projection")
	}
	tag, err = tx.Exec(ctx, `UPDATE voice_match_squad_member_operations SET state='complete',response_bytes=$2,updated_at=$3 WHERE operation_id=$1 AND request_sha256=$4 AND state='pending'`, operation, responseBytes, s.now(), requestHash[:])
	if err != nil || tag.RowsAffected() != 1 {
		return status.Error(codes.Unavailable, "could not complete MatchSquad join operation")
	}
	if err := tx.Commit(ctx); err != nil {
		return status.Error(codes.Unavailable, "could not persist MatchSquad join receipt")
	}
	return nil
}

func (s *MatchSquadMemberService) abortJoin(ctx context.Context, operation, room, match, profile, epoch uuid.UUID) error {
	if err := s.removeProjectionMember(ctx, room, match, profile); err != nil {
		return err
	}
	call, err := s.Calls.GetCall(ctx, room.String())
	if err == nil && s.RoomEffect != nil {
		if err := s.RoomEffect.RemoveParticipant(ctx, call.LivekitRoomName, profile.String()); err != nil {
			return err
		}
	} else if err != nil && !errors.Is(err, store.ErrNotFound) {
		return err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	now := s.now()
	if _, err := tx.Exec(ctx, `UPDATE voice_room_memberships SET membership_state='EJECTED',can_join=false,can_publish_audio=false,can_publish_video=false,can_publish_screen_share=false,can_subscribe=false,updated_at=$3 WHERE profile_id=$1 AND room_id=$2 AND media_epoch=$4 AND membership_state='JOINING'`, profile, room, now, epoch); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE voice_match_squad_member_effects SET state='confirmed',confirmed_at=$3,updated_at=$3 WHERE operation_id=$1 AND effect_kind='join_projection' AND media_epoch=$2 AND state='pending'`, operation, epoch, now); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE voice_match_squad_member_operations SET state='rejected',failure_code='join_no_longer_current',updated_at=$2 WHERE operation_id=$1 AND state='pending'`, operation, now); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	var account uuid.UUID
	if err := s.Pool.QueryRow(ctx, `SELECT account_id FROM voice_room_memberships WHERE profile_id=$1 AND room_id=$2 AND media_epoch=$3 AND membership_state='EJECTED'`, profile, room, epoch).Scan(&account); err != nil {
		return err
	}
	return s.Fences.Release(ctx, account, profile, room.String())
}

func (s *MatchSquadMemberService) loadJoinResponse(ctx context.Context, operation uuid.UUID) (*callsv1.JoinMatchSquadRoomResponse, error) {
	var encoded []byte
	if err := s.Pool.QueryRow(ctx, `SELECT response_bytes FROM voice_match_squad_member_operations WHERE operation_id=$1 AND state='complete'`, operation).Scan(&encoded); err != nil {
		return nil, status.Error(codes.Unavailable, "MatchSquad join receipt unavailable")
	}
	response := new(callsv1.JoinMatchSquadRoomResponse)
	if proto.Unmarshal(encoded, response) != nil {
		return nil, status.Error(codes.Internal, "stored MatchSquad join receipt is invalid")
	}
	return response, nil
}

func (s *MatchSquadMemberService) removeProjectionMember(ctx context.Context, room, match, profile uuid.UUID) error {
	call, err := s.Calls.GetCall(ctx, room.String())
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil || call.RoomID != room.String() || call.MatchSquadMatchID != match.String() || call.LivekitRoomName != "match-squad-"+room.String() {
		return status.Error(codes.Unavailable, "current MatchSquad Redis projection unavailable")
	}
	owned, ok := s.Calls.(interface {
		RemoveMatchSquadParticipant(context.Context, string, string, string) (store.Call, error)
	})
	if !ok {
		return status.Error(codes.Unavailable, "owner-scoped MatchSquad projection removal unavailable")
	}
	_, err = owned.RemoveMatchSquadParticipant(ctx, room.String(), match.String(), profile.String())
	if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrNotParticipant) {
		return nil
	}
	if err != nil {
		return status.Error(codes.Unavailable, "could not remove MatchSquad Redis participant")
	}
	return nil
}

func (s *MatchSquadMemberService) finishLeave(ctx context.Context, actor principal.Principal, operation, match, room, profile, account, epoch uuid.UUID, requestHash [32]byte, resource memberRoom) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return status.Error(codes.Unavailable, "Voice MatchSquad database unavailable")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	// The delegated actor was rechecked after the durable transition and
	// before effects began. Once Leave removes media, always record those
	// cleanup effects even if the token expires during the external calls.
	response := &callsv1.LeaveMatchSquadRoomResponse{CallSession: memberCallSession(store.Call{RoomID: room.String(), LivekitRoomName: resource.livekitRoom, ChatID: resource.chatID, MatchSquadMatchID: match.String(), SessionKind: callsv1.VoiceSessionKind_VOICE_SESSION_KIND_GROUP_VOICE, InitiatorProfileID: actor.ProfileID, MediaKind: callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO, Status: callsv1.CallStatus_CALL_STATUS_ACTIVE, StartedAt: resource.createdAt}, actor.ProfileID), MediaEpoch: epoch.String(), MembershipState: callsv1.MatchSquadMembershipState_MATCH_SQUAD_MEMBERSHIP_STATE_LEAVING}
	var state string
	var expired bool
	if err := tx.QueryRow(ctx, `SELECT membership_state,membership_state='LEFT' OR latest_grant_expires_at IS NULL OR latest_grant_expires_at<=clock_timestamp() FROM voice_room_memberships WHERE profile_id=$1 AND room_id=$2 AND account_id=$3 AND session_epoch=$4 AND media_epoch=$5 FOR UPDATE`, profile, room, account, actor.SessionEpoch, epoch).Scan(&state, &expired); err != nil || (state != "LEAVING" && state != "LEFT") {
		return status.Error(codes.FailedPrecondition, "MatchSquad leave generation is no longer current")
	}
	if expired {
		response.MembershipState = callsv1.MatchSquadMembershipState_MATCH_SQUAD_MEMBERSHIP_STATE_LEFT
		if _, err := tx.Exec(ctx, `UPDATE voice_room_memberships SET membership_state='LEFT',can_join=false,can_publish_audio=false,can_publish_video=false,can_publish_screen_share=false,can_subscribe=false,updated_at=clock_timestamp() WHERE profile_id=$1 AND room_id=$2 AND media_epoch=$3 AND membership_state='LEAVING'`, profile, room, epoch); err != nil {
			return status.Error(codes.Unavailable, "could not finish MatchSquad leave")
		}
	}
	responseBytes, err := marshal(response)
	if err != nil {
		return status.Error(codes.Internal, "could not encode MatchSquad leave response")
	}
	for _, kind := range []string{"leave_projection", "leave_livekit"} {
		if _, err := tx.Exec(ctx, `UPDATE voice_match_squad_member_effects SET state='confirmed',confirmed_at=$3,updated_at=$3 WHERE operation_id=$1 AND effect_kind=$2 AND media_epoch=$4`, operation, kind, s.now(), epoch); err != nil {
			return status.Error(codes.Unavailable, "could not confirm MatchSquad leave effect")
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE voice_match_squad_member_operations SET state='complete',response_bytes=$2,updated_at=$3 WHERE operation_id=$1 AND request_sha256=$4 AND state='pending'`, operation, responseBytes, s.now(), requestHash[:]); err != nil {
		return status.Error(codes.Unavailable, "could not complete MatchSquad leave operation")
	}
	if err := tx.Commit(ctx); err != nil {
		return status.Error(codes.Unavailable, "could not persist MatchSquad leave receipt")
	}
	if response.MembershipState == callsv1.MatchSquadMembershipState_MATCH_SQUAD_MEMBERSHIP_STATE_LEFT {
		if err := s.Fences.Release(ctx, account, profile, room.String()); err != nil {
			return status.Error(codes.Unavailable, "MatchSquad account fence release pending")
		}
	}
	return nil
}

func (s *MatchSquadMemberService) loadLeaveResponse(ctx context.Context, operation uuid.UUID) (*callsv1.LeaveMatchSquadRoomResponse, error) {
	var encoded []byte
	if err := s.Pool.QueryRow(ctx, `SELECT response_bytes FROM voice_match_squad_member_operations WHERE operation_id=$1 AND state='complete'`, operation).Scan(&encoded); err != nil {
		return nil, status.Error(codes.Unavailable, "MatchSquad leave receipt unavailable")
	}
	response := new(callsv1.LeaveMatchSquadRoomResponse)
	if proto.Unmarshal(encoded, response) != nil {
		return nil, status.Error(codes.Internal, "stored MatchSquad leave receipt is invalid")
	}
	return response, nil
}

func memberCallSession(call store.Call, actor string) *callsv1.CallSession {
	kind := callsv1.VoiceSessionKind_VOICE_SESSION_KIND_GROUP_VOICE
	group := chatv1.ChatType_CHAT_TYPE_GROUP
	return &callsv1.CallSession{RoomId: call.RoomID, LivekitRoomName: call.LivekitRoomName, RoomType: "group_voice", LinkedChat: &chatv1.ChatRef{Id: call.ChatID, Type: &group}, RoomTypeEnum: kind.Enum(), InitiatorProfileId: actor, MediaKind: callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO, Status: call.Status, StartedAt: timestamppb.New(call.StartedAt)}
}
