package grpcsvc

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

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
	for field, value := range map[string]string{"from_voice_room_id": fromRoom, "to_voice_room_id": toRoom, "space.id": assertedSpace, "operation_id": operationID} {
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
	if replay, found, err := s.Calls.FindVoiceRoomMove(ctx, moveReq); err != nil {
		return voicestore.VoiceRoomMoveResult{}, moveStoreErr(err)
	} else if found {
		return replay, nil
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
	targetDestination, err := s.resolveCanonicalVoiceRoomAccess(ctx, toRoom, target)
	if err != nil {
		return voicestore.VoiceRoomMoveResult{}, err
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
	profileID, err := callerProfile(ctx)
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
	if err := s.ensureVoiceJoinPermission(ctx, spaceID, profileID, voiceRoomID); err != nil {
		return nil, err
	}

	call, err := s.Calls.GetCallByVoiceRoomID(ctx, voiceRoomID)
	if errors.Is(err, voicestore.ErrNotFound) {
		now := s.now()
		roomID := uuid.NewString()
		call, err = s.Calls.CreateCall(ctx, voicestore.Call{
			RoomID:             roomID,
			LivekitRoomName:    "voice-room-" + voiceRoomID,
			VoiceRoomID:        voiceRoomID,
			SpaceID:            spaceID,
			SessionKind:        callsv1.VoiceSessionKind_VOICE_SESSION_KIND_VOICE_ROOM,
			InitiatorProfileID: profileID,
			MediaKind:          callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO,
			Status:             callsv1.CallStatus_CALL_STATUS_ACTIVE,
			StartedAt:          now,
		})
		if err != nil {
			return nil, storeErr(err)
		}
		s.publishCallStarted(ctx, call)
		s.publishVoiceMemberJoined(ctx, call, profileID)
	} else if err != nil {
		return nil, storeErr(err)
	} else if call.SpaceID != access.SpaceID {
		return nil, status.Error(codes.PermissionDenied, "stored voice room space does not match canonical owner")
	}

	if !call.IsParticipant(profileID) {
		maxParticipants := voicestore.MaxVoiceRoomParticipants
		if s.SpacePro != nil {
			if ok, err := s.SpacePro.HasSpacePro(ctx, spaceID); err == nil && ok {
				maxParticipants = voicestore.MaxSpaceProVoiceParticipants
			}
		}
		call, err = s.Calls.AddParticipant(ctx, call.RoomID, profileID, maxParticipants)
		if err != nil {
			return nil, storeErr(err)
		}
		s.publishVoiceMemberJoined(ctx, call, profileID)
	}

	return &callsv1.JoinVoiceRoomResponse{VoiceSession: voiceSessionToProto(call)}, nil
}

func (s *VoiceGRPC) LeaveVoiceRoom(ctx context.Context, req *callsv1.LeaveVoiceRoomRequest) (*callsv1.LeaveVoiceRoomResponse, error) {
	profileID, err := callerProfile(ctx)
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
	if call.SpaceID != "" {
		if err := s.ensureSpaceMember(ctx, call.SpaceID, profileID); err != nil {
			return nil, err
		}
	}
	if !call.IsParticipant(profileID) {
		return &callsv1.LeaveVoiceRoomResponse{}, nil
	}
	if _, err := s.leaveOpenVoiceSession(ctx, call, profileID); err != nil {
		return nil, err
	}
	return &callsv1.LeaveVoiceRoomResponse{}, nil
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
		return nil
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
