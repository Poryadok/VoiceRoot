package grpcsvc_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	botv1 "voice.app/voice/bot/v1"
	chatv1 "voice.app/voice/chat/v1"
)

func TestBotCRUD_and_webhookURL(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startBotGRPC(t)
	defer cleanup()
	ctx := withAccount(context.Background(), uuid.New(), uuid.New())

	reg, err := client.RegisterBot(ctx, &botv1.RegisterBotRequest{
		Name: "ApiBot", Description: "desc", ScopesJson: `["TEXT_CHAT_SEND_MESSAGES"]`,
	})
	require.NoError(t, err)
	botID := reg.GetBot().GetId()

	got, err := client.GetBot(ctx, &botv1.GetBotRequest{BotId: botID})
	require.NoError(t, err)
	require.Equal(t, "ApiBot", got.GetBot().GetName())

	updatedName := "ApiBot2"
	upd, err := client.UpdateBot(ctx, &botv1.UpdateBotRequest{BotId: botID, Name: &updatedName})
	require.NoError(t, err)
	require.Equal(t, "ApiBot2", upd.GetBot().GetName())

	_, err = client.SetWebhookURL(ctx, &botv1.SetWebhookURLRequest{
		BotId: botID,
		Url:   "https://example.com/hook",
	})
	require.NoError(t, err)
	wh, err := client.GetWebhookURL(ctx, &botv1.GetWebhookURLRequest{BotId: botID})
	require.NoError(t, err)
	require.Equal(t, "https://example.com/hook", wh.GetUrl())

	_, err = client.DeleteBot(ctx, &botv1.DeleteBotRequest{BotId: botID})
	require.NoError(t, err)
}

func TestApplyManifest_subcommandsListedWithGroup(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, st, cleanup := startBotGRPC(t)
	defer cleanup()
	ctx := withAccount(context.Background(), uuid.New(), uuid.New())
	profile, _ := authProfile(ctx)

	reg, err := client.RegisterBot(ctx, &botv1.RegisterBotRequest{
		Name: "QueueBot", ScopesJson: `["TEXT_CHAT_SEND_MESSAGES"]`,
	})
	require.NoError(t, err)
	botID := reg.GetBot().GetId()

	manifestYAML := `name: QueueBot
scopes: [TEXT_CHAT_SEND_MESSAGES]
commands:
  - name: queue
    description: Queue
    subcommands:
      - name: join
        description: Join
`
	_, err = client.ApplyManifest(ctx, &botv1.ApplyManifestRequest{BotId: botID, ManifestYaml: manifestYAML})
	require.NoError(t, err)

	chatID := uuid.New()
	botUUID, _ := uuid.Parse(botID)
	_, err = st.InstallInSpace(ctx, botUUID, uuid.New(), profile, []uuid.UUID{chatID})
	require.NoError(t, err)
	require.NoError(t, st.TouchPresence(ctx, botUUID))

	chatType := chatv1.ChatType_CHAT_TYPE_CHANNEL
	list, err := client.ListSlashCommandsForChat(ctx, &botv1.ListSlashCommandsForChatRequest{
		Chat: &chatv1.ChatRef{Id: chatID.String(), Type: &chatType},
	})
	require.NoError(t, err)
	require.Len(t, list.GetCommands(), 1)
	require.Equal(t, "join", list.GetCommands()[0].GetName())
	require.Equal(t, "queue", list.GetCommands()[0].GetGroupName())
}

