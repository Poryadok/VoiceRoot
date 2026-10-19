package store_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

func uninstallDown(t *testing.T) string {
	t.Helper()
	body, err := os.ReadFile("../../../migrations/bot_db/000012_uninstall_cleanup.down.sql")
	require.NoError(t, err)
	return string(body)
}

func TestUninstallCleanupDownIsolationAndRetainedFirstWriter(t *testing.T) {
	if testing.Short() {
		t.Skip("actual PostgreSQL migration isolation/first writer")
	}
	t.Run("empty_default_RC", func(t *testing.T) {
		st := startBotStore(t)
		_, err := st.Pool.Exec(t.Context(), uninstallDown(t))
		require.NoError(t, err)
	})
	t.Run("old_RR", func(t *testing.T) {
		st := startBotStore(t)
		ctx := t.Context()
		bot, _, err := st.CreateBot(ctx, uuid.New(), "Uninstall old snapshot", "", `[]`, uuid.New())
		require.NoError(t, err)
		intent, err := st.PrepareInstallation(ctx, bot.ID, uuid.New(), uuid.New(), nil)
		require.NoError(t, err)
		old, err := st.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
		require.NoError(t, err)
		defer func() { _ = old.Rollback(context.Background()) }()
		var n int
		require.NoError(t, old.QueryRow(ctx, `SELECT count(*) FROM bot_uninstall_operations`).Scan(&n))
		require.Zero(t, n)
		// Storage-only bytes exercise retained-row protection, not authority minting.
		_, err = st.Pool.Exec(ctx, `INSERT INTO bot_uninstall_operations(operation_id,bot_id,installation_id,owner_account_id,actor_profile_id,decision_bytes,decision_sha256,state) VALUES($1,$2,$3,$4,$5,$6,$7,'PENDING')`, uuid.New(), bot.ID, intent.InstallationID, bot.OwnerAccountID, bot.ActorProfileID, []byte("retained-storage-fixture"), make([]byte, 32))
		require.NoError(t, err)
		_, err = old.Exec(ctx, uninstallDown(t))
		require.ErrorContains(t, err, "READ COMMITTED")
		require.NoError(t, old.Rollback(ctx))
		require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM bot_uninstall_operations`).Scan(&n))
		require.Equal(t, 1, n)
	})
	t.Run("first_writer", func(t *testing.T) {
		st := startBotStore(t)
		ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
		defer cancel()
		bot, _, err := st.CreateBot(ctx, uuid.New(), "Uninstall first writer", "", `[]`, uuid.New())
		require.NoError(t, err)
		intent, err := st.PrepareInstallation(ctx, bot.ID, uuid.New(), uuid.New(), nil)
		require.NoError(t, err)
		writer, err := st.Pool.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = writer.Rollback(context.Background()) }()
		operation := uuid.New()
		_, err = writer.Exec(ctx, `INSERT INTO bot_uninstall_operations(operation_id,bot_id,installation_id,owner_account_id,actor_profile_id,decision_bytes,decision_sha256,state) VALUES($1,$2,$3,$4,$5,$6,$7,'PENDING')`, operation, bot.ID, intent.InstallationID, bot.OwnerAccountID, bot.ActorProfileID, []byte("retained-storage-fixture"), make([]byte, 32))
		require.NoError(t, err)
		conn, err := st.Pool.Acquire(ctx)
		require.NoError(t, err)
		defer conn.Release()
		var pid int32
		require.NoError(t, conn.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid))
		down := uninstallDown(t)
		done := make(chan error, 1)
		go func() { _, e := conn.Exec(ctx, down); done <- e }()
		require.Eventually(t, func() bool {
			var wait bool
			e := st.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE pid=$1 AND wait_event_type='Lock')`, pid).Scan(&wait)
			return e == nil && wait
		}, 5*time.Second, 10*time.Millisecond)
		require.NoError(t, writer.Commit(ctx))
		require.ErrorContains(t, <-done, "must not be dropped")
		_, err = conn.Exec(ctx, `ROLLBACK`)
		require.NoError(t, err)
		var retained uuid.UUID
		require.NoError(t, st.Pool.QueryRow(ctx, `SELECT operation_id FROM bot_uninstall_operations`).Scan(&retained))
		require.Equal(t, operation, retained)
	})
}
