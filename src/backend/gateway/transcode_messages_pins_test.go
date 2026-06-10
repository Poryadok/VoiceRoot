package main

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	chatv1 "voice.app/voice/chat/v1"
	messagingv1 "voice.app/voice/messaging/v1"
)

func TestTranscodeMessagesPinMessage(t *testing.T) {
	t.Parallel()

	grpcRec := &recordingMessagesPins{}
	conn, cleanup := startBufconnMessagingConn(t, grpcRec)
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{messaging: messagingv1.NewMessagingServiceClient(conn)}},
	})

	body := `{"chat":{"id":"chat-1"}}`
	resp := performRequest(h, http.MethodPost, "/api/v1/messages/msg-1/pin", body, map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusNoContent, resp.Code, "body=%s", resp.Body.String())
	require.NotNil(t, grpcRec.lastPin)
	require.Equal(t, "msg-1", grpcRec.lastPin.GetMessageId())
	require.Equal(t, "chat-1", grpcRec.lastPin.GetChat().GetId())
}

func TestTranscodeMessagesUnpinMessage(t *testing.T) {
	t.Parallel()

	grpcRec := &recordingMessagesPins{}
	conn, cleanup := startBufconnMessagingConn(t, grpcRec)
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{messaging: messagingv1.NewMessagingServiceClient(conn)}},
	})

	resp := performRequest(h, http.MethodDelete, "/api/v1/messages/msg-2/pin?chat_id=chat-2", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusNoContent, resp.Code, "body=%s", resp.Body.String())
	require.NotNil(t, grpcRec.lastUnpin)
	require.Equal(t, "msg-2", grpcRec.lastUnpin.GetMessageId())
	require.Equal(t, "chat-2", grpcRec.lastUnpin.GetChat().GetId())
}

func TestTranscodeChatsGetPinnedMessages(t *testing.T) {
	t.Parallel()

	grpcRec := &recordingMessagesPins{}
	msgConn, msgCleanup := startBufconnMessagingConn(t, grpcRec)
	t.Cleanup(msgCleanup)
	chatConn, chatCleanup := startBufconnChatConn(t, &chatv1.UnimplementedChatServiceServer{})
	t.Cleanup(chatCleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{
			messaging: messagingv1.NewMessagingServiceClient(msgConn),
			chat:      chatv1.NewChatServiceClient(chatConn),
		}},
	})

	resp := performRequest(h, http.MethodGet, "/api/v1/chats/chat-3/pinned-messages", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusOK, resp.Code, "body=%s", resp.Body.String())
	require.NotNil(t, grpcRec.lastGetPinned)
	require.Equal(t, "chat-3", grpcRec.lastGetPinned.GetChat().GetId())
}

type recordingMessagesPins struct {
	messagingv1.UnimplementedMessagingServiceServer
	lastPin       *messagingv1.PinMessageRequest
	lastUnpin     *messagingv1.UnpinMessageRequest
	lastGetPinned *messagingv1.GetPinnedMessagesRequest
}

func (s *recordingMessagesPins) PinMessage(_ context.Context, req *messagingv1.PinMessageRequest) (*messagingv1.PinMessageResponse, error) {
	s.lastPin = req
	return &messagingv1.PinMessageResponse{}, nil
}

func (s *recordingMessagesPins) UnpinMessage(_ context.Context, req *messagingv1.UnpinMessageRequest) (*messagingv1.UnpinMessageResponse, error) {
	s.lastUnpin = req
	return &messagingv1.UnpinMessageResponse{}, nil
}

func (s *recordingMessagesPins) GetPinnedMessages(_ context.Context, req *messagingv1.GetPinnedMessagesRequest) (*messagingv1.GetPinnedMessagesResponse, error) {
	s.lastGetPinned = req
	return &messagingv1.GetPinnedMessagesResponse{}, nil
}
