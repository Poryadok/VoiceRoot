package nativeinstaller

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// Derived independently from the pinned migrations 1 through 24 on private
// PostgreSQL 16. A structural match is not by itself upgrade admission.
const legacy24CatalogPG16SHA256 = "9152b7fb2f8e3d1f836832760d11059c528296688f79022dc6d19c282c16e650"

func captureLegacy24Snapshot(ctx context.Context, conn *pgx.Conn) (string, map[string]legacy25TableDigest, error) {
	return captureLegacySourceSnapshot(ctx, conn, 24)
}
