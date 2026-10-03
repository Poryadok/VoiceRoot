package store

import (
	"context"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLifecycleRetryPersistsBackoffStallAndResetsOnlyOnProgress(t *testing.T) {
	st := lifecycleStoreFixture(t)
	ctx := context.Background()
	for _, name := range []string{"000014_ownership_outbox_delivery.up.sql", "000015_audit_ledger.up.sql", "000022_lifecycle_runtime.up.sql"} {
		raw, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/space_db", name))
		require.NoError(t, err)
		_, err = st.Pool.Exec(ctx, string(raw))
		require.NoError(t, err)
	}
	_, _, _, _, id, _, _, _ := reserveLifecycleOperationFixture(t, st)
	attempts, err := st.ListDueLifecycleAttempts(ctx, 100)
	require.NoError(t, err)
	require.Len(t, attempts, 1)
	require.False(t, attempts[0].Stalled)
	require.NoError(t, st.DeferLifecycleAttempt(ctx, id))
	var attempt int
	var seconds float64
	var progress time.Time
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT retry_attempt,extract(epoch FROM next_attempt_at-last_failure_at)::float8,progress_at FROM space_lifecycle_aggregates WHERE space_id=$1`, id).Scan(&attempt, &seconds, &progress))
	require.Equal(t, 1, attempt)
	require.InDelta(t, 1.1, seconds, 0.11)
	attempts, err = st.ListDueLifecycleAttempts(ctx, 100)
	require.NoError(t, err)
	require.Empty(t, attempts, "restart scanner respects durable next-attempt time")
	_, err = st.Pool.Exec(ctx, `UPDATE space_lifecycle_aggregates SET retry_attempt=999999,next_attempt_at=clock_timestamp(),progress_at=clock_timestamp()-interval '16 minutes' WHERE space_id=$1`, id)
	require.NoError(t, err)
	attempts, err = st.ListDueLifecycleAttempts(ctx, 100)
	require.NoError(t, err)
	require.Len(t, attempts, 1)
	require.True(t, attempts[0].Stalled)
	require.NoError(t, st.DeferLifecycleAttempt(ctx, id))
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT retry_attempt,extract(epoch FROM next_attempt_at-last_failure_at)::float8 FROM space_lifecycle_aggregates WHERE space_id=$1`, id).Scan(&attempt, &seconds))
	require.Equal(t, 1000000, attempt)
	require.InDelta(t, 300, seconds, 0.01, "attempt count never stops retrying; total delay remains capped")
	// A failure must not refresh progress and hide the fifteen-minute alert.
	var stillOld bool
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT progress_at<clock_timestamp()-interval '15 minutes' FROM space_lifecycle_aggregates WHERE space_id=$1`, id).Scan(&stillOld))
	require.True(t, stillOld)
	_, err = st.Pool.Exec(ctx, `UPDATE space_lifecycle_aggregates SET phase='FREEZE_PENDING',manifest_id=$2,manifest_sha256=$3,manifest_item_count=0 WHERE space_id=$1`, id, lifecycleManifestFixture().ManifestId, lifecycleManifestFixture().ManifestSha256)
	require.NoError(t, err)
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT retry_attempt,progress_at FROM space_lifecycle_aggregates WHERE space_id=$1`, id).Scan(&attempt, &progress))
	require.Zero(t, attempt)
	require.WithinDuration(t, time.Now().UTC(), progress, time.Second)
}
