package store

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/integrationtest"
)

// The source-disabled seam must be an additive migration, not a test-only schema.
func TestEntitlementOutboxMigration(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL integration")
	}
	ctx := context.Background()
	_, current, _, ok := runtime.Caller(0)
	require.True(t, ok)
	path := filepath.Join(filepath.Dir(current), "../../../migrations/subscription_db/000004_entitlement_outbox.up.sql")
	body, err := os.ReadFile(path)
	require.NoError(t, err)
	pool := integrationtest.StartPostgres(t, ctx, "entitlement_schema", "")
	_, err = pool.Exec(ctx, string(body))
	require.NoError(t, err)
	for _, table := range []string{"subscription_entitlement_aggregates", "subscription_event_outbox"} {
		var name string
		require.NoError(t, pool.QueryRow(ctx, "SELECT to_regclass($1)::text", table).Scan(&name))
		require.Equal(t, table, name)
	}
	// An invalid revision cannot enter the persistent authority.
	_, err = pool.Exec(ctx, `INSERT INTO subscription_entitlement_aggregates
		(aggregate_kind, aggregate_id, aggregate_revision, payload, payload_hash)
		VALUES (1, gen_random_uuid(), 1, '\x01', decode(repeat('00',32),'hex'))`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE subscription_entitlement_aggregates SET aggregate_revision=-1`)
	var pgErr *pgconn.PgError
	require.ErrorAs(t, err, &pgErr)
	require.Equal(t, "23514", pgErr.Code)
}
