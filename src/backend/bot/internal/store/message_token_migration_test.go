package store_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestMessageTokenMigrationPreservesEventsAndRealInteractionUniqueness(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx := context.Background()
	st := startBotLifecycleStore(t)
	bot, _, err := st.CreateBot(ctx, uuid.New(), "Token Migration", "", `[]`, uuid.New())
	require.NoError(t, err)
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	dir := filepath.Join(filepath.Dir(file), "..", "..", "..", "migrations", "bot_db")
	down, err := os.ReadFile(filepath.Join(dir, "000007_message_token_uniqueness.down.sql"))
	require.NoError(t, err)
	up, err := os.ReadFile(filepath.Join(dir, "000007_message_token_uniqueness.up.sql"))
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, string(down))
	require.NoError(t, err)
	insert := func(kind, token string) error {
		_, err := st.Pool.Exec(ctx, `INSERT INTO bot_event_log(id,bot_id,event_type,interaction_token) VALUES($1,$2,$3,$4)`, uuid.New(), bot.ID, kind, token)
		return err
	}
	require.NoError(t, insert("message", ""))
	_, err = st.Pool.Exec(ctx, string(up))
	require.NoError(t, err)
	require.NoError(t, insert("message", ""), "legacy blank-token messages remain independent")
	require.NoError(t, insert("interaction", "real-token"))
	require.Error(t, insert("interaction", "real-token"), "real slash identity still deduplicates")
	conn, err := st.Pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()
	_, err = conn.Exec(ctx, string(down))
	require.Error(t, err, "unsafe downgrade must refuse without deleting messages")
	_, err = conn.Exec(ctx, "ROLLBACK")
	require.NoError(t, err)
	var count int
	require.NoError(t, conn.QueryRow(ctx, `SELECT count(*) FROM bot_event_log WHERE bot_id=$1`, bot.ID).Scan(&count))
	require.Equal(t, 3, count)
	require.Error(t, insert("interaction", "real-token"), "failed downgrade retains real-token constraint")
	require.NoError(t, insert("message", ""), "failed downgrade retains message-compatible constraint")
}
