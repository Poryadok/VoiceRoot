package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/integrationtest"
)

func TestProfileStoreBackfillSearchKeysIsResumableAndReportsCollisions(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "userdb", "")
	integrationtest.ApplyUserDBMigrations(t, ctx, pool, userModuleRepoRoot(t))
	for _, name := range []string{"раураl", "paypal"} {
		_, err := pool.Exec(ctx, `INSERT INTO profiles(id, account_id, username, discriminator, display_name, is_primary)
			VALUES($1,$2,$3,'0001',$3,true)`, uuid.New(), uuid.New(), name)
		require.NoError(t, err)
	}
	store := NewProfileStore(pool)
	first, err := store.BackfillSearchKeys(ctx, 1)
	require.NoError(t, err)
	require.EqualValues(t, 1, first.Scanned)
	require.False(t, first.Done)
	second, err := store.BackfillSearchKeys(ctx, 10)
	require.NoError(t, err)
	require.False(t, second.Done)
	third, err := store.BackfillSearchKeys(ctx, 10)
	require.NoError(t, err)
	require.True(t, third.Done)
	var normalized, collisions int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM profiles WHERE search_normalization_version=$1`, searchNormalizationVersion).Scan(&normalized))
	require.Equal(t, 2, normalized)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM profile_search_key_collisions`).Scan(&collisions))
	require.Positive(t, collisions)
}
