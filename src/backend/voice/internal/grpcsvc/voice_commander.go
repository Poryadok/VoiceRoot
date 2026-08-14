package grpcsvc

import (
	"context"

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
	call, state, err := s.Calls.UpdateVoiceState(ctx, call.RoomID, profileID, voicestore.VoiceStatePatch{
		IsCommander: &enabled,
	})
	if err != nil {
		return nil, storeErr(err)
	}
	s.publishState(ctx, call, state)
	return &callsv1.SetCommanderModeResponse{}, nil
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
