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
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// The explicit DSN must name an owned fixture server with database-create
// permission. No Docker socket, listener or host port is created by this test.
func role13NativeDatabase(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("ROLE_MIGRATION_TEST_DSN")
	if dsn == "" {
		t.Skip("set ROLE_MIGRATION_TEST_DSN to owned PostgreSQL16")
	}
	ctx := context.Background()
	root, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	database := "role13_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	_, err = root.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{database}.Sanitize())
	require.NoError(t, err)
	config, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err)
	config.ConnConfig.Database = database
	pool, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	t.Cleanup(func() {
		pool.Close()
		_, err := root.Exec(ctx, "DROP DATABASE "+pgx.Identifier{database}.Sanitize())
		require.NoError(t, err)
		root.Close()
	})
	_, err = pool.Exec(ctx, role13NativeSQL(t, "up"))
	require.NoError(t, err)
	return pool
}

func role13NativeSQL(t *testing.T, direction string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/role_db/000013_game_session_grants."+direction+".sql"))
	require.NoError(t, err)
	return string(raw)
}

func role13NativeFirstWriter(ctx context.Context, tx pgx.Tx, kind string) error {
	app, environment, session, room := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	if kind == "operation" {
		_, err := tx.Exec(ctx, `INSERT INTO game_session_grant_operations(operation_id,operation_kind,application_id,environment_id,session_id,request_sha256,receipt_id,roster_revision,applied_profile_set_sha256,outcome) VALUES($1,'apply',$2,$3,$4,$5,$6,1,$5,'APPLIED')`, uuid.New(), app, environment, session, make([]byte, 32), uuid.New())
		return err
	}
	_, err := tx.Exec(ctx, `INSERT INTO game_session_grant_sessions(application_id,environment_id,session_id,voice_room_id,roster_revision,profile_set_sha256,status) VALUES($1,$2,$3,$4,1,$5,'active')`, app, environment, session, room, make([]byte, 32))
	if err != nil || kind == "session" {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO game_session_grants(application_id,environment_id,session_id,voice_room_id,profile_id,roster_revision) VALUES($1,$2,$3,$4,$5,1)`, app, environment, session, room, uuid.New())
	return err
}

func role13NativeWaitForDown(t *testing.T, ctx context.Context, pool *pgxpool.Pool, downPID, writerPID uint32, returned <-chan error) {
	t.Helper()
	bounded, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case err := <-returned:
			t.Fatalf("DOWN returned before required writer wait: %v", err)
		default:
		}
		var blocked bool
		err := pool.QueryRow(bounded, `SELECT $1::integer=ANY(pg_blocking_pids($2::integer)) AND EXISTS(SELECT 1 FROM pg_locks WHERE pid=$2 AND NOT granted AND mode='AccessExclusiveLock' AND relation IN (to_regclass('public.game_session_grant_operations'),to_regclass('public.game_session_grants'),to_regclass('public.game_session_grant_sessions')))`, writerPID, downPID).Scan(&blocked)
		require.NoError(t, err)
		if blocked {
			return
		}
		select {
		case <-bounded.Done():
			t.Fatal("DOWN did not establish its protected relation wait")
		case <-ticker.C:
		}
	}
}

func role13NativeAssertRows(t *testing.T, ctx context.Context, pool *pgxpool.Pool, kind string) {
	t.Helper()
	var sessions, grants, operations int
	require.NoError(t, pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM game_session_grant_sessions),(SELECT count(*) FROM game_session_grants),(SELECT count(*) FROM game_session_grant_operations)`).Scan(&sessions, &grants, &operations))
	wantSessions, wantGrants, wantOperations := 1, 0, 0
	if kind == "operation" {
		wantSessions, wantOperations = 0, 1
	}
	if kind == "grant" {
		wantGrants = 1
	}
	require.Equal(t, wantSessions, sessions)
	require.Equal(t, wantGrants, grants)
	require.Equal(t, wantOperations, operations)
}

