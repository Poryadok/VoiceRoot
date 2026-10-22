//go:build spacebanintegration

package store

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// Each deliberate catalog mutation is confined to a rolled-back owner
// transaction. No corrupted state can be enrolled as an expected source pin.
func TestAccountBanCatalogRejectsSemanticDrift(t *testing.T) {
	ctx := context.Background()
	dsn := os.Getenv("SPACE_BAN_TEST_DATABASE_URL")
	require.NotEmpty(t, dsn, "owned PostgreSQL fixture required")
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	t.Run("serving login needs no private schema access", func(t *testing.T) {
		serving := projectionServingLogin(t, ctx, pool)
		tx, err := serving.Begin(ctx)
		require.NoError(t, err)
		defer tx.Rollback(ctx)
		var structure string
		require.NoError(t, tx.QueryRow(ctx, accountBanStructureSQL, accountBanTables).Scan(&structure))
		require.NoError(t, checkAccountBanCatalog(ctx, tx))
	})
	cases := []struct{ name, sql string }{
		{"guard predicate", `DROP TRIGGER account_qualification_immutable ON space_account_qualifications;
CREATE TRIGGER account_qualification_immutable BEFORE INSERT OR UPDATE OR DELETE ON space_account_qualifications FOR EACH ROW WHEN(false) EXECUTE FUNCTION space_account_ban_guard.links()`},
		{"guard event columns", `DROP TRIGGER account_suppression_floor ON space_account_ban_suppressions;
CREATE TRIGGER account_suppression_floor BEFORE UPDATE OF account_id ON space_account_ban_suppressions FOR EACH ROW EXECUTE FUNCTION space_account_ban_guard.links()`},
		{"internal foreign key disabled", `DO $$DECLARE r record; BEGIN SELECT tgname,tgrelid::regclass AS rel INTO STRICT r FROM pg_trigger WHERE tgconstraint IN (SELECT oid FROM pg_constraint WHERE contype='f' AND conrelid='space_account_ban_captures'::regclass) AND tgisinternal ORDER BY oid LIMIT 1; EXECUTE format('ALTER TABLE %s DISABLE TRIGGER %I',r.rel,r.tgname); END$$`},
		{"column public privilege", `GRANT SELECT(actor_account_id) ON space_account_ban_operations TO PUBLIC`},
		{"table public privilege", `GRANT SELECT ON space_account_ban_captures TO PUBLIC`},
		{"private schema relation", `CREATE TABLE space_account_ban_guard.unexpected(value integer)`},
		{"unexpected prefixed relation", `CREATE SEQUENCE public.space_account_ban_unexpected`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := pool.Begin(ctx)
			require.NoError(t, err)
			defer tx.Rollback(ctx)
			require.NoError(t, checkAccountBanCatalog(ctx, tx), "pristine source baseline")
			_, err = tx.Exec(ctx, tc.sql)
			require.NoError(t, err, "actual catalog counterexample must be installed")
			require.Error(t, checkAccountBanCatalog(ctx, tx), "semantic or privilege drift must fail closed")
		})
	}
}
