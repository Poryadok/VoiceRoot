package main

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	chatv1 "voice.app/voice/chat/v1"
	messagingv1 "voice.app/voice/messaging/v1"
)

func TestTranscodeChatsListSharedMedia(t *testing.T) {
	t.Parallel()

	grpcRec := &recordingSharedMedia{}
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

	resp := performRequest(h, http.MethodGet, "/api/v1/chats/chat-9/shared-media?kind=media", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusOK, resp.Code, "body=%s", resp.Body.String())
	require.NotNil(t, grpcRec.lastListShared)
	require.Equal(t, "chat-9", grpcRec.lastListShared.GetChat().GetId())
	require.Equal(t, messagingv1.SharedMediaKind_SHARED_MEDIA_KIND_MEDIA, grpcRec.lastListShared.GetKind())
}

func TestTranscodeChatsListSharedMedia_invalidKind(t *testing.T) {
	t.Parallel()

	msgConn, msgCleanup := startBufconnMessagingConn(t, &recordingSharedMedia{})
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

	resp := performRequest(h, http.MethodGet, "/api/v1/chats/chat-9/shared-media?kind=unknown", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusBadRequest, resp.Code, "body=%s", resp.Body.String())
}

type recordingSharedMedia struct {
	messagingv1.UnimplementedMessagingServiceServer
	lastListShared *messagingv1.ListSharedMediaRequest
}

func (s *recordingSharedMedia) ListSharedMedia(_ context.Context, req *messagingv1.ListSharedMediaRequest) (*messagingv1.ListSharedMediaResponse, error) {
	s.lastListShared = req
	return &messagingv1.ListSharedMediaResponse{}, nil
}
