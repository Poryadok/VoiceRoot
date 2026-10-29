//go:build integration && linux

package nativeinstaller

import (
	"bytes"
	"context"
	"errors"
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
	"golang.org/x/sys/unix"
	"voice/backend/space/internal/store"
)

// Only a fresh server explicitly supplied by the test owner is accepted.
func legacy24OwnedDatabase(t *testing.T) (context.Context, string, string, string) {
	t.Helper()
	adminURL, driver, binary := os.Getenv("NATIVE_INSTALLER_TEST_ADMIN_URL"), os.Getenv("NATIVE_INSTALLER_TEST_MIGRATE"), os.Getenv("NATIVE_INSTALLER_TEST_BINARY")
	require.NotEmpty(t, adminURL)
	require.NotEmpty(t, driver)
	require.NotEmpty(t, binary)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	admin, err := pgx.Connect(ctx, adminURL)
	require.NoError(t, err)
	t.Cleanup(func() { _ = admin.Close(context.Background()) })
	_, err = admin.Exec(ctx, `CREATE DATABASE space_db`)
	require.NoError(t, err)
	var oid uint32
	require.NoError(t, admin.QueryRow(ctx, `SELECT oid FROM pg_database WHERE datname='space_db'`).Scan(&oid))
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		var current uint32
		require.NoError(t, admin.QueryRow(cleanup, `SELECT oid FROM pg_database WHERE datname='space_db'`).Scan(&current))
		require.Equal(t, oid, current)
		_, err := admin.Exec(cleanup, `DROP DATABASE space_db`)
		require.NoError(t, err)
	})
	target, err := url.Parse(adminURL)
	require.NoError(t, err)
	target.Path = "/space_db"
	command := exec.CommandContext(ctx, driver, "-path", "../../../migrations/space_db", "-database", target.String(), "goto", "24")
	output, err := command.CombinedOutput()
	require.NoError(t, err, "pinned24 fixture: %s", output)
	conn, err := pgx.Connect(ctx, target.String())
	require.NoError(t, err)
	_, err = conn.Exec(ctx, `INSERT INTO public.spaces(id,name,owner_profile_id) VALUES('11111111-1111-4111-8111-111111111111','original','22222222-2222-4222-8222-222222222222'); INSERT INTO public.space_members(space_id,profile_id,nickname) VALUES('11111111-1111-4111-8111-111111111111','33333333-3333-4333-8333-333333333333','original member')`)
	require.NoError(t, err)
	require.NoError(t, conn.Close(ctx))
	return ctx, adminURL, driver, binary
}

