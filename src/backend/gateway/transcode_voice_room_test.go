package main

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"

	callsv1 "voice.app/voice/calls/v1"
)

type recordingVoiceRooms struct {
	callsv1.UnimplementedVoiceServiceServer
	joinVoiceRoomID string
	joinSpaceID     string
	leaveVoiceRoom  string
	statesVoiceRoom string
}

func (s *recordingVoiceRooms) JoinVoiceRoom(_ context.Context, req *callsv1.JoinVoiceRoomRequest) (*callsv1.JoinVoiceRoomResponse, error) {
	s.joinVoiceRoomID = req.GetVoiceRoomId()
	if req.GetSpace() != nil {
		s.joinSpaceID = req.GetSpace().GetId()
	}
	return &callsv1.JoinVoiceRoomResponse{
		VoiceSession: &callsv1.VoiceSession{
			RoomId:          "room-vr-1",
			LivekitRoomName: "voice-room-" + req.GetVoiceRoomId(),
			VoiceRoomId:     req.GetVoiceRoomId(),
		},
	}, nil
}

func (s *recordingVoiceRooms) LeaveVoiceRoom(_ context.Context, req *callsv1.LeaveVoiceRoomRequest) (*callsv1.LeaveVoiceRoomResponse, error) {
	s.leaveVoiceRoom = req.GetVoiceRoomId()
	return &callsv1.LeaveVoiceRoomResponse{}, nil
}

func (s *recordingVoiceRooms) GetVoiceStates(ctx context.Context, req *callsv1.GetVoiceStatesRequest) (*callsv1.GetVoiceStatesResponse, error) {
	if req.VoiceRoomId != nil {
		s.statesVoiceRoom = req.GetVoiceRoomId()
	}
	if s.statesVoiceRoom == "" {
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if vals := md.Get("x-voice-room-id"); len(vals) > 0 {
				s.statesVoiceRoom = vals[0]
			}
		}
	}
	return &callsv1.GetVoiceStatesResponse{}, nil
}

func TestTranscodeVoiceRoomJoin(t *testing.T) {
	t.Parallel()

	grpcRec := &recordingVoiceRooms{}
	conn, cleanup := startBufconnVoiceConn(t, grpcRec)
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{voice: callsv1.NewVoiceServiceClient(conn)}},
	})

	body := `{"space":{"id":"space-1"}}`
	resp := performRequest(h, http.MethodPost, "/api/v1/voice/rooms/vr-1/join", body, map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusOK, resp.Code, "body=%s", resp.Body.String())
	require.Equal(t, "vr-1", grpcRec.joinVoiceRoomID)
	require.Equal(t, "space-1", grpcRec.joinSpaceID)
}

func TestTranscodeVoiceRoomLeave(t *testing.T) {
	t.Parallel()

	grpcRec := &recordingVoiceRooms{}
	conn, cleanup := startBufconnVoiceConn(t, grpcRec)
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{voice: callsv1.NewVoiceServiceClient(conn)}},
	})

	resp := performRequest(h, http.MethodPost, "/api/v1/voice/rooms/vr-2/leave", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusNoContent, resp.Code)
	require.Equal(t, "vr-2", grpcRec.leaveVoiceRoom)
}

func TestTranscodeVoiceRoomStates(t *testing.T) {
	t.Parallel()

	grpcRec := &recordingVoiceRooms{}
	conn, cleanup := startBufconnVoiceConn(t, grpcRec)
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{voice: callsv1.NewVoiceServiceClient(conn)}},
	})

	resp := performRequest(h, http.MethodGet, "/api/v1/voice/rooms/vr-3/states", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusOK, resp.Code)
	require.Equal(t, "vr-3", grpcRec.statesVoiceRoom)
}
