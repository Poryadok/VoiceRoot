package grpcsvc

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	moderationv1 "voice.app/voice/moderation/v1"
	"voice/backend/moderation/internal/store"
)

func TestAutoModScanStats_RealPostgres(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool := startScanPostgres(t, ctx)
	_, err := pool.Exec(ctx, `INSERT INTO auto_mod_log (target_profile_id, trigger, action)
VALUES ($1, 'spam_pattern', 'mute'), ($1, 'report_threshold', 'shadow_ban')`, uuid.New())
	require.NoError(t, err)
	appPool, appName := scanApplicationPool(t, ctx, pool)
	m := privateScanMetrics()
	client, cleanup := startModerationGRPCTestServer(t, pool, func(s *ModerationGRPC) {
		s.AutoMod = &store.AutoModStore{Pool: appPool}
		s.ScanMetrics = m
	})
	t.Cleanup(cleanup)
	modCtx := withInternalModCtx(ctx, uuid.New())
	stats, err := client.GetAutoModStats(modCtx, &moderationv1.GetAutoModStatsRequest{})
	require.NoError(t, err)
	require.Zero(t, stats.GetAutoModStats().GetMessagesChecked(), "action history cannot reconstruct historical scans")
	require.EqualValues(t, 2, stats.GetAutoModStats().GetBlocked())

	sender := uuid.New().String()
	clean := &moderationv1.CheckMessageRequest{Content: "hello", SenderProfileId: sender}
	spam := &moderationv1.CheckMessageRequest{Content: "http://a http://b http://c", SenderProfileId: sender}
	for i := 0; i < 2; i++ {
		response, callErr := client.CheckMessage(ctx, clean)
		require.NoError(t, callErr)
		require.True(t, response.GetCheckResult().GetAllowed())
	}
	for _, reason := range []string{"spam_mute", "spam_mute_permanent"} {
		response, callErr := client.CheckMessage(ctx, spam)
		require.NoError(t, callErr)
		require.False(t, response.GetCheckResult().GetAllowed())
		require.Equal(t, reason, response.GetCheckResult().GetBlockReason())
	}
	_, err = client.CheckMessage(ctx, &moderationv1.CheckMessageRequest{Content: spam.Content, SenderProfileId: "invalid"})
	require.Equal(t, codes.InvalidArgument, status.Code(err), "the pattern evaluation still happened before sender validation")

	const concurrent = 32
	var wg sync.WaitGroup
	errors := make(chan error, concurrent)
	for i := 0; i < concurrent; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			response, callErr := client.CheckMessage(ctx, clean)
			if callErr == nil && !response.GetCheckResult().GetAllowed() {
				callErr = status.Error(codes.Internal, "clean message unexpectedly blocked")
			}
			errors <- callErr
		}()
	}
	wg.Wait()
	close(errors)
	for callErr := range errors {
		require.NoError(t, callErr)
	}
	cleanup()
	settleScanApplication(t, ctx, pool, appPool, appName)
	counts := scanOutcomeCounts(t, m)
	require.EqualValues(t, 5+concurrent, scanOutcomeTotal(counts), "exact terminal accounting for the closed batch")
	// Start a new transport after all attributed attempts have settled; it does
	// not issue CheckMessage until the first batch has been reconciled.
	client, cleanup = startModerationGRPCTestServer(t, pool)
	t.Cleanup(cleanup)
	stats, err = client.GetAutoModStats(modCtx, &moderationv1.GetAutoModStatsRequest{})
	require.NoError(t, err)
	durable := stats.GetAutoModStats().GetMessagesChecked()
	require.GreaterOrEqual(t, durable, counts["acknowledged"])
	require.LessOrEqual(t, durable, counts["acknowledged"]+counts["unconfirmed"])
	if counts["unconfirmed"] == 0 && counts["skipped_canceled"] == 0 && counts["skipped_budget"] == 0 {
		require.EqualValues(t, 5+concurrent, durable)
	}
	t.Logf("RPC eligible=37 outcomes=%v quiescent_durable=%d", counts, durable)
	require.EqualValues(t, 3, stats.GetAutoModStats().GetBlocked(), "existing action aggregation is unchanged")

	reopened, err := pgxpool.New(ctx, pool.Config().ConnString())
	require.NoError(t, err)
	t.Cleanup(reopened.Close)
	checked, blocked, err := (&store.AutoModStore{Pool: reopened}).Stats(ctx)
	require.NoError(t, err)
	require.EqualValues(t, durable, checked, "count survives a new pool/store instance, not a server restart")
	require.EqualValues(t, 3, blocked)

	migrationDir := filepath.Join(repoRoot(t), "src", "backend", "migrations", "moderation_db")
	integrationtestApplySQL(t, ctx, pool, filepath.Join(migrationDir, "000005_automod_scan_stats.down.sql"))
	response, err := client.CheckMessage(ctx, clean)
	require.NoError(t, err, "metrics failure must not change an allowed result")
	require.True(t, response.GetCheckResult().GetAllowed())
	response, err = client.CheckMessage(ctx, spam)
	require.NoError(t, err, "metrics failure must not change a blocked result")
	require.Equal(t, "spam_mute_permanent", response.GetCheckResult().GetBlockReason())
	_, err = client.CheckMessage(ctx, &moderationv1.CheckMessageRequest{Content: spam.Content, SenderProfileId: "invalid"})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	_, err = client.GetAutoModStats(modCtx, &moderationv1.GetAutoModStatsRequest{})
	require.Equal(t, codes.Internal, status.Code(err), "missing persistence must not report a fabricated zero")

	integrationtestApplySQL(t, ctx, pool, filepath.Join(migrationDir, "000005_automod_scan_stats.up.sql"))
	stats, err = client.GetAutoModStats(modCtx, &moderationv1.GetAutoModStatsRequest{})
	require.NoError(t, err)
	require.Zero(t, stats.GetAutoModStats().GetMessagesChecked(), "explicit migration reapplication starts a new baseline")
	require.EqualValues(t, 3, stats.GetAutoModStats().GetBlocked(), "down/up preserves existing action history")

	_, err = pool.Exec(ctx, `DELETE FROM auto_mod_scan_stats`)
	require.NoError(t, err)
	response, err = client.CheckMessage(ctx, clean)
	require.NoError(t, err)
	require.True(t, response.GetCheckResult().GetAllowed())
	response, err = client.CheckMessage(ctx, spam)
	require.NoError(t, err)
	require.Equal(t, "spam_mute_permanent", response.GetCheckResult().GetBlockReason())
	_, err = client.CheckMessage(ctx, &moderationv1.CheckMessageRequest{Content: spam.Content, SenderProfileId: "invalid"})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	_, err = client.GetAutoModStats(modCtx, &moderationv1.GetAutoModStatsRequest{})
	require.Equal(t, codes.Internal, status.Code(err), "a missing singleton must not fabricate zero either")
}

