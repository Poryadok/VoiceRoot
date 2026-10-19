package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/integrationtest"
)

func TestNativeAuthorityForwardUpgradeFromOfficial34(t *testing.T) {
	if testing.Short() {
		t.Skip("requires real PostgreSQL migrator native forward upgrade")
	}
	ctx := context.Background()
	directory := filepath.Join(repoRoot(t), "src/backend/migrations/space_db")
	database := integrationtest.NewSourceMigrationFixture(t, ctx, directory)
	database.Run(true, "goto", "34")
	st := &SpaceStore{Pool: database.Pool}
	owner, profile := uuid.New(), uuid.New()
	space, err := st.CreateSpace(ctx, owner, "Official34 before native", "", "public")
	require.NoError(t, err)
	_, err = st.JoinSpace(ctx, space.ID, profile, uuid.New())
	require.NoError(t, err)
	// Missing implementation still reaches the real pre-upgrade database, so
	// the intended RED is absent durable authority, not a compiler/file error.
	_, migrationErr := os.Stat(filepath.Join(directory, "000039_native_membership_authority.up.sql"))
	if migrationErr == nil {
		database.Run(true, "up")
	} else {
		require.True(t, os.IsNotExist(migrationErr))
	}
	var generation, floor int64
	var publication uuid.UUID
	var state string
	var legacy bool
	read := func() {
		t.Helper()
		require.NoError(t, database.Pool.QueryRow(ctx, `SELECT generation,retired_through,publication_id,state,legacy
		 FROM space_ordinary_member_heads WHERE space_id=$1 AND profile_id=$2`, space.ID, profile).
			Scan(&generation, &floor, &publication, &state, &legacy))
	}
	read()
	require.Zero(t, generation)
	require.True(t, legacy)
	require.Equal(t, "ACTIVE", state)
	require.NotEqual(t, uuid.Nil, publication)
	first := publication
	read()
	require.Equal(t, first, publication, "migration allocates once; reads cannot remint a legacy identity")
	var version int64
	require.NoError(t, database.Pool.QueryRow(ctx, `SELECT version FROM schema_migrations WHERE NOT dirty`).Scan(&version))
	require.EqualValues(t, 39, version, "official34 advances forward, never a retroactive33/history rewrite")
	require.NoError(t, st.RemoveMember(ctx, space.ID, profile))
	read()
	require.Equal(t, "RETIRED", state)
	require.Positive(t, generation)
	require.Equal(t, generation, floor)
	var cleanup int
	require.NoError(t, database.Pool.QueryRow(ctx, `SELECT count(*) FROM space_member_role_cleanup WHERE space_id=$1 AND profile_id=$2 AND removal_floor=$3`, space.ID, profile, floor).Scan(&cleanup))
	require.Equal(t, 1, cleanup)
	_, err = database.Pool.Exec(ctx, `INSERT INTO space_members(space_id,profile_id) VALUES($1,$2)`, space.ID, profile)
	require.Error(t, err, "an unadapted writer cannot create another legacy0 or resurrect a retired source")
	_, err = database.Pool.Exec(ctx, `TRUNCATE space_ordinary_member_heads`)
	require.Error(t, err, "source floors survive ordinary/admin truncation")
	// The new native schema must not replace current11 terminal policy.
	var terminalDefinition string
	require.NoError(t, database.Pool.QueryRow(ctx, `SELECT pg_get_functiondef('space_terminal_purge_ready(uuid)'::regprocedure)`).Scan(&terminalDefinition))
	require.Contains(t, terminalDefinition, "11")
	require.Contains(t, terminalDefinition, "participant_id")
}

func TestNativeAuthorityFreshMigrationTo39(t *testing.T) {
	if testing.Short() {
		t.Skip("requires real PostgreSQL full native migration")
	}
	ctx := context.Background()
	database := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	database.Run(true, "up")
	var version int64
	require.NoError(t, database.Pool.QueryRow(ctx, `SELECT version FROM schema_migrations WHERE NOT dirty`).Scan(&version))
	require.EqualValues(t, 39, version)
	var heads int
	require.NoError(t, database.Pool.QueryRow(ctx, `SELECT count(*) FROM space_ordinary_member_heads`).Scan(&heads))
	require.Zero(t, heads)
	var cleanupPredicate string
	require.NoError(t, database.Pool.QueryRow(ctx, `SELECT pg_get_functiondef('space_membership_terminal_cleanup_allowed(uuid)'::regprocedure)`).Scan(&cleanupPredicate))
	require.Contains(t, cleanupPredicate, "space_terminal_purge_ready")
	var allowed bool
	require.NoError(t, database.Pool.QueryRow(ctx, `SELECT space_membership_terminal_cleanup_allowed($1)`, uuid.New()).Scan(&allowed))
	require.False(t, allowed)
}
