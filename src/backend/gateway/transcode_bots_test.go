package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	botv1 "voice.app/voice/bot/v1"
	chatv1 "voice.app/voice/chat/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	rolev1 "voice.app/voice/role/v1"
)

type fakeBotClient struct {
	botv1.BotServiceClient
	lastInstallReq      *botv1.InstallBotInSpaceRequest
	chatMessageBodies   map[string]string // msg id -> content; Phase 16 maps to response messages[]
}

func (f *fakeBotClient) InstallBotInSpace(ctx context.Context, in *botv1.InstallBotInSpaceRequest, _ ...grpc.CallOption) (*botv1.InstallBotInSpaceResponse, error) {
	f.lastInstallReq = in
	return &botv1.InstallBotInSpaceResponse{InstallationId: "inst-1"}, nil
}

func (f *fakeBotClient) ListSlashCommandsForChat(ctx context.Context, in *botv1.ListSlashCommandsForChatRequest, _ ...grpc.CallOption) (*botv1.ListSlashCommandsForChatResponse, error) {
	online := false
	return &botv1.ListSlashCommandsForChatResponse{
		Commands: []*botv1.SlashCommand{{
			Name:        "ping",
			Description: "Ping",
			BotId:       "bot-1",
			Online:      online,
		}},
	}, nil
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

func (f *fakeBotClient) GetBotBySlug(ctx context.Context, in *botv1.GetBotBySlugRequest, _ ...grpc.CallOption) (*botv1.GetBotResponse, error) {
	slug := in.GetSlug()
	return &botv1.GetBotResponse{Bot: &botv1.Bot{Id: "bot-1", Name: "TestBot", Slug: &slug}}, nil
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

func (f *fakeBotClient) TouchPresence(ctx context.Context, in *botv1.TouchPresenceRequest, _ ...grpc.CallOption) (*botv1.TouchPresenceResponse, error) {
	return &botv1.TouchPresenceResponse{}, nil
}

func (f *fakeBotClient) ListSpaceMembersForBot(ctx context.Context, in *botv1.ListSpaceMembersForBotRequest, _ ...grpc.CallOption) (*botv1.ListSpaceMembersForBotResponse, error) {
	return &botv1.ListSpaceMembersForBotResponse{ProfileIds: []string{"profile-1"}}, nil
}

func (f *fakeBotClient) AssignBotRole(ctx context.Context, in *botv1.AssignBotRoleRequest, _ ...grpc.CallOption) (*botv1.AssignBotRoleResponse, error) {
	return &botv1.AssignBotRoleResponse{}, nil
}

func (f *fakeBotClient) RevokeBotRole(ctx context.Context, in *botv1.RevokeBotRoleRequest, _ ...grpc.CallOption) (*botv1.RevokeBotRoleResponse, error) {
	return &botv1.RevokeBotRoleResponse{}, nil
}

func (f *fakeBotClient) CreateBotChat(ctx context.Context, in *botv1.CreateBotChatRequest, _ ...grpc.CallOption) (*botv1.CreateBotChatResponse, error) {
	chatType := chatv1.ChatType_CHAT_TYPE_CHANNEL
	return &botv1.CreateBotChatResponse{
		Chat: &chatv1.ChatRef{Id: "chat-new", Type: &chatType},
	}, nil
}

func (f *fakeBotClient) GetChatMessagesForBot(ctx context.Context, in *botv1.GetChatMessagesForBotRequest, _ ...grpc.CallOption) (*botv1.GetChatMessagesForBotResponse, error) {
	_ = ctx
	_ = in
	bodies := f.chatMessageBodies
	if bodies == nil {
		bodies = map[string]string{"msg-1": "hello"}
	}
	ids := make([]string, 0, len(bodies))
	msgs := make([]*messagingv1.Message, 0, len(bodies))
	for id, content := range bodies {
		ids = append(ids, id)
		msgs = append(msgs, &messagingv1.Message{Id: id, Content: content})
	}
	return &botv1.GetChatMessagesForBotResponse{MessageIds: ids, Messages: msgs}, nil
}

func (f *fakeBotClient) SendEphemeral(ctx context.Context, in *botv1.SendEphemeralRequest, _ ...grpc.CallOption) (*botv1.SendEphemeralResponse, error) {
	_ = ctx
	_ = in
	return &botv1.SendEphemeralResponse{}, nil
}

func (f *fakeBotClient) CreateBotRole(ctx context.Context, in *botv1.CreateBotRoleRequest, _ ...grpc.CallOption) (*botv1.CreateBotRoleResponse, error) {
	return &botv1.CreateBotRoleResponse{Role: &rolev1.Role{Id: "role-new", Name: in.GetName()}}, nil
}

func (f *fakeBotClient) CompleteAutocomplete(ctx context.Context, in *botv1.CompleteAutocompleteRequest, _ ...grpc.CallOption) (*botv1.CompleteAutocompleteResponse, error) {
	return &botv1.CompleteAutocompleteResponse{}, nil
}

func (f *fakeBotClient) RegisterBot(ctx context.Context, in *botv1.RegisterBotRequest, _ ...grpc.CallOption) (*botv1.RegisterBotResponse, error) {
	_ = ctx
	_ = in
	return &botv1.RegisterBotResponse{
		Bot:                   &botv1.Bot{Id: "bot-new", Name: in.GetName()},
		TokenResponse:         &botv1.TokenResponse{Token: "tok-plain"},
		WebhookSecretResponse: &botv1.WebhookSecretResponse{WebhookSecret: "whsec-plain"},
	}, nil
}

func (f *fakeBotClient) RegenerateWebhookSecret(ctx context.Context, in *botv1.RegenerateWebhookSecretRequest, _ ...grpc.CallOption) (*botv1.RegenerateWebhookSecretResponse, error) {
	_ = ctx
	return &botv1.RegenerateWebhookSecretResponse{
		WebhookSecretResponse: &botv1.WebhookSecretResponse{WebhookSecret: "whsec-rotated"},
	}, nil
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
	require.True(t, isBotTokenRESTRoute("/api/v1/bots/me/messages/ephemeral"))
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

func TestServeBots_installPassesAcknowledgePrivilegedScopes(t *testing.T) {
	bot := &fakeBotClient{}
	tc := newTranscoder(&grpcClients{bot: bot})
	body := `{"allowed_chats":[{"id":"chat-1","type":"CHAT_TYPE_GROUP"}],"acknowledge_privileged_scopes":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/bots/bot-1/spaces/space-1/install", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("X-Voice-User-Id", "00000000-0000-0000-0000-000000000001")
	req.Header.Set("X-Voice-Profile-Id", "00000000-0000-0000-0000-000000000002")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ok := tc.serveBots(rec, req, "bot-1/spaces/space-1/install")
	require.True(t, ok)
	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, bot.lastInstallReq)
	require.True(t, bot.lastInstallReq.GetAcknowledgePrivilegedScopes(),
		"install JSON must map acknowledge_privileged_scopes to gRPC request (BOT-C)")
}

func TestServeBots_listCommandsIncludesOnlineField(t *testing.T) {
	tc := newTranscoder(&grpcClients{bot: &fakeBotClient{}})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/bots/commands?chat_id=chat-1&chat_type=CHAT_TYPE_CHANNEL", nil)
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("X-Voice-User-Id", "00000000-0000-0000-0000-000000000001")
	req.Header.Set("X-Voice-Profile-Id", "00000000-0000-0000-0000-000000000002")
	rec := httptest.NewRecorder()
	ok := tc.serveBots(rec, req, "commands")
	require.True(t, ok)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"online"`, "commands response must include online field (BOT-C)")
	require.Contains(t, rec.Body.String(), `"online":false`)
}

func TestServeBots_touchPresenceRouteRegistered(t *testing.T) {
	tc := newTranscoder(&grpcClients{bot: &fakeBotClient{}})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/bots/me/presence", http.NoBody)
	req.Header.Set("Authorization", "Bot vb_testtoken")
	rec := httptest.NewRecorder()
	ok := tc.serveBots(rec, req, "me/presence")
	require.True(t, ok, "POST /api/v1/bots/me/presence must be registered (BOT-C)")
	require.Equal(t, http.StatusNoContent, rec.Code)
}

func TestServeBots_listSpaceMembersRouteRegistered(t *testing.T) {
	tc := newTranscoder(&grpcClients{bot: &fakeBotClient{}})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/bots/me/spaces/space-1/members", http.NoBody)
	req.Header.Set("Authorization", "Bot vb_testtoken")
	rec := httptest.NewRecorder()
	ok := tc.serveBots(rec, req, "me/spaces/space-1/members")
	require.True(t, ok, "GET /api/v1/bots/me/spaces/{space_id}/members must be registered (BOT-C)")
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestServeBots_assignBotRoleRouteRegistered(t *testing.T) {
	tc := newTranscoder(&grpcClients{bot: &fakeBotClient{}})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/bots/me/spaces/space-1/roles/assign", strings.NewReader(`{"profile_id":"p-1","role_id":"r-1"}`))
	req.Header.Set("Authorization", "Bot vb_testtoken")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ok := tc.serveBots(rec, req, "me/spaces/space-1/roles/assign")
	require.True(t, ok, "POST /api/v1/bots/me/spaces/{space_id}/roles/assign must be registered (BOT-C)")
	require.Equal(t, http.StatusNoContent, rec.Code)
}

func TestServeBots_revokeBotRoleRouteRegistered(t *testing.T) {
	tc := newTranscoder(&grpcClients{bot: &fakeBotClient{}})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/bots/me/spaces/space-1/roles/revoke", strings.NewReader(`{"profile_id":"p-1","role_id":"r-1"}`))
	req.Header.Set("Authorization", "Bot vb_testtoken")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ok := tc.serveBots(rec, req, "me/spaces/space-1/roles/revoke")
	require.True(t, ok, "POST /api/v1/bots/me/spaces/{space_id}/roles/revoke must be registered (BOT-C)")
	require.Equal(t, http.StatusNoContent, rec.Code)
}

func TestServeBots_createBotChatRouteRegistered(t *testing.T) {
	tc := newTranscoder(&grpcClients{bot: &fakeBotClient{}})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/bots/me/chats", strings.NewReader(`{"space_id":"space-1","name":"audit","chat_type":"channel"}`))
	req.Header.Set("Authorization", "Bot vb_testtoken")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ok := tc.serveBots(rec, req, "me/chats")
	require.True(t, ok, "POST /api/v1/bots/me/chats must be registered (BOT-C)")
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestServeBots_getChatMessagesForBotRouteRegistered(t *testing.T) {
	tc := newTranscoder(&grpcClients{bot: &fakeBotClient{}})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/bots/me/chats/chat-1/messages?chat_type=CHAT_TYPE_CHANNEL", http.NoBody)
	req.Header.Set("Authorization", "Bot vb_testtoken")
	rec := httptest.NewRecorder()
	ok := tc.serveBots(rec, req, "me/chats/chat-1/messages")
	require.True(t, ok, "GET /api/v1/bots/me/chats/{chat_id}/messages must be registered (BOT-C)")
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"messages"`, "history response must include message bodies (Phase 16)")
	require.Contains(t, rec.Body.String(), `"content":"hello"`)
}

func TestServeBots_sendEphemeralRouteRegistered(t *testing.T) {
	tc := newTranscoder(&grpcClients{bot: &fakeBotClient{}})
	body := `{"chat":{"id":"chat-1","type":"CHAT_TYPE_CHANNEL"},"target_profile_id":"profile-1","content":"only you"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/bots/me/messages/ephemeral", strings.NewReader(body))
	req.Header.Set("Authorization", "Bot vb_testtoken")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ok := tc.serveBots(rec, req, "me/messages/ephemeral")
	require.True(t, ok, "POST /api/v1/bots/me/messages/ephemeral must be registered (Phase 16)")
	require.Equal(t, http.StatusNoContent, rec.Code)
}

func TestServeBots_createBotRoleRouteRegistered(t *testing.T) {
	tc := newTranscoder(&grpcClients{bot: &fakeBotClient{}})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/bots/me/roles", strings.NewReader(`{"space_id":"space-1","name":"Helper","permissions_mask":1,"position":2}`))
	req.Header.Set("Authorization", "Bot vb_testtoken")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ok := tc.serveBots(rec, req, "me/roles")
	require.True(t, ok, "POST /api/v1/bots/me/roles must be registered (BOT-C)")
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestServeBots_completeAutocompleteRouteRegistered(t *testing.T) {
	tc := newTranscoder(&grpcClients{bot: &fakeBotClient{}})
	body := `{"request_id":"ac-req-1","choices":[{"name":"CS2","value":"cs2"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/bots/me/autocomplete/complete", strings.NewReader(body))
	req.Header.Set("Authorization", "Bot vb_testtoken")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ok := tc.serveBots(rec, req, "me/autocomplete/complete")
	require.True(t, ok, "POST /api/v1/bots/me/autocomplete/complete must be registered (BOT-C)")
	require.Equal(t, http.StatusNoContent, rec.Code)
}

func TestServeBots_getBotBySlugRouteRegistered(t *testing.T) {
	bot := &fakeBotClient{}
	tc := newTranscoder(&grpcClients{bot: bot})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/bots/slug/ping-bot", http.NoBody)
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("X-Voice-User-Id", "00000000-0000-0000-0000-000000000001")
	req.Header.Set("X-Voice-Account-Id", "00000000-0000-0000-0000-000000000001")
	rec := httptest.NewRecorder()
	ok := tc.serveBots(rec, req, "slug/ping-bot")
	require.True(t, ok, "GET /api/v1/bots/slug/{slug} must be registered (BOT-B)")
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"slug"`, "GetBotBySlug response must include slug field")
}

func TestServeBots_registerBot_returnsWebhookSecret(t *testing.T) {
	tc := newTranscoder(&grpcClients{bot: &fakeBotClient{}})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/bots", strings.NewReader(`{"name":"DevBot","scopes_json":"[\"TEXT_CHAT_SEND_MESSAGES\"]"}`))
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("X-Voice-User-Id", "00000000-0000-0000-0000-000000000001")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ok := tc.serveBots(rec, req, "")
	require.True(t, ok)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "webhook_secret")
	require.Contains(t, rec.Body.String(), "whsec-plain")
}

func TestServeBots_regenerateWebhookSecretRouteRegistered(t *testing.T) {
	tc := newTranscoder(&grpcClients{bot: &fakeBotClient{}})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/bots/bot-1/webhook-secret/regenerate", http.NoBody)
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("X-Voice-User-Id", "00000000-0000-0000-0000-000000000001")
	rec := httptest.NewRecorder()
	ok := tc.serveBots(rec, req, "bot-1/webhook-secret/regenerate")
	require.True(t, ok, "POST /api/v1/bots/{id}/webhook-secret/regenerate must be registered")
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "whsec-rotated")
}
