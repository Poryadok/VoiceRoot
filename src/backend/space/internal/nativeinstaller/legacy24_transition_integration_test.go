//go:build integration

package nativeinstaller

import (
	"context"
	"net/url"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

// Uses an owned private server only; it never discovers or resets a shared DB.
func TestLegacy24Exact25TransitionPreservesSourceRows(t *testing.T) {
	adminURL, driver := os.Getenv("NATIVE_INSTALLER_TEST_ADMIN_URL"), os.Getenv("NATIVE_INSTALLER_TEST_MIGRATE")
	require.NotEmpty(t, adminURL)
	require.NotEmpty(t, driver)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	admin, err := pgx.Connect(ctx, adminURL)
	require.NoError(t, err)
	t.Cleanup(func() { _ = admin.Close(context.Background()) })
	_, err = admin.Exec(ctx, `CREATE DATABASE space_db`)
	require.NoError(t, err)
	var oid uint32
	require.NoError(t, admin.QueryRow(ctx, `SELECT oid FROM pg_database WHERE datname='space_db'`).Scan(&oid))
	t.Cleanup(func() {
		c, done := context.WithTimeout(context.Background(), 10*time.Second)
		defer done()
		var actual uint32
		require.NoError(t, admin.QueryRow(c, `SELECT oid FROM pg_database WHERE datname='space_db'`).Scan(&actual))
		require.Equal(t, oid, actual)
		_, e := admin.Exec(c, `DROP DATABASE space_db`)
		require.NoError(t, e)
	})
	target, err := url.Parse(adminURL)
	require.NoError(t, err)
	target.Path = "/space_db"
	migrate := func(version string) {
		cmd := exec.CommandContext(ctx, driver, "-path", "../../../migrations/space_db", "-database", target.String(), "goto", version)
		output, e := cmd.CombinedOutput()
		require.NoError(t, e, "pinned migrate failed: %s", output)
	}
	migrate("24")
	conn, err := pgx.Connect(ctx, target.String())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close(context.Background()) })
	const space = "11111111-1111-4111-8111-111111111111"
	const owner = "22222222-2222-4222-8222-222222222222"
	const member = "33333333-3333-4333-8333-333333333333"
	_, err = conn.Exec(ctx, `INSERT INTO public.spaces(id,name,owner_profile_id) VALUES($1,'legacy fixture',$2)`, space, owner)
	require.NoError(t, err)
	_, err = conn.Exec(ctx, `INSERT INTO public.space_members(space_id,profile_id,nickname) VALUES($1,$2,'member fixture')`, space, member)
	require.NoError(t, err)
	dataHash := func() string {
		var value string
		require.NoError(t, conn.QueryRow(ctx, `SELECT encode(sha256(convert_to(jsonb_build_object('spaces',(SELECT jsonb_agg(to_jsonb(s) ORDER BY s.id) FROM public.spaces s),'members',(SELECT jsonb_agg(to_jsonb(m) ORDER BY m.space_id,m.profile_id) FROM public.space_members m))::text,'UTF8')),'hex')`).Scan(&value))
		return value
	}
	var version int
	var dirty bool
	require.NoError(t, conn.QueryRow(ctx, `SELECT version,dirty FROM public.schema_migrations`).Scan(&version, &dirty))
	require.Equal(t, 24, version)
	require.False(t, dirty)
	catalog, preimage, err := captureLegacy24Snapshot(ctx, conn)
	require.NoError(t, err)
	require.Equal(t, legacy24CatalogPG16SHA256, catalog)
	origin, err := observeLegacy24Origin(ctx, conn)
	require.NoError(t, err)
	require.Equal(t, oid, origin.Target.DatabaseOID)
	require.EqualValues(t, 24, origin.Target.Version)
	require.NotEmpty(t, origin.Target.DatabaseSystemID)
	require.Equal(t, preimage, origin.Rows)
	require.Len(t, origin.PreimageSHA256, 64)
	require.Len(t, origin.AccessSHA256, 64)
	aclTx, err := conn.Begin(ctx)
	require.NoError(t, err)
	_, err = aclTx.Exec(ctx, `SET LOCAL search_path=pg_catalog; GRANT SELECT ON public.spaces TO PUBLIC`)
	require.NoError(t, err)
	changedAccess, err := captureLegacyAccessTx(ctx, aclTx)
	require.NoError(t, err)
	require.NotEqual(t, origin.AccessSHA256, changedAccess, "grant drift changes private access observation")
	require.NoError(t, aclTx.Rollback(ctx))
	restoredAccess, err := observeLegacy24Origin(ctx, conn)
	require.NoError(t, err)
	require.Equal(t, origin.AccessSHA256, restoredAccess.AccessSHA256)
	_, err = inspectLegacy25Catalog(ctx, conn)
	require.Error(t, err, "24 must never qualify as original25")
	_, err = conn.Exec(ctx, `UPDATE public.schema_migrations SET dirty=true`)
	require.NoError(t, err)
	_, _, err = captureLegacy24Snapshot(ctx, conn)
	require.Error(t, err, "dirty source cannot be admitted")
	_, err = conn.Exec(ctx, `UPDATE public.schema_migrations SET dirty=false`)
	require.NoError(t, err)
	_, err = conn.Exec(ctx, `ALTER TABLE public.spaces DISABLE TRIGGER space_authority_revision_spaces`)
	require.NoError(t, err)
	_, _, err = captureLegacy24Snapshot(ctx, conn)
	require.Error(t, err, "disabled authority trigger cannot be admitted")
	_, err = conn.Exec(ctx, `ALTER TABLE public.spaces ENABLE TRIGGER space_authority_revision_spaces`)
	require.NoError(t, err)
	before := dataHash()
	require.NoError(t, conn.Close(ctx))
	migrate("25")
	conn, err = pgx.Connect(ctx, target.String())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close(context.Background()) })
	require.NoError(t, conn.QueryRow(ctx, `SELECT version,dirty FROM public.schema_migrations`).Scan(&version, &dirty))
	require.Equal(t, 25, version)
	require.False(t, dirty)
	require.Equal(t, before, dataHash())
	tx, err := beginLegacySourceRead(ctx, conn)
	require.NoError(t, err)
	_, err = inspectLegacy25CatalogTx(ctx, tx)
	require.NoError(t, err, "actual intermediate25 structure")
	postimage, err := captureLegacy25ProjectionTx(ctx, tx, preimage)
	require.NoError(t, err)
	access, err := captureLegacyAccessTx(ctx, tx)
	require.NoError(t, err)
	require.Equal(t, origin.AccessSHA256, access, "source owners, ACLs and database properties survive 25")
	require.NoError(t, tx.Commit(ctx))
	_, _, err = captureLegacy24Snapshot(ctx, conn)
	require.Error(t, err, "25 must not be relabelled as original24")
	require.Equal(t, preimage, postimage, "all original table rows and columns survive the exact 24-to-25 transition")
	var observed uint32
	require.NoError(t, admin.QueryRow(ctx, `SELECT oid FROM pg_database WHERE datname='space_db'`).Scan(&observed))
	require.Equal(t, oid, observed)
}
