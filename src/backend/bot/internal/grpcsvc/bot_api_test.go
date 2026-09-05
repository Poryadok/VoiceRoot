package grpcsvc_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

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
	updatedScopes := `["DM_SEND","TEXT_CHAT_SEND_MESSAGES"]`

	reg, err := client.RegisterBot(ctx, &botv1.RegisterBotRequest{
		Name: "ApiBot", Description: "desc", ScopesJson: updatedScopes,
	})
	require.NoError(t, err)
	botID := reg.GetBot().GetId()

	got, err := client.GetBot(ctx, &botv1.GetBotRequest{BotId: botID})
	require.NoError(t, err)
	require.Equal(t, "ApiBot", got.GetBot().GetName())

	updatedName := "ApiBot2"
	updatedAvatar := "https://example.com/avatar.png"
	requestedScopes := `["TEXT_CHAT_SEND_MESSAGES","DM_SEND"]`
	upd, err := client.UpdateBot(ctx, &botv1.UpdateBotRequest{
		BotId: botID, Name: &updatedName, AvatarUrl: &updatedAvatar, ScopesJson: &requestedScopes,
	})
	require.NoError(t, err)
	require.Equal(t, "ApiBot2", upd.GetBot().GetName())
	require.Equal(t, updatedAvatar, upd.GetBot().GetAvatarUrl())
	require.JSONEq(t, updatedScopes, upd.GetBot().GetScopesJson())

	removedScopes := `["TEXT_CHAT_SEND_MESSAGES"]`
	removed, err := client.UpdateBot(ctx, &botv1.UpdateBotRequest{
		BotId: botID, ScopesJson: &removedScopes,
	})
	require.NoError(t, err)
	require.JSONEq(t, removedScopes, removed.GetBot().GetScopesJson())

	escalatedScopes := `["TEXT_CHAT_SEND_MESSAGES","SPACE_VIEW_MEMBER_LIST"]`
	_, err = client.UpdateBot(ctx, &botv1.UpdateBotRequest{
		BotId: botID, ScopesJson: &escalatedScopes,
	})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	unchanged, err := client.GetBot(ctx, &botv1.GetBotRequest{BotId: botID})
	require.NoError(t, err)
	require.JSONEq(t, removedScopes, unchanged.GetBot().GetScopesJson())

	for _, invalidScopes := range []string{
		`not-json`,
		`["TEXT_CHAT_SEND_MESSAGES","UNKNOWN_SCOPE"]`,
		`["TEXT_CHAT_SEND_MESSAGES","TEXT_CHAT_SEND_MESSAGES"]`,
	} {
		_, err = client.UpdateBot(ctx, &botv1.UpdateBotRequest{
			BotId: botID, ScopesJson: &invalidScopes,
		})
		require.Equal(t, codes.InvalidArgument, status.Code(err), invalidScopes)
		unchanged, err = client.GetBot(ctx, &botv1.GetBotRequest{BotId: botID})
		require.NoError(t, err)
		require.JSONEq(t, removedScopes, unchanged.GetBot().GetScopesJson())
	}

	nameOnly := "ApiBot3"
	preserved, err := client.UpdateBot(ctx, &botv1.UpdateBotRequest{BotId: botID, Name: &nameOnly})
	require.NoError(t, err)
	require.Equal(t, nameOnly, preserved.GetBot().GetName())
	require.Equal(t, updatedAvatar, preserved.GetBot().GetAvatarUrl())
	require.JSONEq(t, removedScopes, preserved.GetBot().GetScopesJson())
	require.Equal(t, "desc", preserved.GetBot().GetDescription())

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

func TestBotUpdateScopes_rechecksAfterConcurrentRemoval(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, st, cleanup := startBotGRPC(t)
	defer cleanup()
	ctx := withAccount(context.Background(), uuid.New(), uuid.New())
	granted := `["DM_SEND","TEXT_CHAT_SEND_MESSAGES"]`

	reg, err := client.RegisterBot(ctx, &botv1.RegisterBotRequest{
		Name: "ConcurrentBot", ScopesJson: granted,
	})
	require.NoError(t, err)
	botID := reg.GetBot().GetId()
	botUUID, err := uuid.Parse(botID)
	require.NoError(t, err)

	tx, err := st.Pool.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx) //nolint:errcheck // commit is asserted below
	removed := `["TEXT_CHAT_SEND_MESSAGES"]`
	_, err = tx.Exec(ctx, `UPDATE bots SET scopes = $2::jsonb WHERE id = $1`, botUUID, removed)
	require.NoError(t, err)

	restore := granted
	updateDone := make(chan error, 1)
	go func() {
		_, updateErr := client.UpdateBot(ctx, &botv1.UpdateBotRequest{
			BotId: botID, ScopesJson: &restore,
		})
		updateDone <- updateErr
	}()

	// The writer must be inside its second transaction before the removal commits;
	// otherwise an old read-then-write implementation could race past this test.
	require.Eventually(t, func() bool {
		return st.Pool.Stat().AcquiredConns() >= 2
	}, 2*time.Second, 10*time.Millisecond)
	select {
	case updateErr := <-updateDone:
		t.Fatalf("update completed before concurrent removal committed: %v", updateErr)
	default:
	}
	require.NoError(t, tx.Commit(ctx))

	updateErr := <-updateDone
	require.Equal(t, codes.PermissionDenied, status.Code(updateErr))
	got, err := client.GetBot(ctx, &botv1.GetBotRequest{BotId: botID})
	require.NoError(t, err)
	require.JSONEq(t, removed, got.GetBot().GetScopesJson())
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