func TestAutoModScanStats_ExactPrimitiveAndEligibility(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool := startScanPostgres(t, ctx)
	st := &store.AutoModStore{Pool: pool}
	for i := 0; i < 5; i++ {
		require.NoError(t, st.RecordScan(ctx))
	}
	var wg sync.WaitGroup
	errors := make(chan error, 32)
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errors <- st.RecordScan(ctx) }()
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		require.NoError(t, err, "every direct increment must be acknowledged")
	}
	checked, _, err := st.Stats(ctx)
	require.NoError(t, err)
	require.EqualValues(t, 37, checked)
	reopened, err := pgxpool.New(ctx, pool.Config().ConnString())
	require.NoError(t, err)
	t.Cleanup(reopened.Close)
	checked, _, err = (&store.AutoModStore{Pool: reopened}).Stats(ctx)
	require.NoError(t, err)
	require.EqualValues(t, 37, checked)
	t.Log("exact primitive: 5+32 concurrent acknowledgements, durable37, new pool37")

	// The handler's event coverage is tested independently of the 50ms storage
	// sample budget. Its spam decisions still use the real store.
	m := privateScanMetrics()
	var attempts atomic.Int64
	s := &ModerationGRPC{AutoMod: st, ScanMetrics: m, scanRecorder: func(context.Context) error { attempts.Add(1); return nil }}
	sender := uuid.NewString()
	clean := &moderationv1.CheckMessageRequest{Content: "hello", SenderProfileId: sender}
	spam := &moderationv1.CheckMessageRequest{Content: "http://a http://b http://c", SenderProfileId: sender}
	for i := 0; i < 2; i++ {
		response, err := s.CheckMessage(ctx, clean)
		require.NoError(t, err)
		require.True(t, response.GetCheckResult().GetAllowed())
	}
	for _, reason := range []string{"spam_mute", "spam_mute_permanent"} {
		response, err := s.CheckMessage(ctx, spam)
		require.NoError(t, err)
		require.Equal(t, reason, response.GetCheckResult().GetBlockReason())
	}
	_, err = s.CheckMessage(ctx, &moderationv1.CheckMessageRequest{Content: spam.Content, SenderProfileId: "invalid"})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	errors = make(chan error, 32)
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			response, err := s.CheckMessage(ctx, clean)
			if err == nil && !response.GetCheckResult().GetAllowed() {
				err = fmt.Errorf("clean unexpectedly blocked")
			}
			errors <- err
		}()
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}
	require.EqualValues(t, 37, attempts.Load())
	require.EqualValues(t, 37, scanOutcomeCounts(t, m)["acknowledged"])
	require.EqualValues(t, 37, scanOutcomeTotal(scanOutcomeCounts(t, m)))
	s.AutoMod = nil
	response, err := s.CheckMessage(ctx, spam)
	require.NoError(t, err)
	require.True(t, response.GetCheckResult().GetAllowed())
	require.EqualValues(t, 37, attempts.Load())
	t.Log("exact handler eligibility37, acknowledged37, bypass adds0")

	// Fault injection: deliberately lose the acknowledgement after one real
	// commit. An ample fixture context guarantees the injected commit itself;
	// this adapter is not a production latency or historical race reproduction.
	s.AutoMod = st
	m = privateScanMetrics()
	s.ScanMetrics = m
	attempts.Store(0)
	s.scanRecorder = func(context.Context) error {
		attempts.Add(1)
		if err := st.RecordScan(ctx); err != nil {
			return err
		}
		return context.DeadlineExceeded
	}
	response, err = s.CheckMessage(ctx, clean)
	require.NoError(t, err)
	require.True(t, response.GetCheckResult().GetAllowed())
	require.EqualValues(t, 1, attempts.Load())
	require.EqualValues(t, 1, scanOutcomeCounts(t, m)["unconfirmed"])
	checked, _, err = st.Stats(ctx)
	require.NoError(t, err)
	require.EqualValues(t, 38, checked, "one real committed sample, no retry")
}

