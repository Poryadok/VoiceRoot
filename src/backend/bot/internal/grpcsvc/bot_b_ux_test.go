package grpcsvc_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	botv1 "voice.app/voice/bot/v1"
	chatv1 "voice.app/voice/chat/v1"
)

func TestSetBotChatEnabled_filtersSlashCommands(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, st, cleanup := startBotGRPC(t)
	defer cleanup()
	ctx := withAccount(context.Background(), uuid.New(), uuid.New())
	profile, _ := authProfile(ctx)

	reg, err := client.RegisterBot(ctx, &botv1.RegisterBotRequest{
		Name: "ToggleBot", ScopesJson: `["TEXT_CHAT_SEND_MESSAGES"]`,
	})
	require.NoError(t, err)
	botID := reg.GetBot().GetId()

	manifestYAML := `name: ToggleBot
scopes: [TEXT_CHAT_SEND_MESSAGES]
commands:
  - name: ping
    description: Ping
`
	_, err = client.ApplyManifest(ctx, &botv1.ApplyManifestRequest{BotId: botID, ManifestYaml: manifestYAML})
	require.NoError(t, err)

	spaceID := uuid.New()
	chatID := uuid.New()
	botUUID, _ := uuid.Parse(botID)
	_, err = st.InstallInSpace(ctx, botUUID, spaceID, profile, []uuid.UUID{chatID})
	require.NoError(t, err)

	chatType := chatv1.ChatType_CHAT_TYPE_CHANNEL
	chatRef := &chatv1.ChatRef{Id: chatID.String(), Type: &chatType}

	list, err := client.ListSlashCommandsForChat(ctx, &botv1.ListSlashCommandsForChatRequest{Chat: chatRef})
	require.NoError(t, err)
	require.Len(t, list.GetCommands(), 1)
	require.True(t, list.GetCommands()[0].GetOnline())

	_, err = client.SetBotChatEnabled(ctx, &botv1.SetBotChatEnabledRequest{
		BotId:   botID,
		Chat:    chatRef,
		SpaceId: spaceID.String(),
		Enabled: false,
	})
	require.NoError(t, err)

	list, err = client.ListSlashCommandsForChat(ctx, &botv1.ListSlashCommandsForChatRequest{Chat: chatRef})
	require.NoError(t, err)
	require.Empty(t, list.GetCommands())

	inChat, err := client.ListBotsInChat(ctx, &botv1.ListBotsInChatRequest{
		Chat:    chatRef,
		SpaceId: spaceID.String(),
	})
	require.NoError(t, err)
	require.Len(t, inChat.GetBots(), 1)
	require.False(t, inChat.GetBots()[0].GetEnabled())
	require.True(t, inChat.GetBots()[0].GetWhitelisted())
}

func TestUninstallFromSpace_clearsWhitelist(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, st, cleanup := startBotGRPC(t)
	defer cleanup()
	ctx := withAccount(context.Background(), uuid.New(), uuid.New())
	profile, _ := authProfile(ctx)

	reg, err := client.RegisterBot(ctx, &botv1.RegisterBotRequest{
		Name: "UninstallBot", ScopesJson: `["TEXT_CHAT_SEND_MESSAGES"]`,
	})
	require.NoError(t, err)
	botID := reg.GetBot().GetId()

	spaceID := uuid.New()
	chatID := uuid.New()
	botUUID, _ := uuid.Parse(botID)
	_, err = st.InstallInSpace(ctx, botUUID, spaceID, profile, []uuid.UUID{chatID})
	require.NoError(t, err)

	_, err = client.UninstallBotFromSpace(ctx, &botv1.UninstallBotFromSpaceRequest{
		BotId:   botID,
		SpaceId: spaceID.String(),
	})
	require.NoError(t, err)

	allowed, err := st.IsChatWhitelisted(ctx, botUUID, chatID)
	require.NoError(t, err)
	require.False(t, allowed)
}

func TestSetBotChatEnabled_enablesWhitelistForNewChat(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, st, cleanup := startBotGRPC(t)
	defer cleanup()
	ctx := withAccount(context.Background(), uuid.New(), uuid.New())
	profile, _ := authProfile(ctx)

	reg, err := client.RegisterBot(ctx, &botv1.RegisterBotRequest{
		Name: "EnableBot", ScopesJson: `["TEXT_CHAT_SEND_MESSAGES"]`,
	})
	require.NoError(t, err)
	botID := reg.GetBot().GetId()

	spaceID := uuid.New()
	chatA := uuid.New()
	chatB := uuid.New()
	botUUID, _ := uuid.Parse(botID)
	_, err = st.InstallInSpace(ctx, botUUID, spaceID, profile, []uuid.UUID{chatA})
	require.NoError(t, err)

	chatType := chatv1.ChatType_CHAT_TYPE_GROUP
	_, err = client.SetBotChatEnabled(ctx, &botv1.SetBotChatEnabledRequest{
		BotId:   botID,
		Chat:    &chatv1.ChatRef{Id: chatB.String(), Type: &chatType},
		SpaceId: spaceID.String(),
		Enabled: true,
	})
	require.NoError(t, err)

	allowed, err := st.IsChatWhitelisted(ctx, botUUID, chatB)
	require.NoError(t, err)
	require.True(t, allowed)
}
