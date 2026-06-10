package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestSpaceModeration_Store_Migration_applies documents space_db moderation schema (space_bans, member timeouts).
func TestSpaceModeration_Store_Migration_applies(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)

	var bansTable, timeoutsTable string
	err := pool.QueryRow(ctx, `
SELECT table_name FROM information_schema.tables
WHERE table_schema = 'public' AND table_name = 'space_bans'
`).Scan(&bansTable)
	require.NoError(t, err)
	require.Equal(t, "space_bans", bansTable)

	err = pool.QueryRow(ctx, `
SELECT table_name FROM information_schema.tables
WHERE table_schema = 'public' AND table_name = 'space_member_timeouts'
`).Scan(&timeoutsTable)
	require.NoError(t, err)
	require.Equal(t, "space_member_timeouts", timeoutsTable)
}
