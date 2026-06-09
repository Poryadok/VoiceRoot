package main

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	callsv1 "voice.app/voice/calls/v1"
	chatv1 "voice.app/voice/chat/v1"
)

// TestTranscodeVoiceStartGroupVoice documents PLAN Phase 4: POST /api/v1/voice/calls with group_voice.
func TestTranscodeVoiceStartGroupVoice(t *testing.T) {
	t.Parallel()

	grpcRec := &recordingVoiceCalls{}
	conn, cleanup := startBufconnVoiceConn(t, grpcRec)
	t.Cleanup(cleanup)

	proxyCalled := false
	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{voice: callsv1.NewVoiceServiceClient(conn)}},
		restUpstreams: map[string]http.Handler{
			"voice": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				proxyCalled = true
				w.WriteHeader(http.StatusAccepted)
			}),
		},
	})

	body := `{"room_type_enum":"VOICE_SESSION_KIND_GROUP_VOICE","linked_chat":{"id":"group-1","type":"CHAT_TYPE_GROUP"},"media_kind":"audio"}`
	resp := performRequest(h, http.MethodPost, "/api/v1/voice/calls", body, map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusOK, resp.Code, "body=%s", resp.Body.String())
	require.False(t, proxyCalled)
	require.NotNil(t, grpcRec.start)
	require.Equal(t, callsv1.VoiceSessionKind_VOICE_SESSION_KIND_GROUP_VOICE, grpcRec.start.GetRoomTypeEnum())
	require.Equal(t, "group-1", grpcRec.start.GetLinkedChat().GetId())
	require.Equal(t, chatv1.ChatType_CHAT_TYPE_GROUP, grpcRec.start.GetLinkedChat().GetType())
	require.Empty(t, grpcRec.start.GetCalleeProfileId())
}

// TestTranscodeVoiceJoinGroupCall documents PLAN Phase 4: POST /api/v1/voice/calls/{room_id}/join.
func TestTranscodeVoiceJoinGroupCall(t *testing.T) {
	t.Parallel()

	grpcRec := &recordingVoiceJoin{}
	conn, cleanup := startBufconnVoiceConn(t, grpcRec)
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-2"},
		},
		transcoder: &transcoder{clients: grpcClients{voice: callsv1.NewVoiceServiceClient(conn)}},
	})

	resp := performRequest(h, http.MethodPost, "/api/v1/voice/calls/room-group-1/join", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusOK, resp.Code, "body=%s", resp.Body.String())
	require.Equal(t, "room-group-1", grpcRec.roomID)
}

type recordingVoiceJoin struct {
	callsv1.UnimplementedVoiceServiceServer
	roomID string
}

func (s *recordingVoiceJoin) JoinCall(_ context.Context, req *callsv1.JoinCallRequest) (*callsv1.JoinCallResponse, error) {
	s.roomID = req.GetRoomId()
	group := chatv1.ChatType_CHAT_TYPE_GROUP
	return &callsv1.JoinCallResponse{
		CallSession: &callsv1.CallSession{
			RoomId:          req.GetRoomId(),
			LivekitRoomName: "voice-group-" + req.GetRoomId(),
			RoomTypeEnum:    callsv1.VoiceSessionKind_VOICE_SESSION_KIND_GROUP_VOICE.Enum(),
			LinkedChat:      &chatv1.ChatRef{Id: "group-1", Type: &group},
			Status:          callsv1.CallStatus_CALL_STATUS_ACTIVE,
			StartedAt:       timestamppb.New(time.Unix(1700000100, 0)),
		},
	}, nil
}
