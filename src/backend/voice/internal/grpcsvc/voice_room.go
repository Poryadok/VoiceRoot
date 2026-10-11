package grpcsvc

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/pkg/principal"
	voicestore "voice/backend/voice/internal/store"

	callsv1 "voice.app/voice/calls/v1"
	spacev1 "voice.app/voice/space/v1"
)

// MoveToVoiceRoom only moves the authenticated actor. Moving another profile
// has a distinct RPC so a client can never assert an actor identity.
func (s *VoiceGRPC) MoveToVoiceRoom(ctx context.Context, req *callsv1.MoveToVoiceRoomRequest) (*callsv1.MoveToVoiceRoomResponse, error) {
	actor, err := callerProfile(ctx)
	if err != nil {
		return nil, err
	}
	result, err := s.moveVoiceRoomParticipant(ctx, actor, actor, req.GetFromVoiceRoomId(), req.GetToVoiceRoomId(), req.GetSpace().GetId(), req.GetOperationId(), false)
	if err != nil {
		return nil, err
	}
	return &callsv1.MoveToVoiceRoomResponse{
		VoiceSession: voiceSessionToProto(result.Destination),
		Receipt:      moveReceipt(actor, actor, req.GetOperationId(), req.GetFromVoiceRoomId(), req.GetToVoiceRoomId(), req.GetSpace().GetId(), callsv1.VoiceRoomLifecycleMethod_VOICE_ROOM_LIFECYCLE_METHOD_SELF_MOVE, result),
	}, nil
}

func (s *VoiceGRPC) MoveVoiceRoomParticipant(ctx context.Context, req *callsv1.MoveVoiceRoomParticipantRequest) (*callsv1.MoveVoiceRoomParticipantResponse, error) {
	actor, err := callerProfile(ctx)
	if err != nil {
		return nil, err
	}
	target := strings.TrimSpace(req.GetParticipantProfileId())
	result, err := s.moveVoiceRoomParticipant(ctx, actor, target, req.GetFromVoiceRoomId(), req.GetToVoiceRoomId(), req.GetSpace().GetId(), req.GetOperationId(), true)
	if err != nil {
		return nil, err
	}
	return &callsv1.MoveVoiceRoomParticipantResponse{Receipt: moveReceipt(actor, target, req.GetOperationId(), req.GetFromVoiceRoomId(), req.GetToVoiceRoomId(), req.GetSpace().GetId(), callsv1.VoiceRoomLifecycleMethod_VOICE_ROOM_LIFECYCLE_METHOD_MODERATOR_MOVE, result)}, nil
}

