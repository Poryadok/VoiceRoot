package store

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	commonv1 "voice.app/voice/common/v1"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/spacecore"
)

type lifecycleTransitionStore interface {
	RecordLifecycleFenceReceipt(context.Context, *commonv1.SpaceLifecycleFenceReceipt) (*spacecore.LifecycleAggregate, error)
	RecordLifecycleRoleRetirementReceipt(context.Context, *rolev1.RetireSpaceReceipt) (*spacecore.LifecycleAggregate, error)
	RecordLifecyclePurgeReceipt(context.Context, *commonv1.SpacePurgeReceipt) (*spacecore.LifecycleAggregate, error)
	CompleteLifecycleRestore(context.Context, uuid.UUID) (*spacecore.LifecycleAggregate, spacecore.LifecycleOutboxRecord, error)
}

func requireLifecycleTransitions(t *testing.T, st *SpaceStore) lifecycleTransitionStore {
	t.Helper()
	transitions, ok := any(st).(lifecycleTransitionStore)
	require.True(t, ok, "SpaceStore must expose atomic typed receipt transitions and database-time restore completion")
	return transitions
}

func TestLifecycleTransitions_TypedSurfaceRequired(t *testing.T) {
	requireLifecycleTransitions(t, &SpaceStore{})
}

func TestLifecycleTransitions_ConcurrentFenceReceiptsSurviveRestartAndAdvancedReplay(t *testing.T) {
	st := lifecycleStoreFixture(t)
	transitions := requireLifecycleTransitions(t, st)
	store := requireDurableLifecycleStore(t, st)
	aggregate, spaceID, operationID := lifecycleAggregateFixture(t, st)
	require.NoError(t, store.PersistLifecycle(context.Background(), aggregate))
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	blocker, err := st.Pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = blocker.Rollback(context.Background()) }()
	_, err = blocker.Exec(ctx, `SELECT pg_advisory_xact_lock($1::bigint)`, spaceMutationAdvisoryKey(spaceID))
	require.NoError(t, err)
	results := make(chan error, len(lifecycleTestParticipants))
	receipts := make([]*commonv1.SpaceLifecycleFenceReceipt, 0, len(lifecycleTestParticipants))
	for _, participant := range lifecycleTestParticipants {
		receipt := lifecycleFenceReceiptFixture(t, spaceID, operationID, participant, 1, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, lifecycleManifestFixture())
		unknown := protowire.AppendTag(nil, 100, protowire.VarintType)
		receipt.ProtoReflect().SetUnknown(protowire.AppendVarint(unknown, uint64(participant)))
		receipts = append(receipts, receipt)
		name := fmt.Sprintf("lifecycle_receipt_%d", participant)
		worker := requireLifecycleTransitions(t, &SpaceStore{Pool: r22SecondSpacePool(t, ctx, st.Pool, name)})
		go func() {
			_, callErr := worker.RecordLifecycleFenceReceipt(ctx, receipt)
			results <- callErr
		}()
		r22WaitForSpaceLock(t, ctx, st.Pool, name)
	}
	require.NoError(t, blocker.Commit(ctx))
	for range receipts {
		require.NoError(t, <-results, "distinct participant acknowledgements must merge against the locked fresh aggregate")
	}
	restarted := requireDurableLifecycleStore(t, &SpaceStore{Pool: r22SecondSpacePool(t, ctx, st.Pool, "lifecycle_receipt_restart")})
	loaded, err := restarted.LoadLifecycle(ctx, spaceID)
	require.NoError(t, err)
	require.Len(t, loaded.Snapshot().FenceReceipts, len(receipts))
	for _, receipt := range receipts {
		require.True(t, proto.Equal(receipt, loaded.Snapshot().FenceReceipts[receipt.GetParticipantId()]), "durable receipt must retain exact protobuf evidence, including unknown fields")
	}
	_, event, err := restarted.CompleteLifecycleSchedule(ctx, spaceID)
	require.NoError(t, err, "all concurrent receipts must jointly satisfy the barrier")
	for _, receipt := range receipts {
		replay, replayErr := transitions.RecordLifecycleFenceReceipt(ctx, receipt)
		require.NoError(t, replayErr, "exact durable evidence remains replayable after phase advancement")
		require.Equal(t, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_SCHEDULED, replay.Phase())
		tampered := proto.Clone(receipt).(*commonv1.SpaceLifecycleFenceReceipt)
		tampered.ProtoReflect().SetUnknown(nil)
		_, replayErr = transitions.RecordLifecycleFenceReceipt(ctx, tampered)
		require.Error(t, replayErr, "changed receipt bytes must fail closed after advancement")
	}
	require.Equal(t, []spacecore.LifecycleOutboxRecord{event}, readyLifecycleEvents(t, restarted))
	_, err = restarted.DecideLifecycleRecovery(ctx, spaceID, 2)
	require.NoError(t, err)
	for _, receipt := range receipts {
		replay, replayErr := transitions.RecordLifecycleFenceReceipt(ctx, receipt)
		require.NoError(t, replayErr, "older generation receipt must replay from the durable participant ledger")
		require.Equal(t, uint64(2), replay.Generation())
		require.Equal(t, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_RESTORE_DECIDED, replay.Phase())
		require.Empty(t, replay.Snapshot().FenceReceipts, "old receipt must not count toward the new LIVE barrier")
	}
}

