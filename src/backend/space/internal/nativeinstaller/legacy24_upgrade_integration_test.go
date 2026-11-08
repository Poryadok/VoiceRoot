//go:build integration

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

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

// Requires a fresh private server and source-bound binaries; never discovers a
// shared target. This is an acceptance test, not a live deployment operation.
func TestProductionLegacy24PopulatedChain(t *testing.T) {
	adminURL, driver, binary := os.Getenv("NATIVE_INSTALLER_TEST_ADMIN_URL"), os.Getenv("NATIVE_INSTALLER_TEST_MIGRATE"), os.Getenv("NATIVE_INSTALLER_TEST_BINARY")
	require.NotEmpty(t, adminURL)
	require.NotEmpty(t, driver)
	require.NotEmpty(t, binary)
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
	cmd := exec.CommandContext(ctx, driver, "-path", "../../../migrations/space_db", "-database", target.String(), "goto", "24")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "pinned migration to24 failed: %s", output)
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
	var version int
	var dirty bool
	require.NoError(t, conn.QueryRow(ctx, `SELECT version,dirty FROM public.schema_migrations`).Scan(&version, &dirty))
	require.Equal(t, 24, version)
	require.False(t, dirty)
	require.NoError(t, conn.Close(ctx))
	cmd = exec.CommandContext(ctx, binary, "-mode", "upgrade-legacy24", "-migrations-dir", "../../../migrations/space_db", "-migrate", driver, "-operation-id", "owned-legacy24-integration", "-snapshot-id", "owned-backup-fixture", "-origin-evidence", filepath.Join(t.TempDir(), "original.json"))
	cmd.Env = append(os.Environ(), "SPACE_NATIVE_ADMIN_URL="+adminURL)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err = cmd.Run()
	require.NoError(t, err, "legacy24 upgrade: %s", stderr.String())
	require.Contains(t, stdout.String(), `"event":"trusted25"`)
	require.Contains(t, stdout.String(), `"event":"upgrade-complete"`)
	conn, err = pgx.Connect(ctx, target.String())
	require.NoError(t, err)
	var count int
	require.NoError(t, conn.QueryRow(ctx, `SELECT count(*) FROM public.spaces WHERE id=$1 AND owner_profile_id=$2`, space, owner).Scan(&count))
	require.Equal(t, 1, count)
	require.NoError(t, conn.QueryRow(ctx, `SELECT count(*) FROM public.space_members WHERE space_id=$1 AND profile_id=$2 AND nickname='member fixture'`, space, member).Scan(&count))
	require.Equal(t, 1, count)
	require.NoError(t, conn.QueryRow(ctx, `SELECT version,dirty FROM public.schema_migrations`).Scan(&version, &dirty))
	require.Equal(t, 51, version)
	require.False(t, dirty)
}
