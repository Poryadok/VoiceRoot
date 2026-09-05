package grpcsvc_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	botv1 "voice.app/voice/bot/v1"
)

func TestApplyManifest_rejectsInstalledBotScopeEscalationWithoutMutation(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	for _, privilegedScope := range []string{
		"TEXT_CHAT_READ_HISTORY",
		"SPACE_MANAGE_ROLES",
	} {
		t.Run(privilegedScope, func(t *testing.T) {
			client, st, cleanup := startBotGRPC(t)
			defer cleanup()

			ctx := withAccount(context.Background(), uuid.New(), uuid.New())
			reg, err := client.RegisterBot(ctx, &botv1.RegisterBotRequest{
				Name: "SendOnlyBot", ScopesJson: `["TEXT_CHAT_SEND_MESSAGES"]`,
			})
			require.NoError(t, err)
			botID := reg.GetBot().GetId()
			botUUID := uuid.MustParse(botID)

			_, err = client.ApplyManifest(ctx, &botv1.ApplyManifestRequest{
				BotId: botID,
				ManifestYaml: `name: SendOnlyBot
description: original description
icon_url: https://example.com/original.png
webhook_url: https://example.com/original
scopes: [TEXT_CHAT_SEND_MESSAGES]
commands:
  - name: ping
    description: original command
`,
			})
			require.NoError(t, err)

			spaceID := uuid.New()
			chatID := uuid.New()
			profileID := uuid.New()
			_, err = st.InstallInSpace(ctx, botUUID, spaceID, profileID, []uuid.UUID{chatID})
			require.NoError(t, err)
			before, err := st.GetBotByID(ctx, botUUID)
			require.NoError(t, err)
			beforeCommands, err := st.ListCommands(ctx, botUUID)
			require.NoError(t, err)
			beforeWhitelist, err := st.ListWhitelistedChatIDs(ctx, botUUID, spaceID)
			require.NoError(t, err)

			_, err = client.ApplyManifest(ctx, &botv1.ApplyManifestRequest{
				BotId: botID,
				ManifestYaml: `name: EscalatedBot
description: changed description
icon_url: https://example.com/changed.png
webhook_url: https://example.com/changed
scopes: [TEXT_CHAT_SEND_MESSAGES, ` + privilegedScope + `]
commands:
  - name: changed
    description: must not persist
`,
			})
			require.Equal(t, codes.PermissionDenied, status.Code(err))

			stored, err := st.GetBotByID(ctx, botUUID)
			require.NoError(t, err)
			require.Equal(t, before.Name, stored.Name)
			require.Equal(t, before.Description, stored.Description)
			require.Equal(t, before.AvatarURL, stored.AvatarURL)
			require.Equal(t, before.WebhookURL, stored.WebhookURL)
			require.Equal(t, before.IsPollingMode, stored.IsPollingMode)
			require.JSONEq(t, before.ScopesJSON, stored.ScopesJSON)

			commands, err := st.ListCommands(ctx, botUUID)
			require.NoError(t, err)
			require.Equal(t, beforeCommands, commands)

			whitelisted, err := st.ListWhitelistedChatIDs(ctx, botUUID, spaceID)
			require.NoError(t, err)
			require.Equal(t, beforeWhitelist, whitelisted)
		})
	}
}