func TestAutoModScanStats_HeldCounterLockPreservesDecision(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool := startScanPostgres(t, ctx)
	for _, lock := range []struct{ name, sql string }{
		{"row", "SELECT singleton FROM auto_mod_scan_stats WHERE singleton = true FOR UPDATE"},
		{"table", "LOCK TABLE auto_mod_scan_stats IN ACCESS EXCLUSIVE MODE"},
	} {
		t.Run(lock.name, func(t *testing.T) {
			appPool, appName := scanApplicationPool(t, ctx, pool)
			m := privateScanMetrics()
			client, cleanup := startModerationGRPCTestServer(t, pool, func(s *ModerationGRPC) {
				s.AutoMod = &store.AutoModStore{Pool: appPool}
				s.ScanMetrics = m
			})
			t.Cleanup(cleanup)
			_, err := client.GetAutoModStats(withInternalModCtx(ctx, uuid.New()), &moderationv1.GetAutoModStatsRequest{})
			require.NoError(t, err)
			tx, beginErr := pool.Begin(ctx)
			require.NoError(t, beginErr)
			t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
			_, lockErr := tx.Exec(ctx, lock.sql)
			require.NoError(t, lockErr, "lock must be held before the RPC starts")
			sender := uuid.New().String()
			for _, content := range []string{"hello", "http://a http://b http://c"} {
				callCtx, callCancel := context.WithTimeout(ctx, 500*time.Millisecond)
				response, callErr := client.CheckMessage(callCtx, &moderationv1.CheckMessageRequest{Content: content, SenderProfileId: sender})
				require.NoError(t, callErr, "a metric lock must not consume the business RPC deadline")
				require.NoError(t, callCtx.Err())
				callCancel()
				require.Equal(t, content == "hello", response.GetCheckResult().GetAllowed())
				if content != "hello" {
					require.Equal(t, "spam_mute", response.GetCheckResult().GetBlockReason())
				}
			}
			var alive int
			require.NoError(t, tx.QueryRow(ctx, "SELECT 1").Scan(&alive), "both responses must precede lock release")
			cleanup()
			settleScanApplication(t, ctx, pool, appPool, appName)
			counts := scanOutcomeCounts(t, m)
			require.EqualValues(t, 2, scanOutcomeTotal(counts))
			require.Zero(t, counts["acknowledged"])
			require.EqualValues(t, 2, counts["unconfirmed"])
			var heldCount int64
			require.NoError(t, tx.QueryRow(ctx, "SELECT messages_checked FROM auto_mod_scan_stats").Scan(&heldCount))
			require.Zero(t, heldCount, "server attempts settled while the lock is still held")
			t.Logf("%s lock still held, outcomes=%v, attributed backends gone, durable=0", lock.name, counts)
			require.NoError(t, tx.Rollback(ctx))
		})
	}
	checked, _, err := (&store.AutoModStore{Pool: pool}).Stats(ctx)
	require.NoError(t, err)
	require.Zero(t, checked, "held-lock attempts cannot fabricate persisted increments")
	// Observe actual production wiring with an acknowledged sample. A no-op
	// implementation cannot pass merely by satisfying reconciliation bounds.
	m := privateScanMetrics()
	client, cleanup := startModerationGRPCTestServer(t, pool, func(s *ModerationGRPC) { s.ScanMetrics = m })
	t.Cleanup(cleanup)
	_, err = client.CheckMessage(ctx, &moderationv1.CheckMessageRequest{Content: "recovered"})
	require.NoError(t, err)
	require.EqualValues(t, 1, scanOutcomeCounts(t, m)["acknowledged"])
	checked, _, err = (&store.AutoModStore{Pool: pool}).Stats(ctx)
	require.NoError(t, err)
	require.EqualValues(t, 1, checked, "persistence resumes after lock release")
}

