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
)

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
	if err := s.ensureSpaceMember(ctx, spaceID, profileID); err != nil {
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
	} else if err != nil {
		return nil, storeErr(err)
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
	s.clearScreenShareForProfile(ctx, call, profileID)
	if _, err := s.Calls.RemoveParticipant(ctx, call.RoomID, profileID); err != nil {
		return nil, storeErr(err)
	}
	return &callsv1.LeaveVoiceRoomResponse{}, nil
}

func (s *VoiceGRPC) ensureSpaceMember(ctx context.Context, spaceID, profileID string) error {
	if s.SpaceMembers == nil {
		return nil
	}
	if err := s.SpaceMembers.EnsureMember(ctx, spaceID, profileID); err != nil {
		if errors.Is(err, ErrNotSpaceMember) {
			return status.Error(codes.PermissionDenied, "not a space member")
		}
		return status.Error(codes.Internal, err.Error())
	}
	return nil
}

func voiceSessionToProto(call voicestore.Call) *callsv1.VoiceSession {
	return &callsv1.VoiceSession{
		RoomId:          call.RoomID,
		LivekitRoomName: call.LivekitRoomName,
		VoiceRoomId:     call.VoiceRoomID,
	}
}
