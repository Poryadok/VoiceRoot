//go:build integration && linux

package nativeinstaller

import (
	"bytes"
	"context"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"voice/backend/space/internal/store"
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
	archiveDir := t.TempDir()
	roundtrip := func(oldName string) {
		archive := filepath.Join(archiveDir, oldName+".dump")
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
	_, err = verifyLegacy24Intermediate(ctx, conn, cloned)
	require.NoError(t, err)
	require.NoError(t, conn.Close(ctx))
	roundtrip("space_original25")
	conn, err = pgx.Connect(ctx, target.String())
	require.NoError(t, err)
	_, err = inspectLegacy25Catalog(ctx, conn)
	require.NoError(t, err, "restored clean25 must qualify")
	require.NoError(t, conn.Close(ctx))
	// Continue the actual original24 clone, whose OID is bound to `cloned`.
	_, err = admin.Exec(ctx, `ALTER DATABASE space_db RENAME TO space_restored25`)
	require.NoError(t, err)
	owned["space_restored25"] = owned["space_db"]
	delete(owned, "space_db")
	_, err = admin.Exec(ctx, `ALTER DATABASE space_original25 RENAME TO space_db`)
	require.NoError(t, err)
	owned["space_db"] = owned["space_original25"]
	delete(owned, "space_original25")
	migrate("44")
	pool, err := pgxpool.New(ctx, target.String())
	require.NoError(t, err)
	defer pool.Close()
	s := &store.SpaceStore{Pool: pool}
	f44, err := s.InspectNativeInstallerSource(ctx, 44)
	require.NoError(t, err, "restored source must qualify at native44")
	migrate("45")
	t45, err := s.TransitionNativeMembershipCatalog45(ctx)
	require.NoError(t, err)
	require.True(t, matchesTransition(f44, t45))
	f45, err := s.InspectNativeInstallerSource(ctx, 45)
	require.NoError(t, err)
	migrate("50")
	t50, err := s.TransitionNativeMembershipCatalog50(ctx)
	require.NoError(t, err)
	require.True(t, matchesTransition(f45, (*store.NativeMembershipCatalog45Receipt)(t50)))
	f50, err := s.InspectNativeInstallerSource(ctx, 50)
	require.NoError(t, err)
	t51, err := runNativeCatalog51(ctx, s, f50, "../../../migrations/space_db", func(context.Context) error { return nil })
	require.NoError(t, err)
	require.True(t, matchesTransition(f50, &t51.NativeMembershipCatalog45Receipt))
	f51, err := s.InspectNativeInstallerSource(ctx, 51)
	require.NoError(t, err)
	require.Equal(t, t51.NewSealSHA256, f51.SealSHA256)
	for _, id := range cloned.Spaces {
		require.NoError(t, s.InitializeNativeMembership(ctx, uuid.MustParse(id)))
	}
	conn, err = pgx.Connect(ctx, target.String())
	require.NoError(t, err)
	_, err = verifyLegacy24Preserved(ctx, conn, cloned)
	require.NoError(t, err, "restored original rows and rights survive native initialization")
	require.NoError(t, conn.Close(ctx))
	pool.Close()
	if binary := os.Getenv("NATIVE_INSTALLER_TEST_BINARY"); binary != "" {
		cliAdmin := os.Getenv("NATIVE_INSTALLER_TEST_CLI_ADMIN_URL")
		require.NotEmpty(t, cliAdmin)
		_, err = admin.Exec(ctx, `ALTER DATABASE space_db RENAME TO space_upgraded`)
		require.NoError(t, err)
		owned["space_upgraded"] = owned["space_db"]
		delete(owned, "space_db")
		create("space_db")
		restoreCmd := exec.CommandContext(ctx, restore, "--dbname", "space_db", filepath.Join(archiveDir, "space_original24.dump"))
		restoreCmd.Env = pgEnv
		require.NoError(t, restoreCmd.Run())
		private := filepath.Join(t.TempDir(), "original.json")
		cli := exec.CommandContext(ctx, binary, "-mode", "upgrade-legacy24", "-migrations-dir", "../../../migrations/space_db", "-migrate", driver, "-operation-id", "owned-clone-test", "-snapshot-id", "owned-clone-backup", "-origin-evidence", private)
		cli.Env = append(os.Environ(), "SPACE_NATIVE_ADMIN_URL="+cliAdmin)
		var out, diag bytes.Buffer
		cli.Stdout, cli.Stderr = &out, &diag
		require.NoError(t, cli.Run(), "actual source-bound clone upgrade failed: %s", diag.String())
		receipt, err := ReadLegacy24Receipt(bytes.NewReader(out.Bytes()))
		require.NoError(t, err)
		require.Equal(t, owned["space_db"], receipt.Source24.DatabaseOID)
		require.Equal(t, original.PreimageSHA256, receipt.PreimageSHA256)
		require.Equal(t, uint64(2), receipt.InitializationCount)
	}
}
