package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	commonv1 "voice.app/voice/common/v1"
	"voice/backend/space/internal/spacecore"
)

func TestLifecycleRestoreOutcome_ConcurrentAdmissionAndCompletionConverge(t *testing.T) {
	f := newLifecycleRestoreFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	first := &SpaceStore{Pool: r22SecondSpacePool(t, ctx, f.st.Pool, "restore_race_first")}
	second := &SpaceStore{Pool: r22SecondSpacePool(t, ctx, f.st.Pool, "restore_race_second")}
	blocker, err := f.st.Pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = blocker.Rollback(context.Background()) }()
	_, err = blocker.Exec(ctx, `SELECT pg_advisory_xact_lock($1::bigint)`, spaceMutationAdvisoryKey(f.space))
	require.NoError(t, err)
	type admission struct {
		aggregate *spacecore.LifecycleAggregate
		err       error
	}
	results := make(chan admission, 2)
	for _, st := range []*SpaceStore{first, second} {
		go func() {
			a, err := st.ReserveLifecycleRestore(ctx, f.account, f.actor, 7, f.restore)
			results <- admission{a, err}
		}()
	}
	r22WaitForSpaceLock(t, ctx, f.st.Pool, "restore_race_first")
	r22WaitForSpaceLock(t, ctx, f.st.Pool, "restore_race_second")
	require.NoError(t, blocker.Commit(ctx))
	a, b := <-results, <-results
	require.NoError(t, a.err)
	require.NoError(t, b.err)
	require.Equal(t, a.aggregate.Snapshot(), b.aggregate.Snapshot())
	var count int
	require.NoError(t, f.st.Pool.QueryRow(ctx, `SELECT count(*) FROM space_lifecycle_operations WHERE space_id=$1 AND method='RESTORE'`, f.space).Scan(&count))
	require.Equal(t, 1, count)
	for _, participant := range lifecycleTestParticipants {
		_, err = f.st.RecordLifecycleFenceReceipt(ctx, lifecycleFenceReceiptFixture(t, f.space, uuid.MustParse(f.request.OperationId), participant, 2, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE, lifecycleManifestFixture()))
		require.NoError(t, err)
	}
	blocker, err = f.st.Pool.Begin(ctx)
	require.NoError(t, err)
	_, err = blocker.Exec(ctx, `SELECT pg_advisory_xact_lock($1::bigint)`, spaceMutationAdvisoryKey(f.space))
	require.NoError(t, err)
	type completion struct {
		event spacecore.LifecycleOutboxRecord
		err   error
	}
	completed := make(chan completion, 2)
	for _, st := range []*SpaceStore{first, second} {
		go func() {
			_, event, err := st.CompleteLifecycleRestore(ctx, f.space)
			completed <- completion{event, err}
		}()
	}
	r22WaitForSpaceLock(t, ctx, f.st.Pool, "restore_race_first")
	r22WaitForSpaceLock(t, ctx, f.st.Pool, "restore_race_second")
	require.NoError(t, blocker.Commit(ctx))
	c, d := <-completed, <-completed
	require.NoError(t, c.err)
	require.NoError(t, d.err)
	require.Equal(t, c.event, d.event)
	require.NotNil(t, f.replay(t, second))
	require.Equal(t, []string{"space.deletion_scheduled", "space.restored"}, lifecycleEventTypes(readyLifecycleEvents(t, f.store)))
}
