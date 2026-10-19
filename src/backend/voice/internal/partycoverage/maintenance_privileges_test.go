package partycoverage

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

func TestMaintenanceSQLScopeChecksActualLoginAndFiniteGrants(t *testing.T) {
	ctx, admin, schema := coverageDB(t)
	_, err := admin.Exec(ctx, `CREATE TABLE native_cleanup_fixture(value int);CREATE TABLE unrelated_fixture(value int)`)
	require.NoError(t, err)
	role := "maintenance_scope_" + uuid.NewString()[:8]
	_, err = admin.Exec(ctx, "CREATE ROLE "+pgx.Identifier{role}.Sanitize())
	require.NoError(t, err)
	t.Cleanup(func() {
		admin.Exec(context.Background(), "RESET ROLE")
		admin.Exec(context.Background(), "DROP OWNED BY "+pgx.Identifier{role}.Sanitize())
		admin.Exec(context.Background(), "DROP ROLE "+pgx.Identifier{role}.Sanitize())
	})
	_, err = admin.Exec(ctx, "GRANT USAGE ON SCHEMA "+pgx.Identifier{schema}.Sanitize()+" TO "+pgx.Identifier{role}.Sanitize())
	require.NoError(t, err)
	_, err = admin.Exec(ctx, "GRANT SELECT,INSERT,UPDATE,DELETE ON native_cleanup_fixture TO "+pgx.Identifier{role}.Sanitize())
	require.NoError(t, err)
	runtime := loginRoleConnection(t, ctx, admin, schema, role)
	var scope maintenanceSQLScope
	scope.RoleName = role
	var target, private uint32
	require.NoError(t, admin.QueryRow(ctx, `SELECT $1::regrole::oid,$2::regnamespace::oid,'native_cleanup_fixture'::regclass::oid,'voice_party_coverage_head'::regclass::oid`, role, schema).Scan(&scope.RoleOID, &scope.SchemaOID, &target, &private))
	scope.WritableRelations = []uint32{target}
	check := func(conn *pgx.Conn, s maintenanceSQLScope) error {
		tx, err := conn.Begin(ctx)
		require.NoError(t, err)
		result := checkMaintenancePrivileges(ctx, tx, s)
		require.NoError(t, tx.Rollback(ctx))
		return result
	}
	require.NoError(t, check(runtime, scope), "finite SQL fixture only, not a production maintenance authority")
	t.Run("administrative login masked by role", func(t *testing.T) {
		_, err := admin.Exec(ctx, "SET ROLE "+pgx.Identifier{role}.Sanitize())
		require.NoError(t, err)
		require.ErrorIs(t, check(admin, scope), ErrUnavailable)
		_, err = admin.Exec(ctx, "RESET ROLE")
		require.NoError(t, err)
	})
	t.Run("column write outside scope", func(t *testing.T) {
		_, err := admin.Exec(ctx, "GRANT UPDATE(value) ON unrelated_fixture TO "+pgx.Identifier{role}.Sanitize())
		require.NoError(t, err)
		require.ErrorIs(t, check(runtime, scope), ErrUnavailable)
		_, err = admin.Exec(ctx, "REVOKE UPDATE(value) ON unrelated_fixture FROM "+pgx.Identifier{role}.Sanitize())
		require.NoError(t, err)
	})
	t.Run("private issuer table cannot be admitted", func(t *testing.T) {
		other := scope
		other.WritableRelations = []uint32{private}
		require.ErrorIs(t, check(runtime, other), ErrUnavailable)
	})
	t.Run("wrong role identity", func(t *testing.T) {
		other := scope
		other.RoleOID++
		require.ErrorIs(t, check(runtime, other), ErrUnavailable)
	})
	for _, permission := range []struct{ name, grant, revoke string }{
		{"table write outside scope", "GRANT INSERT ON unrelated_fixture TO ", "REVOKE INSERT ON unrelated_fixture FROM "},
		{"truncate admitted table", "GRANT TRUNCATE ON native_cleanup_fixture TO ", "REVOKE TRUNCATE ON native_cleanup_fixture FROM "},
		{"schema create", "GRANT CREATE ON SCHEMA " + pgx.Identifier{schema}.Sanitize() + " TO ", "REVOKE CREATE ON SCHEMA " + pgx.Identifier{schema}.Sanitize() + " FROM "},
	} {
		t.Run(permission.name, func(t *testing.T) {
			_, err := admin.Exec(ctx, permission.grant+pgx.Identifier{role}.Sanitize())
			require.NoError(t, err)
			require.ErrorIs(t, check(runtime, scope), ErrUnavailable)
			_, err = admin.Exec(ctx, permission.revoke+pgx.Identifier{role}.Sanitize())
			require.NoError(t, err)
		})
	}
	t.Run("callable definer", func(t *testing.T) {
		_, err := admin.Exec(ctx, `CREATE FUNCTION maintenance_fixture_definer() RETURNS int LANGUAGE SQL SECURITY DEFINER AS 'SELECT 1'`)
		require.NoError(t, err)
		require.ErrorIs(t, check(runtime, scope), ErrUnavailable)
		_, err = admin.Exec(ctx, `DROP FUNCTION maintenance_fixture_definer()`)
		require.NoError(t, err)
	})
	require.NoError(t, check(runtime, scope), "negative fixture grants were removed")
}
