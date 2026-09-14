package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	callsv1 "voice.app/voice/calls/v1"
)

type recordingVoiceRooms struct {
	callsv1.UnimplementedVoiceServiceServer
	joinVoiceRoomID    string
	joinSpaceID        string
	leaveVoiceRoom     string
	statesVoiceRoom    string
	moveRequest        *callsv1.MoveVoiceRoomParticipantRequest
	moveProfileMD      []string
	moveSelfErr        error
	moveParticipantErr error
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
	if s.moveParticipantErr != nil {
		return nil, s.moveParticipantErr
	}
	return &callsv1.MoveVoiceRoomParticipantResponse{}, nil
}

func (s *recordingVoiceRooms) MoveToVoiceRoom(_ context.Context, _ *callsv1.MoveToVoiceRoomRequest) (*callsv1.MoveToVoiceRoomResponse, error) {
	if s.moveSelfErr != nil {
		return nil, s.moveSelfErr
	}
	return &callsv1.MoveToVoiceRoomResponse{}, nil
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

func TestTranscodeVoiceRoomMoveFailedPreconditionUsesConflictEnvelope(t *testing.T) {
	for _, tc := range []struct {
		name, path string
		configure  func(*recordingVoiceRooms)
	}{
		{
			name: "self move stale source",
			path: "/api/v1/voice/rooms/source-1/move",
			configure: func(s *recordingVoiceRooms) {
				s.moveSelfErr = status.Error(codes.FailedPrecondition, "backend stale source diagnostic")
			},
		},
		{
			name: "moderator changed operation id",
			path: "/api/v1/voice/rooms/source-1/participants/profile-target/move",
			configure: func(s *recordingVoiceRooms) {
				s.moveParticipantErr = status.Error(codes.FailedPrecondition, "operation id conflicts with a different canonical request")
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			grpcRec := &recordingVoiceRooms{}
			tc.configure(grpcRec)
			conn, cleanup := startBufconnVoiceConn(t, grpcRec)
			t.Cleanup(cleanup)
			h := newGatewayForContract(t, gatewayTestOptions{
				tokenClaims: map[string]tokenClaims{"valid-user-token": {UserID: "account-1", ProfileID: "profile-actor"}},
				transcoder:  &transcoder{clients: grpcClients{voice: callsv1.NewVoiceServiceClient(conn)}},
			})
			resp := performRequest(h, http.MethodPost, tc.path, `{"to_voice_room_id":"dest-1","operation_id":"op-1"}`, map[string]string{"Authorization": "Bearer valid-user-token"})
			require.Equal(t, http.StatusConflict, resp.Code, "body=%s", resp.Body.String())
			var envelope struct {
				ErrorCode string `json:"error_code"`
				Message   string `json:"message"`
			}
			decodeJSON(t, resp.Body, &envelope)
			require.Equal(t, "failed_precondition", envelope.ErrorCode)
			require.Contains(t, []string{"voice room move is no longer possible", "voice room move conflicts with current state"}, envelope.Message)
		})
	}
}

func TestWriteGRPCError_UnrelatedFailedPreconditionRemains412(t *testing.T) {
	resp := httptest.NewRecorder()
	writeGRPCError(resp, status.Error(codes.FailedPrecondition, "registration_conflict"))
	require.Equal(t, http.StatusPreconditionFailed, resp.Code)
	var envelope struct {
		ErrorCode string `json:"error_code"`
		Message   string `json:"message"`
	}
	decodeJSON(t, resp.Body, &envelope)
	require.Equal(t, "registration_conflict", envelope.ErrorCode)
	require.Equal(t, "registration_conflict", envelope.Message)
}

func TestWriteVoiceRoomMoveError_UsesFiniteMessagesForEveryTransportClass(t *testing.T) {
	for code, expected := range map[codes.Code]struct {
		status int
		key    string
	}{
		codes.InvalidArgument:    {http.StatusBadRequest, "invalid_argument"},
		codes.Unauthenticated:    {http.StatusUnauthorized, "unauthenticated"},
		codes.PermissionDenied:   {http.StatusForbidden, "permission_denied"},
		codes.NotFound:           {http.StatusNotFound, "not_found"},
		codes.FailedPrecondition: {http.StatusConflict, "failed_precondition"},
		codes.ResourceExhausted:  {http.StatusTooManyRequests, "resource_exhausted"},
		codes.Unavailable:        {http.StatusServiceUnavailable, "unavailable"},
		codes.Internal:           {http.StatusInternalServerError, "internal"},
	} {
		t.Run(code.String(), func(t *testing.T) {
			resp := httptest.NewRecorder()
			writeVoiceRoomMoveError(resp, status.Error(code, "backend diagnostic must never reach REST"))
			require.Equal(t, expected.status, resp.Code)
			var envelope struct {
				ErrorCode string `json:"error_code"`
				Message   string `json:"message"`
			}
			decodeJSON(t, resp.Body, &envelope)
			require.Equal(t, expected.key, envelope.ErrorCode)
			require.NotEmpty(t, envelope.Message)
			require.NotContains(t, envelope.Message, "backend diagnostic")
		})
	}
}

func TestTranscodeSpaceVoiceRoomMoveFailedPreconditionUsesConflictEnvelope(t *testing.T) {
	grpcRec := &recordingVoiceRooms{moveSelfErr: status.Error(codes.FailedPrecondition, "stale source backend diagnostic")}
	conn, cleanup := startBufconnVoiceConn(t, grpcRec)
	t.Cleanup(cleanup)
	tc := &transcoder{clients: grpcClients{voice: callsv1.NewVoiceServiceClient(conn)}}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/spaces/space-path/voice-rooms/source-path/move", strings.NewReader(`{"to_voice_room_id":"dest-path","operation_id":"op-path"}`))
	resp := httptest.NewRecorder()
	require.True(t, tc.serveSpacesVoiceRooms(resp, req, "space-path/voice-rooms/source-path/move"))
	require.Equal(t, http.StatusConflict, resp.Code, "body=%s", resp.Body.String())
	var envelope struct {
		ErrorCode string `json:"error_code"`
		Message   string `json:"message"`
	}
	decodeJSON(t, resp.Body, &envelope)
	require.Equal(t, "failed_precondition", envelope.ErrorCode)
	require.Contains(t, []string{"voice room move is no longer possible", "voice room move conflicts with current state"}, envelope.Message)
}

func TestTranscodeSpaceVoiceRoomModeratorMoveFailedPreconditionUsesConflictEnvelope(t *testing.T) {
	grpcRec := &recordingVoiceRooms{moveParticipantErr: status.Error(codes.FailedPrecondition, "stale source backend diagnostic")}
	conn, cleanup := startBufconnVoiceConn(t, grpcRec)
	t.Cleanup(cleanup)
	tc := &transcoder{clients: grpcClients{voice: callsv1.NewVoiceServiceClient(conn)}}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/spaces/space-path/voice-rooms/source-path/participants/target-path/move", strings.NewReader(`{"to_voice_room_id":"dest-path","operation_id":"op-path"}`))
	resp := httptest.NewRecorder()
	require.True(t, tc.serveSpacesVoiceRooms(resp, req, "space-path/voice-rooms/source-path/participants/target-path/move"))
	require.Equal(t, http.StatusConflict, resp.Code, "body=%s", resp.Body.String())
	var envelope struct {
		ErrorCode string `json:"error_code"`
		Message   string `json:"message"`
	}
	decodeJSON(t, resp.Body, &envelope)
	require.Equal(t, "failed_precondition", envelope.ErrorCode)
	require.Equal(t, "voice room move is no longer possible", envelope.Message)
}

func TestTranscodeVoiceRoomMoveUnavailableUsesSafeServiceUnavailableEnvelope(t *testing.T) {
	for _, tc := range []struct {
		name, path string
		configure  func(*recordingVoiceRooms)
	}{
		{
			name: "self move",
			path: "/api/v1/voice/rooms/source-1/move",
			configure: func(s *recordingVoiceRooms) {
				s.moveSelfErr = status.Error(codes.Unavailable, "redis retry diagnostics must not reach REST")
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			grpcRec := &recordingVoiceRooms{}
			tc.configure(grpcRec)
			conn, cleanup := startBufconnVoiceConn(t, grpcRec)
			t.Cleanup(cleanup)
			h := newGatewayForContract(t, gatewayTestOptions{
				tokenClaims: map[string]tokenClaims{"valid-user-token": {UserID: "account-1", ProfileID: "profile-actor"}},
				transcoder:  &transcoder{clients: grpcClients{voice: callsv1.NewVoiceServiceClient(conn)}},
			})
			resp := performRequest(h, http.MethodPost, tc.path, `{"to_voice_room_id":"dest-1","operation_id":"op-1"}`, map[string]string{"Authorization": "Bearer valid-user-token"})
			require.Equal(t, http.StatusServiceUnavailable, resp.Code, "body=%s", resp.Body.String())
			var envelope struct {
				ErrorCode string `json:"error_code"`
				Message   string `json:"message"`
			}
			decodeJSON(t, resp.Body, &envelope)
			require.Equal(t, "unavailable", envelope.ErrorCode)
			require.Equal(t, "voice room roster unavailable", envelope.Message)
		})
	}
}

func TestTranscodeVoiceRoomModeratorMoveUnavailableUsesSafeServiceUnavailableEnvelope(t *testing.T) {
	grpcRec := &recordingVoiceRooms{moveParticipantErr: status.Error(codes.Unavailable, "redis retry diagnostics must not reach REST")}
	conn, cleanup := startBufconnVoiceConn(t, grpcRec)
	t.Cleanup(cleanup)
	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{"valid-user-token": {UserID: "account-1", ProfileID: "profile-actor"}},
		transcoder:  &transcoder{clients: grpcClients{voice: callsv1.NewVoiceServiceClient(conn)}},
	})
	resp := performRequest(h, http.MethodPost, "/api/v1/voice/rooms/source-1/participants/target-1/move", `{"to_voice_room_id":"dest-1","operation_id":"op-1"}`, map[string]string{"Authorization": "Bearer valid-user-token"})
	require.Equal(t, http.StatusServiceUnavailable, resp.Code, "body=%s", resp.Body.String())
	var envelope struct {
		ErrorCode string `json:"error_code"`
		Message   string `json:"message"`
	}
	decodeJSON(t, resp.Body, &envelope)
	require.Equal(t, "unavailable", envelope.ErrorCode)
	require.Equal(t, "voice room roster unavailable", envelope.Message)
}

