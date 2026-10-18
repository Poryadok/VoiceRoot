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

func TestPrivilegeAdmissionRejectsAdministrativeSessionHiddenBySetRole(t *testing.T) {
	ctx, c, schema := coverageDB(t)
	up, err := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "voice_db", "000016_party_resource_retirement.up.sql"))
	require.NoError(t, err)
	_, err = c.Exec(ctx, string(up))
	require.NoError(t, err)
	role := "session_mask_" + uuid.NewString()[:8]
	_, err = c.Exec(ctx, "CREATE ROLE "+pgx.Identifier{role}.Sanitize())
	require.NoError(t, err)
	t.Cleanup(func() {
		c.Exec(context.Background(), "ROLLBACK")
		c.Exec(context.Background(), "RESET ROLE")
		c.Exec(context.Background(), "DROP OWNED BY "+pgx.Identifier{role}.Sanitize())
		c.Exec(context.Background(), "DROP ROLE "+pgx.Identifier{role}.Sanitize())
	})
	_, err = c.Exec(ctx, "GRANT USAGE ON SCHEMA "+pgx.Identifier{schema}.Sanitize()+" TO "+pgx.Identifier{role}.Sanitize())
	require.NoError(t, err)
	_, err = c.Exec(ctx, "GRANT SELECT ON ALL TABLES IN SCHEMA "+pgx.Identifier{schema}.Sanitize()+" TO "+pgx.Identifier{role}.Sanitize())
	require.NoError(t, err)
	var owner uint32
	require.NoError(t, c.QueryRow(ctx, `SELECT relowner FROM pg_class WHERE oid='voice_party_resource_retirements'::regclass`).Scan(&owner))
	_, err = c.Exec(ctx, "SET ROLE "+pgx.Identifier{role}.Sanitize())
	require.NoError(t, err)
	t.Run("serving", func(t *testing.T) {
		tx, err := c.Begin(ctx)
		require.NoError(t, err)
		err = CheckRuntimePrivileges(ctx, tx)
		require.NoError(t, tx.Rollback(ctx))
		require.ErrorIs(t, err, ErrUnavailable, "actual session credential can RESET ROLE and mint")
	})
	// The protected reader must reject the session before looking up even an
	// unknown proof. checkReader is tested directly to disambiguate no-row deny.
	reader, err := NewResourceProofReader(c, schema, owner)
	require.NoError(t, err)
	t.Run("issuer-reader", func(t *testing.T) {
		tx, err := c.Begin(ctx)
		require.NoError(t, err)
		err = reader.checkReader(ctx, tx)
		require.NoError(t, tx.Rollback(ctx))
		require.ErrorIs(t, err, retirementproof.ErrUnavailable)
	})
	_, err = c.Exec(ctx, "RESET ROLE")
	require.NoError(t, err)
	_, err = c.Exec(ctx, "SET SESSION AUTHORIZATION "+pgx.Identifier{role}.Sanitize())
	require.NoError(t, err)
	defer c.Exec(context.Background(), "RESET SESSION AUTHORIZATION")
	var authenticated, current string
	require.NoError(t, c.QueryRow(ctx, `SELECT usename,current_user FROM pg_stat_activity WHERE pid=pg_backend_pid()`).Scan(&authenticated, &current))
	require.NotEqual(t, authenticated, current, "backend login identity must remain observable despite session masking")
	t.Run("serving-authenticated-login", func(t *testing.T) {
		tx, err := c.Begin(ctx)
		require.NoError(t, err)
		err = CheckRuntimePrivileges(ctx, tx)
		require.NoError(t, tx.Rollback(ctx))
		require.ErrorIs(t, err, ErrUnavailable)
	})
	t.Run("reader-authenticated-login", func(t *testing.T) {
		tx, err := c.Begin(ctx)
		require.NoError(t, err)
		err = reader.checkReader(ctx, tx)
		require.NoError(t, tx.Rollback(ctx))
		require.ErrorIs(t, err, retirementproof.ErrUnavailable)
	})
}
