//go:build integration && linux

package nativeinstaller

import (
	"context"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

// A private PostgreSQL server and pinned PG16 dump/restore executables are
// supplied by the test owner. No live database or shared target is discovered.
func TestLegacy24LogicalRestoreCanonicalEquivalence(t *testing.T) {
	adminURL, driver := os.Getenv("NATIVE_INSTALLER_TEST_ADMIN_URL"), os.Getenv("NATIVE_INSTALLER_TEST_MIGRATE")
	dump, restore := os.Getenv("NATIVE_INSTALLER_TEST_PG_DUMP"), os.Getenv("NATIVE_INSTALLER_TEST_PG_RESTORE")
	if dump == "" || restore == "" {
		t.Skip("owned PG16 dump and restore tools not supplied")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	admin, err := pgx.Connect(ctx, adminURL)
	require.NoError(t, err)
	t.Cleanup(func() { _ = admin.Close(context.Background()) })
	owned := map[string]uint32{}
	create := func(name string) {
		_, e := admin.Exec(ctx, `CREATE DATABASE `+pgx.Identifier{name}.Sanitize())
		require.NoError(t, e)
		var oid uint32
		require.NoError(t, admin.QueryRow(ctx, `SELECT oid FROM pg_database WHERE datname=$1`, name).Scan(&oid))
		owned[name] = oid
	}
	t.Cleanup(func() {
		c, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		for name, oid := range owned {
			var actual uint32
			require.NoError(t, admin.QueryRow(c, `SELECT oid FROM pg_database WHERE datname=$1`, name).Scan(&actual))
			require.Equal(t, oid, actual)
			_, e := admin.Exec(c, `DROP DATABASE `+pgx.Identifier{name}.Sanitize())
			require.NoError(t, e)
		}
	})
	create("space_db")
	target, err := url.Parse(adminURL)
	require.NoError(t, err)
	target.Path = "/space_db"
	migrate := func(version string) {
		cmd := exec.CommandContext(ctx, driver, "-path", "../../../migrations/space_db", "-database", target.String(), "goto", version)
		output, e := cmd.CombinedOutput()
		require.NoError(t, e, "owned fixture migration failed: %s", output)
	}
	migrate("5")
	conn, err := pgx.Connect(ctx, target.String())
	require.NoError(t, err)
	_, err = conn.Exec(ctx, `INSERT INTO public.spaces(id,name,owner_profile_id) VALUES ('11111111-1111-4111-8111-111111111111','before column','22222222-2222-4222-8222-222222222222')`)
	require.NoError(t, err)
	require.NoError(t, conn.Close(ctx))
	migrate("6")
	conn, err = pgx.Connect(ctx, target.String())
	require.NoError(t, err)
	var originalDefault bool
	require.NoError(t, conn.QueryRow(ctx, `SELECT allow_guests FROM public.spaces WHERE id='11111111-1111-4111-8111-111111111111'`).Scan(&originalDefault))
	require.True(t, originalDefault, "old row observes original fast default")
	require.NoError(t, conn.Close(ctx))
	migrate("24")
	conn, err = pgx.Connect(ctx, target.String())
	require.NoError(t, err)
	require.NoError(t, conn.QueryRow(ctx, `SELECT allow_guests FROM public.spaces WHERE id='11111111-1111-4111-8111-111111111111'`).Scan(&originalDefault))
	require.False(t, originalDefault, "migration16 materializes the closed default")
	_, err = conn.Exec(ctx, `UPDATE public.spaces SET allow_guests=true WHERE id='11111111-1111-4111-8111-111111111111'; INSERT INTO public.spaces(id,name,owner_profile_id,allow_guests) VALUES ('55555555-5555-4555-8555-555555555555','new false','22222222-2222-4222-8222-222222222222',false)`)
	require.NoError(t, err)
	original, err := observeLegacy24Origin(ctx, conn)
	require.NoError(t, err)
	require.NoError(t, conn.Close(ctx))
	host := target.Hostname()
	if socket := target.Query().Get("host"); socket != "" {
		host = socket
	}
	pgEnv := append(os.Environ(), "PGHOST="+host, "PGPORT="+target.Port(), "PGUSER="+target.User.Username())
	if password, ok := target.User.Password(); ok {
		pgEnv = append(pgEnv, "PGPASSWORD="+password)
	}
	archive := filepath.Join(t.TempDir(), "original.dump")
	roundtrip := func(oldName string) {
		cmd := exec.CommandContext(ctx, dump, "--format=custom", "--file", archive, "space_db")
		cmd.Env = pgEnv
		require.NoError(t, cmd.Run(), "private dump failed")
		create("space_clone")
		cmd = exec.CommandContext(ctx, restore, "--dbname", "space_clone", archive)
		cmd.Env = pgEnv
		require.NoError(t, cmd.Run(), "private restore failed")
		_, e := admin.Exec(ctx, `ALTER DATABASE space_db RENAME TO `+pgx.Identifier{oldName}.Sanitize())
		require.NoError(t, e)
		owned[oldName] = owned["space_db"]
		delete(owned, "space_db")
		_, e = admin.Exec(ctx, `ALTER DATABASE space_clone RENAME TO space_db`)
		require.NoError(t, e)
		owned["space_db"] = owned["space_clone"]
		delete(owned, "space_clone")
	}
	roundtrip("space_original24")
	conn, err = pgx.Connect(ctx, target.String())
	require.NoError(t, err)
	cloned, err := observeLegacy24Origin(ctx, conn)
	require.NoError(t, err, "restored clean24 must qualify")
	require.Equal(t, original.CatalogSHA256, cloned.CatalogSHA256)
	require.Equal(t, original.PreimageSHA256, cloned.PreimageSHA256)
	var oldTrue, newFalse bool
	require.NoError(t, conn.QueryRow(ctx, `SELECT (SELECT allow_guests FROM public.spaces WHERE id='11111111-1111-4111-8111-111111111111'),(SELECT NOT allow_guests FROM public.spaces WHERE id='55555555-5555-4555-8555-555555555555')`).Scan(&oldTrue, &newFalse))
	require.True(t, oldTrue)
	require.True(t, newFalse)
	drift, err := conn.Begin(ctx)
	require.NoError(t, err)
	_, err = drift.Exec(ctx, `UPDATE public.spaces SET allow_guests=false WHERE id='11111111-1111-4111-8111-111111111111'`)
	require.NoError(t, err)
	changedRows, err := captureLegacy25ProjectionTx(ctx, drift, original.Rows)
	require.NoError(t, err)
	require.NotEqual(t, original.Rows["spaces"].SHA256, changedRows["spaces"].SHA256, "actual old true row cannot silently become false")
	require.NoError(t, drift.Rollback(ctx))
	require.NoError(t, conn.Close(ctx))
	migrate("25")
	conn, err = pgx.Connect(ctx, target.String())
	require.NoError(t, err)
	_, err = inspectLegacy25Catalog(ctx, conn)
	require.NoError(t, err)
	require.NoError(t, conn.Close(ctx))
	roundtrip("space_original25")
	conn, err = pgx.Connect(ctx, target.String())
	require.NoError(t, err)
	defer conn.Close(context.Background())
	_, err = inspectLegacy25Catalog(ctx, conn)
	require.NoError(t, err, "restored clean25 must qualify")
}
