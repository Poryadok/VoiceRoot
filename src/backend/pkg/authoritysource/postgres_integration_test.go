package authoritysource_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/authoritysource"
	"voice/backend/pkg/integrationtest"
)

func TestCatalogBatchPreservesIdentifierAndPerFunctionProof(t *testing.T) {
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "catalog_batch", "")
	const body = "BEGIN RETURN 1; END"
	hash := sha256.Sum256([]byte(body))
	longTrigger := strings.Repeat("catalog_trigger_", 5)
	_, err := pool.Exec(ctx, `CREATE TABLE schema_migrations(version bigint PRIMARY KEY, dirty boolean NOT NULL);
INSERT INTO schema_migrations VALUES (1,false);
CREATE TABLE catalog_fixture(id integer);
CREATE FUNCTION catalog_first(integer) RETURNS bigint LANGUAGE plpgsql AS $$`+body+`$$;
CREATE FUNCTION catalog_second(integer) RETURNS bigint LANGUAGE plpgsql AS $$`+body+`$$;
CREATE FUNCTION catalog_trigger() RETURNS trigger LANGUAGE plpgsql AS $$BEGIN RETURN NEW; END$$;
CREATE TRIGGER `+pgx.Identifier{longTrigger}.Sanitize()+` AFTER INSERT ON catalog_fixture FOR EACH ROW EXECUTE FUNCTION catalog_trigger()`)
	require.NoError(t, err)
	catalog := authoritysource.Catalog{
		Version: 1, Tables: []string{"catalog_fixture"},
		Triggers: []authoritysource.CatalogTrigger{{Table: "catalog_fixture", Name: longTrigger, Function: "catalog_trigger", Type: 5}},
		Functions: []authoritysource.CatalogFunction{
			{Name: "catalog_first", Arguments: 1, ReturnType: "bigint", BodySHA256: hex.EncodeToString(hash[:])},
			{Name: "catalog_second", Arguments: 1, ReturnType: "bigint", BodySHA256: hex.EncodeToString(hash[:])},
		},
	}
	tx, err := authoritysource.BeginRead(ctx, pool, catalog)
	require.NoError(t, err, "declared long identifiers must match PostgreSQL's stored name")
	require.NoError(t, tx.Rollback(ctx))

	// The total matching row count remains two: an extra identical-body
	// overload of the first function must not replace the missing second one.
	_, err = pool.Exec(ctx, `DROP FUNCTION catalog_second(integer);
CREATE FUNCTION catalog_first(text) RETURNS bigint LANGUAGE plpgsql AS $$`+body+`$$`)
	require.NoError(t, err)
	tx, err = authoritysource.BeginRead(ctx, pool, catalog)
	if tx != nil {
		_ = tx.Rollback(ctx)
	}
	require.Error(t, err, "every expected function needs its own unique proof")
}