func TestProductionLegacy24OriginalVerifyAndNoAdoption(t *testing.T) {
	ctx, adminURL, driver, binary := legacy24OwnedDatabase(t)
	dir := t.TempDir()
	origin := filepath.Join(dir, "original.json")
	prior := filepath.Join(dir, "completed.jsonl")
	run := func(mode string, args ...string) ([]byte, error) {
		argv := append([]string{"-mode", mode, "-migrations-dir", "../../../migrations/space_db", "-migrate", driver, "-origin-evidence", origin}, args...)
		cmd := exec.CommandContext(ctx, binary, argv...)
		cmd.Env = append(os.Environ(), "SPACE_NATIVE_ADMIN_URL="+adminURL)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Run()
		if err != nil {
			return stdout.Bytes(), errors.New(stderr.String())
		}
		return stdout.Bytes(), nil
	}
	completed, err := run("upgrade-legacy24", "-operation-id", "owned-test", "-snapshot-id", "owned-backup")
	require.NoError(t, err)
	receipt, err := ReadLegacy24Receipt(bytes.NewReader(completed))
	require.NoError(t, err)
	require.Equal(t, uint64(1), receipt.InitializationCount)
	require.NoError(t, os.WriteFile(prior, completed, 0600))
	verified, err := run("verify-legacy24", "-prior-evidence", prior)
	require.NoError(t, err)
	require.Contains(t, string(verified), `"event":"verified-legacy24"`)
	// Later cycles verify the original installation, not an immutable copy of
	// business data. Exercise real production mutation/fence producers first.
	target, err := url.Parse(adminURL)
	require.NoError(t, err)
	target.Path = "/space_db"
	pool, err := pgxpool.New(ctx, target.String())
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	spaceStore := &store.SpaceStore{Pool: pool}
	spaceID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	newName := "legitimate later business name"
	_, err = spaceStore.UpdateSpace(ctx, spaceID, store.UpdateSpaceInput{Name: &newName})
	require.NoError(t, err)
	pool.Close()
	verified, err = run("verify-legacy24", "-prior-evidence", prior)
	require.NoError(t, err, "original-image verification accepts legitimate later business writes")
	require.Contains(t, string(verified), `"event":"verified-legacy24"`)
	// The actual completed target cannot be adopted by another upgrade.
	_, err = run("upgrade-legacy24", "-operation-id", "another", "-snapshot-id", "another")
	require.Error(t, err)
	// A current snapshot cannot substitute for privately retained original bytes.
	raw, err := os.ReadFile(origin)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(origin, append(raw, '\n'), 0600))
	_, err = run("verify-legacy24", "-prior-evidence", prior)
	require.Error(t, err)
	require.NoError(t, os.WriteFile(origin, raw, 0600))
	verified, err = run("verify-legacy24", "-prior-evidence", prior)
	require.NoError(t, err)
	require.NotContains(t, string(verified), `"event":"trusted25"`)
	conn, err := pgx.Connect(ctx, target.String())
	require.NoError(t, err)
	_, err = conn.Exec(ctx, `ALTER TABLE public.space_members ADD COLUMN unexpected_verifier_column integer`)
	require.NoError(t, err)
	require.NoError(t, conn.Close(ctx))
	verified, err = run("verify-legacy24", "-prior-evidence", prior)
	require.ErrorContains(t, err, "native51")
	require.NotContains(t, string(verified), `"event":"verified-legacy24"`)
}

func TestProductionLegacy24FixedSearchPath(t *testing.T) {
	ctx, adminURL, _, _ := legacy24OwnedDatabase(t)
	target, err := url.Parse(adminURL)
	require.NoError(t, err)
	target.Path = "/space_db"
	conn, err := pgx.Connect(ctx, target.String())
	require.NoError(t, err)
	defer conn.Close(context.Background())
	tx, err := conn.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(context.Background())
	_, err = tx.Exec(ctx, `CREATE FUNCTION public.length(text) RETURNS integer LANGUAGE sql IMMUTABLE AS 'SELECT 999'; CREATE TEMP TABLE pg_class(fake integer)`)
	require.NoError(t, err)
	// Actual counterexample to the old explicit built-ins-last path.
	_, err = tx.Exec(ctx, `SET LOCAL search_path=public,pg_catalog`)
	require.NoError(t, err)
	var length int
	require.NoError(t, tx.QueryRow(ctx, `SELECT length('abc'::text)`).Scan(&length))
	require.Equal(t, 999, length)
	_, err = tx.Exec(ctx, `SELECT pg_catalog.set_config('search_path',$1,true)`, legacy24TargetSearchPath)
	require.NoError(t, err)
	var schemas []string
	var catalogFirst bool
	require.NoError(t, tx.QueryRow(ctx, `SELECT current_schemas(true),'pg_class'::regclass='pg_catalog.pg_class'::regclass,length('abc'::text)`).Scan(&schemas, &catalogFirst, &length))
	require.GreaterOrEqual(t, len(schemas), 3)
	require.Equal(t, []string{"pg_catalog", "public"}, schemas[:2])
	require.Contains(t, schemas[2], "pg_temp_")
	require.True(t, catalogFirst)
	require.Equal(t, 3, length)
	_, err = tx.Exec(ctx, `CREATE TABLE legacy24_default_namespace_probe(value text)`)
	require.NoError(t, err)
	var publicCreated bool
	require.NoError(t, tx.QueryRow(ctx, `SELECT to_regclass('public.legacy24_default_namespace_probe') IS NOT NULL AND to_regclass('pg_catalog.legacy24_default_namespace_probe') IS NULL`).Scan(&publicCreated))
	require.True(t, publicCreated)
	require.NoError(t, tx.Rollback(ctx))
}

