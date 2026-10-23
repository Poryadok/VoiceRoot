//go:build spacebanintegration

package store

import (
	"context"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	migrateengine "github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	userv1 "voice.app/voice/user/v1"
)

// These entrypoints consume only the controller's disposable, genuinely
// migrated historical fixture. They do not make provision adopt an existing DB.
// The controller retains exclusive privileged-DDL custody throughout the run.
func native51FullCheck(t *testing.T, check string, ok bool, err error) {
	t.Helper()
	// All callers supply source literals, never database/error text or input.
	if !ok {
		t.Fatalf("NATIVE51_FULL_V1 check=%s result=FAIL error=%s", check, native51ProofErrorClass(err))
	}
	t.Logf("NATIVE51_FULL_V1 check=%s result=PASS error=NONE", check)
}

func native51FullPool(t *testing.T) (context.Context, *pgxpool.Pool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	dsn := os.Getenv("SPACE_BAN_TEST_DATABASE_URL")
	native51FullCheck(t, "ENV_DSN", dsn != "", nil)
	pool, err := pgxpool.New(ctx, dsn)
	native51FullCheck(t, "POOL_OPEN", err == nil, err)
	t.Cleanup(pool.Close)
	return ctx, pool
}

// TestNative51FixtureCommitTrusted50 is the missing populated-fixture bridge.
// The input must already have real44/45/50 history, retained old50 seal and no
// writers. Every new51 mutation still goes through the production adapter and
// pinned real engine/file source, including its outer commit authority.
func TestNative51FixtureCommitTrusted50(t *testing.T) {
	ctx, pool := native51FullPool(t)
	st := &SpaceStore{Pool: pool}
	prior, err := st.InspectNativeInstallerSource(ctx, 50)
	native51FullCheck(t, "TRUSTED_50", err == nil, err)
	before := native51Snapshot(t, ctx, pool)
	var oldOID uint32
	err = pool.QueryRow(ctx, `SELECT 'native_membership_guard.catalog()'::regprocedure::oid`).Scan(&oldOID)
	native51FullCheck(t, "CATALOG_OID_BEFORE", err == nil, err)
	// Isolate this controller attempt from every other client, including clients
	// sharing its owner login. The adapter's dedicated backend inherits this tag.
	pool.Close()
	cfg, err := pgxpool.ParseConfig(os.Getenv("SPACE_BAN_TEST_DATABASE_URL"))
	native51FullCheck(t, "WINDOW_CONFIG", err == nil, err)
	app := "native51-fixture-" + uuid.NewString()
	cfg.ConnConfig.RuntimeParams["application_name"] = app
	cfg.MaxConns = 2
	pool, err = pgxpool.NewWithConfig(ctx, cfg)
	native51FullCheck(t, "WINDOW_POOL", err == nil, err)
	t.Cleanup(pool.Close)
	st = &SpaceStore{Pool: pool}
	window := func(windowCtx context.Context) error {
		var exclusive bool
		err := pool.QueryRow(windowCtx, `SELECT current_user=session_user AND NOT pg_is_in_recovery() AND NOT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND backend_type='client backend' AND application_name<>$1)`, app).Scan(&exclusive)
		if err != nil || !exclusive {
			return ErrMemberAuthorityUnavailable
		}
		return nil
	}
	native51FullCheck(t, "WINDOW_EXCLUSIVE", window(ctx) == nil, nil)
	driver, err := st.NewNativeCatalog51Migration(ctx, prior, window)
	native51FullCheck(t, "REAL_DRIVER", err == nil, err)
	t.Cleanup(func() { native51FullCheck(t, "DRIVER_CLOSE", driver.Close() == nil, nil) })
	dir, err := filepath.Abs(filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	native51FullCheck(t, "FILE_SOURCE", err == nil, err)
	engine, err := migrateengine.NewWithDatabaseInstance((&url.URL{Scheme: "file", Path: filepath.ToSlash(dir)}).String(), "native51-full-fixture", driver)
	native51FullCheck(t, "REAL_ENGINE", err == nil, err)
	t.Cleanup(func() {
		sourceErr, closeErr := engine.Close()
		native51FullCheck(t, "ENGINE_CLOSE", sourceErr == nil && closeErr == nil, errors.Join(sourceErr, closeErr))
	})
	engineErr := engine.Migrate(51)
	receipt, commitErr := driver.CommitQualified(engineErr)
	native51FullCheck(t, "ENGINE_MIGRATE_51", engineErr == nil, engineErr)
	native51FullCheck(t, "OUTER_COMMIT_51", commitErr == nil && receipt != nil, commitErr)
	native51FullCheck(t, "RECEIPT_PARENT", receipt.Changed && receipt.OldSealSHA256 == prior.SealSHA256 && receipt.Epoch == prior.Epoch && receipt.DatabaseName == prior.DatabaseName && receipt.DatabaseOID == prior.DatabaseOID && receipt.DatabaseSystemID == prior.DatabaseSystemID && receipt.InstallerRole == prior.InstallerRole, nil)
	native51FullCheck(t, "RECEIPT_SOURCE", receipt.Profile == NativeCatalogRowJSONProfile && receipt.MigrationSHA256 == nativeCatalogRowJSONMigrationSHA256 && receipt.Engine == "github.com/golang-migrate/migrate/v4@v4.18.1" && receipt.TransactionID != "", nil)
	// Release the dedicated backend before a separate committed-state verifier.
	native51FullCheck(t, "DRIVER_RELEASE", driver.Close() == nil, nil)
	facts, err := st.InspectNativeInstallerSource(ctx, 51)
	native51FullCheck(t, "FRESH_FULL_51", err == nil, err)
	native51FullCheck(t, "COMMITTED_IDENTITY", facts.DatabaseOID == prior.DatabaseOID && facts.Epoch == prior.Epoch && facts.SealSHA256 == receipt.NewSealSHA256 && facts.SealSHA256 != prior.SealSHA256, nil)
	after := native51Snapshot(t, ctx, pool)
	var newOID uint32
	err = pool.QueryRow(ctx, `SELECT 'native_membership_guard.catalog()'::regprocedure::oid`).Scan(&newOID)
	native51FullCheck(t, "CATALOG_OID_RETAINED", err == nil && newOID == oldOID, err)
	native51FullCheck(t, "BODY_SEAL_HISTORY_COMMITTED", after.version == 51 && !after.dirty && after.definition != before.definition && after.epoch == before.epoch && string(after.seal) == string(after.catalog) && string(after.seal) != string(before.seal), nil)
}

// A separate, genuinely qualified old50 fixture supplies the actual parent.
// Only copied EXPECTED facts are stale; no database seal, epoch, body or marker
// is changed to manufacture a failure or successful adoption authority.
func TestNative51StaleExpectedParentCannotCommit(t *testing.T) {
	ctx, pool := native51FullPool(t)
	cfg := pool.Config().Copy()
	pool.Close()
	app := "native51-stale-parent-" + uuid.NewString()
	cfg.ConnConfig.RuntimeParams["application_name"] = app
	cfg.MaxConns = 2
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	native51FullCheck(t, "STALE_WINDOW_POOL", err == nil, err)
	t.Cleanup(pool.Close)
	st := &SpaceStore{Pool: pool}
	prior, err := st.InspectNativeInstallerSource(ctx, 50)
	native51FullCheck(t, "STALE_TRUSTED_50", err == nil, err)
	before := native51Snapshot(t, ctx, pool)
	window := func(windowCtx context.Context) error {
		var exclusive bool
		err := pool.QueryRow(windowCtx, `SELECT current_user=session_user AND NOT pg_is_in_recovery() AND NOT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND backend_type='client backend' AND application_name<>$1)`, app).Scan(&exclusive)
		if err != nil || !exclusive {
			return ErrMemberAuthorityUnavailable
		}
		return nil
	}
	dir, err := filepath.Abs(filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	native51FullCheck(t, "STALE_FILE_SOURCE", err == nil, err)
	for _, field := range []string{"epoch", "seal"} {
		t.Run(field, func(t *testing.T) {
			expected := *prior
			if field == "epoch" {
				expected.Epoch = uuid.NewString()
			} else {
				first := "0"
				if expected.SealSHA256[:1] == first {
					first = "1"
				}
				expected.SealSHA256 = first + expected.SealSHA256[1:]
			}
			driver, err := st.NewNativeCatalog51Migration(ctx, &expected, window)
			native51FullCheck(t, "STALE_REAL_DRIVER", err == nil, err)
			t.Cleanup(func() { native51FullCheck(t, "STALE_DRIVER_CLOSE", driver.Close() == nil, nil) })
			engine, err := migrateengine.NewWithDatabaseInstance((&url.URL{Scheme: "file", Path: filepath.ToSlash(dir)}).String(), "native51-stale-parent", driver)
			native51FullCheck(t, "STALE_REAL_ENGINE", err == nil, err)
			t.Cleanup(func() {
				sourceErr, closeErr := engine.Close()
				native51FullCheck(t, "STALE_ENGINE_CLOSE", sourceErr == nil && closeErr == nil, errors.Join(sourceErr, closeErr))
			})
			engineErr := engine.Migrate(51)
			receipt, commitErr := driver.CommitQualified(engineErr)
			native51FullCheck(t, "STALE_PARENT_REJECTED", errors.Is(engineErr, ErrMemberAuthorityUnavailable) && errors.Is(commitErr, ErrMemberAuthorityUnavailable) && receipt == nil && driver.receipt == nil && driver.state.phase != catalog51Committed, commitErr)
			native51FullCheck(t, "STALE_RELEASE", driver.Close() == nil, nil)
			afterFacts, err := st.InspectNativeInstallerSource(ctx, 50)
			native51FullCheck(t, "STALE_FRESH_50_UNCHANGED", err == nil && reflect.DeepEqual(prior, afterFacts), err)
			native51FullCheck(t, "STALE_BODY_SEAL_HISTORY_UNCHANGED", reflect.DeepEqual(before, native51Snapshot(t, ctx, pool)), nil)
		})
	}
}

// TestNative51FixtureFailedOver100Capture leaves ONLY the genuine failed
// logical intent that unchanged TestAccountBanOver100Retry50 requires. Run on
// its own fresh107 fixture, then run that unchanged retry test in a new process.
func TestNative51FixtureFailedOver100Capture(t *testing.T) {
	ctx, owner := native51FullPool(t)
	_, err := (&SpaceStore{Pool: owner}).InspectNativeInstallerSource(ctx, 51)
	native51FullCheck(t, "CAPTURE_SOURCE_51", err == nil, err)
	space, operation := nativeBanSpace(13), uuid.New()
	admin := &SpaceStore{Pool: owner}
	err = admin.withOwnershipScope(ctx, []uuid.UUID{space}, func(scoped *SpaceStore) error { return scoped.InitializeScopedNativeMembership(ctx, space) })
	native51FullCheck(t, "CAPTURE_ENROLL", err == nil, err)
	serving := projectionServingLogin(t, ctx, owner)
	var beforeMembers, afterMembers string
	err = serving.QueryRow(ctx, `SELECT jsonb_build_object('space',(SELECT to_jsonb(s) FROM spaces s WHERE id=$1),'members',(SELECT jsonb_agg(to_jsonb(m) ORDER BY profile_id) FROM space_members m WHERE space_id=$1),'community',(SELECT jsonb_agg(to_jsonb(h) ORDER BY profile_id) FROM space_community_member_heads h WHERE space_id=$1))::text`, space).Scan(&beforeMembers)
	native51FullCheck(t, "FAILED_CAPTURE_BEFORE_EFFECTS", err == nil, err)
	var calls int
	st := &SpaceStore{Pool: serving, AccountCaptures: nativeBanCaptureFixture(func(_ context.Context, request *userv1.CaptureSpaceMemberAccountBindingsRequest) (*userv1.CaptureSpaceMemberAccountBindingsResponse, error) {
		calls++
		native51FullCheck(t, "FAILED_CAPTURE_FULL_107", len(request.Cohort.Entries) == 107, nil)
		// The real owner <=2s check must reject even a complete late response.
		time.Sleep(2100 * time.Millisecond)
		return nativeBanOwnerFixture(request), nil
	})}
	err = nativeBanExecute(t, st, space, nativeBanAccount(2), nativeBanProfile(1), operation, nil, func(context.Context) error { return nil })
	native51FullCheck(t, "FAILED_CAPTURE_UNAVAILABLE", errors.Is(err, ErrAccountBanUnavailable) && calls == 1, err)
	nativeBanAssertNoEffect(t, serving, space)
	err = serving.QueryRow(ctx, `SELECT jsonb_build_object('space',(SELECT to_jsonb(s) FROM spaces s WHERE id=$1),'members',(SELECT jsonb_agg(to_jsonb(m) ORDER BY profile_id) FROM space_members m WHERE space_id=$1),'community',(SELECT jsonb_agg(to_jsonb(h) ORDER BY profile_id) FROM space_community_member_heads h WHERE space_id=$1))::text`, space).Scan(&afterMembers)
	native51FullCheck(t, "FAILED_CAPTURE_NO_MEMBER_EFFECTS", err == nil && beforeMembers == afterMembers, err)
	var pending, allOperations, qualifications, responses, frames int
	err = owner.QueryRow(ctx, `SELECT (SELECT count(*) FROM space_account_ban_operations WHERE space_id=$1 AND actor_profile_id=$2 AND client_operation_id=$3 AND state='CAPTURING'),(SELECT count(*) FROM space_account_ban_operations WHERE space_id=$1),(SELECT count(*) FROM space_account_qualifications WHERE space_id=$1),(SELECT count(*) FROM space_account_ban_operations WHERE space_id=$1 AND response_bytes IS NOT NULL),(SELECT count(*) FROM native_membership_guard.frames)`, space, nativeBanProfile(1), operation).Scan(&pending, &allOperations, &qualifications, &responses, &frames)
	native51FullCheck(t, "REAL_CAPTURING_ONLY", err == nil && pending == 1 && allOperations == 1 && qualifications == 0 && responses == 0 && frames == 0, err)
}

// Flush every idle backend BEFORE the statistics baseline and AFTER Ban. A
// second roundtrip observes that each force-next-flush command has gone idle.
// This is builtin statistics instrumentation, never a catalog invocation,
// counter reset, extension, role grant or function/guard change.
func native51FullFlushStats(t *testing.T, ctx context.Context, pool *pgxpool.Pool) []int32 {
	t.Helper()
	conns := pool.AcquireAllIdle(ctx)
	defer func() {
		for _, conn := range conns {
			conn.Release()
		}
	}()
	native51FullCheck(t, "ALL_BACKENDS_IDLE", len(conns) > 0 && int32(len(conns)) == pool.Stat().TotalConns(), nil)
	var pids []int32
	for _, conn := range conns {
		_, err := conn.Exec(ctx, `SELECT pg_catalog.pg_stat_force_next_flush()`)
		native51FullCheck(t, "STATS_FORCE_FLUSH", err == nil, err)
		var pid int32
		err = conn.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid)
		native51FullCheck(t, "STATS_BACKEND", err == nil, err)
		pids = append(pids, pid)
	}
	return pids
}

func native51FullReadCalls(t *testing.T, ctx context.Context, owner *pgxpool.Pool, oid uint32, pids []int32) int64 {
	t.Helper()
	_, err := owner.Exec(ctx, `SELECT pg_catalog.pg_stat_clear_snapshot()`)
	native51FullCheck(t, "STATS_FRESH_SNAPSHOT", err == nil, err)
	var calls int64
	var isolated bool
	err = owner.QueryRow(ctx, `SELECT COALESCE((SELECT calls FROM pg_stat_user_functions WHERE funcid=$1),0),NOT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND backend_type='client backend' AND NOT(pid=ANY($2::integer[])))`, oid, pids).Scan(&calls, &isolated)
	native51FullCheck(t, "STATS_ISOLATED_READ", err == nil && isolated, err)
	return calls
}

// TestNative51Over100BanOnlyCatalogCalls uses a SEPARATE fresh107 fixture. The
// original107/307/retry bodies are still run unchanged on other fresh fixtures.
// The delta proves actual invocations of the exact catalog function during
// this real Ban; it is not an inferred count or a substitute for full effects.
func TestNative51Over100BanOnlyCatalogCalls(t *testing.T) {
	ctx, owner := native51FullPool(t)
	_, err := (&SpaceStore{Pool: owner}).InspectNativeInstallerSource(ctx, 51)
	native51FullCheck(t, "COUNT_SOURCE_51", err == nil, err)
	space, operation := nativeBanSpace(13), uuid.New()
	err = (&SpaceStore{Pool: owner}).withOwnershipScope(ctx, []uuid.UUID{space}, func(scoped *SpaceStore) error { return scoped.InitializeScopedNativeMembership(ctx, space) })
	native51FullCheck(t, "COUNT_ENROLL", err == nil, err)
	serving := projectionServingLogin(t, ctx, owner)
	var tracking string
	var oid uint32
	err = serving.QueryRow(ctx, `SELECT current_setting('track_functions')`).Scan(&tracking)
	native51FullCheck(t, "COUNT_TRACKING_ENABLED", err == nil && (tracking == "pl" || tracking == "all"), err)
	err = owner.QueryRow(ctx, `SELECT 'native_membership_guard.catalog()'::regprocedure::oid`).Scan(&oid)
	native51FullCheck(t, "COUNT_CATALOG_OID", err == nil, err)
	st := &SpaceStore{Pool: serving, AccountCaptures: nativeBanCaptureFixture(func(_ context.Context, request *userv1.CaptureSpaceMemberAccountBindingsRequest) (*userv1.CaptureSpaceMemberAccountBindingsResponse, error) {
		native51FullCheck(t, "COUNT_FULL_107", len(request.Cohort.Entries) == 107, nil)
		return nativeBanOwnerFixture(request), nil
	})}
	pids := append(native51FullFlushStats(t, ctx, owner), native51FullFlushStats(t, ctx, serving)...)
	before := native51FullReadCalls(t, ctx, owner, oid, pids)
	started := time.Now()
	err = nativeBanExecute(t, st, space, nativeBanAccount(2), nativeBanProfile(1), operation, nil, func(context.Context) error { return nil })
	elapsed := time.Since(started)
	// No post-effect assertion, native readiness probe or catalog invocation is
	// permitted until this AFTER count. Collection is outside the phase timer.
	afterPIDs := append(native51FullFlushStats(t, ctx, owner), native51FullFlushStats(t, ctx, serving)...)
	after := native51FullReadCalls(t, ctx, owner, oid, afterPIDs)
	native51FullCheck(t, "COUNT_BAN_SUCCESS", err == nil, err)
	native51FullCheck(t, "COUNT_BAN_BUDGET", elapsed < 5*time.Second, nil)
	native51FullCheck(t, "COUNT_ACTUAL_325", after-before == 325, nil)
	t.Logf("NATIVE51_BAN_WINDOW_V1 catalog_calls=%d elapsed_ns=%d", after-before, elapsed.Nanoseconds())
	var matched, members, retained, active int
	err = serving.QueryRow(ctx, `SELECT (SELECT cardinality(matched_profiles) FROM space_account_ban_operations WHERE actor_profile_id=$1 AND client_operation_id=$2 AND state='COMMITTED'),(SELECT member_count FROM spaces WHERE id=$3),(SELECT count(*) FROM space_members WHERE space_id=$3),(SELECT count(*) FROM space_community_member_heads WHERE space_id=$3 AND state='ACTIVE')`, nativeBanProfile(1), operation, space).Scan(&matched, &members, &retained, &active)
	native51FullCheck(t, "COUNT_FULL_EFFECTS", err == nil && matched == 104 && members == 2 && retained == 2 && active == 0, err)
}
