package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"voice/backend/pkg/authoritysource"
	"voice/backend/pkg/integrationtest"
	"voice/backend/user/internal/userauthoritycatalog"
)

// A selected catalog can become stale before BeginRead acquires its locks.
// The real driver advances the ledger and catalog between hint and recheck.
func TestUserAuthorityStaleCatalogRecheckedUnderLock(t *testing.T) {
	ctx := context.Background()
	f := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(userModuleRepoRoot(t), "src", "backend", "migrations", "user_db"))
	f.Run(true, "goto", "21")
	var hint int64
	require.NoError(t, f.Pool.QueryRow(ctx, `SELECT version FROM schema_migrations WHERE NOT dirty`).Scan(&hint))
	require.EqualValues(t, 21, hint)
	old := userAuthorityCatalog
	require.EqualValues(t, hint, old.Version)
	f.Run(true, "goto", "22")
	tx, err := authoritysource.BeginRead(ctx, f.Pool, old)
	if tx != nil {
		authoritysource.RollbackRead(ctx, tx)
	}
	require.Error(t, err, "stale 21 selection cannot authorize a 22 cut")

	current := userauthoritycatalog.Catalog()
	tx, err = authoritysource.BeginRead(ctx, f.Pool, current)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
	_, err = f.Pool.Exec(ctx, `UPDATE schema_migrations SET dirty=true`)
	require.NoError(t, err)
	tx, err = authoritysource.BeginRead(ctx, f.Pool, current)
	if tx != nil {
		authoritysource.RollbackRead(ctx, tx)
	}
	require.Error(t, err, "a now-dirty marker cannot authorize the selected catalog")
}