func TestApplyManifest_allowsScopeReductionAndNonScopeEdits(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	client, st, cleanup := startBotGRPC(t)
	defer cleanup()

	ctx := withAccount(context.Background(), uuid.New(), uuid.New())
	reg, err := client.RegisterBot(ctx, &botv1.RegisterBotRequest{
		Name: "ReducibleBot", ScopesJson: `["DM_SEND","TEXT_CHAT_SEND_MESSAGES"]`,
	})
	require.NoError(t, err)
	botID := reg.GetBot().GetId()

	_, err = client.ApplyManifest(ctx, &botv1.ApplyManifestRequest{
		BotId: botID,
		ManifestYaml: `name: OriginalBot
description: original description
scopes: [DM_SEND, TEXT_CHAT_SEND_MESSAGES]
commands:
  - name: original
    description: original command
`,
	})
	require.NoError(t, err)

	botUUID := uuid.MustParse(botID)
	spaceID := uuid.New()
	chatID := uuid.New()
	_, err = st.InstallInSpace(ctx, botUUID, spaceID, uuid.New(), []uuid.UUID{chatID})
	require.NoError(t, err)

	_, err = client.ApplyManifest(ctx, &botv1.ApplyManifestRequest{
		BotId: botID,
		ManifestYaml: `name: EditedBot
description: updated without escalation
scopes: [DM_SEND, TEXT_CHAT_SEND_MESSAGES]
commands:
  - name: updated
    description: updated command
`,
	})
	require.NoError(t, err)

	stored, err := st.GetBotByID(ctx, botUUID)
	require.NoError(t, err)
	require.Equal(t, "EditedBot", stored.Name)
	require.Equal(t, "updated without escalation", stored.Description)
	require.JSONEq(t, `["DM_SEND","TEXT_CHAT_SEND_MESSAGES"]`, stored.ScopesJSON)

	commands, err := st.ListCommands(ctx, botUUID)
	require.NoError(t, err)
	require.Len(t, commands, 1)
	require.Equal(t, "updated", commands[0].Name)

	_, err = client.ApplyManifest(ctx, &botv1.ApplyManifestRequest{
		BotId: botID,
		ManifestYaml: `name: EditedBot
description: updated without escalation
scopes: [TEXT_CHAT_SEND_MESSAGES]
commands:
  - name: updated
    description: updated command
`,
	})
	require.NoError(t, err)

	stored, err = st.GetBotByID(ctx, botUUID)
	require.NoError(t, err)
	require.JSONEq(t, `["TEXT_CHAT_SEND_MESSAGES"]`, stored.ScopesJSON)
}

func TestApplyManifest_rechecksScopesAfterConcurrentReduction(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	client, st, cleanup := startBotGRPC(t)
	defer cleanup()

	ctx := withAccount(context.Background(), uuid.New(), uuid.New())
	reg, err := client.RegisterBot(ctx, &botv1.RegisterBotRequest{
		Name: "ConcurrentBot", ScopesJson: `["DM_SEND","TEXT_CHAT_SEND_MESSAGES"]`,
	})
	require.NoError(t, err)
	botID := reg.GetBot().GetId()
	botUUID := uuid.MustParse(botID)

	_, err = client.ApplyManifest(ctx, &botv1.ApplyManifestRequest{
		BotId: botID,
		ManifestYaml: `name: ConcurrentBot
description: original description
scopes: [DM_SEND, TEXT_CHAT_SEND_MESSAGES]
commands:
  - name: original
    description: original command
`,
	})
	require.NoError(t, err)
	_, err = st.InstallInSpace(ctx, botUUID, uuid.New(), uuid.New(), []uuid.UUID{uuid.New()})
	require.NoError(t, err)

	tx, err := st.Pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	var lockedID uuid.UUID
	require.NoError(t, tx.QueryRow(ctx, `SELECT id FROM bots WHERE id = $1 FOR UPDATE`, botUUID).Scan(&lockedID))

	errCh := make(chan error, 1)
	go func() {
		_, err := client.ApplyManifest(ctx, &botv1.ApplyManifestRequest{
			BotId: botID,
			ManifestYaml: `name: ConcurrentlyEscalatedBot
description: changed description
scopes: [DM_SEND, TEXT_CHAT_SEND_MESSAGES]
commands:
  - name: changed
    description: must not persist
`,
		})
		errCh <- err
	}()

	require.Eventually(t, func() bool {
		var waiting bool
		err := st.Pool.QueryRow(ctx, `
SELECT EXISTS (
  SELECT 1 FROM pg_stat_activity
  WHERE query LIKE '%UPDATE bots SET name%' AND wait_event_type = 'Lock'
)`).Scan(&waiting)
		return err == nil && waiting
	}, time.Second, 10*time.Millisecond, "ApplyManifest must be blocked on the bot row")

	_, err = tx.Exec(ctx, `UPDATE bots SET scopes = $2::jsonb WHERE id = $1`, botUUID, `["TEXT_CHAT_SEND_MESSAGES"]`)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))

	err = <-errCh
	require.Equal(t, codes.PermissionDenied, status.Code(err))

	stored, err := st.GetBotByID(ctx, botUUID)
	require.NoError(t, err)
	require.Equal(t, "ConcurrentBot", stored.Name)
	require.Equal(t, "original description", stored.Description)
	require.JSONEq(t, `["TEXT_CHAT_SEND_MESSAGES"]`, stored.ScopesJSON)
	commands, err := st.ListCommands(ctx, botUUID)
	require.NoError(t, err)
	require.Len(t, commands, 1)
	require.Equal(t, "original", commands[0].Name)
}