func TestAutocompleteSlashOption_webhookChoices(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, st, cleanup := startBotGRPC(t)
	defer cleanup()
	ctx := withAccount(context.Background(), uuid.New(), uuid.New())
	profile, _ := authProfile(ctx)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		_ = json.NewDecoder(r.Body).Decode(&payload)
		require.Equal(t, "autocomplete", payload["type"])
		_, _ = w.Write([]byte(`{"choices":[{"name":"CS2","value":"cs2"}]}`))
	}))
	defer srv.Close()

	reg, err := client.RegisterBot(ctx, &botv1.RegisterBotRequest{
		Name: "StatsBot", ScopesJson: `["TEXT_CHAT_SEND_MESSAGES"]`,
	})
	require.NoError(t, err)
	botID := reg.GetBot().GetId()

	manifestYAML := `name: StatsBot
webhook_url: ` + srv.URL + `
scopes: [TEXT_CHAT_SEND_MESSAGES]
commands:
  - name: stats
    description: Stats
    options:
      - name: game
        type: string
        required: true
        autocomplete: true
`
	_, err = client.ApplyManifest(ctx, &botv1.ApplyManifestRequest{BotId: botID, ManifestYaml: manifestYAML})
	require.NoError(t, err)

	chatID := uuid.New()
	botUUID, _ := uuid.Parse(botID)
	_, err = st.InstallInSpace(ctx, botUUID, uuid.New(), profile, []uuid.UUID{chatID})
	require.NoError(t, err)
	require.NoError(t, st.TouchPresence(ctx, botUUID))

	chatType := chatv1.ChatType_CHAT_TYPE_CHANNEL
	resp, err := client.AutocompleteSlashOption(ctx, &botv1.AutocompleteSlashOptionRequest{
		Chat:         &chatv1.ChatRef{Id: chatID.String(), Type: &chatType},
		BotId:        botID,
		CommandName:  "stats",
		OptionName:   "game",
		FocusedValue: "cs",
	})
	require.NoError(t, err)
	require.Len(t, resp.GetChoices(), 1)
	require.Equal(t, "CS2", resp.GetChoices()[0].GetName())
}

func TestExecuteSlashInteraction_timeoutWritesEventLog(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, st, cleanup := startBotGRPC(t)
	defer cleanup()
	ctx := withAccount(context.Background(), uuid.New(), uuid.New())
	profile, _ := authProfile(ctx)

	reg, err := client.RegisterBot(ctx, &botv1.RegisterBotRequest{
		Name: "SlowBot", ScopesJson: `["TEXT_CHAT_SEND_MESSAGES"]`,
	})
	require.NoError(t, err)
	botID := reg.GetBot().GetId()
	botUUID, _ := uuid.Parse(botID)

	manifestYAML := `name: SlowBot
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
	require.NoError(t, st.TouchPresence(ctx, botUUID))

	chatType := chatv1.ChatType_CHAT_TYPE_CHANNEL
	_, err = client.ExecuteSlashInteraction(ctx, &botv1.ExecuteSlashInteractionRequest{
		Chat: &chatv1.ChatRef{Id: chatID.String(), Type: &chatType}, BotId: botID, CommandName: "ping",
	})
	require.NoError(t, err)

	var status string
	err = st.Pool.QueryRow(ctx, `
SELECT delivery_status FROM bot_event_log WHERE bot_id = $1 ORDER BY created_at DESC LIMIT 1`, botUUID).Scan(&status)
	require.NoError(t, err)
	require.Equal(t, "timeout", status)
}

func TestUninstallAndListInstalledBots(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, st, cleanup := startBotGRPC(t)
	defer cleanup()
	ctx := withAccount(context.Background(), uuid.New(), uuid.New())
	profile, _ := authProfile(ctx)

	reg, err := client.RegisterBot(ctx, &botv1.RegisterBotRequest{
		Name: "SpaceBot", ScopesJson: `["TEXT_CHAT_SEND_MESSAGES"]`,
	})
	require.NoError(t, err)
	botID := reg.GetBot().GetId()
	botUUID, _ := uuid.Parse(botID)
	spaceID := uuid.New()
	chatID := uuid.New()
	_, err = st.InstallInSpace(ctx, botUUID, spaceID, profile, []uuid.UUID{chatID})
	require.NoError(t, err)
	require.NoError(t, st.TouchPresence(ctx, botUUID))

	list, err := client.ListInstalledBots(ctx, &botv1.ListInstalledBotsRequest{SpaceId: spaceID.String()})
	require.NoError(t, err)
	require.Len(t, list.GetInstalledBots(), 1)

	_, err = client.UninstallBotFromSpace(ctx, &botv1.UninstallBotFromSpaceRequest{
		BotId: botID, SpaceId: spaceID.String(),
	})
	require.NoError(t, err)

	list, err = client.ListInstalledBots(ctx, &botv1.ListInstalledBotsRequest{SpaceId: spaceID.String()})
	require.NoError(t, err)
	require.Empty(t, list.GetInstalledBots())
}
