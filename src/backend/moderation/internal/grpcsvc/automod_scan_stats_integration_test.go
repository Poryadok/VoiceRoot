package grpcsvc

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
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
	pool := startModerationPostgresPlatform(t, ctx)
	_, err := pool.Exec(ctx, `INSERT INTO auto_mod_log (target_profile_id, trigger, action)
VALUES ($1, 'spam_pattern', 'mute'), ($1, 'report_threshold', 'shadow_ban')`, uuid.New())
	require.NoError(t, err)
	client, cleanup := startModerationGRPCTestServer(t, pool)
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
	stats, err = client.GetAutoModStats(modCtx, &moderationv1.GetAutoModStatsRequest{})
	require.NoError(t, err)
	require.EqualValues(t, 5+concurrent, stats.GetAutoModStats().GetMessagesChecked())
	require.EqualValues(t, 3, stats.GetAutoModStats().GetBlocked(), "existing action aggregation is unchanged")

	reopened, err := pgxpool.New(ctx, pool.Config().ConnString())
	require.NoError(t, err)
	t.Cleanup(reopened.Close)
	checked, blocked, err := (&store.AutoModStore{Pool: reopened}).Stats(ctx)
	require.NoError(t, err)
	require.EqualValues(t, 5+concurrent, checked, "count survives a new pool/store instance")
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
	_, err = client.GetAutoModStats(modCtx, &moderationv1.GetAutoModStatsRequest{})
	require.Equal(t, codes.Internal, status.Code(err), "a missing singleton must not fabricate zero either")
}

func TestAutoModScanStats_HeldCounterLockPreservesDecision(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool := startModerationPostgresPlatform(t, ctx)
	client, cleanup := startModerationGRPCTestServer(t, pool)
	t.Cleanup(cleanup)
	// Warm the real gRPC connection before measuring lock isolation.
	_, err := client.GetAutoModStats(withInternalModCtx(ctx, uuid.New()), &moderationv1.GetAutoModStatsRequest{})
	require.NoError(t, err)
	for _, lock := range []struct{ name, sql string }{
		{"row", "SELECT singleton FROM auto_mod_scan_stats WHERE singleton = true FOR UPDATE"},
		{"table", "LOCK TABLE auto_mod_scan_stats IN ACCESS EXCLUSIVE MODE"},
	} {
		t.Run(lock.name, func(t *testing.T) {
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
		})
	}
	checked, _, err := (&store.AutoModStore{Pool: pool}).Stats(ctx)
	require.NoError(t, err)
	require.Zero(t, checked, "held-lock attempts cannot fabricate persisted increments")
	_, err = client.CheckMessage(ctx, &moderationv1.CheckMessageRequest{Content: "recovered"})
	require.NoError(t, err)
	checked, _, err = (&store.AutoModStore{Pool: pool}).Stats(ctx)
	require.NoError(t, err)
	require.EqualValues(t, 1, checked, "persistence resumes after lock release")
}
