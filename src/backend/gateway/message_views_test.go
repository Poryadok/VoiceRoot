package main

import (
	"context"
	"github.com/stretchr/testify/require"
	"net/http"
	"testing"
	messagingv1 "voice.app/voice/messaging/v1"
)

type recordingMessageViews struct {
	messagingv1.UnimplementedMessagingServiceServer
	last *messagingv1.RecordMessageViewRequest
}

func (s *recordingMessageViews) RecordMessageView(_ context.Context, req *messagingv1.RecordMessageViewRequest) (*messagingv1.RecordMessageViewResponse, error) {
	s.last = req
	return &messagingv1.RecordMessageViewResponse{ViewCount: 7}, nil
}
func TestTranscodeRecordMessageView(t *testing.T) {
	server := &recordingMessageViews{}
	conn, cleanup := startBufconnMessagingConn(t, server)
	defer cleanup()
	h := newGatewayForContract(t, gatewayTestOptions{tokenClaims: map[string]tokenClaims{"valid-user-token": {UserID: "account", ProfileID: "profile"}}, transcoder: &transcoder{clients: grpcClients{messaging: messagingv1.NewMessagingServiceClient(conn)}}})
	response := performRequest(h, http.MethodPost, "/api/v1/messages/views", `{"chat_id":"chat","message_id":"message"}`, map[string]string{"Authorization": "Bearer valid-user-token", "Content-Type": "application/json"})
	require.Equal(t, http.StatusOK, response.Code)
	require.NotNil(t, server.last)
	require.Equal(t, "chat", server.last.ChatId)
	require.Equal(t, "message", server.last.MessageId)
	require.Contains(t, response.Body.String(), `"7"`)
}