func TestLifecycleTransitions_InvalidFenceLeavesDurableStateUnchanged(t *testing.T) {
	st := lifecycleStoreFixture(t)
	transitions := requireLifecycleTransitions(t, st)
	store := requireDurableLifecycleStore(t, st)
	aggregate, spaceID, operationID := lifecycleAggregateFixture(t, st)
	require.NoError(t, store.PersistLifecycle(context.Background(), aggregate))
	valid := lifecycleFenceReceiptFixture(t, spaceID, operationID, lifecycleTestParticipants[0], 1, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, lifecycleManifestFixture())
	for _, mutate := range []func(*commonv1.SpaceLifecycleFenceReceipt){
		func(r *commonv1.SpaceLifecycleFenceReceipt) { r.Generation++ },
		func(r *commonv1.SpaceLifecycleFenceReceipt) { r.DeletionOperationId = uuid.NewString() },
		func(r *commonv1.SpaceLifecycleFenceReceipt) { r.RequestSha256[0] ^= 0xff },
		func(r *commonv1.SpaceLifecycleFenceReceipt) { r.ManifestSha256[0] ^= 0xff },
		func(r *commonv1.SpaceLifecycleFenceReceipt) { r.ParticipantId = 99 },
		func(r *commonv1.SpaceLifecycleFenceReceipt) {
			r.AppliedState = commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE
		},
	} {
		bad := proto.Clone(valid).(*commonv1.SpaceLifecycleFenceReceipt)
		mutate(bad)
		_, err := transitions.RecordLifecycleFenceReceipt(context.Background(), bad)
		require.Error(t, err)
		loaded, err := store.LoadLifecycle(context.Background(), spaceID)
		require.NoError(t, err)
		require.Equal(t, aggregate.Snapshot(), loaded.Snapshot())
	}
	_, err := transitions.RecordLifecycleFenceReceipt(context.Background(), valid)
	require.NoError(t, err, "invalid evidence must not poison a later valid acknowledgement")
}

