package lifecyclecoord

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
	"voice/backend/space/internal/spacecore"
)

type restoringExpiryStore struct {
	*memoryLifecycleStore
	restored *spacecore.LifecycleAggregate
}

func (s *restoringExpiryStore) DecideExpiredLifecyclePurge(context.Context, uuid.UUID) (*spacecore.LifecycleAggregate, error) {
	s.aggregate = s.restored
	return s.restored, nil
}

func TestRecoveryConcurrentRestoreDoesNotEnterPurgeBarrier(t *testing.T) {
	snapshot := purgingAggregate(t).Snapshot()
	// Reconstruct the earlier scheduled phase from its complete schedule barrier.
	scheduled, err := spacecore.NewLifecycleAggregate(snapshot.SpaceID, snapshot.DeletionOperationID)
	require.NoError(t, err)
	require.NoError(t, scheduled.BeginSchedule(1))
	require.NoError(t, scheduled.BeginFreeze(snapshot.Manifest))
	participants := &recordingParticipants{}
	for _, id := range spacecore.CanonicalLifecycleParticipants() {
		request, err := scheduled.FenceRequest(id)
		require.NoError(t, err)
		receipt, err := participants.ApplySpaceLifecycleFence(context.Background(), id, request)
		require.NoError(t, err)
		require.NoError(t, scheduled.RecordFenceReceipt(receipt))
	}
	_, err = scheduled.CompleteSchedule(time.Now().UTC())
	require.NoError(t, err)
	restored, err := spacecore.RestoreLifecycleAggregate(scheduled.Snapshot())
	require.NoError(t, err)
	_, err = restored.DecideRecovery(time.Now().UTC(), 2)
	require.NoError(t, err)
	for _, id := range spacecore.CanonicalLifecycleParticipants() {
		request, err := restored.FenceRequest(id)
		require.NoError(t, err)
		receipt, err := participants.ApplySpaceLifecycleFence(context.Background(), id, request)
		require.NoError(t, err)
		require.NoError(t, restored.RecordFenceReceipt(receipt))
	}
	_, err = restored.CompleteRestore(time.Now().UTC())
	require.NoError(t, err)
	st := &restoringExpiryStore{memoryLifecycleStore: &memoryLifecycleStore{aggregate: scheduled}, restored: restored}
	coordinator := New(Dependencies{Store: st})
	require.NoError(t, coordinator.Recover(context.Background(), uuid.MustParse(snapshot.SpaceID), nil), "a restore that wins the locked decision leaves LIVE inert")
}
