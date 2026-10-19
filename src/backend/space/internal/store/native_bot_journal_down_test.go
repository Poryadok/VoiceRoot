package store

import (
	"context"
	"errors"
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

func nativeBotJournalDownFixture(t *testing.T) (integrationtest.SourceMigrationFixture, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	db.Run(true, "goto", "34")
	space, err := (&SpaceStore{Pool: db.Pool}).CreateSpace(ctx, uuid.New(), "Bot DOWN fixture", "", "public")
	require.NoError(t, err)
	db.Run(true, "up")
	return db, space.ID
}

type botJournalDML interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

func insertCanceledBotJournal(t *testing.T, ctx context.Context, db botJournalDML, space uuid.UUID) {
	t.Helper()
	bot, profile, installation, operation := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	_, err := db.Exec(ctx, `INSERT INTO space_bot_installation_attempts(install_operation_id,installation_id,space_id,bot_id,profile_id,generation,state)
 VALUES($1,$2,$3,$4,$5,1,'CANCELED')`, operation, installation, space, bot, profile)
	require.NoError(t, err)
	_, err = db.Exec(ctx, `INSERT INTO space_bot_installation_heads(space_id,bot_id,profile_id,installation_id,generation,retired_through,install_operation_id,state)
 VALUES($1,$2,$3,$4,1,1,$5,'CANCELED')`, space, bot, profile, installation, operation)
	require.NoError(t, err)
}

func nativeBotDownSQL(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/space_db/000042_native_bot_membership_journal.down.sql"))
	require.NoError(t, err)
	sql := string(raw)
	if os.Getenv("VOICE_NATIVE_BOT_UNSAFE_DOWN") == "1" {
		// Controlled negative oracle only: remove the pre-lock isolation guard
		// from this test's SQL string. The production migration stays unchanged.
		start, end := strings.Index(sql, "DO $$ BEGIN"), strings.Index(sql, "END $$;")
		require.GreaterOrEqual(t, start, 0)
		require.Greater(t, end, start)
		sql = sql[:start] + sql[end+len("END $$;"):]
		t.Log("controlled unsafe DOWN variant: pre-lock RC guard removed; production file unchanged")
	}
	return sql
}

func TestNativeBotJournalDownRejectsOldRepeatableSnapshot(t *testing.T) {
	if testing.Short() {
		t.Skip("requires private PostgreSQL DOWN interleaving")
	}
	db, space := nativeBotJournalDownFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	old, err := db.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	require.NoError(t, err)
	defer old.Rollback(context.Background())
	var count int
	require.NoError(t, old.QueryRow(ctx, `SELECT count(*) FROM space_bot_installation_heads`).Scan(&count))
	require.Zero(t, count)
	insertCanceledBotJournal(t, ctx, db.Pool, space)
	_, err = old.Exec(ctx, nativeBotDownSQL(t))
	_ = old.Rollback(ctx)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "55000" {
		t.Errorf("old RR DOWN must refuse before acquiring destructive locks; got %v", err)
	}
	var relation *string
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT to_regclass('public.space_bot_installation_heads')::text`).Scan(&relation))
	require.NotNil(t, relation, "a first committed installation floor was erased by stale snapshot DOWN")
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM space_bot_installation_heads WHERE space_id=$1`, space).Scan(&count))
	require.Equal(t, 1, count)
}

func TestNativeBotJournalDownRCWaitsForFirstCommittedWriter(t *testing.T) {
	if testing.Short() {
		t.Skip("requires private PostgreSQL DOWN interleaving")
	}
	db, space := nativeBotJournalDownFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	writer, err := db.Pool.Begin(ctx)
	require.NoError(t, err)
	defer writer.Rollback(context.Background())
	_, err = writer.Exec(ctx, `LOCK TABLE space_bot_installation_heads,space_bot_installation_attempts IN ROW EXCLUSIVE MODE`)
	require.NoError(t, err)
	remover, err := db.Pool.Acquire(ctx)
	require.NoError(t, err)
	defer remover.Release()
	var pid int32
	require.NoError(t, remover.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid))
	done := make(chan error, 1)
	sql := nativeBotDownSQL(t)
	go func() {
		_, err := remover.Exec(ctx, sql)
		_, _ = remover.Exec(context.Background(), `ROLLBACK`)
		done <- err
	}()
	blocked := false
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); {
		require.NoError(t, writer.QueryRow(ctx, `SELECT cardinality(pg_blocking_pids($1))>0`, pid).Scan(&blocked))
		if blocked {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	require.True(t, blocked, "DOWN must reach the actual table lock before the first insert")
	insertCanceledBotJournal(t, ctx, writer, space)
	require.NoError(t, writer.Commit(ctx))
	var pgErr *pgconn.PgError
	require.ErrorAs(t, <-done, &pgErr)
	require.Equal(t, "55000", pgErr.Code)
	var count int
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM space_bot_installation_heads WHERE space_id=$1`, space).Scan(&count))
	require.Equal(t, 1, count)
}

func TestNativeBotJournalPinnedDriverEmptyDownReplayAndRetainedRefusal(t *testing.T) {
	if testing.Short() {
		t.Skip("requires pinned migration driver")
	}
	db, space := nativeBotJournalDownFixture(t)
	db.Run(true, "down", "1")
	var version int64
	var dirty bool
	require.NoError(t, db.Pool.QueryRow(context.Background(), `SELECT version,dirty FROM schema_migrations`).Scan(&version, &dirty))
	require.EqualValues(t, 41, version)
	require.False(t, dirty)
	db.Run(true, "up")
	insertCanceledBotJournal(t, context.Background(), db.Pool, space)
	db.Run(false, "down", "1")
	var count int
	require.NoError(t, db.Pool.QueryRow(context.Background(), `SELECT count(*) FROM space_bot_installation_heads WHERE space_id=$1`, space).Scan(&count))
	require.Equal(t, 1, count)
	require.NoError(t, db.Pool.QueryRow(context.Background(), `SELECT version,dirty FROM schema_migrations`).Scan(&version, &dirty))
	require.True(t, dirty, "failed destructive downgrade leaves explicit unqualified marker")
}
