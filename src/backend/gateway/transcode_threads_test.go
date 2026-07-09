package main

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	chatv1 "voice.app/voice/chat/v1"
	messagingv1 "voice.app/voice/messaging/v1"
)

// TestTranscodeMessagesGetThreadMessages documents roles/threads (docs/features/roles.md):
// GET /api/v1/messages/thread → MessagingService.GetThreadMessages.
func TestTranscodeMessagesGetThreadMessages(t *testing.T) {
	t.Parallel()

	grpcRec := &recordingMessagesThreads{}
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

	resp := performRequest(h, http.MethodGet, "/api/v1/messages/thread?chat_id=chat-1&thread_parent_id=parent-1", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusOK, resp.Code, "body=%s", resp.Body.String())
	require.False(t, proxyCalled)
	require.NotNil(t, grpcRec.lastGetThread)
	require.Equal(t, "chat-1", grpcRec.lastGetThread.GetChat().GetId())
	require.Equal(t, "parent-1", grpcRec.lastGetThread.GetThreadParentId())
}

type recordingMessagesThreads struct {
	messagingv1.UnimplementedMessagingServiceServer
	lastGetThread *messagingv1.GetThreadMessagesRequest
}

func (s *recordingMessagesThreads) GetThreadMessages(_ context.Context, req *messagingv1.GetThreadMessagesRequest) (*messagingv1.GetThreadMessagesResponse, error) {
	s.lastGetThread = req
	dm := chatv1.ChatType_CHAT_TYPE_DM
	return &messagingv1.GetThreadMessagesResponse{
		MessageList: &messagingv1.MessageList{
			Messages: []*messagingv1.Message{
				{
					Id:             "reply-1",
					Chat:           &chatv1.ChatRef{Id: req.GetChat().GetId(), Type: &dm},
					SenderProfileId: "profile-2",
					Content:        "thread body",
					ThreadParentId: &req.ThreadParentId,
				},
			},
		},
	}, nil
}
