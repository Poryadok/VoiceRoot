package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func restoreMigrationSQL(t *testing.T, direction string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), "src", "backend", "migrations", "space_db", "000017_lifecycle_restore_outcome."+direction+".sql"))
	require.NoError(t, err)
	return string(raw)
}

func TestLifecycleRestoreMigration_ExpandAndSafeRollback(t *testing.T) {
	up, down := restoreMigrationSQL(t, "up"), restoreMigrationSQL(t, "down")
	if testing.Short() {
		return
	}
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationsThrough12ForStoreTest(t, ctx, pool)
	applyLifecycleMigration(t, ctx, pool, "up")
	_, err := pool.Exec(ctx, up)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, down)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, up)
	require.NoError(t, err)
	st := &SpaceStore{Pool: pool}
	_, _, _, _, _, request, _, _ := reserveLifecycleOperationFixture(t, st)
	// Existing DELETE reservations survive expand/contract without losing factors.
	_, err = pool.Exec(ctx, down)
	require.NoError(t, err)
	var method string
	require.NoError(t, pool.QueryRow(ctx, `SELECT method FROM space_lifecycle_operations WHERE operation_id=$1`, request.OperationId).Scan(&method))
	require.Equal(t, "DELETE", method)
}
