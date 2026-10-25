package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// VOICE_MIGRATION_TEST_DATABASE_URL permits the same oracle in a private Linux
// runner with an already-owned PostgreSQL instance and no published host port.
func TestSpaceFileProducerMigrationDown(t *testing.T) {
	if testing.Short() {
		t.Skip("real PostgreSQL migration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	var pool *pgxpool.Pool
	if dsn := os.Getenv("VOICE_MIGRATION_TEST_DATABASE_URL"); dsn != "" {
		var err error
		pool, err = pgxpool.New(ctx, dsn)
		require.NoError(t, err)
		t.Cleanup(pool.Close)
	} else {
		pool = startPostgresForStoreTest(t, ctx)
	}
	root := filepath.Join(repoRoot(t), "src", "backend", "migrations", "messaging_db")
	up, err := os.ReadFile(filepath.Join(root, "000025_space_file_producer.up.sql"))
	require.NoError(t, err)
	down, err := os.ReadFile(filepath.Join(root, "000025_space_file_producer.down.sql"))
	require.NoError(t, err)
	fixture := []byte("immutable P3 File producer references")
	insert := `INSERT INTO messaging_space_file_producers(space_id,deletion_operation_id,schedule_generation,references_bytes) VALUES($1,$2,1,$3)`
	prepare := func(t *testing.T) {
		t.Helper()
		_, err := pool.Exec(ctx, string(up))
		require.NoError(t, err)
		t.Cleanup(func() {
			_, err := pool.Exec(ctx, `DROP TABLE IF EXISTS messaging_space_file_producers`)
			require.NoError(t, err)
		})
	}
	t.Run("empty down and up", func(t *testing.T) {
		prepare(t)
		_, err := pool.Exec(ctx, string(down))
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(up))
		require.NoError(t, err)
	})
	t.Run("committed evidence refuses", func(t *testing.T) {
		prepare(t)
		_, err := pool.Exec(ctx, insert, uuid.New(), uuid.New(), fixture)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(down))
		require.ErrorContains(t, err, "cannot remove immutable P3 File producer snapshots")
	})
	for _, commit := range []bool{true, false} {
		name := "first writer rollback permits"
		if commit {
			name = "first writer commit preserves"
		}
		t.Run(name, func(t *testing.T) {
			prepare(t)
			writer, err := pool.Begin(ctx)
			require.NoError(t, err)
			defer writer.Rollback(ctx)
			space, operation := uuid.New(), uuid.New()
			_, err = writer.Exec(ctx, insert, space, operation, fixture)
			require.NoError(t, err)
			rollback, err := pool.Acquire(ctx)
			require.NoError(t, err)
			defer rollback.Release()
			pid := rollback.Conn().PgConn().PID()
			result := make(chan error, 1)
			go func() { _, err := rollback.Exec(ctx, string(down)); result <- err }()
			// Synchronize on the actual destructive lock, not elapsed sleep: the
			// writer must remain uncommitted until DOWN waits on this relation.
			require.Eventually(t, func() bool {
				var waiting bool
				err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE pid=$1 AND relation='messaging_space_file_producers'::regclass AND mode='AccessExclusiveLock' AND NOT granted)`, pid).Scan(&waiting)
				return err == nil && waiting
			}, 5*time.Second, 10*time.Millisecond)
			if commit {
				require.NoError(t, writer.Commit(ctx))
			} else {
				require.NoError(t, writer.Rollback(ctx))
			}
			select {
			case err = <-result:
			case <-ctx.Done():
				t.Fatal("DOWN did not finish after writer terminal state")
			}
			if commit {
				require.ErrorContains(t, err, "cannot remove immutable P3 File producer snapshots")
				var saved []byte
				require.NoError(t, pool.QueryRow(ctx, `SELECT references_bytes FROM messaging_space_file_producers WHERE space_id=$1 AND deletion_operation_id=$2 AND schedule_generation=1`, space, operation).Scan(&saved))
				require.Equal(t, fixture, saved)
			} else {
				require.NoError(t, err)
				var absent bool
				require.NoError(t, pool.QueryRow(ctx, `SELECT to_regclass('messaging_space_file_producers') IS NULL`).Scan(&absent))
				require.True(t, absent)
			}
		})
	}
	t.Run("stale transaction snapshot refuses", func(t *testing.T) {
		prepare(t)
		tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
		require.NoError(t, err)
		defer tx.Rollback(ctx)
		_, err = tx.Exec(ctx, `SELECT count(*) FROM messaging_space_file_producers`)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, insert, uuid.New(), uuid.New(), fixture)
		require.NoError(t, err)
		_, err = tx.Exec(ctx, string(down))
		require.ErrorContains(t, err, "requires READ COMMITTED")
		require.NoError(t, tx.Rollback(ctx))
		var saved []byte
		require.NoError(t, pool.QueryRow(ctx, `SELECT references_bytes FROM messaging_space_file_producers`).Scan(&saved))
		require.Equal(t, fixture, saved)
	})
}