func TestRole13DownRejectsConcurrentFirstWriters(t *testing.T) {
	for _, kind := range []string{"operation", "session", "grant"} {
		t.Run(kind, func(t *testing.T) {
			pool := role13NativeDatabase(t)
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			writer, err := pool.Begin(ctx)
			require.NoError(t, err)
			defer writer.Rollback(context.Background())
			require.NoError(t, role13NativeFirstWriter(ctx, writer, kind))
			down, err := pool.Acquire(ctx)
			require.NoError(t, err)
			defer down.Release()
			body := role13NativeSQL(t, "down")
			returned := make(chan error, 1)
			go func() { _, downErr := down.Exec(ctx, body); returned <- downErr }()
			role13NativeWaitForDown(t, ctx, pool, down.Conn().PgConn().PID(), writer.Conn().PgConn().PID(), returned)
			require.NoError(t, writer.Commit(ctx))
			var pgErr *pgconn.PgError
			require.ErrorAs(t, <-returned, &pgErr, "DOWN must retain the first committed authority/receipt writer")
			require.Equal(t, "P0001", pgErr.Code)
			role13NativeAssertRows(t, ctx, pool, kind)
		})
	}
}

func TestRole13DownRefusesExistingEvidence(t *testing.T) {
	for _, kind := range []string{"operation", "session", "grant"} {
		t.Run(kind, func(t *testing.T) {
			pool := role13NativeDatabase(t)
			ctx := context.Background()
			writer, err := pool.Begin(ctx)
			require.NoError(t, err)
			defer writer.Rollback(ctx)
			require.NoError(t, role13NativeFirstWriter(ctx, writer, kind))
			require.NoError(t, writer.Commit(ctx))
			_, err = pool.Exec(ctx, role13NativeSQL(t, "down"))
			require.Error(t, err)
			role13NativeAssertRows(t, ctx, pool, kind)
		})
	}
}

func TestRole13EmptyDownUpRoundtrip(t *testing.T) {
	pool := role13NativeDatabase(t)
	ctx := context.Background()
	_, err := pool.Exec(ctx, role13NativeSQL(t, "down"))
	require.NoError(t, err)
	var absent bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT to_regclass('public.game_session_grant_sessions') IS NULL AND to_regclass('public.game_session_grants') IS NULL AND to_regclass('public.game_session_grant_operations') IS NULL`).Scan(&absent))
	require.True(t, absent)
	_, err = pool.Exec(ctx, role13NativeSQL(t, "up"))
	require.NoError(t, err)
	writer, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer writer.Rollback(ctx)
	require.NoError(t, role13NativeFirstWriter(ctx, writer, "grant"))
	require.NoError(t, writer.Commit(ctx))
	role13NativeAssertRows(t, ctx, pool, "grant")
}

func TestRole13DownRejectsStaleTransactionSnapshots(t *testing.T) {
	for _, isolation := range []pgx.TxIsoLevel{pgx.RepeatableRead, pgx.Serializable} {
		t.Run(string(isolation), func(t *testing.T) {
			pool := role13NativeDatabase(t)
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			down, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: isolation})
			require.NoError(t, err)
			defer down.Rollback(context.Background())
			var empty bool
			require.NoError(t, down.QueryRow(ctx, `SELECT NOT EXISTS(SELECT 1 FROM game_session_grant_operations)`).Scan(&empty))
			require.True(t, empty)
			writer, err := pool.Begin(ctx)
			require.NoError(t, err)
			defer writer.Rollback(context.Background())
			require.NoError(t, role13NativeFirstWriter(ctx, writer, "operation"))
			require.NoError(t, writer.Commit(ctx))
			_, err = down.Exec(ctx, role13NativeSQL(t, "down"))
			var pgErr *pgconn.PgError
			require.ErrorAs(t, err, &pgErr, "a stale empty snapshot must never authorize DROP")
			require.Equal(t, "0A000", pgErr.Code)
			require.NoError(t, down.Rollback(ctx))
			role13NativeAssertRows(t, ctx, pool, "operation")
		})
	}
}
