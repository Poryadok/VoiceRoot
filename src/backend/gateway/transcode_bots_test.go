package main

import (
	"context"
	"io"
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

func (f *fakeBotClient) DeferResponse(ctx context.Context, in *botv1.DeferResponseRequest, _ ...grpc.CallOption) (*botv1.DeferResponseResponse, error) {
	return &botv1.DeferResponseResponse{}, nil
}

func (f *fakeBotClient) PollEvents(ctx context.Context, in *botv1.PollEventsRequest, _ ...grpc.CallOption) (botv1.BotService_PollEventsClient, error) {
	return &fakeBotPollStream{}, nil
}

type fakeBotPollStream struct {
	grpc.ClientStream
}

func (f *fakeBotPollStream) Recv() (*botv1.PollEventsResponse, error) {
	return nil, io.EOF
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
	require.True(t, isBotTokenRESTRoute("/api/v1/bots/me/interactions/defer"))
	require.False(t, isBotTokenRESTRoute("/api/v1/bots/interactions"))
}

func TestServeBots_deferRouteRegistered(t *testing.T) {
	tc := newTranscoder(&grpcClients{bot: &fakeBotClient{}})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/bots/me/interactions/defer", http.NoBody)
	req.Header.Set("Authorization", "Bot vb_testtoken")
	rec := httptest.NewRecorder()
	ok := tc.serveBots(rec, req, "me/interactions/defer")
	require.True(t, ok)
	require.Equal(t, http.StatusNoContent, rec.Code)
}
