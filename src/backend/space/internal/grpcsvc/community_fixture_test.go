package grpcsvc

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"voice/backend/pkg/integrationtest"
)

// Community publication uses the native39 journal contract. The sparse legacy
// fixture predates that contract and cannot exercise a successful publication.
func communityPublicationFixture(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	db := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	db.Run(true, "goto", "39")
	return db.Pool
}
