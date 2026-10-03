package grpcsvc_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/bot/internal/store"
)

func seedSlashOutboxRow(t *testing.T) (context.Context, uuid.UUID, uuid.UUID, func(), *store.BotStore) {
	t.Helper()
	_, st, cleanup := startBotGRPC(t)
	ctx := context.Background()
	bot, _, err := st.CreateBot(ctx, uuid.New(), "LeaseBot", "", `["TEXT_CHAT_SEND_MESSAGES"]`, uuid.New())
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `UPDATE bots SET webhook_url = 'https://example.invalid/hook' WHERE id = $1`, bot.ID)
	require.NoError(t, err)
	id, err := st.EnqueueEvent(ctx, bot.ID, "interaction", map[string]any{"chat_id": uuid.NewString()}, uuid.NewString())
	require.NoError(t, err)
	return ctx, bot.ID, id, cleanup, st
}

func TestSlashOutboxStaleClaimCannotTransitionNewClaim(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx, _, id, cleanup, st := seedSlashOutboxRow(t)
	defer cleanup()
	first, err := st.ClaimSlashInteraction(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, first)
	_, err = st.Pool.Exec(ctx, `UPDATE bot_event_log SET claimed_until = now() - interval '1 second' WHERE id = $1`, id)
	require.NoError(t, err)
	accepted, err := st.CompleteSlashDelivery(ctx, first.ID, first.Attempts, false)
	require.NoError(t, err)
	require.False(t, accepted)
	second, err := st.ClaimSlashInteraction(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, second)
	require.Greater(t, second.Attempts, first.Attempts)

	accepted, err = st.CompleteSlashDelivery(ctx, first.ID, first.Attempts, false)
	require.NoError(t, err)
	require.False(t, accepted)
	accepted, err = st.RetrySlashDelivery(ctx, first.ID, first.Attempts)
	require.NoError(t, err)
	require.False(t, accepted)
	accepted, err = st.FailSlashDelivery(ctx, first.ID, first.Attempts)
	require.NoError(t, err)
	require.False(t, accepted)
	accepted, err = st.CompleteSlashDelivery(ctx, second.ID, second.Attempts, false)
	require.NoError(t, err)
	require.True(t, accepted)
}

func TestSlashOutboxRetryBudgetEndsInTerminalFailure(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx, _, id, cleanup, st := seedSlashOutboxRow(t)
	defer cleanup()
	for attempt := 1; attempt <= 8; attempt++ {
		claim, err := st.ClaimSlashInteraction(ctx, id)
		require.NoError(t, err)
		require.NotNil(t, claim)
		accepted, err := st.RetrySlashDelivery(ctx, claim.ID, claim.Attempts)
		require.NoError(t, err)
		require.True(t, accepted)
		_, err = st.Pool.Exec(ctx, `UPDATE bot_event_log SET next_attempt_at = now() WHERE id = $1`, id)
		require.NoError(t, err)
	}
	var state string
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT delivery_status FROM bot_event_log WHERE id = $1`, id).Scan(&state))
	require.Equal(t, "failed", state)
	claim, err := st.ClaimSlashInteraction(ctx, id)
	require.NoError(t, err)
	require.Nil(t, claim)
}

func TestSlashOutboxExpiresBeforeFirstClaim(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx, _, id, cleanup, st := seedSlashOutboxRow(t)
	defer cleanup()
	_, err := st.Pool.Exec(ctx, `UPDATE bot_event_log SET created_at = now() - interval '25 hours' WHERE id = $1`, id)
	require.NoError(t, err)
	claim, err := st.ClaimSlashInteraction(ctx, id)
	require.NoError(t, err)
	require.Nil(t, claim)
	var state string
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT delivery_status FROM bot_event_log WHERE id = $1`, id).Scan(&state))
	require.Equal(t, "failed", state)
}

func TestSlashOutboxExpiresWithoutCurrentWebhook(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx, botID, id, cleanup, st := seedSlashOutboxRow(t)
	defer cleanup()
	_, err := st.Pool.Exec(ctx, `UPDATE bots SET webhook_url = NULL WHERE id = $1`, botID)
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `UPDATE bot_event_log SET created_at = now() - interval '25 hours' WHERE id = $1`, id)
	require.NoError(t, err)
	claim, err := st.ClaimSlashInteraction(ctx, uuid.Nil)
	require.NoError(t, err)
	require.Nil(t, claim)
	var state string
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT delivery_status FROM bot_event_log WHERE id = $1`, id).Scan(&state))
	require.Equal(t, "failed", state)
}
