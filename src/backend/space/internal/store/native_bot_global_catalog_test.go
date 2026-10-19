package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"voice/backend/pkg/integrationtest"
)

// Canonical exports are built ONLY from this checkout's actual fresh immutable
// migration inputs. They are review material for compiled closed pins, never a
// runtime mechanism for learning or accepting an arbitrary installed catalog.
func TestNativeBotGlobal46CanonicalStructure(t *testing.T) {
	if testing.Short() {
		t.Skip("requires actual canonical PostgreSQL16/18 migration matrices")
	}
	for _, major := range []int{16, 18} {
		t.Run(fmt.Sprint(major), func(t *testing.T) {
			ctx := context.Background()
			image := "postgres:16-bookworm"
			if major == 18 {
				image = "postgres:18-alpine"
			}
			db := integrationtest.NewSourceMigrationFixtureWithImage(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"), image)
			db.Run(true, "up")
			var shape string
			require.NoError(t, db.Pool.QueryRow(ctx, nativeReadinessStructureSQL).Scan(&shape))
			digest := sha256.Sum256([]byte(shape))
			t.Logf("canonical PostgreSQL%d schema46 structure SHA256 %s", major, hex.EncodeToString(digest[:]))
			require.NoError(t, os.WriteFile(fmt.Sprintf("/tmp/native46-pg%d-structure.json", major), []byte(shape), 0600))
			tx, err := db.Pool.Begin(ctx)
			require.NoError(t, err)
			require.NoError(t, checkMemberAuthorityCatalog(ctx, tx), "fixed46 source qualifies only this actual canonical structure")
			var unactivated bool
			require.NoError(t, tx.QueryRow(ctx, `SELECT catalog_sha256<>native_membership_guard.catalog() FROM native_membership_guard.epoch WHERE singleton`).Scan(&unactivated))
			require.True(t, unactivated, "structural qualification must not promote the retained44/45 seal")
			require.NoError(t, tx.Rollback(ctx))
		})
	}
}
