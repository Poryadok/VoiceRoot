package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/integrationtest"
)

// Controlled immutable-cohort extraction, not a runtime expected-hash fallback.
func TestNativeP3Child49CanonicalMigrationAndUnusedDown(t *testing.T) {
	ctx := context.Background()
	for _, major := range []int{16, 18} {
		t.Run(fmt.Sprintf("PG%d", major), func(t *testing.T) {
			image := "postgres:16-bookworm"
			if major == 18 {
				image = "postgres:18-alpine"
			}
			db := integrationtest.NewSourceMigrationFixtureWithImage(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"), image)
			db.Run(true, "goto", "48")
			var before []byte
			require.NoError(t, db.Pool.QueryRow(ctx, `SELECT native_membership_guard.catalog()`).Scan(&before))
			db.Run(true, "up", "1")
			var structure string
			require.NoError(t, db.Pool.QueryRow(ctx, nativeReadinessStructureSQL).Scan(&structure))
			require.NoError(t, os.WriteFile(fmt.Sprintf("/tmp/p3child49-pg%d-structure.json", major), []byte(structure), 0600))
			digest := sha256.Sum256([]byte(structure))
			t.Log("controlled49 full structure", major, hex.EncodeToString(digest[:]))
			var valid bool
			require.NoError(t, db.Pool.QueryRow(ctx, `SELECT to_regclass('native_membership_guard.bot_purge_children') IS NOT NULL AND to_regclass('native_membership_guard.catalog49_transition') IS NOT NULL AND (SELECT count(*)=6 FROM pg_trigger WHERE tgname IN ('bot_purge49_immutable','bot_purge49_no_truncate','bot_purge49_transition_immutable','bot_purge49_transition_no_truncate','zzzzzzzz_bot_purge49_parent_terminal','zzzzzzzz_bot_purge49_parent_minimize') AND NOT tgisinternal)`).Scan(&valid))
			require.True(t, valid)
			db.Run(true, "down", "1")
			var after []byte
			require.NoError(t, db.Pool.QueryRow(ctx, `SELECT native_membership_guard.catalog()`).Scan(&after))
			require.Equal(t, before, after, "unused DOWN restores exact48 catalog, no old body/metadata changes")
		})
	}
}

func TestNativeP3Child49DownFencesAdmissionAndOldSnapshot(t *testing.T) {
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixtureWithImage(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"), "postgres:18-alpine")
	admin := &SpaceStore{Pool: db.Pool}
	db.Run(true, "goto", "46")
	_, err := admin.TransitionNativeMembershipCatalog46(ctx)
	require.NoError(t, err)
	db.Run(true, "up", "1")
	_, err = admin.TransitionNativeMembershipCatalog47(ctx)
	require.NoError(t, err)
	db.Run(true, "up", "1")
	_, err = admin.TransitionNativeMembershipCatalog48(ctx)
	require.NoError(t, err)
	db.Run(true, "up", "1")
	down, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/space_db/000049_bot_purge_child_evidence.down.sql"))
	require.NoError(t, err)
	old, err := db.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	require.NoError(t, err)
	defer old.Rollback(ctx)
	var databases int
	require.NoError(t, old.QueryRow(ctx, `SELECT count(*) FROM pg_database`).Scan(&databases))
	blocker, err := db.Pool.Begin(ctx)
	require.NoError(t, err)
	defer blocker.Rollback(ctx)
	var blockerPID int32
	require.NoError(t, blocker.QueryRow(ctx, `SELECT pg_backend_pid() FROM native_membership_guard.epoch FOR UPDATE`).Scan(&blockerPID))
	admitted := make(chan error, 1)
	go func() { _, err := admin.TransitionNativeMembershipCatalog49(ctx); admitted <- err }()
	waitFor := func(query string) {
		t.Helper()
		blocked := false
		for until := time.Now().Add(3 * time.Second); time.Now().Before(until); {
			require.NoError(t, db.Pool.QueryRow(ctx, query, blockerPID).Scan(&blocked))
			if blocked {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		require.True(t, blocked, "must observe actual PostgreSQL lock chain")
	}
	waitFor(`SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid)))`)
	downgraded := make(chan error, 1)
	go func() {
		tx, err := db.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
		if err == nil {
			_, err = tx.Exec(ctx, string(down))
			_ = tx.Rollback(ctx)
		}
		downgraded <- err
	}()
	waitFor(`SELECT EXISTS(SELECT 1 FROM pg_stat_activity d JOIN pg_stat_activity a ON a.pid=ANY(pg_blocking_pids(d.pid)) WHERE $1=ANY(pg_blocking_pids(a.pid)))`)
	require.NoError(t, blocker.Commit(ctx))
	require.NoError(t, <-admitted)
	require.Error(t, <-downgraded, "queued RC DOWN must see the real committed first activation")
	var staleEmpty bool
	require.NoError(t, old.QueryRow(ctx, `SELECT NOT EXISTS(SELECT 1 FROM native_membership_guard.catalog49_transition)`).Scan(&staleEmpty))
	require.True(t, staleEmpty, "old RR really sees the pre-admission empty snapshot")
	_, err = old.Exec(ctx, string(down))
	require.Error(t, err, "pre-lock RC guard must reject stale RR before it can erase a committed activation")
	require.NoError(t, old.Rollback(ctx))
	var preserved bool
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM native_membership_guard.catalog49_transition)
 AND to_regclass('native_membership_guard.bot_purge_children') IS NOT NULL
 AND (SELECT catalog_sha256=native_membership_guard.catalog() FROM native_membership_guard.epoch)`).Scan(&preserved))
	require.True(t, preserved)
}

func TestNativeP3Child49TrustedTransitionRequiresExactNewGuards(t *testing.T) {
	ctx := context.Background()
	for _, major := range []int{16, 18} {
		t.Run(fmt.Sprintf("PG%d", major), func(t *testing.T) {
			image := "postgres:16-bookworm"
			if major == 18 {
				image = "postgres:18-alpine"
			}
			db := integrationtest.NewSourceMigrationFixtureWithImage(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"), image)
			admin := &SpaceStore{Pool: db.Pool}
			db.Run(true, "goto", "46")
			_, err := admin.TransitionNativeMembershipCatalog46(ctx)
			require.NoError(t, err)
			db.Run(true, "up", "1")
			_, err = admin.TransitionNativeMembershipCatalog47(ctx)
			require.NoError(t, err)
			db.Run(true, "up", "1")
			_, err = admin.TransitionNativeMembershipCatalog48(ctx)
			require.NoError(t, err)
			var catalogBody, oldJSON string
			require.NoError(t, db.Pool.QueryRow(ctx, `SELECT prosrc FROM pg_proc WHERE oid='native_membership_guard.catalog()'::regprocedure`).Scan(&catalogBody))
			jsonSQL := func(sql string) string {
				return strings.Replace(strings.Replace(sql, "sha256(convert_to(jsonb_build_object(", "jsonb_build_object(", 1), ")::text,'UTF8'))", ")::text", 1)
			}
			baseline, err := db.Pool.Begin(ctx)
			require.NoError(t, err)
			_, err = baseline.Exec(ctx, `SET LOCAL search_path=pg_catalog,public,native_membership_guard,pg_temp`)
			require.NoError(t, err)
			require.NoError(t, baseline.QueryRow(ctx, jsonSQL(catalogBody)).Scan(&oldJSON))
			require.NoError(t, baseline.Rollback(ctx))
			require.NoError(t, os.WriteFile(fmt.Sprintf("/tmp/p3child49-pg%d-old48.json", major), []byte(oldJSON), 0600))
			db.Run(true, "up", "1")
			var retained []byte
			require.NoError(t, db.Pool.QueryRow(ctx, `SELECT catalog_sha256 FROM native_membership_guard.epoch`).Scan(&retained))
			_, err = db.Pool.Exec(ctx, `ALTER TABLE native_membership_guard.bot_purge_children DISABLE TRIGGER bot_purge49_immutable`)
			require.NoError(t, err)
			_, err = admin.TransitionNativeMembershipCatalog49(ctx)
			require.ErrorIs(t, err, ErrMemberAuthorityUnavailable)
			var preserved bool
			require.NoError(t, db.Pool.QueryRow(ctx, `SELECT catalog_sha256=$1 AND NOT EXISTS(SELECT 1 FROM native_membership_guard.catalog49_transition) FROM native_membership_guard.epoch`, retained).Scan(&preserved))
			require.True(t, preserved)
			_, err = db.Pool.Exec(ctx, `ALTER TABLE native_membership_guard.bot_purge_children ENABLE TRIGGER bot_purge49_immutable`)
			require.NoError(t, err)
			qualified, err := db.Pool.Begin(ctx)
			require.NoError(t, err)
			catalogErr := checkMemberAuthorityCatalog(ctx, qualified)
			_ = qualified.Rollback(ctx)
			require.NoError(t, catalogErr, "restored49 structural/body/trigger closure must qualify before transition")
			var tableOIDs, functionOIDs, triggerOIDs []uint32
			require.NoError(t, db.Pool.QueryRow(ctx, `SELECT array_agg(oid) FROM (SELECT oid FROM pg_class WHERE oid IN ('native_membership_guard.bot_purge_children'::regclass,'native_membership_guard.catalog49_transition'::regclass) UNION SELECT indexrelid FROM pg_index WHERE indrelid IN ('native_membership_guard.bot_purge_children'::regclass,'native_membership_guard.catalog49_transition'::regclass)) q`).Scan(&tableOIDs))
			for _, pin := range nativeReadinessPurgeChild49Pins {
				var oid uint32
				require.NoError(t, db.Pool.QueryRow(ctx, `SELECT to_regprocedure($1)::oid`, pin.signature).Scan(&oid))
				functionOIDs = append(functionOIDs, oid)
			}
			require.NoError(t, db.Pool.QueryRow(ctx, `SELECT array_agg(oid) FROM pg_trigger WHERE (tgrelid='public.spaces'::regclass AND tgname='zzzzzzzz_bot_purge49_parent_terminal') OR (tgrelid='public.space_lifecycle_aggregates'::regclass AND tgname='zzzzzzzz_bot_purge49_parent_minimize')`).Scan(&triggerOIDs))
			var projected string
			projection, err := db.Pool.Begin(ctx)
			require.NoError(t, err)
			_, err = projection.Exec(ctx, `SET LOCAL search_path=pg_catalog,public,native_membership_guard,pg_temp`)
			require.NoError(t, err)
			require.NoError(t, projection.QueryRow(ctx, jsonSQL(nativeReadinessPre48ProjectionSQL), tableOIDs, functionOIDs, triggerOIDs, map[string]string{}).Scan(&projected))
			require.NoError(t, projection.Rollback(ctx))
			require.NoError(t, os.WriteFile(fmt.Sprintf("/tmp/p3child49-pg%d-projected48.json", major), []byte(projected), 0600))
			require.True(t, oldJSON == projected, "fixed old48 projection must preserve every original catalog fact")
			first, err := admin.TransitionNativeMembershipCatalog49(ctx)
			require.NoError(t, err)
			require.True(t, first.Changed)
			replay, err := admin.TransitionNativeMembershipCatalog49(ctx)
			require.NoError(t, err)
			require.False(t, replay.Changed)
			require.Equal(t, first.OldSealSHA256, replay.OldSealSHA256)
			require.Equal(t, first.NewSealSHA256, replay.NewSealSHA256)
		})
	}
}
