package partycoverage

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"voice/backend/voice/internal/retirementproof"
)

func TestPrivilegeAdmissionRejectsColumnWriteAuthority(t *testing.T) {
	ctx, admin, schema := coverageDB(t)
	up, err := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "voice_db", "000016_party_resource_retirement.up.sql"))
	require.NoError(t, err)
	_, err = admin.Exec(ctx, string(up))
	require.NoError(t, err)
	role := "column_writer_" + uuid.NewString()[:8]
	_, err = admin.Exec(ctx, "CREATE ROLE "+pgx.Identifier{role}.Sanitize())
	require.NoError(t, err)
	t.Cleanup(func() {
		admin.Exec(context.Background(), "DROP OWNED BY "+pgx.Identifier{role}.Sanitize())
		admin.Exec(context.Background(), "DROP ROLE "+pgx.Identifier{role}.Sanitize())
	})
	_, err = admin.Exec(ctx, "GRANT USAGE ON SCHEMA "+pgx.Identifier{schema}.Sanitize()+" TO "+pgx.Identifier{role}.Sanitize())
	require.NoError(t, err)
	_, err = admin.Exec(ctx, "GRANT SELECT ON ALL TABLES IN SCHEMA "+pgx.Identifier{schema}.Sanitize()+" TO "+pgx.Identifier{role}.Sanitize())
	require.NoError(t, err)
	runtime := loginRoleConnection(t, ctx, admin, schema, role)
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
	for _, privilege := range []string{"INSERT", "UPDATE"} {
		t.Run(privilege, func(t *testing.T) {
			_, err := admin.Exec(ctx, "GRANT "+privilege+"(protocol) ON voice_party_resource_retirements TO "+pgx.Identifier{role}.Sanitize())
			require.NoError(t, err)
			var tableWrite, columnWrite bool
			require.NoError(t, runtime.QueryRow(ctx, `SELECT has_table_privilege(current_user,'voice_party_resource_retirements',$1),has_any_column_privilege(current_user,'voice_party_resource_retirements',$1)`, privilege).Scan(&tableWrite, &columnWrite))
			require.False(t, tableWrite)
			require.True(t, columnWrite)
			assert.ErrorIs(t, check(CheckRuntimePrivileges), ErrUnavailable)
			assert.ErrorIs(t, check(reader.checkReader), retirementproof.ErrUnavailable)
			_, err = admin.Exec(ctx, "REVOKE "+privilege+"(protocol) ON voice_party_resource_retirements FROM "+pgx.Identifier{role}.Sanitize())
			require.NoError(t, err)
		})
	}
}
