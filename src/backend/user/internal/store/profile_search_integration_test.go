package store

import (
	"context"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/pkg/integrationtest"
)

func userModuleRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	// internal/store/*.go -> monorepo root
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", ".."))
}

func TestProfileStore_SearchProfilesAfter_postgres(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "userdb", "")
	integrationtest.ApplyUserDBMigrations(t, ctx, pool, userModuleRepoRoot(t))

	viewerAcc := uuid.New()
	otherAcc := uuid.New()
	pidOther := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary)
		VALUES ($1, $2, 'likeuser', '0001', 'Literal 50%_off sale', true)`,
		pidOther, otherAcc)
	require.NoError(t, err)

	st := NewProfileStore(pool)

	t.Run("literal percent and underscore in display_name", func(t *testing.T) {
		rows, err := st.SearchProfilesAfter(ctx, viewerAcc, "50%_off", nil, 20)
		require.NoError(t, err)
		require.Len(t, rows, 1)
		require.Equal(t, pidOther, rows[0].ID)
	})

	t.Run("keyset cursor continues after first page", func(t *testing.T) {
		acc1, acc2 := uuid.New(), uuid.New()
		p1, p2 := uuid.New(), uuid.New()
		_, err := pool.Exec(ctx, `
			INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary)
			VALUES ($1, $2, 'csr_a', '0002', 'Cursor A', true)`,
			p1, acc1)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `
			INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary)
			VALUES ($1, $2, 'csr_b', '0003', 'Cursor B', true)`,
			p2, acc2)
		require.NoError(t, err)

		first, err := st.SearchProfilesAfter(ctx, viewerAcc, "Cursor", nil, 1)
		require.NoError(t, err)
		require.Len(t, first, 1)
		c := ProfileSearchCursor{
			UsernameLower: strings.ToLower(first[0].Username),
			Discriminator: first[0].Discriminator,
			ID:            first[0].ID,
		}
		second, err := st.SearchProfilesAfter(ctx, viewerAcc, "Cursor", &c, 10)
		require.NoError(t, err)
		require.Len(t, second, 1)
		require.Equal(t, "csr_b", second[0].Username)
	})
}
