//go:build spacebanintegration

package store

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// Both expressions run at the same statement snapshot, including the function's
// own complete current definition. Each counterexample is real rolled-back DDL.
func TestNativeCatalog50LiteralProjectionAndDrift(t *testing.T) {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, os.Getenv("SPACE_BAN_TEST_DATABASE_URL"))
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	cases := []struct{ name, ddl string }{
		{"function cost", `ALTER FUNCTION native_membership_guard.terminal_cleanup() COST 101`},
		{"function ACL", `GRANT EXECUTE ON FUNCTION native_membership_guard.terminal_cleanup() TO PUBLIC`},
		{"function owner", `CREATE ROLE native_text_owner_probe NOLOGIN; ALTER FUNCTION native_membership_guard.terminal_cleanup() OWNER TO native_text_owner_probe`},
		{"escaped Unicode body", `CREATE FUNCTION public."native Ж quote"" slash\ line
next"() RETURNS text LANGUAGE sql AS $body$SELECT 'Ж " \
 body'::text$body$`},
		{"unexpected function", `CREATE FUNCTION public.native_catalog_counterexample() RETURNS integer LANGUAGE sql AS 'SELECT 1'`},
		{"private index attributes", `CREATE INDEX native_catalog_expression_counterexample ON native_membership_guard.profiles ((profile_id::text)); ALTER INDEX native_membership_guard.native_catalog_expression_counterexample ALTER COLUMN 1 SET STATISTICS 321`},
		{"column", `ALTER TABLE native_membership_guard.profiles ADD COLUMN native_catalog_counterexample integer`},
		{"constraint", `ALTER TABLE native_membership_guard.profiles ADD CONSTRAINT native_catalog_counterexample CHECK (true)`},
		{"trigger disabled", `ALTER TABLE public.space_members DISABLE TRIGGER zzzzzz_native_membership_observe`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := pool.Begin(ctx)
			require.NoError(t, err)
			defer tx.Rollback(ctx)
			var before, after []byte
			var equal bool
			assertBytes := func() {
				t.Helper()
				var original, candidate []byte
				var originalSHA, candidateSHA []byte
				require.NoError(t, tx.QueryRow(ctx, `SELECT convert_to((`+nativeCatalog44TextExpression+`),'UTF8'),convert_to((`+nativeCatalog50TextExpression+`),'UTF8'),sha256(convert_to((`+nativeCatalog44TextExpression+`),'UTF8')),sha256(convert_to((`+nativeCatalog50TextExpression+`),'UTF8'))`).Scan(&original, &candidate, &originalSHA, &candidateSHA))
				require.Equal(t, original, candidate, "exact canonical bytes, not just equivalent JSON values")
				require.Equal(t, originalSHA, candidateSHA)
			}
			assertBytes()
			require.NoError(t, tx.QueryRow(ctx, `SELECT native_membership_guard.catalog(),native_membership_guard.catalog()=(`+nativeCatalog44LiteralExpression+`)`).Scan(&before, &equal))
			require.True(t, equal)
			_, err = tx.Exec(ctx, tc.ddl)
			require.NoError(t, err, "install the actual catalog counterexample")
			require.NoError(t, tx.QueryRow(ctx, `SELECT native_membership_guard.catalog(),native_membership_guard.catalog()=(`+nativeCatalog44LiteralExpression+`)`).Scan(&after, &equal))
			require.True(t, equal, "complete original metadata/null/order projection stays identical")
			require.NotEqual(t, before, after, "both projections observe actual catalog drift")
			assertBytes()
		})
	}
}
