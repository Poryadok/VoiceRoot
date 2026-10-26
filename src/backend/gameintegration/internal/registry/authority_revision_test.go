package registry

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/integrationtest"
)

func TestGISAuthoritySourceActualMigrationClock(t *testing.T) {
	ctx := context.Background()
	fixture := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join("..", "..", "..", "migrations", "game_integration_db"))
	_, err := fixture.Pool.Exec(ctx, `CREATE DATABASE game_integration_db`)
	require.NoError(t, err)
	_, err = fixture.Pool.Exec(ctx, `CREATE ROLE gameintegration_runtime NOLOGIN`)
	require.NoError(t, err)
	fixture.Run(true, "up")
	pool := fixture.Pool
	read := func() int64 {
		var value int64
		require.NoError(t, pool.QueryRow(ctx, `SELECT revision FROM gis_authority_revision WHERE singleton`).Scan(&value))
		return value
	}
	require.EqualValues(t, 1, read())
	app, owner, env := uuid.New(), uuid.New(), uuid.New()
	_, err = pool.Exec(ctx, `INSERT INTO applications(id,owner_account_id,name,status) VALUES($1,$2,'source','draft')`, app, owner)
	require.NoError(t, err)
	require.EqualValues(t, 2, read())
	_, err = pool.Exec(ctx, `INSERT INTO environments(id,application_id,kind,status) VALUES($1,$2,'sandbox','pending')`, env, app)
	require.NoError(t, err)
	before := read()
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `UPDATE environments SET status='active' WHERE id=$1`, env)
	require.NoError(t, err)
	require.NoError(t, tx.Rollback(ctx))
	require.Equal(t, before, read(), "aborted authority cannot advance the committed clock")
	_, err = pool.Exec(ctx, `UPDATE environments SET status='active' WHERE id=$1`, env)
	require.NoError(t, err)
	require.Greater(t, read(), before, "status change also conservatively invalidates credential state")
	committed := read()
	for _, sql := range []string{
		`DELETE FROM gis_authority_revision`, `TRUNCATE gis_authority_revision`,
		`UPDATE gis_authority_revision SET revision=revision-1`,
		`INSERT INTO gis_authority_revision(singleton,revision) VALUES(false,99)`,
		`TRUNCATE applications CASCADE`,
	} {
		_, err = pool.Exec(ctx, sql)
		require.Error(t, err, sql)
		require.Equal(t, committed, read())
	}
	_, err = pool.Exec(ctx, `DELETE FROM environments WHERE id=$1`, env)
	require.NoError(t, err)
	require.Equal(t, committed+1, read(), "deletion must invalidate a complete source even when no row remains")
	fixture.Run(false, "down", "1")
	var dirty bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT dirty FROM schema_migrations`).Scan(&dirty))
	require.True(t, dirty)
	require.Equal(t, committed+1, read(), "refused downgrade retains authority floor")
}
