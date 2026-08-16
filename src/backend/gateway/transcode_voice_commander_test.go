package main

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	callsv1 "voice.app/voice/calls/v1"
)

type recordingCommanderVoice struct {
	callsv1.UnimplementedVoiceServiceServer
	commanderEnabled bool
	broadcastEnabled bool
	raised           *bool
	grantProfile     string
	revokeProfile    string
}

func (s *recordingCommanderVoice) SetCommanderMode(_ context.Context, req *callsv1.SetCommanderModeRequest) (*callsv1.SetCommanderModeResponse, error) {
	s.commanderEnabled = req.GetEnabled()
	return &callsv1.SetCommanderModeResponse{}, nil
}

func (s *recordingCommanderVoice) SetBroadcasting(_ context.Context, req *callsv1.SetBroadcastingRequest) (*callsv1.SetBroadcastingResponse, error) {
	s.broadcastEnabled = req.GetEnabled()
	return &callsv1.SetBroadcastingResponse{}, nil
}

func (s *recordingCommanderVoice) RaiseHand(_ context.Context, _ *callsv1.RaiseHandRequest) (*callsv1.RaiseHandResponse, error) {
	v := true
	s.raised = &v
	return &callsv1.RaiseHandResponse{}, nil
}

func (s *recordingCommanderVoice) LowerHand(_ context.Context, _ *callsv1.LowerHandRequest) (*callsv1.LowerHandResponse, error) {
	v := false
	s.raised = &v
	return &callsv1.LowerHandResponse{}, nil
}

func (s *recordingCommanderVoice) GrantFloor(_ context.Context, req *callsv1.GrantFloorRequest) (*callsv1.GrantFloorResponse, error) {
	s.grantProfile = req.GetProfileId()
	return &callsv1.GrantFloorResponse{}, nil
}

func (s *recordingCommanderVoice) RevokeFloor(_ context.Context, req *callsv1.RevokeFloorRequest) (*callsv1.RevokeFloorResponse, error) {
	s.revokeProfile = req.GetProfileId()
	return &callsv1.RevokeFloorResponse{}, nil
}

func TestTranscodeVoiceCommanderFloorRoutes(t *testing.T) {
	t.Parallel()

	grpcRec := &recordingCommanderVoice{}
	conn, cleanup := startBufconnVoiceConn(t, grpcRec)
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{voice: callsv1.NewVoiceServiceClient(conn)}},
	})

	auth := map[string]string{"Authorization": "Bearer valid-user-token"}

	resp := performRequest(h, http.MethodPost, "/api/v1/voice/calls/room-1/commander", `{"enabled":true}`, auth)
	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
	require.True(t, grpcRec.commanderEnabled)

	resp = performRequest(h, http.MethodPost, "/api/v1/voice/calls/room-1/raise-hand", "", auth)
	require.Equal(t, http.StatusNoContent, resp.Code)
	require.NotNil(t, grpcRec.raised)
	require.True(t, *grpcRec.raised)

	resp = performRequest(h, http.MethodPost, "/api/v1/voice/calls/room-1/grant-floor", `{"profile_id":"profile-2"}`, auth)
	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
	require.Equal(t, "profile-2", grpcRec.grantProfile)

	resp = performRequest(h, http.MethodPost, "/api/v1/voice/calls/room-1/revoke-floor", `{"profile_id":"profile-2"}`, auth)
	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
	require.Equal(t, "profile-2", grpcRec.revokeProfile)

	resp = performRequest(h, http.MethodPost, "/api/v1/voice/calls/room-1/broadcast", `{"enabled":true}`, auth)
	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
	require.True(t, grpcRec.broadcastEnabled)

	resp = performRequest(h, http.MethodPost, "/api/v1/voice/calls/room-1/lower-hand", "", auth)
	require.Equal(t, http.StatusNoContent, resp.Code)
	require.NotNil(t, grpcRec.raised)
	require.False(t, *grpcRec.raised)
}
