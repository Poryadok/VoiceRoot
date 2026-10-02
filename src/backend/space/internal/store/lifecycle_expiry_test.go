package store

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/spacecore"
)

func TestLifecycleExpiryEarlyScanCannotRestoreAndDeadlineUsesDatabaseTime(t *testing.T) {
	st := lifecycleStoreFixture(t)
	_, _, spaceID, _, _ := completeStoredSchedule(t, st)
	expiry, ok := any(st).(interface {
		DecideExpiredLifecyclePurge(context.Context, uuid.UUID) (*spacecore.LifecycleAggregate, error)
	})
	require.True(t, ok)
	aggregate, err := expiry.DecideExpiredLifecyclePurge(context.Background(), spaceID)
	require.NoError(t, err)
	require.Equal(t, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_SCHEDULED, aggregate.Phase())
	require.Equal(t, uint64(1), aggregate.Generation())
	_, err = st.Pool.Exec(context.Background(), `UPDATE space_lifecycle_aggregates SET purge_after=clock_timestamp() WHERE space_id=$1`, spaceID)
	require.NoError(t, err)
	aggregate, err = expiry.DecideExpiredLifecyclePurge(context.Background(), spaceID)
	require.NoError(t, err)
	require.Equal(t, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGE_DECIDED, aggregate.Phase())
	require.Equal(t, uint64(2), aggregate.Generation())
	replay, err := expiry.DecideExpiredLifecyclePurge(context.Background(), spaceID)
	require.NoError(t, err)
	require.Equal(t, aggregate.Snapshot(), replay.Snapshot())
}
