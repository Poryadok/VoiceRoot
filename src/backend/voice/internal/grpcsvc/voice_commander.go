package grpcsvc

import (
	"context"
	"errors"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	callsv1 "voice.app/voice/calls/v1"
	voicestore "voice/backend/voice/internal/store"
)

func (s *VoiceGRPC) SetCommanderMode(ctx context.Context, req *callsv1.SetCommanderModeRequest) (*callsv1.SetCommanderModeResponse, error) {
	profileID, err := callerProfile(ctx)
	if err != nil {
		return nil, err
	}
	call, err := s.requireActiveCall(ctx, req.GetRoomId(), profileID)
	if err != nil {
		return nil, err
	}
	enabled := req.GetEnabled()
	if enabled {
		if err := s.ensureMuteOthersPermission(ctx, call, profileID); err != nil {
			return nil, err
		}
	}
	call, state, err := s.Calls.UpdateVoiceState(ctx, call.RoomID, profileID, voicestore.VoiceStatePatch{
		IsCommander: &enabled,
	})
	if err != nil {
		return nil, storeErr(err)
	}
	s.publishState(ctx, call, state)
	return &callsv1.SetCommanderModeResponse{}, nil
}

func (s *VoiceGRPC) SetBroadcasting(ctx context.Context, req *callsv1.SetBroadcastingRequest) (*callsv1.SetBroadcastingResponse, error) {
	profileID, err := callerProfile(ctx)
	if err != nil {
		return nil, err
	}
	call, err := s.requireActiveCall(ctx, req.GetRoomId(), profileID)
	if err != nil {
		return nil, err
	}
	state := call.States[profileID]
	if !state.IsCommander {
		return nil, status.Error(codes.PermissionDenied, "commander mode required")
	}
	enabled := req.GetEnabled()
	if enabled {
		if err := s.ensureMuteOthersPermission(ctx, call, profileID); err != nil {
			return nil, err
		}
		if err := s.ensureVoiceSpeakPermission(ctx, call, profileID); err != nil {
			return nil, err
		}
	}
	call, state, err = s.Calls.UpdateVoiceState(ctx, call.RoomID, profileID, voicestore.VoiceStatePatch{
		IsBroadcasting: &enabled,
	})
	if err != nil {
		return nil, storeErr(err)
	}
	s.publishState(ctx, call, state)
	return &callsv1.SetBroadcastingResponse{}, nil
}

func (s *VoiceGRPC) RaiseHand(ctx context.Context, req *callsv1.RaiseHandRequest) (*callsv1.RaiseHandResponse, error) {
	profileID, err := callerProfile(ctx)
	if err != nil {
		return nil, err
	}
	call, err := s.requireActiveCall(ctx, req.GetRoomId(), profileID)
	if err != nil {
		return nil, err
	}
	raised := true
	call, state, err := s.Calls.UpdateVoiceState(ctx, call.RoomID, profileID, voicestore.VoiceStatePatch{
		HandRaised: &raised,
	})
	if err != nil {
		return nil, storeErr(err)
	}
	s.publishState(ctx, call, state)
	return &callsv1.RaiseHandResponse{}, nil
}

func (s *VoiceGRPC) LowerHand(ctx context.Context, req *callsv1.LowerHandRequest) (*callsv1.LowerHandResponse, error) {
	profileID, err := callerProfile(ctx)
	if err != nil {
		return nil, err
	}
	call, err := s.requireActiveCall(ctx, req.GetRoomId(), profileID)
	if err != nil {
		return nil, err
	}
	raised := false
	call, state, err := s.Calls.UpdateVoiceState(ctx, call.RoomID, profileID, voicestore.VoiceStatePatch{
		HandRaised: &raised,
	})
	if err != nil {
		return nil, storeErr(err)
	}
	s.publishState(ctx, call, state)
	return &callsv1.LowerHandResponse{}, nil
}

func (s *VoiceGRPC) GrantFloor(ctx context.Context, req *callsv1.GrantFloorRequest) (*callsv1.GrantFloorResponse, error) {
	organizerID, err := callerProfile(ctx)
	if err != nil {
		return nil, err
	}
	targetID := strings.TrimSpace(req.GetProfileId())
	if targetID == "" {
		return nil, status.Error(codes.InvalidArgument, "profile_id required")
	}
	call, err := s.requireActiveCall(ctx, req.GetRoomId(), organizerID)
	if err != nil {
		return nil, err
	}
	if err := s.ensureOrganizerFloorControl(ctx, call, organizerID); err != nil {
		return nil, err
	}
	if !call.IsParticipant(targetID) {
		return nil, status.Error(codes.FailedPrecondition, "target is not a participant")
	}
	hasFloor := true
	raised := false
	call, state, err := s.Calls.UpdateVoiceState(ctx, call.RoomID, targetID, voicestore.VoiceStatePatch{
		HasFloor:   &hasFloor,
		HandRaised: &raised,
	})
	if err != nil {
		return nil, storeErr(err)
	}
	s.publishState(ctx, call, state)
	return &callsv1.GrantFloorResponse{}, nil
}

func (s *VoiceGRPC) RevokeFloor(ctx context.Context, req *callsv1.RevokeFloorRequest) (*callsv1.RevokeFloorResponse, error) {
	organizerID, err := callerProfile(ctx)
	if err != nil {
		return nil, err
	}
	targetID := strings.TrimSpace(req.GetProfileId())
	if targetID == "" {
		return nil, status.Error(codes.InvalidArgument, "profile_id required")
	}
	call, err := s.requireActiveCall(ctx, req.GetRoomId(), organizerID)
	if err != nil {
		return nil, err
	}
	if err := s.ensureOrganizerFloorControl(ctx, call, organizerID); err != nil {
		return nil, err
	}
	if !call.IsParticipant(targetID) {
		return nil, status.Error(codes.FailedPrecondition, "target is not a participant")
	}
	hasFloor := false
	call, state, err := s.Calls.UpdateVoiceState(ctx, call.RoomID, targetID, voicestore.VoiceStatePatch{
		HasFloor: &hasFloor,
	})
	if err != nil {
		return nil, storeErr(err)
	}
	s.publishState(ctx, call, state)
	return &callsv1.RevokeFloorResponse{}, nil
}

// ensureOrganizerFloorControl requires Role Service in Space voice rooms and
// preserves the existing commander/initiator policy for calls without role scope.
func (s *VoiceGRPC) ensureOrganizerFloorControl(ctx context.Context, call voicestore.Call, organizerID string) error {
	if call.IsVoiceRoom() && call.SpaceID != "" {
		return s.ensureMuteOthersPermission(ctx, call, organizerID)
	}
	state := call.States[organizerID]
	if state.IsCommander || call.InitiatorProfileID == organizerID {
		return nil
	}
	if call.SpaceID != "" && s.Roles != nil {
		if err := s.Roles.EnsureMuteOthers(ctx, call.SpaceID, organizerID, call.VoiceRoomID); err != nil {
			if errors.Is(err, ErrMuteOthersDenied) {
				return status.Error(codes.PermissionDenied, "organizer floor control required")
			}
			return err
		}
		return nil
	}
	return status.Error(codes.PermissionDenied, "organizer floor control required")
}
