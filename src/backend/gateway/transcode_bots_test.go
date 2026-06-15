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

func (f *fakeBotClient) GetBot(ctx context.Context, in *botv1.GetBotRequest, _ ...grpc.CallOption) (*botv1.GetBotResponse, error) {
	return &botv1.GetBotResponse{Bot: &botv1.Bot{Id: in.GetBotId(), Name: "TestBot"}}, nil
}

func (f *fakeBotClient) UpdateBot(ctx context.Context, in *botv1.UpdateBotRequest, _ ...grpc.CallOption) (*botv1.UpdateBotResponse, error) {
	name := in.GetName()
	return &botv1.UpdateBotResponse{Bot: &botv1.Bot{Id: in.GetBotId(), Name: name}}, nil
}

func (f *fakeBotClient) DeleteBot(ctx context.Context, in *botv1.DeleteBotRequest, _ ...grpc.CallOption) (*botv1.DeleteBotResponse, error) {
	return &botv1.DeleteBotResponse{}, nil
}

func (f *fakeBotClient) GetWebhookURL(ctx context.Context, in *botv1.GetWebhookURLRequest, _ ...grpc.CallOption) (*botv1.GetWebhookURLResponse, error) {
	url := "https://example.com/hook"
	return &botv1.GetWebhookURLResponse{Url: &url}, nil
}

func (f *fakeBotClient) SetWebhookURL(ctx context.Context, in *botv1.SetWebhookURLRequest, _ ...grpc.CallOption) (*botv1.SetWebhookURLResponse, error) {
	return &botv1.SetWebhookURLResponse{}, nil
}

func (f *fakeBotClient) ListInstalledBots(ctx context.Context, in *botv1.ListInstalledBotsRequest, _ ...grpc.CallOption) (*botv1.ListInstalledBotsResponse, error) {
	return &botv1.ListInstalledBotsResponse{}, nil
}

func (f *fakeBotClient) UninstallBotFromSpace(ctx context.Context, in *botv1.UninstallBotFromSpaceRequest, _ ...grpc.CallOption) (*botv1.UninstallBotFromSpaceResponse, error) {
	return &botv1.UninstallBotFromSpaceResponse{}, nil
}

func (f *fakeBotClient) AutocompleteSlashOption(ctx context.Context, in *botv1.AutocompleteSlashOptionRequest, _ ...grpc.CallOption) (*botv1.AutocompleteSlashOptionResponse, error) {
	return &botv1.AutocompleteSlashOptionResponse{
		Choices: []*botv1.AutocompleteChoice{{Name: "CS2", Value: "cs2"}},
	}, nil
}

func (f *fakeBotClient) ListBotsInChat(ctx context.Context, in *botv1.ListBotsInChatRequest, _ ...grpc.CallOption) (*botv1.ListBotsInChatResponse, error) {
	return &botv1.ListBotsInChatResponse{
		Bots: []*botv1.ChatBotEntry{{
			Bot:         &botv1.Bot{Id: "bot-1", Name: "TestBot"},
			Enabled:     true,
			Whitelisted: true,
		}},
	}, nil
}

func (f *fakeBotClient) SetBotChatEnabled(ctx context.Context, in *botv1.SetBotChatEnabledRequest, _ ...grpc.CallOption) (*botv1.SetBotChatEnabledResponse, error) {
	return &botv1.SetBotChatEnabledResponse{}, nil
}

