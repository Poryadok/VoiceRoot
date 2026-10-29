//go:build integration

package nativeinstaller

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// These independent wrappers are used only by integration fixtures. Production
// qualification uses the shared single-snapshot transaction-taking helpers.
func inspectLegacy25Catalog(ctx context.Context, conn *pgx.Conn) (string, error) {
	tx, err := beginLegacySourceRead(ctx, conn)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	digest, err := inspectLegacy25CatalogTx(ctx, tx)
	if err != nil {
		return "", err
	}
	if err = tx.Commit(ctx); err != nil {
		return "", err
	}
	return digest, nil
}

func captureLegacy25Rows(ctx context.Context, conn *pgx.Conn) (map[string]legacy25TableDigest, error) {
	return captureLegacy25Projection(ctx, conn, nil)
}

func captureLegacy25Projection(ctx context.Context, conn *pgx.Conn, original map[string]legacy25TableDigest) (map[string]legacy25TableDigest, error) {
	tx, err := beginLegacySourceRead(ctx, conn)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	rows, err := captureLegacy25ProjectionTx(ctx, tx, original)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return rows, nil
}

func captureLegacy24Snapshot(ctx context.Context, conn *pgx.Conn) (string, map[string]legacy25TableDigest, error) {
	return captureLegacySourceSnapshot(ctx, conn, 24)
}

func captureLegacy25Snapshot(ctx context.Context, conn *pgx.Conn) (string, map[string]legacy25TableDigest, error) {
	return captureLegacySourceSnapshot(ctx, conn, 25)
}

func captureLegacySourceSnapshot(ctx context.Context, conn *pgx.Conn, version int) (string, map[string]legacy25TableDigest, error) {
	var expected string
	switch version {
	case 24:
		expected = legacy24CatalogPG16SHA256
	case 25:
		expected = legacy25CatalogPG16SHA256
	default:
		return "", nil, errors.New("legacy source unavailable")
	}
	tx, err := beginLegacySourceRead(ctx, conn)
	if err != nil {
		return "", nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	digest, err := inspectLegacyCatalogTx(ctx, tx, version, expected)
	if err != nil {
		return "", nil, err
	}
	rows, err := captureLegacy25ProjectionTx(ctx, tx, nil)
	if err != nil {
		return "", nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return "", nil, err
	}
	return digest, rows, nil
}
