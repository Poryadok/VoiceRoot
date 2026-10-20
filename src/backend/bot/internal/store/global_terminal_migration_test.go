package store_test

import (
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"voice/backend/bot/internal/store"
	"voice/backend/pkg/integrationtest"
)

func globalTerminalMigration(t *testing.T, direction string) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	b, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "..", "migrations", "bot_db", "000014_global_terminal_obligations."+direction+".sql"))
	require.NoError(t, err)
	return string(b)
}

func TestBotGlobalTerminalMigrationPreservesOldDecisionAndReopensMissingProof(t *testing.T) {
	if testing.Short() {
		t.Skip("real PostgreSQL forward journal migration")
	}
	ctx := t.Context()
	pool := integrationtest.StartPostgres(t, ctx, "botdb", "")
	up := globalTerminalMigration(t, "up")
	beforeHandoff := strings.TrimSuffix(migrationSQL(t), purgeHandoffMigration(t, "up")+"\n")
	_, err := pool.Exec(ctx, strings.TrimSuffix(beforeHandoff, up+"\n"))
	require.NoError(t, err)
	st := &store.BotStore{Pool: pool}
	bot, _, err := st.CreateBot(ctx, uuid.New(), "Retained root", "", `[]`, uuid.New())
	require.NoError(t, err)
	operation := uuid.New()
	// Historical decision bytes are deliberately opaque here: migration keeps
	// them as unqualified input and never manufactures remote owner authority.
	decision := []byte("retained historical decision")
	digest := sha256.Sum256(decision)
	_, err = pool.Exec(ctx, `INSERT INTO bot_deletion_operations(operation_id,bot_id,owner_account_id,actor_profile_id,decision_bytes,decision_sha256,state,completed_at) VALUES($1,$2,$3,$4,$5,$6,'COMPLETE',now())`, operation, bot.ID, bot.OwnerAccountID, bot.ActorProfileID, decision, digest[:])
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE bots SET status='disabled' WHERE id=$1`, bot.ID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, up)
	require.NoError(t, err)
	var state string
	var saved []byte
	var completed *time.Time
	require.NoError(t, pool.QueryRow(ctx, `SELECT state,decision_bytes,completed_at FROM bot_deletion_operations WHERE operation_id=$1`, operation).Scan(&state, &saved, &completed))
	require.Equal(t, "RETRY_WAIT", state)
	require.Equal(t, decision, saved)
	require.Nil(t, completed)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM bot_deletion_terminal_effects WHERE operation_id=$1 AND state='PENDING' AND request_bytes IS NULL AND receipt_bytes IS NULL`, operation).Scan(&count))
	require.Equal(t, 2, count)
	_, err = pool.Exec(ctx, globalTerminalMigration(t, "down"))
	require.ErrorContains(t, err, "retained Bot global terminal")
}

func TestBotGlobalTerminalDownOldSnapshotAndEmptyReadCommitted(t *testing.T) {
	if testing.Short() {
		t.Skip("real PostgreSQL retained terminal DOWN isolation")
	}
	t.Run("empty default RC", func(t *testing.T) {
		st := startBotStore(t)
		_, err := st.Pool.Exec(t.Context(), globalTerminalMigration(t, "down"))
		require.NoError(t, err)
		_, err = st.Pool.Exec(t.Context(), globalTerminalMigration(t, "up"))
		require.NoError(t, err)
	})
	t.Run("old RR cannot drop later accepted obligation", func(t *testing.T) {
		st := startBotStore(t)
		ctx := t.Context()
		tx, err := st.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
		require.NoError(t, err)
		defer func() { _ = tx.Rollback(context.Background()) }()
		var count int
		require.NoError(t, tx.QueryRow(ctx, `SELECT count(*) FROM bot_deletion_terminal_effects`).Scan(&count))
		require.Zero(t, count)
		bot, _, err := st.CreateBot(ctx, uuid.New(), "Later root", "", `[]`, uuid.New())
		require.NoError(t, err)
		_, err = st.BeginBotDeletion(ctx, bot.ID, bot.OwnerAccountID)
		require.NoError(t, err)
		_, err = tx.Exec(ctx, globalTerminalMigration(t, "down"))
		require.ErrorContains(t, err, "READ COMMITTED")
		require.NoError(t, tx.Rollback(ctx))
		require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM bot_deletion_terminal_effects`).Scan(&count))
		require.Equal(t, 2, count)
	})
}

func TestBotGlobalTerminalDownWaitsForRealFirstDeletionWriter(t *testing.T) {
	if testing.Short() {
		t.Skip("real PostgreSQL accepted writer versus DOWN")
	}
	st := startBotStore(t)
	ctx, cancel := context.WithTimeout(t.Context(), 25*time.Second)
	defer cancel()
	bot, _, err := st.CreateBot(ctx, uuid.New(), "First writer", "", `[]`, uuid.New())
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `CREATE FUNCTION hold_terminal_first_writer() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN PERFORM pg_advisory_xact_lock(247014); RETURN NEW; END $$; CREATE TRIGGER terminal_first_writer BEFORE UPDATE ON bots FOR EACH ROW WHEN (NEW.status='disabled') EXECUTE FUNCTION hold_terminal_first_writer()`)
	require.NoError(t, err)
	blocker, err := st.Pool.Acquire(ctx)
	require.NoError(t, err)
	defer blocker.Release()
	_, err = blocker.Exec(ctx, `SELECT pg_advisory_lock(247014)`)
	require.NoError(t, err)
	defer func() { _, _ = blocker.Exec(context.Background(), `SELECT pg_advisory_unlock(247014)`) }()
	written := make(chan error, 1)
	go func() { _, e := st.BeginBotDeletion(ctx, bot.ID, bot.OwnerAccountID); written <- e }()
	require.Eventually(t, func() bool {
		var waiting bool
		return st.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE locktype='advisory' AND NOT granted)`).Scan(&waiting) == nil && waiting
	}, 5*time.Second, 10*time.Millisecond)
	down := make(chan error, 1)
	go func() { _, e := st.Pool.Exec(ctx, globalTerminalMigration(t, "down")); down <- e }()
	require.Eventually(t, func() bool {
		var waiting bool
		return st.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE relation='bot_deletion_operations'::regclass AND mode='AccessExclusiveLock' AND NOT granted)`).Scan(&waiting) == nil && waiting
	}, 5*time.Second, 10*time.Millisecond)
	_, err = blocker.Exec(ctx, `SELECT pg_advisory_unlock(247014)`)
	require.NoError(t, err)
	require.NoError(t, <-written)
	require.ErrorContains(t, <-down, "retained Bot global terminal")
	var count int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM bot_deletion_terminal_effects`).Scan(&count))
	require.Equal(t, 2, count)
}
