package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/pkg/integrationtest"
)

// TestProfileStore_CountByAccountID_ExcludesSoftDeleted_postgres proves an
// archived profile no longer consumes its account's profile allowance.
func TestProfileStore_CountByAccountID_ExcludesSoftDeleted_postgres(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "userdb", "")
	integrationtest.ApplyUserDBMigrations(t, ctx, pool, userModuleRepoRoot(t))

	accountID := uuid.New()
	otherAccountID := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary, deleted_at)
		VALUES
			($1, $2, 'activeprimary', '0001', 'Active primary', true, NULL),
			($3, $2, 'activealt', '0002', 'Active alt', false, NULL),
			($4, $2, 'archivedalt', '0003', 'Archived alt', false, now()),
			($5, $6, 'otheraccount', '0004', 'Other account', true, NULL)`,
		uuid.New(), accountID, uuid.New(), uuid.New(), uuid.New(), otherAccountID)
	require.NoError(t, err)

	count, err := NewProfileStore(pool).CountByAccountID(ctx, accountID)
	require.NoError(t, err)
	require.Equal(t, 2, count, "only active profiles for this account consume the limit")
}