func TestProductionLegacy24LostEventNeverReplaysCommittedPrefix(t *testing.T) {
	for _, test := range []struct {
		name            string
		failAt, version int
	}{{"before-first-DDL", 1, 24}, {"after-committed25", 2, 25}, {"after-atomic51", 6, 51}, {"after-initialization", 7, 51}, {"lost-completion", 8, 51}} {
		t.Run(test.name, func(t *testing.T) {
			ctx, adminURL, driver, binary := legacy24OwnedDatabase(t)
			origin := filepath.Join(t.TempDir(), "original.json")
			command := func(path string) *exec.Cmd {
				cmd := exec.CommandContext(ctx, binary, "-mode", "upgrade-legacy24", "-migrations-dir", "../../../migrations/space_db", "-migrate", driver, "-operation-id", "owned-failure-test", "-snapshot-id", "owned-backup", "-origin-evidence", path)
				cmd.Env = append(os.Environ(), "SPACE_NATIVE_ADMIN_URL="+adminURL)
				return cmd
			}
			reader, writer, err := os.Pipe()
			require.NoError(t, err)
			capacity, err := unix.FcntlInt(writer.Fd(), unix.F_SETPIPE_SZ, 4096)
			require.NoError(t, err)
			t.Cleanup(func() { _ = reader.Close(); _ = writer.Close() })
			cmd := command(origin)
			cmd.Stdout = writer
			var stderr, accepted bytes.Buffer
			cmd.Stderr = &stderr
			if test.failAt == 1 {
				require.NoError(t, reader.Close())
			}
			require.NoError(t, cmd.Start())
			require.NoError(t, writer.Close())
			if test.failAt > 1 {
				// No read-ahead into the next event: its full write must fail
				// against the closed reader, including the immediate completion.
				lastSize := 0
				for i := 1; i < test.failAt; i++ {
					lastSize = 0
					for {
						var one [1]byte
						_, readErr := reader.Read(one[:])
						require.NoError(t, readErr, "actual stage before pipe failure")
						accepted.WriteByte(one[0])
						lastSize++
						require.LessOrEqual(t, lastSize, 1<<20)
						if one[0] == '\n' {
							break
						}
					}
				}
				if test.failAt == 8 {
					require.Greater(t, lastSize, capacity, "completion must exceed unread pipe capacity")
				}
				require.NoError(t, reader.Close())
			}
			require.Error(t, cmd.Wait(), "closed actual stdout must prevent successful completion")
			require.FileExists(t, origin)
			target, err := url.Parse(adminURL)
			require.NoError(t, err)
			target.Path = "/space_db"
			conn, err := pgx.Connect(ctx, target.String())
			require.NoError(t, err)
			var version int
			var dirty bool
			require.NoError(t, conn.QueryRow(ctx, `SELECT version,dirty FROM public.schema_migrations`).Scan(&version, &dirty))
			require.Equal(t, test.version, version)
			require.False(t, dirty)
			require.NoError(t, conn.Close(ctx))
			_, err = command(origin).CombinedOutput()
			require.Error(t, err, "same one-use original path cannot be replayed")
			if test.version != 24 {
				output, err := command(filepath.Join(t.TempDir(), "new-original.json")).CombinedOutput()
				require.Error(t, err)
				require.Contains(t, string(output), "not admitted", "new path cannot adopt a committed prefix")
			}
			require.NotContains(t, accepted.String(), `"event":"upgrade-complete"`)
		})
	}
}
