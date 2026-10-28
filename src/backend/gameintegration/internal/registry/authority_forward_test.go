package registry

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"voice/backend/pkg/authoritysource"
	"voice/backend/pkg/integrationtest"
)

// The source must capture actual fresh33 facts. Accepting an older catalog or
// reusing its receipt would conceal whether the forward migration ran.
func TestGISAuthoritySourceForward33Catalog(t *testing.T) {
	ctx, store := gisSourceFixture(t)
	var version int64
	var dirty bool
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT version,dirty FROM schema_migrations`).Scan(&version, &dirty))
	require.EqualValues(t, 33, version)
	require.False(t, dirty)
	require.NoError(t, store.CheckAuthoritySourceSchema(ctx))
	old := gisAuthorityCatalog
	old.Version = 32
	tx, err := authoritysource.BeginRead(ctx, store.Pool, old)
	if tx != nil {
		defer authoritysource.RollbackRead(ctx, tx)
	}
	require.Error(t, err, "a clean33 database cannot supply a clean32 source capture")
	_, err = store.Pool.Exec(ctx, `ALTER TABLE gis_authority_revision DISABLE TRIGGER gis_authority_floor_change`)
	require.NoError(t, err)
	require.Error(t, store.CheckAuthoritySourceSchema(ctx), "version33 must retain the original clock guard checks")
}

func TestGISAuthoritySourceForward33RefusesUnmigrated32(t *testing.T) {
	ctx := context.Background()
	directory := filepath.Join("..", "..", "..", "migrations", "game_integration_db")
	legacy := t.TempDir()
	files, err := os.ReadDir(directory)
	require.NoError(t, err)
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".sql") {
			continue
		}
		version, err := strconv.Atoi(file.Name()[:6])
		require.NoError(t, err)
		if version > 14 && version != 31 && version != 32 {
			continue
		}
		body, err := os.ReadFile(filepath.Join(directory, file.Name()))
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(legacy, file.Name()), body, 0600))
	}
	fixture := integrationtest.NewSourceMigrationFixture(t, ctx, legacy)
	_, err = fixture.Pool.Exec(ctx, `CREATE DATABASE game_integration_db`)
	require.NoError(t, err)
	_, err = fixture.Pool.Exec(ctx, `CREATE ROLE gameintegration_runtime NOLOGIN`)
	require.NoError(t, err)
	fixture.Run(true, "up")
	store := &Store{Pool: fixture.Pool}
	require.Error(t, store.CheckAuthoritySourceSchema(ctx), "an actual clean32 capture cannot be resealed as source33")
	old := gisAuthorityCatalog
	old.Version = 32
	tx, err := authoritysource.BeginRead(ctx, fixture.Pool, old)
	require.NoError(t, err, "the fixture is the real former catalog, not a fabricated version marker")
	require.NoError(t, tx.Commit(ctx))
}