func (s *VoiceGRPC) moveVoiceRoomParticipant(ctx context.Context, actor, target, fromRoom, toRoom, assertedSpace, operationID string, moderator bool) (voicestore.VoiceRoomMoveResult, error) {
	if s == nil || s.Calls == nil {
		return voicestore.VoiceRoomMoveResult{}, status.Error(codes.FailedPrecondition, "voice persistence not configured")
	}
	actor, target = strings.TrimSpace(actor), strings.TrimSpace(target)
	fromRoom, toRoom = strings.TrimSpace(fromRoom), strings.TrimSpace(toRoom)
	assertedSpace, operationID = strings.TrimSpace(assertedSpace), strings.TrimSpace(operationID)
	if target == "" {
		return voicestore.VoiceRoomMoveResult{}, status.Error(codes.InvalidArgument, "participant_profile_id is required")
	}
	for field, value := range map[string]string{"from_voice_room_id": fromRoom, "to_voice_room_id": toRoom, "operation_id": operationID} {
		if value == "" {
			return voicestore.VoiceRoomMoveResult{}, status.Errorf(codes.InvalidArgument, "%s is required", field)
		}
		if _, err := uuid.Parse(value); err != nil {
			return voicestore.VoiceRoomMoveResult{}, status.Errorf(codes.InvalidArgument, "invalid %s", field)
		}
	}
	if _, err := uuid.Parse(target); err != nil {
		return voicestore.VoiceRoomMoveResult{}, status.Error(codes.InvalidArgument, "invalid participant_profile_id")
	}
	if fromRoom == toRoom {
		return voicestore.VoiceRoomMoveResult{}, status.Error(codes.FailedPrecondition, "source and destination voice rooms must differ")
	}
	moveReq := voicestore.VoiceRoomMoveRequest{
		ActorProfileID: actor, ParticipantProfileID: target, OperationID: operationID,
		FromVoiceRoomID: fromRoom, ToVoiceRoomID: toRoom, SpaceID: assertedSpace,
	}
	// The authoritative resolver proves membership and canonical ownership for
	// each involved profile.  Deliberately do not resolve destination access for
	// actor: a moderator is allowed to move someone there without VOICE_JOIN.
	actorSource, err := s.resolveCanonicalVoiceRoomAccess(ctx, fromRoom, actor)
	if err != nil {
		return voicestore.VoiceRoomMoveResult{}, err
	}
	targetSource, err := s.resolveCanonicalVoiceRoomAccess(ctx, fromRoom, target)
	if err != nil {
		return voicestore.VoiceRoomMoveResult{}, err
	}
	// Space transfers need both room-head generations and operation-owned
	// participant fences. Until that atomic transfer exists, fail closed before
	// replay lookup or any roster/event mutation. Ordinary DM/group moves keep
	// their existing path.
	if actorSource.SpaceID != "" || targetSource.SpaceID != "" {
		return voicestore.VoiceRoomMoveResult{}, status.Error(codes.Unavailable, "Space voice-room transfer is not available")
	}
	if replay, found, err := s.Calls.FindVoiceRoomMove(ctx, moveReq); err != nil {
		return voicestore.VoiceRoomMoveResult{}, moveStoreErr(err)
	} else if found {
		if err := s.transferAccountVoiceProfile(ctx, target, replay.Source.RoomID, replay.Destination.RoomID, actor); err != nil {
			return voicestore.VoiceRoomMoveResult{}, err
		}
		return replay, nil
	}
	targetDestination, err := s.resolveCanonicalVoiceRoomAccess(ctx, toRoom, target)
	if err != nil {
		return voicestore.VoiceRoomMoveResult{}, err
	}
	if targetDestination.SpaceID != "" {
		return voicestore.VoiceRoomMoveResult{}, status.Error(codes.Unavailable, "Space voice-room transfer is not available")
	}
	if actorSource.SpaceID != assertedSpace || targetSource.SpaceID != assertedSpace || targetDestination.SpaceID != assertedSpace {
		return voicestore.VoiceRoomMoveResult{}, status.Error(codes.PermissionDenied, "voice rooms and participants must share the asserted space")
	}
	if moderator {
		if err := s.ensureVoiceMoveOthersPermission(ctx, assertedSpace, actor, fromRoom); err != nil {
			return voicestore.VoiceRoomMoveResult{}, err
		}
	}
	if s.Roles == nil {
		return voicestore.VoiceRoomMoveResult{}, status.Error(codes.FailedPrecondition, "voice join permission check not configured")
	}
	if err := s.ensureVoiceJoinPermission(ctx, assertedSpace, target, toRoom); err != nil {
		return voicestore.VoiceRoomMoveResult{}, err
	}
	maxParticipants := voicestore.MaxVoiceRoomParticipants
	if s.SpacePro != nil {
		pro, err := s.SpacePro.HasSpacePro(ctx, assertedSpace)
		if err != nil {
			return voicestore.VoiceRoomMoveResult{}, status.Error(codes.Unavailable, "space subscription check unavailable")
		}
		if pro {
			maxParticipants = voicestore.MaxSpaceProVoiceParticipants
		}
	}
	moveReq.MaxParticipants, moveReq.DestinationRoomID, moveReq.Now = maxParticipants, uuid.NewString(), s.now()
	result, err := s.Calls.MoveVoiceRoomParticipant(ctx, moveReq)
	if err != nil {
		return voicestore.VoiceRoomMoveResult{}, moveStoreErr(err)
	}
	if err := s.transferAccountVoiceProfile(ctx, target, result.Source.RoomID, result.Destination.RoomID, actor); err != nil {
		return voicestore.VoiceRoomMoveResult{}, err
	}
	if !result.Replayed {
		s.publishVoiceMemberJoined(ctx, result.Destination, target)
	}
	return result, nil
}

