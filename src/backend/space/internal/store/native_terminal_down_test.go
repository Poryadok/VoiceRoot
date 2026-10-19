package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/integrationtest"
)

func TestNativeTerminal47DownSerializesFirstAdmissionAndRejectsOldRR(t *testing.T) {
	if testing.Short() {
		t.Skip("requires actual pinned migrator and first trusted47 admission")
	}
	ctx, stop := context.WithTimeout(context.Background(), 40*time.Second)
	defer stop()
	db := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	db.Run(true, "goto", "46")
	admin := &SpaceStore{Pool: db.Pool}
	_, err := admin.TransitionNativeMembershipCatalog46(ctx)
	require.NoError(t, err)
	db.Run(true, "up", "1")
	down, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/space_db/000047_native_terminal_maintenance.down.sql"))
	require.NoError(t, err)
	old, err := db.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	require.NoError(t, err)
	defer old.Rollback(context.Background())
	var count int
	require.NoError(t, old.QueryRow(ctx, `SELECT count(*) FROM pg_database`).Scan(&count))
	// An unrelated snapshot fixes RR before the first47 admission without
	// blocking the real installer on an evidence-table snapshot lock.
	blocker, err := db.Pool.Begin(ctx)
	require.NoError(t, err)
	defer blocker.Rollback(context.Background())
	var blockerPID int32
	require.NoError(t, blocker.QueryRow(ctx, `SELECT pg_backend_pid(),count(*) FROM native_membership_guard.catalog47_transition`).Scan(&blockerPID, &count))
	require.Zero(t, count)
	admission := make(chan error, 1)
	go func() { _, err := admin.TransitionNativeMembershipCatalog47(ctx); admission <- err }()
	var installerPID int32
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); {
		err = db.Pool.QueryRow(ctx, `SELECT pid FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid)) LIMIT 1`, blockerPID).Scan(&installerPID)
		if err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	require.NotZero(t, installerPID, "actual trusted installer must be waiting at its final private-table lock")
	remover, err := db.Pool.Acquire(ctx)
	require.NoError(t, err)
	defer remover.Release()
	var removerPID int32
	require.NoError(t, remover.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&removerPID))
	removed := make(chan error, 1)
	go func() {
		_, err := remover.Exec(ctx, string(down))
		_, _ = remover.Exec(context.Background(), `ROLLBACK`)
		removed <- err
	}()
	blocked := false
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); {
		require.NoError(t, db.Pool.QueryRow(ctx, `SELECT $1=ANY(pg_blocking_pids($2))`, installerPID, removerPID).Scan(&blocked))
		if blocked {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	require.True(t, blocked, "default RC DOWN must wait behind the actual first installer")
	require.NoError(t, blocker.Commit(ctx))
	require.NoError(t, <-admission)
	var pgErr *pgconn.PgError
	require.ErrorAs(t, <-removed, &pgErr)
	require.Equal(t, "55000", pgErr.Code, "RC must observe and retain the first admitted proof")
	_, err = old.Exec(ctx, string(down))
	require.ErrorAs(t, err, &pgErr)
	require.Equal(t, "55000", pgErr.Code, "old RR must refuse before destructive locks/reads")
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM native_membership_guard.catalog47_transition`).Scan(&count))
	require.Equal(t, 1, count)
}
