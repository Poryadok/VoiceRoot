package partycoverage

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"voice/backend/voice/internal/retirementproof"
)

func TestPrivilegeAdmissionRejectsCallableDefinerAuthorityChain(t *testing.T) {
	ctx, admin, schema := coverageDB(t)
	up, err := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "voice_db", "000016_party_resource_retirement.up.sql"))
	require.NoError(t, err)
	_, err = admin.Exec(ctx, string(up))
	require.NoError(t, err)
	runtimeRole := "chain_runtime_" + uuid.NewString()[:8]
	bridgeRole := "chain_bridge_" + uuid.NewString()[:8]
	for _, role := range []string{runtimeRole, bridgeRole} {
		_, err = admin.Exec(ctx, "CREATE ROLE "+pgx.Identifier{role}.Sanitize())
		require.NoError(t, err)
	}
	t.Cleanup(func() {
		admin.Exec(context.Background(), "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE")
		for _, role := range []string{runtimeRole, bridgeRole} {
			admin.Exec(context.Background(), "DROP OWNED BY "+pgx.Identifier{role}.Sanitize())
			admin.Exec(context.Background(), "DROP ROLE "+pgx.Identifier{role}.Sanitize())
		}
	})
	for _, role := range []string{runtimeRole, bridgeRole} {
		_, err = admin.Exec(ctx, "GRANT USAGE ON SCHEMA "+pgx.Identifier{schema}.Sanitize()+" TO "+pgx.Identifier{role}.Sanitize())
		require.NoError(t, err)
	}
	_, err = admin.Exec(ctx, "GRANT SELECT ON ALL TABLES IN SCHEMA "+pgx.Identifier{schema}.Sanitize()+" TO "+pgx.Identifier{runtimeRole}.Sanitize())
	require.NoError(t, err)
	runtime := loginRoleConnection(t, ctx, admin, schema, runtimeRole)
	var owner uint32
	require.NoError(t, admin.QueryRow(ctx, `SELECT relowner FROM pg_class WHERE oid='voice_party_resource_retirements'::regclass`).Scan(&owner))
	reader, err := NewResourceProofReader(runtime, schema, owner)
	require.NoError(t, err)
	check := func(f func(context.Context, pgx.Tx) error) error {
		tx, err := runtime.Begin(ctx)
		require.NoError(t, err)
		result := f(ctx, tx)
		require.NoError(t, tx.Rollback(ctx))
		return result
	}
	require.NoError(t, check(CheckRuntimePrivileges))
	require.NoError(t, check(reader.checkReader))
	// Harmless function bodies: this verifies reachable definer authority,
	// without executing a privileged write. The restricted bridge has no table
	// DML and runtime cannot call the issuer-owned function directly.
	upper := pgx.Identifier{schema, "issuer_context"}.Sanitize()
	lower := pgx.Identifier{schema, "restricted_bridge"}.Sanitize()
	_, err = admin.Exec(ctx, "CREATE FUNCTION "+upper+"() RETURNS int LANGUAGE SQL SECURITY DEFINER AS 'SELECT 1'")
	require.NoError(t, err)
	_, err = admin.Exec(ctx, "REVOKE ALL ON FUNCTION "+upper+"() FROM PUBLIC")
	require.NoError(t, err)
	_, err = admin.Exec(ctx, "GRANT EXECUTE ON FUNCTION "+upper+"() TO "+pgx.Identifier{bridgeRole}.Sanitize())
	require.NoError(t, err)
	_, err = admin.Exec(ctx, "CREATE FUNCTION "+lower+"() RETURNS int LANGUAGE SQL SECURITY DEFINER AS 'SELECT "+upper+"()'")
	require.NoError(t, err)
	_, err = admin.Exec(ctx, "ALTER FUNCTION "+lower+"() OWNER TO "+pgx.Identifier{bridgeRole}.Sanitize())
	require.NoError(t, err)
	var one int
	require.NoError(t, runtime.QueryRow(ctx, "SELECT "+lower+"()").Scan(&one))
	require.Equal(t, 1, one)
	t.Run("serving", func(t *testing.T) { require.ErrorIs(t, check(CheckRuntimePrivileges), ErrUnavailable) })
	t.Run("protected-reader", func(t *testing.T) { require.ErrorIs(t, check(reader.checkReader), retirementproof.ErrUnavailable) })
}
