package main

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	callsv1 "voice.app/voice/calls/v1"
)

type recordingScreenShareVoice struct {
	callsv1.UnimplementedVoiceServiceServer
	startRoom string
	stopRoom  string
	stopReq   *callsv1.StopScreenShareRequest
}

func (s *recordingScreenShareVoice) StartScreenShare(_ context.Context, req *callsv1.StartScreenShareRequest) (*callsv1.StartScreenShareResponse, error) {
	s.startRoom = req.GetRoomId()
	return &callsv1.StartScreenShareResponse{
		ScreenShareSession: &callsv1.ScreenShareSession{StreamId: "stream-test"},
	}, nil
}

func (s *recordingScreenShareVoice) StopScreenShare(_ context.Context, req *callsv1.StopScreenShareRequest) (*callsv1.StopScreenShareResponse, error) {
	s.stopRoom = req.GetRoomId()
	s.stopReq = req
	return &callsv1.StopScreenShareResponse{}, nil
}

func TestTranscodeVoiceScreenShareStartStop(t *testing.T) {
	t.Parallel()

	grpcRec := &recordingScreenShareVoice{}
	conn, cleanup := startBufconnVoiceConn(t, grpcRec)
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{voice: callsv1.NewVoiceServiceClient(conn)}},
	})

	resp := performRequest(h, http.MethodPost, "/api/v1/voice/calls/room-1/screen-share/start", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
	require.Equal(t, "room-1", grpcRec.startRoom)
	require.Contains(t, resp.Body.String(), "stream-test")

	resp = performRequest(h, http.MethodPost, "/api/v1/voice/calls/room-1/screen-share/stop", `{"stream_id":"stream-test"}`, map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusNoContent, resp.Code)
	require.Equal(t, "room-1", grpcRec.stopRoom)
	require.Equal(t, "stream-test", grpcRec.stopReq.GetStreamId())
}
