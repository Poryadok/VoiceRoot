package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"voice/backend/pkg/integrationtest"
)

func applySpaceMigrations(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	for _, name := range []string{"000001_init.up.sql", "000002_tree.up.sql", "000003_invites.up.sql", "000004_moderation.up.sql", "000005_space_subscriptions.up.sql"} {
		migrationPath := filepath.Join(repoRoot(t), "src", "backend", "migrations", "space_db", name)
		sqlBytes, err := os.ReadFile(migrationPath)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(sqlBytes))
		require.NoError(t, err)
	}
}

func TestAreCoMembers_SharedSpace(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "spacedb", "")
	applySpaceMigrations(t, ctx, pool)
	store := &SpaceStore{Pool: pool}

	owner := uuid.New()
	memberA := uuid.New()
	memberB := uuid.New()
	spaceID := uuid.New()

	_, err := pool.Exec(ctx, `
INSERT INTO spaces (id, name, description, visibility, owner_profile_id, member_count)
VALUES ($1, 'team', '', 'private', $2, 2)`,
		spaceID, owner)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO space_members (space_id, profile_id) VALUES ($1, $2), ($1, $3)`,
		spaceID, memberA, memberB)
	require.NoError(t, err)

	ok, err := store.AreCoMembers(ctx, memberA, memberB, nil)
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = store.AreCoMembers(ctx, memberA, uuid.New(), nil)
	require.NoError(t, err)
	require.False(t, ok)
}
