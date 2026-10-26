package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"

	"voice/backend/pkg/integrationtest"
)

func TestTerminalSearchReleaseDownRefusesPendingAndConcurrentFirstWriter(t *testing.T) {
	if testing.Short() {
		t.Skip("requires isolated PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	pool := StartMatchmakingDBForStoreTest(t, ctx)
	ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)
	down, err := os.ReadFile(filepath.Join(repoRoot(t), "src", "backend", "migrations", "matchmaking_db", "000019_terminal_search_releases.down.sql"))
	require.NoError(t, err)
	writer, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = writer.Rollback(ctx) }()
	id := uuid.New()
	_, err = writer.Exec(ctx, `INSERT INTO matchmaking_terminal_search_releases
	(search_session_id,generation,profile_id,game_id,mode,region) VALUES($1,1,$2,$3,'duo','eu')`, id, uuid.New(), uuid.New())
	require.NoError(t, err)
	// NOWAIT refuses before observing an empty snapshot or waiting behind the
	// first uncommitted writer. No pending obligation can disappear in that gap.
	_, err = pool.Exec(ctx, string(down))
	var pgErr *pgconn.PgError
	require.ErrorAs(t, err, &pgErr)
	require.Equal(t, "55P03", pgErr.Code)
	require.NoError(t, writer.Commit(ctx))
	_, err = pool.Exec(ctx, string(down))
	require.ErrorContains(t, err, "cannot remove pending terminal search release obligations")
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_terminal_search_releases WHERE search_session_id=$1`, id).Scan(&count))
	require.Equal(t, 1, count)
	// An empty stream has no pending evidence; its additive schema can roll back.
	_, err = pool.Exec(ctx, `DELETE FROM matchmaking_terminal_search_releases WHERE search_session_id=$1`, id)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(down))
	require.NoError(t, err)
	var absent bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT to_regclass('matchmaking_terminal_search_releases') IS NULL`).Scan(&absent))
	require.True(t, absent)
}

func TestTerminalSearchReleaseDownRunnerPreservesPendingAndDirtyMarker(t *testing.T) {
	if testing.Short() {
		t.Skip("requires isolated PostgreSQL and pinned migration runner")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	fixture := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src", "backend", "migrations", "matchmaking_db"))
	fixture.Run(true, "up")
	_, err := fixture.Pool.Exec(ctx, `INSERT INTO matchmaking_terminal_search_releases
	(search_session_id,generation,profile_id,game_id,mode,region) VALUES($1,1,$2,$3,'duo','eu')`, uuid.New(), uuid.New(), uuid.New())
	require.NoError(t, err)
	fixture.Run(false, "down", "1")
	var version int64
	var dirty bool
	require.NoError(t, fixture.Pool.QueryRow(ctx, `SELECT version,dirty FROM schema_migrations`).Scan(&version, &dirty))
	require.EqualValues(t, 18, version)
	require.True(t, dirty, "failed DOWN preserves the normal operator-repair marker")
	var count int
	require.NoError(t, fixture.Pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_terminal_search_releases`).Scan(&count))
	require.Equal(t, 1, count)
}
