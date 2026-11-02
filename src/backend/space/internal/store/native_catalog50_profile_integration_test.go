//go:build spacebanintegration

package store

import (
	"context"
	"errors"
	"math"
	"os"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	userv1 "voice.app/voice/user/v1"
)

// These statistics are diagnostic observations, not Ban acceptance evidence.
type native50ProfileStats struct {
	calls   int64
	totalMS float64
	selfMS  float64
	present bool
}

type native50ProfileBackend struct {
	pid     int32
	started time.Time
}

func native50ProfileBackends(t *testing.T, ctx context.Context, owner *pgxpool.Pool, pids []int32) []native50ProfileBackend {
	t.Helper()
	rows, err := owner.Query(ctx, `SELECT pid,backend_start FROM pg_stat_activity WHERE datname=current_database() AND backend_type='client backend' ORDER BY pid`)
	native51FullCheck(t, "M50_BACKEND_INVENTORY", err == nil, err)
	var backends []native50ProfileBackend
	for rows.Next() {
		var backend native50ProfileBackend
		if err = rows.Scan(&backend.pid, &backend.started); err != nil {
			rows.Close()
			native51FullCheck(t, "M50_BACKEND_SCAN", false, err)
		}
		backends = append(backends, backend)
	}
	err = rows.Err()
	rows.Close()
	native51FullCheck(t, "M50_BACKEND_COMPLETE", err == nil && len(backends) == len(pids), err)
	for i := range backends {
		native51FullCheck(t, "M50_BACKEND_PID", backends[i].pid == pids[i] && !backends[i].started.IsZero(), nil)
	}
	return backends
}

func native50ProfileErrorClass(err error) string {
	if errors.Is(err, ErrEffectiveEpisodeUnavailable) {
		return "EFFECTIVE_UNAVAILABLE"
	}
	if status.Code(err) == codes.Unavailable {
		return "RPC_UNAVAILABLE"
	}
	return native51ProofErrorClass(err)
}

func native50ProfileDelta(before, after native50ProfileStats) (native50ProfileStats, bool) {
	valid := func(s native50ProfileStats) bool {
		return s.present && s.calls >= 0 && !math.IsNaN(s.totalMS) && !math.IsNaN(s.selfMS) &&
			!math.IsInf(s.totalMS, 0) && !math.IsInf(s.selfMS, 0) && s.totalMS >= 0 && s.selfMS >= 0 && s.selfMS <= s.totalMS
	}
	if !valid(before) || !valid(after) || after.calls < before.calls || after.totalMS < before.totalMS || after.selfMS < before.selfMS {
		return native50ProfileStats{}, false
	}
	return native50ProfileStats{calls: after.calls - before.calls, totalMS: after.totalMS - before.totalMS, selfMS: after.selfMS - before.selfMS, present: true}, true
}

func native50ProfileSnapshot(t *testing.T, ctx context.Context, owner *pgxpool.Pool, oid uint32, pids []int32) native50ProfileStats {
	t.Helper()
	_, err := owner.Exec(ctx, `SELECT pg_catalog.pg_stat_clear_snapshot()`)
	native51FullCheck(t, "M50_STATS_SNAPSHOT", err == nil, err)
	var s native50ProfileStats
	var isolated bool
	err = owner.QueryRow(ctx, `SELECT f.calls,f.total_time,f.self_time,
 NOT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND backend_type='client backend' AND NOT(pid=ANY($2::integer[])))
 FROM pg_stat_user_functions f WHERE f.funcid=$1`, oid, pids).Scan(&s.calls, &s.totalMS, &s.selfMS, &isolated)
	native51FullCheck(t, "M50_STATS_PRESENT_ISOLATED", err == nil && isolated, err)
	s.present = true
	return s
}

