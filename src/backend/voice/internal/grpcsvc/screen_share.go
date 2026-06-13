package grpcsvc

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	voicestore "voice/backend/voice/internal/store"

	callsv1 "voice.app/voice/calls/v1"
	eventsv1 "voice.app/voice/events/v1"
)

func (s *VoiceGRPC) StartScreenShare(ctx context.Context, req *callsv1.StartScreenShareRequest) (*callsv1.StartScreenShareResponse, error) {
	profileID, err := callerProfile(ctx)
	if err != nil {
		return nil, err
	}
	if s == nil || s.Calls == nil {
		return nil, status.Error(codes.FailedPrecondition, "voice persistence not configured")
	}
	roomID := req.GetRoomId()
	call, err := s.requireActiveCall(ctx, roomID, profileID)
	if err != nil {
		return nil, err
	}
	if err := s.ensureScreenSharePermission(ctx, call, profileID); err != nil {
		return nil, err
	}
	streamID := uuid.NewString()
	call, entry, err := s.Calls.StartScreenShare(ctx, roomID, profileID, streamID)
	if err != nil {
		return nil, storeErr(err)
	}
	s.publishScreenShareStarted(ctx, call, entry)
	return &callsv1.StartScreenShareResponse{
		ScreenShareSession: &callsv1.ScreenShareSession{StreamId: entry.StreamID},
	}, nil
}

func (s *VoiceGRPC) StopScreenShare(ctx context.Context, req *callsv1.StopScreenShareRequest) (*callsv1.StopScreenShareResponse, error) {
	profileID, err := callerProfile(ctx)
	if err != nil {
		return nil, err
	}
	if s == nil || s.Calls == nil {
		return nil, status.Error(codes.FailedPrecondition, "voice persistence not configured")
	}
	roomID := req.GetRoomId()
	call, err := s.requireActiveCall(ctx, roomID, profileID)
	if err != nil {
		return nil, err
	}
	streamID := ""
	if req.StreamId != nil {
		streamID = req.GetStreamId()
	}
	call, err = s.Calls.StopScreenShare(ctx, roomID, profileID, streamID)
	if err != nil {
		return nil, storeErr(err)
	}
	s.publishScreenShareStopped(ctx, call, profileID, streamID)
	return &callsv1.StopScreenShareResponse{}, nil
}

func (s *VoiceGRPC) requireActiveCall(ctx context.Context, roomID, profileID string) (voicestore.Call, error) {
	call, err := s.requireCall(ctx, roomID, profileID)
	if err != nil {
		return voicestore.Call{}, err
	}
	if call.Status != callsv1.CallStatus_CALL_STATUS_ACTIVE {
		return voicestore.Call{}, status.Error(codes.FailedPrecondition, "call is not active")
	}
	return call, nil
}

func (s *VoiceGRPC) ensureScreenSharePermission(ctx context.Context, call voicestore.Call, profileID string) error {
	if !call.IsVoiceRoom() || call.SpaceID == "" {
		return nil
	}
	if s.Roles == nil {
		return status.Error(codes.PermissionDenied, "screen share permission check unavailable")
	}
	if err := s.Roles.EnsureScreenShare(ctx, call.SpaceID, profileID, call.VoiceRoomID); err != nil {
		if errors.Is(err, ErrScreenShareDenied) {
			return status.Error(codes.PermissionDenied, "screen share not permitted")
		}
		return status.Error(codes.PermissionDenied, "screen share permission check unavailable")
	}
	return nil
}

func (s *VoiceGRPC) clearScreenShareForProfile(ctx context.Context, call voicestore.Call, profileID string) {
	if s == nil || s.Calls == nil {
		return
	}
	updated, err := s.Calls.StopScreenSharesForProfile(ctx, call.RoomID, profileID)
	if err != nil {
		return
	}
	for _, share := range call.ScreenShares {
		if share.ProfileID == profileID {
			s.publishScreenShareStopped(ctx, updated, profileID, share.StreamID)
		}
	}
}

func (s *VoiceGRPC) publishScreenShareStarted(ctx context.Context, call voicestore.Call, entry voicestore.ScreenShareEntry) {
	if s == nil || s.Events == nil {
		return
	}
	_ = s.Events.PublishScreenShareStarted(ctx, &eventsv1.ScreenShareStarted{
		RoomId:     call.RoomID,
		ProfileId:  entry.ProfileID,
		StreamId:   entry.StreamID,
		ProfileIds: call.ProfileIDs(),
	})
}

func (s *VoiceGRPC) publishScreenShareStopped(ctx context.Context, call voicestore.Call, profileID, streamID string) {
	if s == nil || s.Events == nil {
		return
	}
	_ = s.Events.PublishScreenShareStopped(ctx, &eventsv1.ScreenShareStopped{
		RoomId:     call.RoomID,
		ProfileId:  profileID,
		StreamId:   streamID,
		ProfileIds: call.ProfileIDs(),
	})
}
