package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestConcurrentFirstChatEnableSucceedsAfterInvisibleRecipientInsert(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	st := startBotLifecycleStore(t)
	owner, space, chat := uuid.New(), uuid.New(), uuid.New()
	bot, _, err := st.CreateBot(ctx, owner, "Concurrent Enable", "", `[]`, uuid.New())
	require.NoError(t, err)
	_, err = st.InstallInSpace(ctx, bot.ID, space, owner, nil)
	require.NoError(t, err)
	blocker, err := st.Pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = blocker.Rollback(context.Background()) }()
	_, err = blocker.Exec(ctx, `INSERT INTO bot_chat_whitelist(bot_id,chat_id,space_id,enabled,added_by_profile_id) VALUES($1,$2,$3,false,$4)`, bot.ID, chat, space, owner)
	require.NoError(t, err)
	done := make(chan error, 2)
	for range 2 {
		go func() { done <- st.SetChatEnabled(ctx, bot.ID, chat, space, owner, true) }()
	}
	// Both requests observed absence and reached INSERT while the initializer
	// remains invisible. This establishes the race without scheduler sleeps.
	require.Eventually(t, func() bool {
		var waiting int
		err := st.Pool.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE '%INSERT INTO bot_chat_whitelist%'`).Scan(&waiting)
		return err == nil && waiting == 2
	}, 5*time.Second, 10*time.Millisecond)
	require.NoError(t, blocker.Commit(ctx))
	for range 2 {
		require.NoError(t, <-done)
	}
	var enabled bool
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT enabled FROM bot_chat_whitelist WHERE bot_id=$1 AND chat_id=$2 AND space_id=$3`, bot.ID, chat, space).Scan(&enabled))
	require.True(t, enabled)
}