// The controller supplies a distinct disposable, qualified M50 107-entry
// fixture and enables track_functions=all before opening any test connection.
// This instrumented test is a diagnostic, not the unchanged acceptance test.
func TestNative50Over100ProfileCatalog(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cfg, err := pgxpool.ParseConfig(os.Getenv("SPACE_BAN_TEST_DATABASE_URL"))
	native51FullCheck(t, "M50_POOL_CONFIG", err == nil, err)
	cfg.MaxConns = 1
	owner, err := pgxpool.NewWithConfig(ctx, cfg)
	native51FullCheck(t, "M50_POOL_OPEN", err == nil, err)
	t.Cleanup(owner.Close)
	var version int
	err = owner.QueryRow(ctx, `SELECT version FROM public.schema_migrations WHERE NOT dirty`).Scan(&version)
	native51FullCheck(t, "M50_EXACT_SOURCE", err == nil && version == 50, err)
	space := nativeBanSpace(13)
	err = (&SpaceStore{Pool: owner}).withOwnershipScope(ctx, []uuid.UUID{space}, func(scoped *SpaceStore) error {
		return scoped.InitializeScopedNativeMembership(ctx, space)
	})
	native51FullCheck(t, "M50_ENROLL", err == nil, err)
	serving := projectionServingLogin(t, ctx, owner)
	var ownerTrack, servingTrack string
	err = owner.QueryRow(ctx, `SELECT current_setting('track_functions')`).Scan(&ownerTrack)
	native51FullCheck(t, "M50_OWNER_TRACKING", err == nil && ownerTrack == "all", err)
	err = serving.QueryRow(ctx, `SELECT current_setting('track_functions')`).Scan(&servingTrack)
	native51FullCheck(t, "M50_SERVING_TRACKING", err == nil && servingTrack == "all", err)
	var oid uint32
	err = owner.QueryRow(ctx, `SELECT 'native_membership_guard.catalog()'::regprocedure::oid`).Scan(&oid)
	native51FullCheck(t, "M50_CATALOG_OID", err == nil, err)
	pids := append(native51FullFlushStats(t, ctx, owner), native51FullFlushStats(t, ctx, serving)...)
	sort.Slice(pids, func(i, j int) bool { return pids[i] < pids[j] })
	backends := native50ProfileBackends(t, ctx, owner, pids)
	before := native50ProfileSnapshot(t, ctx, owner, oid, pids)
	st := &SpaceStore{Pool: serving, AccountCaptures: nativeBanCaptureFixture(func(_ context.Context, request *userv1.CaptureSpaceMemberAccountBindingsRequest) (*userv1.CaptureSpaceMemberAccountBindingsResponse, error) {
		native51FullCheck(t, "M50_FULL_107", len(request.Cohort.Entries) == 107, nil)
		return nativeBanOwnerFixture(request), nil
	})}
	started := time.Now()
	banErr := nativeBanExecute(t, st, space, nativeBanAccount(2), nativeBanProfile(1), uuid.New(), nil, func(context.Context) error { return nil })
	elapsed := time.Since(started)
	afterPIDs := append(native51FullFlushStats(t, ctx, owner), native51FullFlushStats(t, ctx, serving)...)
	sort.Slice(afterPIDs, func(i, j int) bool { return afterPIDs[i] < afterPIDs[j] })
	native51FullCheck(t, "M50_BACKENDS_UNCHANGED", reflect.DeepEqual(pids, afterPIDs), nil)
	afterBackends := native50ProfileBackends(t, ctx, owner, afterPIDs)
	native51FullCheck(t, "M50_BACKEND_STARTS_UNCHANGED", reflect.DeepEqual(backends, afterBackends), nil)
	after := native50ProfileSnapshot(t, ctx, owner, oid, afterPIDs)
	delta, ok := native50ProfileDelta(before, after)
	native51FullCheck(t, "M50_STATS_VALID", ok && delta.calls > 0, nil)
	t.Logf("NATIVE50_PROFILE_V1 result=%s elapsed_ns=%d catalog_calls=%d catalog_total_ms=%.3f catalog_self_ms=%.3f", native50ProfileErrorClass(banErr), elapsed.Nanoseconds(), delta.calls, delta.totalMS, delta.selfMS)
	native51FullCheck(t, "M50_BAN_SUCCESS", banErr == nil, banErr)
}
