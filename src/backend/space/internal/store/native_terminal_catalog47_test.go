package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"voice/backend/pkg/integrationtest"
)

func TestNativeTerminal47MigrationPreservesClosedBaseline(t *testing.T) {
	if testing.Short() {
		t.Skip("requires private pinned migrate and PostgreSQL16/18")
	}
	for _, major := range []int{16, 18} {
		t.Run(fmt.Sprint(major), func(t *testing.T) {
			ctx := context.Background()
			db := integrationtest.NewSourceMigrationFixtureWithImage(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"), fmt.Sprintf("postgres:%d-alpine", major))
			db.Run(true, "goto", "46")
			admin := &SpaceStore{Pool: db.Pool}
			_, err := admin.TransitionNativeMembershipCatalog46(ctx)
			require.NoError(t, err)
			var before, after, current []byte
			require.NoError(t, db.Pool.QueryRow(ctx, `SELECT catalog_sha256 FROM native_membership_guard.epoch`).Scan(&before))
			db.Run(true, "up", "1")
			require.NoError(t, db.Pool.QueryRow(ctx, `SELECT catalog_sha256,native_membership_guard.catalog() FROM native_membership_guard.epoch`).Scan(&after, &current))
			require.Equal(t, before, after, "migration must not bless the new live catalog")
			require.NotEqual(t, after, current, "serving remains closed until trusted exact forward qualification")
			var structure string
			require.NoError(t, db.Pool.QueryRow(ctx, nativeReadinessStructureSQL).Scan(&structure))
			require.NoError(t, os.WriteFile(fmt.Sprintf("/tmp/native47-pg%d-structure.json", major), []byte(structure), 0600))
			// No47 proof has been admitted: real DOWN is safe and the original
			// fixed46 closure/seal must be restored exactly.
			db.Run(true, "down", "1")
			var restored []byte
			require.NoError(t, db.Pool.QueryRow(ctx, `SELECT native_membership_guard.catalog()`).Scan(&restored))
			require.Equal(t, before, restored, "unused DOWN must restore the exact admitted46 seal, including old function body bytes")
		})
	}
}
