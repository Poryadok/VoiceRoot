package spacecore

import (
	"github.com/stretchr/testify/require"
	"testing"
	"time"
	commonv1 "voice.app/voice/common/v1"
	spacev1 "voice.app/voice/space/v1"
)

// Realtime is an additive mandatory participant. Ten historical receipts cannot
// authorize destructive scheduling while the enqueue-drain receipt is missing.
func TestTenHistoricalFenceReceiptsCannotScheduleWithoutRealtime(t *testing.T) {
	aggregate := newLifecycleAggregate(t)
	beginFreeze(t, aggregate, 1)
	for id := commonv1.ParticipantId(1); id <= 10; id++ {
		require.NoError(t, aggregate.RecordFenceReceipt(fenceReceipt(id, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, 1)))
	}
	_, err := aggregate.CompleteSchedule(time.Now().UTC())
	require.Error(t, err)
	require.Equal(t, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_FREEZE_PENDING, aggregate.Phase())
}