func TestTranscodeSpaceVoiceRoomSelfMoveUnavailableUsesSafeServiceUnavailableEnvelope(t *testing.T) {
	grpcRec := &recordingVoiceRooms{moveSelfErr: status.Error(codes.Unavailable, "redis retry diagnostics must not reach REST")}
	conn, cleanup := startBufconnVoiceConn(t, grpcRec)
	t.Cleanup(cleanup)
	tc := &transcoder{clients: grpcClients{voice: callsv1.NewVoiceServiceClient(conn)}}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/spaces/space-path/voice-rooms/source-path/move", strings.NewReader(`{"to_voice_room_id":"dest-path","operation_id":"op-path"}`))
	resp := httptest.NewRecorder()
	require.True(t, tc.serveSpacesVoiceRooms(resp, req, "space-path/voice-rooms/source-path/move"))
	require.Equal(t, http.StatusServiceUnavailable, resp.Code, "body=%s", resp.Body.String())
	var envelope struct {
		ErrorCode string `json:"error_code"`
		Message   string `json:"message"`
	}
	decodeJSON(t, resp.Body, &envelope)
	require.Equal(t, "unavailable", envelope.ErrorCode)
	require.Equal(t, "voice room roster unavailable", envelope.Message)
}

func TestTranscodeSpaceVoiceRoomModeratorMoveUnavailableUsesSafeServiceUnavailableEnvelope(t *testing.T) {
	grpcRec := &recordingVoiceRooms{moveParticipantErr: status.Error(codes.Unavailable, "redis retry diagnostics must not reach REST")}
	conn, cleanup := startBufconnVoiceConn(t, grpcRec)
	t.Cleanup(cleanup)
	tc := &transcoder{clients: grpcClients{voice: callsv1.NewVoiceServiceClient(conn)}}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/spaces/space-path/voice-rooms/source-path/participants/target-path/move", strings.NewReader(`{"to_voice_room_id":"dest-path","operation_id":"op-path"}`))
	resp := httptest.NewRecorder()
	require.True(t, tc.serveSpacesVoiceRooms(resp, req, "space-path/voice-rooms/source-path/participants/target-path/move"))
	require.Equal(t, http.StatusServiceUnavailable, resp.Code, "body=%s", resp.Body.String())
	var envelope struct {
		ErrorCode string `json:"error_code"`
		Message   string `json:"message"`
	}
	decodeJSON(t, resp.Body, &envelope)
	require.Equal(t, "unavailable", envelope.ErrorCode)
	require.Equal(t, "voice room roster unavailable", envelope.Message)
}
