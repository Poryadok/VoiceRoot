package main

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	messagingv1 "voice.app/voice/messaging/v1"
)

// TestTranscodeMessagesAddReaction documents PLAN Phase 4 / text-chat.md:
// POST /api/v1/messages/{message_id}/reactions → MessagingService.AddReaction.
func TestTranscodeMessagesAddReaction(t *testing.T) {
	t.Parallel()

	grpcRec := &recordingMessagesReactions{}
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

	body := `{"emoji":"👍"}`
	resp := performRequest(h, http.MethodPost, "/api/v1/messages/msg-1/reactions", body, map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusNoContent, resp.Code, "body=%s", resp.Body.String())
	require.False(t, proxyCalled)
	require.NotNil(t, grpcRec.lastAdd)
	require.Equal(t, "msg-1", grpcRec.lastAdd.GetMessageId())
	require.Equal(t, "👍", grpcRec.lastAdd.GetEmoji())
}

// TestTranscodeMessagesRemoveReaction documents PLAN Phase 4:
// DELETE /api/v1/messages/{message_id}/reactions?emoji=… → MessagingService.RemoveReaction.
func TestTranscodeMessagesRemoveReaction(t *testing.T) {
	t.Parallel()

	grpcRec := &recordingMessagesReactions{}
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

	resp := performRequest(h, http.MethodDelete, "/api/v1/messages/msg-2/reactions?emoji=%F0%9F%91%8D", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusNoContent, resp.Code, "body=%s", resp.Body.String())
	require.False(t, proxyCalled)
	require.NotNil(t, grpcRec.lastRemove)
	require.Equal(t, "msg-2", grpcRec.lastRemove.GetMessageId())
	require.Equal(t, "👍", grpcRec.lastRemove.GetEmoji())
}

type recordingMessagesReactions struct {
	messagingv1.UnimplementedMessagingServiceServer
	lastAdd    *messagingv1.AddReactionRequest
	lastRemove *messagingv1.RemoveReactionRequest
}

func (s *recordingMessagesReactions) AddReaction(_ context.Context, req *messagingv1.AddReactionRequest) (*messagingv1.AddReactionResponse, error) {
	s.lastAdd = req
	return &messagingv1.AddReactionResponse{}, nil
}

func (s *recordingMessagesReactions) RemoveReaction(_ context.Context, req *messagingv1.RemoveReactionRequest) (*messagingv1.RemoveReactionResponse, error) {
	s.lastRemove = req
	return &messagingv1.RemoveReactionResponse{}, nil
}