func moveStoreErr(err error) error {
	mapped := storeErr(err)
	if status.Code(mapped) == codes.Internal {
		return status.Error(codes.Unavailable, "voice room roster unavailable")
	}
	return mapped
}

func moveReceipt(actor, target, operationID, fromRoom, toRoom, spaceID string, method callsv1.VoiceRoomLifecycleMethod, result voicestore.VoiceRoomMoveResult) *callsv1.VoiceRoomLifecycleReceipt {
	return &callsv1.VoiceRoomLifecycleReceipt{
		OperationId: operationID, ActorProfileId: actor, SubjectProfileId: target,
		Space: &spacev1.SpaceRef{Id: spaceID}, Method: method,
		Outcome:           callsv1.VoiceRoomLifecycleOutcome_VOICE_ROOM_LIFECYCLE_OUTCOME_MOVED,
		SourceVoiceRoomId: &fromRoom, DestinationVoiceRoomId: &toRoom, RoomId: &result.Destination.RoomID,
	}
}

func (s *VoiceGRPC) JoinVoiceRoom(ctx context.Context, req *callsv1.JoinVoiceRoomRequest) (*callsv1.JoinVoiceRoomResponse, error) {
	profileID, _, _, err := callerVoiceUser(ctx)
	if err != nil {
		return nil, err
	}
	if s == nil || s.Calls == nil {
		return nil, status.Error(codes.FailedPrecondition, "voice persistence not configured")
	}
	voiceRoomID := strings.TrimSpace(req.GetVoiceRoomId())
	spaceID := strings.TrimSpace(req.GetSpace().GetId())
	if voiceRoomID == "" {
		return nil, status.Error(codes.InvalidArgument, "voice_room_id is required")
	}
	if spaceID == "" {
		return nil, status.Error(codes.InvalidArgument, "space.id is required")
	}
	if _, err := uuid.Parse(voiceRoomID); err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid voice_room_id")
	}
	if _, err := uuid.Parse(spaceID); err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid space id")
	}
	access, err := s.resolveCanonicalVoiceRoomAccess(ctx, voiceRoomID, profileID)
	if err != nil {
		return nil, err
	}
	if access.SpaceID != spaceID {
		return nil, status.Error(codes.PermissionDenied, "space assertion does not match canonical voice room owner")
	}
	spaceID = access.SpaceID
	if s.Roles == nil {
		return nil, status.Error(codes.Unavailable, "voice join permission check unavailable")
	}
	if err := s.ensureVoiceJoinPermission(ctx, spaceID, profileID, voiceRoomID); err != nil {
		return nil, err
	}

	call, err := s.Calls.GetCallByVoiceRoomID(ctx, voiceRoomID)
	created := false
	if errors.Is(err, voicestore.ErrNotFound) {
		now := s.now()
		roomID := uuid.NewString()
		call = voicestore.Call{
			RoomID:             roomID,
			LivekitRoomName:    "voice-room-" + voiceRoomID,
			VoiceRoomID:        voiceRoomID,
			SpaceID:            spaceID,
			SessionKind:        callsv1.VoiceSessionKind_VOICE_SESSION_KIND_VOICE_ROOM,
			InitiatorProfileID: profileID,
			MediaKind:          callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO,
			Status:             callsv1.CallStatus_CALL_STATUS_UNSPECIFIED,
			StartedAt:          now,
			States:             map[string]voicestore.ParticipantState{},
		}
		created = true
	} else if err != nil {
		return nil, storeErr(err)
	} else if call.SpaceID != access.SpaceID {
		return nil, status.Error(codes.PermissionDenied, "stored voice room space does not match canonical owner")
	}
	if s.SpaceVoiceRoomGrants == nil {
		return nil, status.Error(codes.Unavailable, "Space voice authority is not configured")
	}
	maxParticipants := voicestore.MaxVoiceRoomParticipants
	if s.SpacePro != nil {
		pro, err := s.SpacePro.HasSpacePro(ctx, spaceID)
		if err != nil {
			return nil, status.Error(codes.Unavailable, "Space subscription check unavailable")
		}
		if pro {
			maxParticipants = voicestore.MaxSpaceProVoiceParticipants
		}
	}
	call, err = s.admitSpaceVoiceRoom(ctx, call, profileID, access, maxParticipants, created)
	if err != nil {
		return nil, err
	}
	return &callsv1.JoinVoiceRoomResponse{VoiceSession: voiceSessionToProto(call)}, nil
}

