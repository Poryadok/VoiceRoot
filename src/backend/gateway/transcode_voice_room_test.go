package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
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
	moveRequest     *callsv1.MoveVoiceRoomParticipantRequest
	moveProfileMD   []string
}

func TestTranscodeSpaceVoiceRoomModeratorMovePathBindsSpaceAndTarget(t *testing.T) {
	grpcRec := &recordingVoiceRooms{}
	conn, cleanup := startBufconnVoiceConn(t, grpcRec)
	t.Cleanup(cleanup)
	tc := &transcoder{clients: grpcClients{voice: callsv1.NewVoiceServiceClient(conn)}}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/spaces/space-path/voice-rooms/source-path/participants/target-path/move", strings.NewReader(`{"space":{"id":"forged"},"to_voice_room_id":"dest-path","operation_id":"op-path"}`))
	req.Header.Set("X-Voice-Profile-Id", "actor-path")
	resp := httptest.NewRecorder()
	require.True(t, tc.serveSpacesVoiceRooms(resp, req, "space-path/voice-rooms/source-path/participants/target-path/move"))
	require.Equal(t, http.StatusOK, resp.Code)
	require.Equal(t, "space-path", grpcRec.moveRequest.GetSpace().GetId())
	require.Equal(t, "source-path", grpcRec.moveRequest.GetFromVoiceRoomId())
	require.Equal(t, "target-path", grpcRec.moveRequest.GetParticipantProfileId())
	require.Equal(t, []string{"actor-path"}, grpcRec.moveProfileMD)
}

func (s *recordingVoiceRooms) MoveVoiceRoomParticipant(ctx context.Context, req *callsv1.MoveVoiceRoomParticipantRequest) (*callsv1.MoveVoiceRoomParticipantResponse, error) {
	s.moveRequest = req
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		s.moveProfileMD = md.Get("x-voice-profile-id")
	}
	return &callsv1.MoveVoiceRoomParticipantResponse{}, nil
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

func TestTranscodeVoiceRoomModeratorMoveUsesPathTargetAndDelegatedActor(t *testing.T) {
	grpcRec := &recordingVoiceRooms{}
	conn, cleanup := startBufconnVoiceConn(t, grpcRec)
	t.Cleanup(cleanup)
	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{"valid-user-token": {UserID: "account-1", ProfileID: "profile-actor"}},
		transcoder:  &transcoder{clients: grpcClients{voice: callsv1.NewVoiceServiceClient(conn)}},
	})
	body := `{"to_voice_room_id":"dest-1","space":{"id":"space-1"},"operation_id":"op-1","participant_profile_id":"forged-body-target"}`
	resp := performRequest(h, http.MethodPost, "/api/v1/voice/rooms/source-1/participants/profile-target/move", body, map[string]string{"Authorization": "Bearer valid-user-token"})
	require.Equal(t, http.StatusOK, resp.Code, "body=%s", resp.Body.String())
	require.NotNil(t, grpcRec.moveRequest)
	require.Equal(t, "source-1", grpcRec.moveRequest.GetFromVoiceRoomId())
	require.Equal(t, "dest-1", grpcRec.moveRequest.GetToVoiceRoomId())
	require.Equal(t, "profile-target", grpcRec.moveRequest.GetParticipantProfileId(), "the target is path-bound")
	require.Equal(t, []string{"profile-actor"}, grpcRec.moveProfileMD, "Gateway forwards authenticated actor metadata")
}
