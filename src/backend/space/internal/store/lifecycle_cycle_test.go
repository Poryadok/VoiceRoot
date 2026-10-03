package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	commonv1 "voice.app/voice/common/v1"
	spacev1 "voice.app/voice/space/v1"
)

func TestLifecycleCycle_RestoredSpaceReschedulesWithHigherGenerationAndPreservesReplay(t *testing.T) {
	f := newLifecycleRestoreFixture(t)
	ctx := context.Background()
	f.admit(t, len(lifecycleTestParticipants))
	_, _, err := requireLifecycleTransitions(t, f.st).CompleteLifecycleRestore(ctx, f.space)
	require.NoError(t, err)
	savedRestore := f.replay(t, f.st)
	oldDelete := readLifecycleOutcomeEvidence(t, f.lifecycleOutcomeFixture)

	request := proto.Clone(f.request).(*spacev1.DeleteSpaceRequest)
	request.OperationId = uuid.NewString()
	request.Proof = "fresh-proof-for-second-cycle"
	st := &SpaceStore{Pool: f.st.Pool} // a restarted owner reads the durable generation
	next, err := st.ReserveLifecycleSchedule(ctx, f.account, f.actor, 7, request)
	require.NoError(t, err)
	require.Equal(t, uint64(3), next.Generation())
	require.Equal(t, request.OperationId, next.Snapshot().DeletionOperationID)
	require.Equal(t, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_SCHEDULE_PENDING, next.Phase())
	require.Nil(t, next.Snapshot().Manifest)
	second := proto.Clone(request).(*spacev1.DeleteSpaceRequest)
	second.OperationId = uuid.NewString()
	_, err = st.ReserveLifecycleSchedule(ctx, f.account, f.actor, 7, second)
	require.ErrorIs(t, err, ErrLifecycleConflict, "pending intent cannot be replaced")
	replayed, err := st.ReserveLifecycleSchedule(ctx, f.account, f.actor, 7, request)
	require.NoError(t, err)
	require.Equal(t, next.Snapshot(), replayed.Snapshot())
	_, err = requireLifecycleTransitions(t, st).RecordLifecycleFenceReceipt(ctx, lifecycleFenceReceiptFixture(t,
		f.space, uuid.MustParse(f.request.OperationId), lifecycleTestParticipants[0], 2,
		commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE, lifecycleManifestFixture()))
	require.NoError(t, err, "exact retained receipt replay is inert")
	current, err := st.LoadLifecycle(ctx, f.space)
	require.NoError(t, err)
	require.Equal(t, next.Snapshot(), current.Snapshot(), "late old-cycle LIVE receipt cannot unfreeze the new intent")
	requireLifecycleOutcomeReplay(t, f.lifecycleOutcomeFixture, st)
	restoreReplay, err := st.ReplayLifecycleRestoreOutcome(ctx, f.account, f.actor, 7, f.restore)
	require.NoError(t, err)
	require.True(t, proto.Equal(savedRestore, restoreReplay))
	require.Equal(t, oldDelete, readLifecycleOutcomeEvidence(t, f.lifecycleOutcomeFixture))

	// A second complete freeze/restore proves historical restore evidence does
	// not constrain recovery of the active cycle.
	op := uuid.MustParse(request.OperationId)
	_, confirmationHash, proofDigest := lifecycleScheduleRequestEvidence(t, request)
	proofBytes, proofHash := lifecycleAuthReceiptEvidence(t, f.account, f.actor, f.space, op, 7, confirmationHash, proofDigest)
	_, err = st.RecordLifecycleDeletionProofReceipt(ctx, f.actor, op, proofBytes, proofHash)
	require.NoError(t, err)
	require.NoError(t, next.BeginFreeze(lifecycleManifestFixture()))
	recordLifecycleFenceBarrier(t, next, f.space, op, 3, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN)
	require.NoError(t, st.PersistLifecycle(ctx, next))
	_, _, err = st.CompleteLifecycleSchedule(ctx, f.space)
	require.NoError(t, err)
	restore := &spacev1.RestoreSpaceRequest{SpaceId: f.space.String(), OperationId: uuid.NewString()}
	active, err := st.ReserveLifecycleRestore(ctx, f.account, f.actor, 7, restore)
	require.NoError(t, err)
	require.Equal(t, uint64(4), active.Generation())
	for _, participant := range lifecycleTestParticipants {
		_, err = st.RecordLifecycleFenceReceipt(ctx, lifecycleFenceReceiptFixture(t, f.space, op, participant, 4,
			commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE, lifecycleManifestFixture()))
		require.NoError(t, err)
	}
	_, _, err = st.CompleteLifecycleRestore(ctx, f.space)
	require.NoError(t, err)
	active, err = st.LoadLifecycle(ctx, f.space)
	require.NoError(t, err)
	require.Equal(t, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_LIVE, active.Phase())
	require.Equal(t, uint64(4), active.Generation())
	restoreReplay, err = st.ReplayLifecycleRestoreOutcome(ctx, f.account, f.actor, 7, f.restore)
	require.NoError(t, err)
	require.True(t, proto.Equal(savedRestore, restoreReplay))
}