func (s *VoiceGRPC) LeaveVoiceRoom(ctx context.Context, req *callsv1.LeaveVoiceRoomRequest) (*callsv1.LeaveVoiceRoomResponse, error) {
	profileID, _, _, err := callerVoiceUser(ctx)
	if err != nil {
		return nil, err
	}
	if s == nil || s.Calls == nil {
		return nil, status.Error(codes.FailedPrecondition, "voice persistence not configured")
	}
	voiceRoomID := strings.TrimSpace(req.GetVoiceRoomId())
	if voiceRoomID == "" {
		return nil, status.Error(codes.InvalidArgument, "voice_room_id is required")
	}

	call, err := s.Calls.GetCallByVoiceRoomID(ctx, voiceRoomID)
	if errors.Is(err, voicestore.ErrNotFound) {
		return &callsv1.LeaveVoiceRoomResponse{}, nil
	}
	if err != nil {
		return nil, storeErr(err)
	}
	if !call.IsParticipant(profileID) {
		return &callsv1.LeaveVoiceRoomResponse{}, nil
	}
	if call.SpaceID != "" || len(call.SpaceMedia) > 0 {
		participant, ok := call.SpaceMedia[profileID]
		if call.SpaceID == "" || !ok {
			return nil, status.Error(codes.Unavailable, "Space media admission is not confirmed")
		}
		if err := s.leaveSpaceMediaParticipant(ctx, call, profileID, participant); err != nil {
			return nil, err
		}
		return &callsv1.LeaveVoiceRoomResponse{}, nil
	}
	if _, err := s.leaveOpenVoiceSession(ctx, call, profileID); err != nil {
		return nil, err
	}
	if participant, ok := call.SpaceMedia[profileID]; ok {
		if s.SpaceMediaAdmissions == nil {
			return nil, status.Error(codes.Unavailable, "Space media admission fence is unavailable")
		}
		operationID, opErr := uuid.Parse(participant.AdmissionOperationID)
		accountID, accountErr := uuid.Parse(participant.AccountID)
		profileUUID, profileErr := uuid.Parse(profileID)
		spaceUUID, spaceErr := uuid.Parse(call.SpaceID)
		if opErr != nil || accountErr != nil || profileErr != nil || spaceErr != nil {
			return nil, status.Error(codes.Unavailable, "Space media admission fence is invalid")
		}
		if err := s.SpaceMediaAdmissions.ReleaseFence(ctx, voicestore.SpaceMediaAdmission{
			OperationID: operationID, Generation: participant.Generation, AccountID: accountID,
			ProfileID: profileUUID, SpaceID: spaceUUID, RoomID: call.RoomID,
		}); err != nil {
			return nil, status.Error(codes.Unavailable, "Space media admission fence release failed")
		}
	}
	return &callsv1.LeaveVoiceRoomResponse{}, nil
}

func callerVoiceUser(ctx context.Context) (profileID, accountID string, epoch int64, err error) {
	if verified, ok := principal.FromContext(ctx); ok {
		account, accountErr := uuid.Parse(verified.AccountID)
		profile, profileErr := uuid.Parse(verified.ProfileID)
		if verified.Issuer != "gateway" || verified.Audience != "voice" || accountErr != nil || account.String() != verified.AccountID || profileErr != nil || profile.String() != verified.ProfileID || verified.SessionEpoch <= 0 {
			return "", "", 0, status.Error(codes.Unauthenticated, "invalid delegated Voice identity")
		}
		return verified.ProfileID, verified.AccountID, verified.SessionEpoch, nil
	}
	profileID, err = callerProfile(ctx)
	return profileID, "", 0, err
}

