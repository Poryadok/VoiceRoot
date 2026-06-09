package main

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	messagingv1 "voice.app/voice/messaging/v1"
)

// TestTranscodeMessagesForward documents PLAN Phase 4 / forward-messages.md:
// POST /api/v1/messages/forward → MessagingService.ForwardMessage.
func TestTranscodeMessagesForward(t *testing.T) {
	t.Parallel()

	grpcRec := &recordingMessagesForward{}
	conn, cleanup := startBufconnMessagingConn(t, grpcRec)
	t.Cleanup(cleanup)

	proxyCalled := false
	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{messaging: messagingv1.NewMessagingServiceClient(conn)}},
		restUpstreams: map[string]http.Handler{
			"messages": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				proxyCalled = true
				w.WriteHeader(http.StatusAccepted)
			}),
		},
	})

	body := `{"source_message_id":"msg-src-1","target_chat":{"id":"chat-target-1","type":"CHAT_TYPE_DM"}}`
	resp := performRequest(h, http.MethodPost, "/api/v1/messages/forward", body, map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusOK, resp.Code, "body=%s", resp.Body.String())
	require.False(t, proxyCalled)
	require.NotNil(t, grpcRec.last)
	require.Equal(t, "msg-src-1", grpcRec.last.GetSourceMessageId())
	require.Equal(t, "chat-target-1", grpcRec.last.GetTargetChat().GetId())
}

type recordingMessagesForward struct {
	messagingv1.UnimplementedMessagingServiceServer
	last *messagingv1.ForwardMessageRequest
}

func (s *recordingMessagesForward) ForwardMessage(_ context.Context, req *messagingv1.ForwardMessageRequest) (*messagingv1.ForwardMessageResponse, error) {
	s.last = req
	kind := messagingv1.MessageKind_MESSAGE_KIND_FORWARD
	sender := "Alice"
	return &messagingv1.ForwardMessageResponse{
		Message: &messagingv1.Message{
			Id:                "msg-fwd-1",
			Chat:              req.GetTargetChat(),
			SenderProfileId:   "profile-1",
			Content:           "hello",
			Type:              "forward",
			MessageKind:       &kind,
			ForwardFromId:     &req.SourceMessageId,
			ForwardFromSender: &sender,
		},
	}, nil
}
