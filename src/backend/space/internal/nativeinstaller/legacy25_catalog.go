package nativeinstaller

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"
	"strconv"

	"github.com/jackc/pgx/v5"
)

// This structural comparison alone is not legacy25 admission: owner/ACL,
// preservation preimage, application state, source and maintenance custody
// are separate prerequisites. The PG16 fixture was made by running the pinned
// source migration inventory through version 25 on an isolated database.
//
//go:embed legacy25_catalog.sql
var legacy25CatalogSQL string

const legacy25CatalogPG16SHA256 = "ab30b1233f3e0ff3e0d1b9f0644d58f6452052ab325b94f3a789648269797c7e"
const legacy25CatalogSQLSHA256 = "ca25d6fb262a715d25e5ee33dbbe4ed2dda4df9aee21a3dd8f6663f104f960b2"

func inspectLegacy25Catalog(ctx context.Context, conn *pgx.Conn) (string, error) {
	if ctx == nil || conn == nil {
		return "", errors.New("legacy25 catalog unavailable")
	}
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return "", errors.New("legacy25 catalog unavailable")
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(ctx, `SET LOCAL search_path=pg_catalog`); err != nil {
		return "", errors.New("legacy25 catalog unavailable")
	}
	digest, err := inspectLegacy25CatalogTx(ctx, tx)
	if err != nil {
		return "", err
	}
	if err = tx.Commit(ctx); err != nil {
		return "", errors.New("legacy25 catalog unavailable")
	}
	return digest, nil
}

func inspectLegacy25CatalogTx(ctx context.Context, tx pgx.Tx) (string, error) {
	queryHash := sha256.Sum256([]byte(legacy25CatalogSQL))
	if hex.EncodeToString(queryHash[:]) != legacy25CatalogSQLSHA256 {
		return "", errors.New("legacy25 catalog source changed")
	}
	var db, version string
	if err := tx.QueryRow(ctx, `SELECT current_database(),current_setting('server_version_num')`).Scan(&db, &version); err != nil || db != "space_db" {
		return "", errors.New("legacy25 catalog target unavailable")
	}
	server, err := strconv.Atoi(version)
	if err != nil || server/10000 != 16 {
		return "", errors.New("legacy25 catalog server unsupported")
	}
	var rows, clean int64
	if err := tx.QueryRow(ctx, `SELECT count(*),count(*) FILTER (WHERE version=25 AND dirty=false) FROM public.schema_migrations`).Scan(&rows, &clean); err != nil || rows != 1 || clean != 1 {
		return "", errors.New("legacy25 migration marker unavailable")
	}
	var catalog string
	if err := tx.QueryRow(ctx, legacy25CatalogSQL).Scan(&catalog); err != nil {
		return "", errors.New("legacy25 catalog unavailable")
	}
	sum := sha256.Sum256([]byte(catalog))
	digest := hex.EncodeToString(sum[:])
	if digest != legacy25CatalogPG16SHA256 {
		return "", errors.New("legacy25 catalog differs")
	}
	return digest, nil
}
