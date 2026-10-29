package nativeinstaller

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// Derived independently from the pinned migrations 1 through 24 on private
// PostgreSQL 16. A structural match is not by itself upgrade admission.
const legacy24CatalogPG16SHA256 = "c612b198e699953bb460be25a96f939b2c77a35f3bc5db0df1c9908342bf72bf"

func captureLegacy24Snapshot(ctx context.Context, conn *pgx.Conn) (string, map[string]legacy25TableDigest, error) {
	return captureLegacySourceSnapshot(ctx, conn, 24)
}