func TestLifecycleTransitions_RoleRetirementPrecedesPurgeAndReceiptsAreImmutable(t *testing.T) {
	st := lifecycleStoreFixture(t)
	transitions := requireLifecycleTransitions(t, st)
	store, _, spaceID, operationID, _ := completeStoredSchedule(t, st)
	_, err := st.Pool.Exec(context.Background(), `UPDATE space_lifecycle_aggregates SET scheduled_at=scheduled_at-interval '8 days',purge_after=purge_after-interval '8 days' WHERE space_id=$1`, spaceID)
	require.NoError(t, err)
	aggregate, err := store.DecideLifecycleRecovery(context.Background(), spaceID, 2)
	require.NoError(t, err)
	recordLifecycleFenceBarrier(t, aggregate, spaceID, operationID, 2, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED)
	require.NoError(t, aggregate.BeginPurging())
	require.NoError(t, store.PersistLifecycle(context.Background(), aggregate))
	purgeDecidedAt := aggregate.Snapshot().PurgeDecidedAt
	role := lifecycleRoleRetirementReceipt(t, spaceID, operationID, 2, purgeDecidedAt)
	purge := lifecyclePurgeReceipt(t, spaceID, operationID, commonv1.ParticipantId_PARTICIPANT_ID_MESSAGING, 2, purgeDecidedAt)
	_, err = transitions.RecordLifecyclePurgeReceipt(context.Background(), purge)
	require.Error(t, err, "participant purge cannot complete before durable Role retirement")
	loaded, err := store.LoadLifecycle(context.Background(), spaceID)
	require.NoError(t, err)
	require.Nil(t, loaded.Snapshot().RoleReceipt)
	require.Empty(t, loaded.Snapshot().PurgeReceipts)
	badRole := proto.Clone(role).(*rolev1.RetireSpaceReceipt)
	badRole.RequestSha256[0] ^= 0xff
	_, err = transitions.RecordLifecycleRoleRetirementReceipt(context.Background(), badRole)
	require.Error(t, err)
	_, err = transitions.RecordLifecycleRoleRetirementReceipt(context.Background(), role)
	require.NoError(t, err)
	_, err = transitions.RecordLifecycleRoleRetirementReceipt(context.Background(), role)
	require.NoError(t, err)
	badRole = proto.Clone(role).(*rolev1.RetireSpaceReceipt)
	badRole.ReceiptId += "-changed"
	_, err = transitions.RecordLifecycleRoleRetirementReceipt(context.Background(), badRole)
	require.Error(t, err)
	_, err = transitions.RecordLifecyclePurgeReceipt(context.Background(), purge)
	require.NoError(t, err)
	_, err = transitions.RecordLifecyclePurgeReceipt(context.Background(), purge)
	require.NoError(t, err)
	badPurge := proto.Clone(purge).(*commonv1.SpacePurgeReceipt)
	badPurge.ReceiptId += "-changed"
	_, err = transitions.RecordLifecyclePurgeReceipt(context.Background(), badPurge)
	require.Error(t, err)
	restarted := requireDurableLifecycleStore(t, &SpaceStore{Pool: r22SecondSpacePool(t, context.Background(), st.Pool, "lifecycle_purge_restart")})
	loaded, err = restarted.LoadLifecycle(context.Background(), spaceID)
	require.NoError(t, err)
	require.True(t, proto.Equal(role, loaded.Snapshot().RoleReceipt))
	require.Len(t, loaded.Snapshot().PurgeReceipts, 1)
	require.True(t, proto.Equal(purge, loaded.Snapshot().PurgeReceipts[purge.ParticipantId]))
	require.Equal(t, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGING, loaded.Phase())
	require.Equal(t, []string{"space.deletion_scheduled"}, lifecycleEventTypes(readyLifecycleEvents(t, restarted)))
	for _, participant := range lifecycleTestParticipants[1:] {
		if participant == purge.ParticipantId {
			continue
		}
		_, err = transitions.RecordLifecyclePurgeReceipt(context.Background(), lifecyclePurgeReceipt(t, spaceID, operationID, participant, 2, purgeDecidedAt))
		require.NoError(t, err)
	}
	loaded, err = restarted.LoadLifecycle(context.Background(), spaceID)
	require.NoError(t, err)
	require.NoError(t, loaded.RecordLocalPurgeCompleted())
	_, err = loaded.CompletePurge(time.Now().UTC())
	require.NoError(t, err)
	require.NoError(t, store.CompleteLifecyclePurge(context.Background(), loaded, bytes.Repeat([]byte{0xa1}, 32), bytes.Repeat([]byte{0xb2}, 32), "space-tombstone-k7"))
	terminal, err := transitions.RecordLifecycleRoleRetirementReceipt(context.Background(), role)
	require.NoError(t, err, "typed Role evidence remains replayable after PURGED")
	require.Equal(t, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGED, terminal.Phase())
	terminal, err = transitions.RecordLifecyclePurgeReceipt(context.Background(), purge)
	require.NoError(t, err, "purge evidence remains replayable after PURGED")
	require.Equal(t, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGED, terminal.Phase())
	_, err = transitions.RecordLifecycleRoleRetirementReceipt(context.Background(), badRole)
	require.Error(t, err)
	_, err = transitions.RecordLifecyclePurgeReceipt(context.Background(), badPurge)
	require.Error(t, err)
	loaded, err = restarted.LoadLifecycle(context.Background(), spaceID)
	require.NoError(t, err)
	require.True(t, proto.Equal(role, loaded.Snapshot().RoleReceipt))
	require.True(t, proto.Equal(purge, loaded.Snapshot().PurgeReceipts[purge.ParticipantId]))
}

