package store_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestCreateBot_returnsWebhookSecretOnce(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	st := startBotStore(t)

	row, plainToken, err := st.CreateBot(ctx, uuid.New(), "HookBot", "desc", `["TEXT_CHAT_SEND_MESSAGES"]`, uuid.New())
	require.NoError(t, err)
	require.NotEmpty(t, plainToken)
	require.NotEmpty(t, row.WebhookSecret)
	require.Len(t, row.WebhookSecret, 64)
}

func TestRegenerateWebhookSecret_rotatesSecret(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	st := startBotStore(t)

	row, _, err := st.CreateBot(ctx, uuid.New(), "RotateBot", "", `["TEXT_CHAT_SEND_MESSAGES"]`, uuid.New())
	require.NoError(t, err)
	oldSecret := row.WebhookSecret

	newSecret, err := st.RegenerateWebhookSecret(ctx, row.ID)
	require.NoError(t, err)
	require.NotEmpty(t, newSecret)
	require.NotEqual(t, oldSecret, newSecret)

	got, err := st.GetBotByID(ctx, row.ID)
	require.NoError(t, err)
	require.Equal(t, newSecret, got.WebhookSecret)
}