func (f *fakeBotClient) EditBotMessage(ctx context.Context, in *botv1.EditBotMessageRequest, _ ...grpc.CallOption) (*botv1.EditBotMessageResponse, error) {
	return &botv1.EditBotMessageResponse{}, nil
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
	require.True(t, isBotTokenRESTRoute("/api/v1/bots/me/messages/msg-1"))
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

func TestServeBots_getBotRouteRegistered(t *testing.T) {
	tc := newTranscoder(&grpcClients{bot: &fakeBotClient{}})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/bots/bot-1", http.NoBody)
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("X-Voice-User-Id", "00000000-0000-0000-0000-000000000001")
	req.Header.Set("X-Voice-Account-Id", "00000000-0000-0000-0000-000000000001")
	rec := httptest.NewRecorder()
	ok := tc.serveBots(rec, req, "bot-1")
	require.True(t, ok)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestServeBots_autocompleteRouteRegistered(t *testing.T) {
	tc := newTranscoder(&grpcClients{bot: &fakeBotClient{}})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/bots/autocomplete", http.NoBody)
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("X-Voice-User-Id", "00000000-0000-0000-0000-000000000001")
	req.Header.Set("X-Voice-Profile-Id", "00000000-0000-0000-0000-000000000002")
	rec := httptest.NewRecorder()
	ok := tc.serveBots(rec, req, "autocomplete")
	require.True(t, ok)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestServeBots_editMessageRouteRegistered(t *testing.T) {
	tc := newTranscoder(&grpcClients{bot: &fakeBotClient{}})
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/bots/me/messages/msg-42", http.NoBody)
	req.Header.Set("Authorization", "Bot vb_testtoken")
	rec := httptest.NewRecorder()
	ok := tc.serveBots(rec, req, "me/messages/msg-42")
	require.True(t, ok)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestServeBots_listInstalledBotsRouteRegistered(t *testing.T) {
	tc := newTranscoder(&grpcClients{bot: &fakeBotClient{}})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/bots/spaces/space-1/installed", http.NoBody)
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("X-Voice-User-Id", "00000000-0000-0000-0000-000000000001")
	rec := httptest.NewRecorder()
	ok := tc.serveBots(rec, req, "spaces/space-1/installed")
	require.True(t, ok)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestServeBots_webhookURLRoutesRegistered(t *testing.T) {
	tc := newTranscoder(&grpcClients{bot: &fakeBotClient{}})
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/bots/bot-1/webhook", http.NoBody)
	getReq.Header.Set("Authorization", "Bearer token")
	getReq.Header.Set("X-Voice-User-Id", "00000000-0000-0000-0000-000000000001")
	getReq.Header.Set("X-Voice-Account-Id", "00000000-0000-0000-0000-000000000001")
	getRec := httptest.NewRecorder()
	require.True(t, tc.serveBots(getRec, getReq, "bot-1/webhook"))
	require.Equal(t, http.StatusOK, getRec.Code)

	patchReq := httptest.NewRequest(http.MethodPatch, "/api/v1/bots/bot-1/webhook", http.NoBody)
	patchReq.Header.Set("Authorization", "Bearer token")
	patchReq.Header.Set("X-Voice-User-Id", "00000000-0000-0000-0000-000000000001")
	patchReq.Header.Set("X-Voice-Account-Id", "00000000-0000-0000-0000-000000000001")
	patchRec := httptest.NewRecorder()
	require.True(t, tc.serveBots(patchRec, patchReq, "bot-1/webhook"))
	require.Equal(t, http.StatusNoContent, patchRec.Code)
}

func TestServeBots_listBotsInChatRouteRegistered(t *testing.T) {
	tc := newTranscoder(&grpcClients{bot: &fakeBotClient{}})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/bots/chats/chat-1?space_id=space-1&chat_type=CHAT_TYPE_CHANNEL", http.NoBody)
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("X-Voice-User-Id", "00000000-0000-0000-0000-000000000001")
	req.Header.Set("X-Voice-Profile-Id", "00000000-0000-0000-0000-000000000002")
	rec := httptest.NewRecorder()
	ok := tc.serveBots(rec, req, "chats/chat-1")
	require.True(t, ok)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestServeBots_setBotChatEnabledRouteRegistered(t *testing.T) {
	tc := newTranscoder(&grpcClients{bot: &fakeBotClient{}})
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/bots/bot-1/chats/chat-1/enabled", http.NoBody)
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("X-Voice-User-Id", "00000000-0000-0000-0000-000000000001")
	req.Header.Set("X-Voice-Profile-Id", "00000000-0000-0000-0000-000000000002")
	rec := httptest.NewRecorder()
	ok := tc.serveBots(rec, req, "bot-1/chats/chat-1/enabled")
	require.True(t, ok)
	require.Equal(t, http.StatusNoContent, rec.Code)
}
