//go:build integration

package nativeinstaller

import (
	"bytes"
	"context"
	"net/url"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

// The caller supplies a private fresh server and source-bound installer and
// migrate binaries. This fixture never discovers or resets shared databases.
func TestProductionLegacy25PopulatedChainAndRetainedVerify(t *testing.T) {
	adminURL := os.Getenv("NATIVE_INSTALLER_TEST_ADMIN_URL")
	driver := os.Getenv("NATIVE_INSTALLER_TEST_MIGRATE")
	binary := os.Getenv("NATIVE_INSTALLER_TEST_BINARY")
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
		cleanup, done := context.WithTimeout(context.Background(), 10*time.Second)
		defer done()
		var current uint32
		require.NoError(t, admin.QueryRow(cleanup, `SELECT oid FROM pg_database WHERE datname='space_db'`).Scan(&current))
		require.Equal(t, oid, current)
		_, e := admin.Exec(cleanup, `DROP DATABASE space_db`)
		require.NoError(t, e)
	})
	target, err := url.Parse(adminURL)
	require.NoError(t, err)
	target.Path = "/space_db"
	cmd := exec.CommandContext(ctx, driver, "-path", "../../../migrations/space_db", "-database", target.String(), "goto", "25")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "canonical migration to 25 failed: %s", output)
	conn, err := pgx.Connect(ctx, target.String())
	require.NoError(t, err)
	initialConn := conn
	t.Cleanup(func() { _ = initialConn.Close(context.Background()) })
	const space = "11111111-1111-4111-8111-111111111111"
	const owner = "22222222-2222-4222-8222-222222222222"
	const member = "33333333-3333-4333-8333-333333333333"
	const room = "44444444-4444-4444-8444-444444444444"
	const node = "55555555-5555-4555-8555-555555555555"
	const banned = "66666666-6666-4666-8666-666666666666"
	_, err = conn.Exec(ctx, `INSERT INTO public.spaces (id,name,owner_profile_id) VALUES ($1,'legacy fixture',$2)`, space, owner)
	require.NoError(t, err)
	_, err = conn.Exec(ctx, `INSERT INTO public.space_members (space_id,profile_id) VALUES ($1,$2)`, space, owner)
	require.NoError(t, err)
	_, err = conn.Exec(ctx, `INSERT INTO public.space_members (space_id,profile_id,nickname) VALUES ($1,$2,'existing member')`, space, member)
	require.NoError(t, err)
	_, err = conn.Exec(ctx, `INSERT INTO public.voice_rooms (id,space_id,name) VALUES ($1,$2,'existing voice')`, room, space)
	require.NoError(t, err)
	_, err = conn.Exec(ctx, `INSERT INTO public.space_tree_nodes (id,space_id,kind,voice_room_id) VALUES ($1,$2,'voice_room',$3)`, node, space, room)
	require.NoError(t, err)
	_, err = conn.Exec(ctx, `INSERT INTO public.space_bans (space_id,account_id,banned_by_profile_id,reason) VALUES ($1,$2,$3,'existing ban')`, space, banned, owner)
	require.NoError(t, err)
	_, err = conn.Exec(ctx, `CREATE SCHEMA unrelated_fixture; CREATE TABLE unrelated_fixture.records (id integer PRIMARY KEY, value text NOT NULL); INSERT INTO unrelated_fixture.records VALUES (7,'preserved')`)
	require.NoError(t, err)
	var version int
	var dirty bool
	require.NoError(t, conn.QueryRow(ctx, `SELECT version,dirty FROM public.schema_migrations`).Scan(&version, &dirty))
	require.Equal(t, 25, version)
	require.False(t, dirty)
	ok := t.Run("structural-inventory", func(t *testing.T) {
		_, err = inspectLegacy25Catalog(ctx, conn)
		require.NoError(t, err, "canonical25 structural inventory")
		_, err = conn.Exec(ctx, `CREATE VIEW public.space_fixture_unexpected AS SELECT id FROM public.spaces`)
		require.NoError(t, err)
		_, err = inspectLegacy25Catalog(ctx, conn)
		require.Error(t, err, "unexpected source-owned namespace object must HOLD")
		_, err = conn.Exec(ctx, `DROP VIEW public.space_fixture_unexpected`)
		require.NoError(t, err)
		_, err = inspectLegacy25Catalog(ctx, conn)
		require.NoError(t, err)
		before, err := captureLegacy25Rows(ctx, conn)
		require.NoError(t, err)
		catalog, combined, err := captureLegacy25Snapshot(ctx, conn)
		require.NoError(t, err)
		require.Equal(t, legacy25CatalogPG16SHA256, catalog)
		require.Equal(t, before, combined)
		again, err := captureLegacy25Rows(ctx, conn)
		require.NoError(t, err)
		require.Equal(t, before, again, "stable snapshot yields identical private digest")
		projected, err := captureLegacy25Projection(ctx, conn, before)
		require.NoError(t, err)
		require.Equal(t, before, projected, "qualified original projection remains stable")
		_, err = conn.Exec(ctx, `UPDATE public.voice_rooms SET name='changed' WHERE id=$1`, room)
		require.NoError(t, err)
		changed, err := captureLegacy25Rows(ctx, conn)
		require.NoError(t, err)
		require.NotEqual(t, before["voice_rooms"].SHA256, changed["voice_rooms"].SHA256)
		_, err = conn.Exec(ctx, `UPDATE public.voice_rooms SET name='existing voice' WHERE id=$1`, room)
		require.NoError(t, err)
		tx, err := conn.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
		require.NoError(t, err)
		_, err = tx.Exec(ctx, `SET LOCAL search_path=pg_catalog; SET LOCAL TIME ZONE 'UTC'; SET LOCAL DateStyle='ISO, YMD'`)
		require.NoError(t, err)
		_, err = inspectLegacy25CatalogTx(ctx, tx)
		require.NoError(t, err)
		snapshot, err := captureLegacy25ProjectionTx(ctx, tx, nil)
		require.NoError(t, err)
		other, err := pgx.Connect(ctx, target.String())
		require.NoError(t, err)
		_, err = other.Exec(ctx, `UPDATE public.voice_rooms SET name='concurrent change' WHERE id=$1`, room)
		require.NoError(t, err)
		require.NoError(t, other.Close(ctx))
		againSnapshot, err := captureLegacy25ProjectionTx(ctx, tx, nil)
		require.NoError(t, err)
		require.Equal(t, snapshot, againSnapshot, "one transaction retains its original row snapshot")
		require.NoError(t, tx.Commit(ctx))
		_, err = conn.Exec(ctx, `UPDATE public.voice_rooms SET name='existing voice' WHERE id=$1`, room)
		require.NoError(t, err)
		_, err = conn.Exec(ctx, `ALTER TABLE public.spaces DISABLE TRIGGER space_authority_revision_spaces`)
		require.NoError(t, err)
		_, err = inspectLegacy25Catalog(ctx, conn)
		require.Error(t, err, "disabled source trigger must HOLD")
		_, err = conn.Exec(ctx, `ALTER TABLE public.spaces ENABLE TRIGGER space_authority_revision_spaces`)
		require.NoError(t, err)
		_, err = conn.Exec(ctx, `UPDATE public.schema_migrations SET dirty=true`)
		require.NoError(t, err)
		_, err = inspectLegacy25Catalog(ctx, conn)
		require.Error(t, err, "dirty marker must HOLD")
		_, err = conn.Exec(ctx, `UPDATE public.schema_migrations SET dirty=false`)
		require.NoError(t, err)
		_, err = inspectLegacy25Catalog(ctx, conn)
		require.NoError(t, err)
	})
	require.True(t, ok)
	require.NoError(t, conn.Close(ctx))
	cmd = exec.CommandContext(ctx, binary, "-mode", "upgrade-legacy25", "-migrations-dir", "../../../migrations/space_db", "-migrate", driver)
	cmd.Env = append(os.Environ(), "SPACE_NATIVE_ADMIN_URL="+adminURL)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err = cmd.Run()
	require.NoError(t, err, "legacy25 upgrade: %s", stderr.String())
	require.Contains(t, stdout.String(), `"event":"upgrade-complete"`)
	conn, err = pgx.Connect(ctx, target.String())
	require.NoError(t, err)
	defer conn.Close(context.Background())
	var count int
	require.NoError(t, conn.QueryRow(ctx, `SELECT count(*) FROM public.spaces WHERE id=$1 AND owner_profile_id=$2`, space, owner).Scan(&count))
	require.Equal(t, 1, count)
	require.NoError(t, conn.QueryRow(ctx, `SELECT count(*) FROM unrelated_fixture.records WHERE id=7 AND value='preserved'`).Scan(&count))
	require.Equal(t, 1, count)
	require.NoError(t, conn.QueryRow(ctx, `SELECT count(*) FROM public.space_members WHERE space_id=$1 AND profile_id=$2 AND nickname='existing member'`, space, member).Scan(&count))
	require.Equal(t, 1, count)
	require.NoError(t, conn.QueryRow(ctx, `SELECT count(*) FROM public.space_tree_nodes WHERE id=$1 AND space_id=$2 AND voice_room_id=$3`, node, space, room).Scan(&count))
	require.Equal(t, 1, count)
	require.NoError(t, conn.QueryRow(ctx, `SELECT count(*) FROM public.space_bans WHERE space_id=$1 AND account_id=$2 AND reason='existing ban'`, space, banned).Scan(&count))
	require.Equal(t, 1, count)
	require.NoError(t, conn.QueryRow(ctx, `SELECT version,dirty FROM public.schema_migrations`).Scan(&version, &dirty))
	require.Equal(t, 51, version)
	require.False(t, dirty)
}