func TestLifecycleTransitions_RestoreBarrierAndFreshDatabaseClockWithStableReplay(t *testing.T) {
	st := lifecycleStoreFixture(t)
	transitions := requireLifecycleTransitions(t, st)
	store, _, spaceID, operationID, scheduleEvent := completeStoredSchedule(t, st)
	_, err := store.DecideLifecycleRecovery(context.Background(), spaceID, 2)
	require.NoError(t, err)
	for _, participant := range lifecycleTestParticipants[:len(lifecycleTestParticipants)-1] {
		receipt := lifecycleFenceReceiptFixture(t, spaceID, operationID, participant, 2, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE, lifecycleManifestFixture())
		_, err = transitions.RecordLifecycleFenceReceipt(context.Background(), receipt)
		require.NoError(t, err)
	}
	_, _, err = transitions.CompleteLifecycleRestore(context.Background(), spaceID)
	require.Error(t, err, "nine acknowledgements cannot complete restoration")
	loaded, err := store.LoadLifecycle(context.Background(), spaceID)
	require.NoError(t, err)
	require.Equal(t, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_RESTORE_DECIDED, loaded.Phase())
	require.Equal(t, []spacecore.LifecycleOutboxRecord{scheduleEvent}, readyLifecycleEvents(t, store))
	_, err = st.Pool.Exec(context.Background(), `UPDATE space_lifecycle_outbox SET state='DELIVERED',delivered_at=clock_timestamp() WHERE event_id=$1`, uuid.MustParse(scheduleEvent.EventID))
	require.NoError(t, err)
	last := lifecycleFenceReceiptFixture(t, spaceID, operationID, lifecycleTestParticipants[len(lifecycleTestParticipants)-1], 2, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE, lifecycleManifestFixture())
	_, err = transitions.RecordLifecycleFenceReceipt(context.Background(), last)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	blocker, err := st.Pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = blocker.Rollback(context.Background()) }()
	_, err = blocker.Exec(ctx, `SELECT pg_advisory_xact_lock($1::bigint)`, spaceMutationAdvisoryKey(spaceID))
	require.NoError(t, err)
	worker := requireLifecycleTransitions(t, &SpaceStore{Pool: r22SecondSpacePool(t, ctx, st.Pool, "lifecycle_restore_clock")})
	type completion struct {
		aggregate *spacecore.LifecycleAggregate
		event     spacecore.LifecycleOutboxRecord
		err       error
	}
	result := make(chan completion, 1)
	go func() {
		aggregate, event, callErr := worker.CompleteLifecycleRestore(ctx, spaceID)
		result <- completion{aggregate, event, callErr}
	}()
	r22WaitForSpaceLock(t, ctx, st.Pool, "lifecycle_restore_clock")
	var beforeUnlock time.Time
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&beforeUnlock))
	require.NoError(t, blocker.Commit(ctx))
	completed := <-result
	require.NoError(t, completed.err)
	var afterCompletion time.Time
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&afterCompletion))
	require.False(t, completed.event.OccurredAt.Before(beforeUnlock), "restore timestamp must be sampled after the shared lock, not at transaction start")
	require.False(t, completed.event.OccurredAt.After(afterCompletion))
	require.Equal(t, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_LIVE, completed.aggregate.Phase())
	require.Equal(t, "space.restored", completed.event.EventType)
	require.Equal(t, uint64(2), completed.event.Generation)
	restartedStore := &SpaceStore{Pool: r22SecondSpacePool(t, ctx, st.Pool, "lifecycle_restore_restart")}
	replayed, event, err := requireLifecycleTransitions(t, restartedStore).CompleteLifecycleRestore(ctx, spaceID)
	require.NoError(t, err)
	require.Equal(t, completed.event, event)
	require.Equal(t, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_LIVE, replayed.Phase())
	require.Equal(t, []spacecore.LifecycleOutboxRecord{event}, readyLifecycleEvents(t, requireDurableLifecycleStore(t, restartedStore)))
	_, err = st.Pool.Exec(ctx, `UPDATE space_lifecycle_outbox SET state='DELIVERED',delivered_at=clock_timestamp() WHERE event_id=$1`, uuid.MustParse(event.EventID))
	require.NoError(t, err)
	_, deliveredReplay, err := requireLifecycleTransitions(t, restartedStore).CompleteLifecycleRestore(ctx, spaceID)
	require.NoError(t, err, "completed replay must not rewrite an already delivered outbox row")
	require.Equal(t, event, deliveredReplay)
	var deliveryState string
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT state FROM space_lifecycle_outbox WHERE event_id=$1`, uuid.MustParse(event.EventID)).Scan(&deliveryState))
	require.Equal(t, "DELIVERED", deliveryState)
}

func TestLifecycleTransitions_ExactScheduleReplayReturnsSavedAdvancedPhase(t *testing.T) {
	for _, scheduled := range []bool{false, true} {
		t.Run(fmt.Sprintf("scheduled_%t", scheduled), func(t *testing.T) {
			st := lifecycleStoreFixture(t)
			store, aggregate, accountID, actorID, spaceID, request, proofBytes, proofHash := reserveLifecycleOperationFixture(t, st)
			operationID := uuid.MustParse(request.OperationId)
			_, err := store.RecordLifecycleDeletionProofReceipt(context.Background(), actorID, operationID, proofBytes, proofHash)
			require.NoError(t, err)
			require.NoError(t, aggregate.BeginFreeze(lifecycleManifestFixture()))
			if scheduled {
				recordLifecycleFenceBarrier(t, aggregate, spaceID, operationID, 1, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN)
			}
			require.NoError(t, store.PersistLifecycle(context.Background(), aggregate))
			if scheduled {
				_, _, err = store.CompleteLifecycleSchedule(context.Background(), spaceID)
				require.NoError(t, err)
			}
			_, err = st.Pool.Exec(context.Background(), `UPDATE spaces SET name='changed after reservation',owner_profile_id=$2 WHERE id=$1`, spaceID, uuid.New())
			require.NoError(t, err)
			expected, err := store.LoadLifecycle(context.Background(), spaceID)
			require.NoError(t, err)
			replay, err := store.ReserveLifecycleSchedule(context.Background(), accountID, actorID, 7, request)
			require.NoError(t, err, "saved operation binding must be checked before current name/owner and must not recreate generation one")
			require.Equal(t, expected.Snapshot(), replay.Snapshot())
			for _, mutate := range []func(*spacev1.DeleteSpaceRequest){
				func(r *spacev1.DeleteSpaceRequest) { r.ConfirmationName = "changed after reservation" },
				func(r *spacev1.DeleteSpaceRequest) { r.Proof += "-changed" },
			} {
				changed := proto.Clone(request).(*spacev1.DeleteSpaceRequest)
				mutate(changed)
				_, err = store.ReserveLifecycleSchedule(context.Background(), accountID, actorID, 7, changed)
				require.Error(t, err, "same operation with a changed body must conflict")
			}
			for _, identity := range []struct {
				account, actor uuid.UUID
				epoch          int64
			}{
				{uuid.New(), actorID, 7}, {accountID, uuid.New(), 7}, {accountID, actorID, 8},
			} {
				_, err = store.ReserveLifecycleSchedule(context.Background(), identity.account, identity.actor, identity.epoch, request)
				require.Error(t, err, "replay must retain the original authenticated principal binding")
			}
			loaded, err := store.LoadLifecycle(context.Background(), spaceID)
			require.NoError(t, err)
			require.Equal(t, expected.Snapshot(), loaded.Snapshot())
		})
	}
}

func TestLifecycleTransitions_NewScheduleOperationCannotReplacePendingReservation(t *testing.T) {
	st := lifecycleStoreFixture(t)
	store, _, accountID, actorID, spaceID, request, _, _ := reserveLifecycleOperationFixture(t, st)
	before, err := store.LoadLifecycle(context.Background(), spaceID)
	require.NoError(t, err)
	changed := proto.Clone(request).(*spacev1.DeleteSpaceRequest)
	changed.OperationId = uuid.NewString()
	_, err = store.ReserveLifecycleSchedule(context.Background(), accountID, actorID, 7, changed)
	require.Error(t, err, "new operation must not replace an already reserved lifecycle")
	after, err := store.LoadLifecycle(context.Background(), spaceID)
	require.NoError(t, err)
	require.Equal(t, before.Snapshot(), after.Snapshot())
	var count int
	require.NoError(t, st.Pool.QueryRow(context.Background(), `SELECT count(*) FROM space_lifecycle_operations WHERE operation_id=$1`, changed.OperationId).Scan(&count))
	require.Zero(t, count, "rejected reservation must not leave an orphan operation row")
}
