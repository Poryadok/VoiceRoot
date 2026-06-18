package grpcsvc_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	botv1 "voice.app/voice/bot/v1"
)

func TestRegisterBot_returnsWebhookSecretOnce(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startBotGRPC(t)
	defer cleanup()
	ctx := withAccount(context.Background(), uuid.New(), uuid.New())

	reg, err := client.RegisterBot(ctx, &botv1.RegisterBotRequest{
		Name: "SecretBot", ScopesJson: `["TEXT_CHAT_SEND_MESSAGES"]`,
	})
	require.NoError(t, err)
	require.NotEmpty(t, reg.GetTokenResponse().GetToken())
	require.NotEmpty(t, reg.GetWebhookSecretResponse().GetWebhookSecret())
	require.Len(t, reg.GetWebhookSecretResponse().GetWebhookSecret(), 64)

	got, err := client.GetBot(ctx, &botv1.GetBotRequest{BotId: reg.GetBot().GetId()})
	require.NoError(t, err)
	require.Empty(t, got.GetBot().GetWebhookUrl()) // secret never on Bot message
}

func TestRegenerateWebhookSecret_ownerOnly(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startBotGRPC(t)
	defer cleanup()
	owner := uuid.New()
	ctx := withAccount(context.Background(), owner, uuid.New())

	reg, err := client.RegisterBot(ctx, &botv1.RegisterBotRequest{
		Name: "RotateBot", ScopesJson: `["TEXT_CHAT_SEND_MESSAGES"]`,
	})
	require.NoError(t, err)
	botID := reg.GetBot().GetId()
	oldSecret := reg.GetWebhookSecretResponse().GetWebhookSecret()

	resp, err := client.RegenerateWebhookSecret(ctx, &botv1.RegenerateWebhookSecretRequest{BotId: botID})
	require.NoError(t, err)
	require.NotEmpty(t, resp.GetWebhookSecretResponse().GetWebhookSecret())
	require.NotEqual(t, oldSecret, resp.GetWebhookSecretResponse().GetWebhookSecret())

	otherCtx := withAccount(context.Background(), uuid.New(), uuid.New())
	_, err = client.RegenerateWebhookSecret(otherCtx, &botv1.RegenerateWebhookSecretRequest{BotId: botID})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}