// Each application pool has its own server identity. No new submitters may be
// started after settleScanApplication; backend absence is stronger than merely
// observing a canceled client or an idle statement snapshot.
func scanApplicationPool(t *testing.T, ctx context.Context, admin *pgxpool.Pool) (*pgxpool.Pool, string) {
	t.Helper()
	config := admin.Config().Copy()
	name := "be238-" + uuid.NewString()
	config.ConnConfig.RuntimeParams["application_name"] = name
	previousAfterConnect := config.AfterConnect
	config.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		t.Logf("application=%s connected backend_pid=%d", name, conn.PgConn().PID())
		if previousAfterConnect != nil {
			return previousAfterConnect(ctx, conn)
		}
		return nil
	}
	pool, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	var pid int
	require.NoError(t, pool.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&pid))
	t.Logf("application=%s initial_pid=%d GOMAXPROCS=%d NumCPU=%d pool_max=%d pool_min=%d", name, pid, runtime.GOMAXPROCS(0), runtime.NumCPU(), config.MaxConns, config.MinConns)
	return pool, name
}

// BE238_TEST_DSN must name an exclusively owned disposable database. It enables
// the same tests on an isolated Docker network without publishing host ports.
// The default local/CI path retains the existing testcontainers fixture.
func startScanPostgres(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("BE238_TEST_DSN")
	if dsn == "" {
		return startModerationPostgresPlatform(t, ctx)
	}
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	_, err = pool.Exec(ctx, "DROP SCHEMA public CASCADE; CREATE SCHEMA public")
	require.NoError(t, err)
	paths, err := filepath.Glob(filepath.Join(repoRoot(t), "src", "backend", "migrations", "moderation_db", "*.up.sql"))
	require.NoError(t, err)
	require.NotEmpty(t, paths)
	for _, path := range paths {
		t.Logf("migration=%s", filepath.Base(path))
		integrationtestApplySQL(t, ctx, pool, path)
	}
	t.Logf("fixture default pool_max=%d pool_min=%d GOMAXPROCS=%d NumCPU=%d", pool.Config().MaxConns, pool.Config().MinConns, runtime.GOMAXPROCS(0), runtime.NumCPU())
	return pool
}

func settleScanApplication(t *testing.T, ctx context.Context, admin, app *pgxpool.Pool, name string) {
	t.Helper()
	stats := app.Stat()
	t.Logf("application=%s acquire_count=%d canceled_acquire=%d empty_acquire=%d acquire_duration=%s", name, stats.AcquireCount(), stats.CanceledAcquireCount(), stats.EmptyAcquireCount(), stats.AcquireDuration())
	closed := make(chan struct{})
	go func() { app.Close(); close(closed) }()
	settleCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	select {
	case <-closed:
	case <-settleCtx.Done():
		t.Fatal("attributed pool did not close before quiescence deadline")
	}
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var active int
		err := admin.QueryRow(settleCtx, `SELECT count(*) FROM pg_stat_activity WHERE application_name=$1`, name).Scan(&active)
		require.NoError(t, err)
		if active == 0 {
			t.Logf("application=%s server backend count=0; no future submitters", name)
			return
		}
		select {
		case <-ticker.C:
		case <-settleCtx.Done():
			t.Fatalf("application=%s has %d server backends after close; cannot prove quiescence", name, active)
		}
	}
}