func (s *VoiceGRPC) leaveSpaceMediaParticipant(ctx context.Context, call voicestore.Call, profileID string, participant voicestore.SpaceMediaParticipant) error {
	if s.SpaceMediaRevoker == nil || s.SpaceMediaAdmissions == nil {
		return status.Error(codes.Unavailable, "Space media revocation is not ready")
	}
	operationID, operationErr := uuid.Parse(participant.AdmissionOperationID)
	accountID, accountErr := uuid.Parse(participant.AccountID)
	profileUUID, profileErr := uuid.Parse(profileID)
	spaceUUID, spaceErr := uuid.Parse(call.SpaceID)
	if operationErr != nil || accountErr != nil || profileErr != nil || spaceErr != nil || participant.Generation == "" {
		return status.Error(codes.Unavailable, "Space media admission fence is invalid")
	}
	if err := s.SpaceMediaAdmissions.BeginRoomLeave(ctx, operationID, participant.Generation); err != nil {
		return status.Error(codes.Unavailable, "Space media participant drain is pending")
	}
	updated, removed, err := s.SpaceMediaRevoker.RevokeSpaceMediaParticipant(ctx, call, participant)
	if err != nil {
		return status.Error(codes.Unavailable, "Space media participant removal is pending")
	}
	if err := s.SpaceMediaAdmissions.CompleteRoomLeave(ctx, voicestore.SpaceMediaAdmission{
		OperationID: operationID, Generation: participant.Generation, AccountID: accountID,
		ProfileID: profileUUID, SpaceID: spaceUUID, RoomID: call.RoomID,
	}); err != nil {
		return status.Error(codes.Unavailable, "Space media participant cleanup is pending")
	}
	if !removed {
		return nil
	}
	for _, share := range call.ScreenShares {
		if share.ProfileID == profileID {
			s.publishScreenShareStopped(ctx, updated, profileID, share.StreamID)
		}
	}
	if updated.Status == callsv1.CallStatus_CALL_STATUS_ENDED {
		s.publishEnded(ctx, updated, "hangup", profileID)
	}
	return nil
}

func (s *VoiceGRPC) ensureSpaceMember(ctx context.Context, spaceID, profileID string) error {
	if s.SpaceMembers == nil {
		return status.Error(codes.FailedPrecondition, "space membership check not configured")
	}
	if err := s.SpaceMembers.EnsureMember(ctx, spaceID, profileID); err != nil {
		if errors.Is(err, ErrNotSpaceMember) {
			return status.Error(codes.PermissionDenied, "not a space member")
		}
		return status.Error(codes.Internal, err.Error())
	}
	return nil
}

func (s *VoiceGRPC) ensureVoiceJoinPermission(ctx context.Context, spaceID, profileID, voiceRoomID string) error {
	if s.Roles == nil {
		return status.Error(codes.Unavailable, "voice join permission check unavailable")
	}
	if err := s.Roles.EnsureVoiceJoin(ctx, spaceID, profileID, voiceRoomID); err != nil {
		if errors.Is(err, ErrVoiceJoinDenied) {
			return status.Error(codes.PermissionDenied, "voice join not permitted")
		}
		if status.Code(err) == codes.Unavailable {
			return status.Error(codes.Unavailable, "voice join permission check unavailable")
		}
		return status.Error(codes.PermissionDenied, "voice join permission check unavailable")
	}
	return nil
}

func voiceSessionToProto(call voicestore.Call) *callsv1.VoiceSession {
	out := &callsv1.VoiceSession{
		RoomId:          call.RoomID,
		LivekitRoomName: call.LivekitRoomName,
		VoiceRoomId:     call.VoiceRoomID,
	}
	if hasPersistedRoomBinding(call) {
		spaceID := call.SpaceID
		out.SpaceId = &spaceID
	}
	return out
}
