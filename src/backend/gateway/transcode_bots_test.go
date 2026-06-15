package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	botv1 "voice.app/voice/bot/v1"
)

type fakeBotClient struct {
	botv1.BotServiceClient
}

func (f *fakeBotClient) ExecuteSlashInteraction(ctx context.Context, in *botv1.ExecuteSlashInteractionRequest, _ ...grpc.CallOption) (*botv1.ExecuteSlashInteractionResponse, error) {
	content := "pong"
	return &botv1.ExecuteSlashInteractionResponse{Content: &content}, nil
}

func TestServeBots_interactionsRouteRegistered(t *testing.T) {
	tc := newTranscoder(&grpcClients{bot: &fakeBotClient{}})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/bots/interactions", http.NoBody)
	req.Header.Set("Authorization", "Bearer valid-user-token")
	req.Header.Set("X-Voice-User-Id", "00000000-0000-0000-0000-000000000001")
	req.Header.Set("X-Voice-Profile-Id", "00000000-0000-0000-0000-000000000002")
	rec := httptest.NewRecorder()
	ok := tc.serveBots(rec, req, "interactions")
	require.True(t, ok)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestBotBearerToken_parsesBotPrefix(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/bots/me/interactions/poll", nil)
	req.Header.Set("Authorization", "Bot vb_testtoken")
	require.Equal(t, "vb_testtoken", botBearerToken(req))
}

func TestIsBotTokenRESTRoute(t *testing.T) {
	require.True(t, isBotTokenRESTRoute("/api/v1/bots/me/interactions/poll"))
	require.False(t, isBotTokenRESTRoute("/api/v1/bots/interactions"))
}
