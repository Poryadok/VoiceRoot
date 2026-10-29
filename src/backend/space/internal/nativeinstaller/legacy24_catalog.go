package nativeinstaller

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// Derived independently from the pinned migrations 1 through 24 on private
// PostgreSQL 16, after the bounded logical-restore normalization. A structural
// match is not by itself upgrade admission or a substitute for original rows.
const legacy24CatalogPG16SHA256 = "b6b2a0c72816bd21604e7488ac334ada49347bc5b4197e6b5912b6bfb760a43d"

func captureLegacy24Snapshot(ctx context.Context, conn *pgx.Conn) (string, map[string]legacy25TableDigest, error) {
	return captureLegacySourceSnapshot(ctx, conn, 24)
}
