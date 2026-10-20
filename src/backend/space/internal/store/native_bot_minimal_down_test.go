package store

import (
	"context"
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

func minimalBotTerminalDownSQL(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/space_db/000048_native_bot_minimal_terminal.down.sql"))
	require.NoError(t, err)
	sql := string(raw)
	if os.Getenv("VOICE_NATIVE_BOT_UNSAFE_DOWN") == "1" {
		start, end := strings.Index(sql, "DO $$ BEGIN"), strings.Index(sql, "END $$;")
		require.GreaterOrEqual(t, start, 0)
		require.Greater(t, end, start)
		sql = sql[:start] + sql[end+len("END $$;"):]
		t.Log("controlled unsafe48 DOWN removes only pre-lock RC guard; production bytes unchanged")
	}
	return sql
}

func insertMinimalBotTerminal(t *testing.T, ctx context.Context, db botJournalDML) uuid.UUID {
	t.Helper()
	bot, root := uuid.New(), uuid.New()
	_, err := db.Exec(ctx, `INSERT INTO native_membership_guard.bot_terminal_registrations(bot_id,decision_id,receipt_id,registered_at) VALUES($1,$2,$3,clock_timestamp())`, bot, root, uuid.New())
	require.NoError(t, err)
	return bot
}

func TestMinimalTerminal48DownRejectsOldRepeatableSnapshot(t *testing.T) {
	if testing.Short() {
		t.Skip("requires real PostgreSQL old-snapshot destructive DOWN discriminator")
	}
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	db.Run(true, "goto", "48")
	old, err := db.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	require.NoError(t, err)
	defer old.Rollback(ctx)
	var count int
	require.NoError(t, old.QueryRow(ctx, `SELECT count(*) FROM native_membership_guard.bot_terminal_registrations`).Scan(&count))
	require.Zero(t, count)
	bot := insertMinimalBotTerminal(t, ctx, db.Pool)
	_, err = old.Exec(ctx, minimalBotTerminalDownSQL(t))
	_ = old.Rollback(ctx)
	var pgErr *pgconn.PgError
	require.ErrorAs(t, err, &pgErr, "stale empty snapshot must not erase a first committed global revocation")
	require.Equal(t, "55000", pgErr.Code)
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM native_membership_guard.bot_terminal_registrations WHERE bot_id=$1`, bot).Scan(&count))
	require.Equal(t, 1, count)
}

func TestMinimalTerminal48DownRCSeesFirstCommittedWriter(t *testing.T) {
	if testing.Short() {
		t.Skip("requires real PostgreSQL first-writer destructive DOWN interleaving")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	db.Run(true, "goto", "48")
	writer, err := db.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	require.NoError(t, err)
	defer writer.Rollback(context.Background())
	_, err = writer.Exec(ctx, `LOCK TABLE native_membership_guard.bot_terminal_registrations IN ROW EXCLUSIVE MODE`)
	require.NoError(t, err)
	remover, err := db.Pool.Acquire(ctx)
	require.NoError(t, err)
	defer remover.Release()
	var pid int32
	require.NoError(t, remover.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid))
	done := make(chan error, 1)
	sql := minimalBotTerminalDownSQL(t)
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
	bot := insertMinimalBotTerminal(t, ctx, writer)
	require.NoError(t, writer.Commit(ctx))
	var pgErr *pgconn.PgError
	require.ErrorAs(t, <-done, &pgErr)
	require.True(t, blocked, "DOWN must wait on the actual first writer table lock")
	require.Equal(t, "55000", pgErr.Code)
	var count int
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM native_membership_guard.bot_terminal_registrations WHERE bot_id=$1`, bot).Scan(&count))
	require.Equal(t, 1, count)
}
