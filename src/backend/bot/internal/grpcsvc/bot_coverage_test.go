package grpcsvc_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	botv1 "voice.app/voice/bot/v1"
	chatv1 "voice.app/voice/chat/v1"
)

func TestRegisterAndGetCommands(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startBotGRPC(t)
	defer cleanup()
	ctx := withAccount(context.Background(), uuid.New(), uuid.New())

	reg, err := client.RegisterBot(ctx, &botv1.RegisterBotRequest{
		Name: "CmdBot", ScopesJson: `["TEXT_CHAT_SEND_MESSAGES"]`,
	})
	require.NoError(t, err)
	botID := reg.GetBot().GetId()

	_, err = client.RegisterCommands(ctx, &botv1.RegisterCommandsRequest{
		BotId: botID,
		CommandsJson: `[{"name":"ping","description":"ping","options":[]}]`,
	})
	require.NoError(t, err)

	got, err := client.GetCommands(ctx, &botv1.GetCommandsRequest{BotId: botID})
	require.NoError(t, err)
	require.Contains(t, got.GetCommandList().GetCommandsJson(), "ping")
}

func TestChatWhitelist_roundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, st, cleanup := startBotGRPC(t)
	defer cleanup()
	ctx := withAccount(context.Background(), uuid.New(), uuid.New())
	profile, _ := authProfile(ctx)

	reg, err := client.RegisterBot(ctx, &botv1.RegisterBotRequest{
		Name: "WhiteBot", ScopesJson: `["TEXT_CHAT_SEND_MESSAGES"]`,
	})
	require.NoError(t, err)
	botID := reg.GetBot().GetId()
	botUUID, _ := uuid.Parse(botID)
	spaceID := uuid.New()
	chatID := uuid.New()
	_, err = st.InstallInSpace(ctx, botUUID, spaceID, profile, []uuid.UUID{chatID})
	require.NoError(t, err)

	chatType := chatv1.ChatType_CHAT_TYPE_CHANNEL
	_, err = client.SetChatWhitelist(ctx, &botv1.SetChatWhitelistRequest{
		BotId: botID,
		AllowedChats: []*chatv1.ChatRef{
			{Id: chatID.String(), Type: &chatType},
		},
	})
	require.NoError(t, err)

	whitelist, err := client.GetChatWhitelist(ctx, &botv1.GetChatWhitelistRequest{BotId: botID})
	require.NoError(t, err)
	require.Len(t, whitelist.GetAllowedChats(), 1)
}

func TestExecuteSlashInteraction_webhookFailureMarksEventLog(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, st, cleanup := startBotGRPC(t)
	defer cleanup()
	ctx := withAccount(context.Background(), uuid.New(), uuid.New())
	profile, _ := authProfile(ctx)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	reg, err := client.RegisterBot(ctx, &botv1.RegisterBotRequest{
		Name: "FailBot", ScopesJson: `["TEXT_CHAT_SEND_MESSAGES"]`,
	})
	require.NoError(t, err)
	botID := reg.GetBot().GetId()
	botUUID, _ := uuid.Parse(botID)

	manifestYAML := `name: FailBot
webhook_url: ` + srv.URL + `
scopes: [TEXT_CHAT_SEND_MESSAGES]
commands:
  - name: ping
    description: ping
`
	_, err = client.ApplyManifest(ctx, &botv1.ApplyManifestRequest{BotId: botID, ManifestYaml: manifestYAML})
	require.NoError(t, err)

	chatID := uuid.New()
	_, err = st.InstallInSpace(ctx, botUUID, uuid.New(), profile, []uuid.UUID{chatID})
	require.NoError(t, err)

	chatType := chatv1.ChatType_CHAT_TYPE_CHANNEL
	_, err = client.ExecuteSlashInteraction(ctx, &botv1.ExecuteSlashInteractionRequest{
		Chat: &chatv1.ChatRef{Id: chatID.String(), Type: &chatType}, BotId: botID, CommandName: "ping",
	})
	require.Error(t, err)
	require.Equal(t, codes.Unavailable, status.Code(err))

	var deliveryStatus string
	err = st.Pool.QueryRow(ctx, `
SELECT delivery_status FROM bot_event_log WHERE bot_id = $1 ORDER BY created_at DESC LIMIT 1`, botUUID).Scan(&deliveryStatus)
	require.NoError(t, err)
	require.Equal(t, "failed", deliveryStatus)
}

func TestRegenerateToken(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startBotGRPC(t)
	defer cleanup()
	ctx := withAccount(context.Background(), uuid.New(), uuid.New())

	reg, err := client.RegisterBot(ctx, &botv1.RegisterBotRequest{
		Name: "TokenBot", ScopesJson: `["TEXT_CHAT_SEND_MESSAGES"]`,
	})
	require.NoError(t, err)

	resp, err := client.RegenerateToken(ctx, &botv1.RegenerateTokenRequest{BotId: reg.GetBot().GetId()})
	require.NoError(t, err)
	require.NotEmpty(t, resp.GetTokenResponse().GetToken())
}

func TestListBots_returnsOwnerBots(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startBotGRPC(t)
	defer cleanup()
	ctx := withAccount(context.Background(), uuid.New(), uuid.New())

	_, err := client.RegisterBot(ctx, &botv1.RegisterBotRequest{
		Name: "ListedBot", ScopesJson: `["TEXT_CHAT_SEND_MESSAGES"]`,
	})
	require.NoError(t, err)

	list, err := client.ListBots(ctx, &botv1.ListBotsRequest{})
	require.NoError(t, err)
	require.NotEmpty(t, list.GetBotList().GetBots())
}
