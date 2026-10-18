package store_test

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestCurrentInstallationScope_RevocationAndReinstallNeverReuseAuthority(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx := context.Background()
	st := startBotLifecycleStore(t)
	actor, space := uuid.New(), uuid.New()
	bot, _, err := st.CreateBot(ctx, uuid.New(), "Scope decision", "", `["TEXT_CHAT_CREATE_IN_SPACE"]`, actor)
	require.NoError(t, err)
	installation, err := st.InstallInSpace(ctx, bot.ID, space, actor, nil)
	require.NoError(t, err)
	check := func(actorID, installationID uuid.UUID, want bool) {
		t.Helper()
		allowed, err := st.CheckCurrentInstallationScope(ctx, bot.ID, space, actorID, installationID, "TEXT_CHAT_CREATE_IN_SPACE")
		require.NoError(t, err)
		require.Equal(t, want, allowed)
	}
	check(actor, installation, true)
	check(uuid.New(), installation, false)
	check(actor, uuid.New(), false)
	_, err = st.Pool.Exec(ctx, `UPDATE bots SET scopes='[]'::jsonb WHERE id=$1`, bot.ID)
	require.NoError(t, err)
	check(actor, installation, false)
	_, err = st.Pool.Exec(ctx, `UPDATE bots SET scopes='["TEXT_CHAT_CREATE_IN_SPACE"]'::jsonb,status='disabled' WHERE id=$1`, bot.ID)
	require.NoError(t, err)
	check(actor, installation, false)
	_, err = st.Pool.Exec(ctx, `UPDATE bots SET status='live' WHERE id=$1`, bot.ID)
	require.NoError(t, err)
	check(actor, installation, true)
	require.NoError(t, st.UninstallFromSpace(ctx, bot.ID, space))
	check(actor, installation, false)
	replacement, err := st.InstallInSpace(ctx, bot.ID, space, actor, nil)
	require.NoError(t, err)
	require.NotEqual(t, installation, replacement)
	check(actor, installation, false)
	check(actor, replacement, true)
}
