package store

import (
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/integrationtest"
)

func nativeBotGlobalDownSQL(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/space_db/000046_native_bot_global_removal.down.sql"))
	require.NoError(t, err)
	sql := string(raw)
	if os.Getenv("VOICE_NATIVE_BOT_UNSAFE_DOWN") == "1" {
		start, end := strings.Index(sql, "DO $$ BEGIN"), strings.Index(sql, "END $$;")
		require.GreaterOrEqual(t, start, 0)
		require.Greater(t, end, start)
		sql = sql[:start] + sql[end+len("END $$;"):]
		t.Log("controlled unsafe46 DOWN removes only pre-lock RC guard; production bytes unchanged")
	}
	return sql
}

func insertNativeBotTerminal(t *testing.T, ctx context.Context, db botJournalDML) uuid.UUID {
	t.Helper()
	bot, root, actor := uuid.New(), uuid.New(), uuid.New()
	rootSHA, actorSHA := sha256.Sum256(root[:]), sha256.Sum256(actor[:])
	_, err := db.Exec(ctx, `INSERT INTO native_membership_guard.bot_terminals(bot_id,decision_id,decision_sha256,actor_binding_sha256,created_at) VALUES($1,$2,$3,$4,clock_timestamp())`, bot, root, rootSHA[:], actorSHA[:])
	require.NoError(t, err)
	return bot
}

func TestNativeBotGlobal46DownRejectsOldRepeatableSnapshot(t *testing.T) {
	if testing.Short() {
		t.Skip("requires real PostgreSQL old-snapshot destructive DOWN discriminator")
	}
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	db.Run(true, "goto", "46")
	old, err := db.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	require.NoError(t, err)
	defer old.Rollback(ctx)
	var count int
	require.NoError(t, old.QueryRow(ctx, `SELECT count(*) FROM native_membership_guard.bot_terminals`).Scan(&count))
	require.Zero(t, count)
	bot := insertNativeBotTerminal(t, ctx, db.Pool)
	_, err = old.Exec(ctx, nativeBotGlobalDownSQL(t))
	_ = old.Rollback(ctx)
	var pgErr *pgconn.PgError
	require.ErrorAs(t, err, &pgErr, "stale empty snapshot must not erase a first committed global revocation")
	require.Equal(t, "55000", pgErr.Code)
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM native_membership_guard.bot_terminals WHERE bot_id=$1`, bot).Scan(&count))
	require.Equal(t, 1, count)
}

func TestNativeBotGlobal46DownRCSeesFirstCommittedWriter(t *testing.T) {
	if testing.Short() {
		t.Skip("requires real PostgreSQL first-writer destructive DOWN interleaving")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	db.Run(true, "goto", "46")
	writer, err := db.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	require.NoError(t, err)
	defer writer.Rollback(context.Background())
	_, err = writer.Exec(ctx, `LOCK TABLE native_membership_guard.bot_terminals IN ROW EXCLUSIVE MODE`)
	require.NoError(t, err)
	remover, err := db.Pool.Acquire(ctx)
	require.NoError(t, err)
	defer remover.Release()
	var pid int32
	require.NoError(t, remover.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid))
	done := make(chan error, 1)
	sql := nativeBotGlobalDownSQL(t)
	go func() {
		_, e := remover.Exec(ctx, sql)
		_, _ = remover.Exec(context.Background(), `ROLLBACK`)
		done <- e
	}()
	blocked := false
	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); {
		require.NoError(t, writer.QueryRow(ctx, `SELECT cardinality(pg_blocking_pids($1))>0`, pid).Scan(&blocked))
		if blocked {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	bot := insertNativeBotTerminal(t, ctx, writer)
	require.NoError(t, writer.Commit(ctx))
	var pgErr *pgconn.PgError
	require.ErrorAs(t, <-done, &pgErr)
	require.True(t, blocked, "DOWN must wait on the actual first writer table lock")
	require.Equal(t, "55000", pgErr.Code)
	var count int
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM native_membership_guard.bot_terminals WHERE bot_id=$1`, bot).Scan(&count))
	require.Equal(t, 1, count)
}
