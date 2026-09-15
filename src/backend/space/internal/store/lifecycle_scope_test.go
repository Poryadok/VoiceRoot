package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	commonv1 "voice.app/voice/common/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/spacecore"
)

var lifecycleFrozenPhases = []string{
	"SCHEDULE_PENDING", "FREEZE_PENDING", "SCHEDULED",
	"RESTORE_DECIDED", "PURGE_DECIDED", "PURGING", "PURGED",
}

// This is an ordinary-access authority fixture, not evidence that participant
// receipts or the destructive purge coordinator have completed.
func seedLifecycleScopePhase(t *testing.T, st *SpaceStore, spaceID uuid.UUID, phase string) {
	t.Helper()
	_, err := st.Pool.Exec(context.Background(), `INSERT INTO space_lifecycle_aggregates(
		space_id,deletion_operation_id,phase,generation) VALUES($1,$2,$3,1)`, spaceID, uuid.New(), phase)
	require.NoError(t, err)
}

func lifecycleScopeSnapshot(t *testing.T, st *SpaceStore, spaceID uuid.UUID) string {
	t.Helper()
	var lifecycle string
	require.NoError(t, st.Pool.QueryRow(context.Background(), `SELECT jsonb_build_object(
		'aggregate',(SELECT jsonb_agg(to_jsonb(x)) FROM space_lifecycle_aggregates x WHERE space_id=$1),
		'operations',(SELECT jsonb_agg(to_jsonb(x) ORDER BY operation_id) FROM space_lifecycle_operations x WHERE space_id=$1),
		'participants',(SELECT jsonb_agg(to_jsonb(x) ORDER BY generation,participant_id,request_kind) FROM space_lifecycle_participants x WHERE space_id=$1),
		'outbox',(SELECT jsonb_agg(to_jsonb(x) ORDER BY event_id) FROM space_lifecycle_outbox x WHERE space_id=$1)
	)::text`, spaceID).Scan(&lifecycle))
	return r20ScopeSnapshot(t, st, spaceID) + "\n" + lifecycle
}

func TestLifecycleScope_OrdinaryMethodsRejectEveryNonLivePhase(t *testing.T) {
	st := lifecycleStoreFixture(t)
	for _, phase := range lifecycleFrozenPhases {
		for _, tc := range r20OrdinaryScopeCases() {
			// Deletion hides rows without making unrelated live Spaces unavailable.
			if tc.name == "ListMySpacesPage" {
				continue
			}
			t.Run(phase+"/"+tc.name, func(t *testing.T) {
				f := newR20ScopeFixture(t, st, "")
				seedLifecycleScopePhase(t, st, f.binding.SpaceID, phase)
				before := lifecycleScopeSnapshot(t, st, f.binding.SpaceID)
				assert.ErrorIs(t, tc.call(st, f), ErrLifecycleFrozen)
				assert.Equal(t, before, lifecycleScopeSnapshot(t, st, f.binding.SpaceID), "denied ordinary operation mutated durable data")
			})
		}
	}
}

func TestLifecycleScope_ListHidesEveryNonLivePhaseAndKeepsLiveSpace(t *testing.T) {
	st := lifecycleStoreFixture(t)
	for _, phase := range lifecycleFrozenPhases {
		t.Run(phase, func(t *testing.T) {
			frozen, live := newR20ScopeFixture(t, st, ""), newR20ScopeFixture(t, st, "")
			second := newR20ScopeFixture(t, st, "")
			_, err := st.Pool.Exec(context.Background(), `INSERT INTO space_members(space_id,profile_id) VALUES($1,$2)`, live.binding.SpaceID, frozen.member)
			require.NoError(t, err)
			_, err = st.Pool.Exec(context.Background(), `INSERT INTO space_members(space_id,profile_id) VALUES($1,$2)`, second.binding.SpaceID, frozen.member)
			require.NoError(t, err)
			for i, id := range []uuid.UUID{second.binding.SpaceID, frozen.binding.SpaceID, live.binding.SpaceID} {
				_, err = st.Pool.Exec(context.Background(), `UPDATE space_members SET joined_at=$3 WHERE space_id=$1 AND profile_id=$2`, id, frozen.member, time.Date(2026, 1, i+1, 0, 0, 0, 0, time.UTC))
				require.NoError(t, err)
			}
			seedLifecycleScopePhase(t, st, frozen.binding.SpaceID, phase)
			page, err := st.ListMySpacesPage(context.Background(), frozen.member, "", 1)
			require.NoError(t, err)
			require.NotNil(t, page)
			require.Len(t, page.Rows, 1)
			require.Equal(t, live.binding.SpaceID, page.Rows[0].ID)
			require.NotEmpty(t, page.NextCursor)
			page, err = st.ListMySpacesPage(context.Background(), frozen.member, page.NextCursor, 1)
			require.NoError(t, err)
			require.Len(t, page.Rows, 1)
			require.Equal(t, second.binding.SpaceID, page.Rows[0].ID)
			require.Empty(t, page.NextCursor, "hidden rows must not create a phantom extra page")
		})
	}
}

func TestLifecycleScope_CoMembersFiltersImplicitButRejectsExplicitFrozenScope(t *testing.T) {
	st := lifecycleStoreFixture(t)
	for _, phase := range lifecycleFrozenPhases {
		t.Run(phase, func(t *testing.T) {
			frozen := newR20ScopeFixture(t, st, "")
			seedLifecycleScopePhase(t, st, frozen.binding.SpaceID, phase)
			shared, err := st.AreCoMembers(context.Background(), frozen.owner, frozen.member, nil)
			require.NoError(t, err)
			require.False(t, shared, "non-LIVE Space is not shared live membership")
			live := newR20ScopeFixture(t, st, "")
			for _, profile := range []uuid.UUID{frozen.owner, frozen.member} {
				_, err = st.Pool.Exec(context.Background(), `INSERT INTO space_members(space_id,profile_id) VALUES($1,$2)`, live.binding.SpaceID, profile)
				require.NoError(t, err)
			}
			shared, err = st.AreCoMembers(context.Background(), frozen.owner, frozen.member, nil)
			require.NoError(t, err)
			require.True(t, shared, "unrelated shared live Space remains usable")
			for _, ids := range [][]uuid.UUID{{frozen.binding.SpaceID}, {live.binding.SpaceID, frozen.binding.SpaceID}, {frozen.binding.SpaceID, live.binding.SpaceID}} {
				shared, err = st.AreCoMembers(context.Background(), frozen.owner, frozen.member, ids)
				require.ErrorIs(t, err, ErrLifecycleFrozen)
				require.False(t, shared)
			}
		})
	}
}

func TestLifecycleScope_OrdinaryAccessResumesAfterCompleteRestore(t *testing.T) {
	st := lifecycleStoreFixture(t)
	store, _, spaceID, operationID, _ := completeStoredSchedule(t, st)
	ctx := context.Background()
	_, err := st.GetSpace(ctx, spaceID)
	require.ErrorIs(t, err, ErrLifecycleFrozen)
	_, err = store.DecideLifecycleRecovery(ctx, spaceID, 2)
	require.NoError(t, err)
	transitions := requireLifecycleTransitions(t, st)
	for _, participant := range lifecycleTestParticipants[:len(lifecycleTestParticipants)-1] {
		_, err = transitions.RecordLifecycleFenceReceipt(ctx, lifecycleFenceReceiptFixture(t, spaceID, operationID, participant, 2, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE, lifecycleManifestFixture()))
		require.NoError(t, err)
	}
	_, err = st.GetSpace(ctx, spaceID)
	require.ErrorIs(t, err, ErrLifecycleFrozen, "partial restoration must remain frozen")
	last := lifecycleTestParticipants[len(lifecycleTestParticipants)-1]
	_, err = transitions.RecordLifecycleFenceReceipt(ctx, lifecycleFenceReceiptFixture(t, spaceID, operationID, last, 2, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE, lifecycleManifestFixture()))
	require.NoError(t, err)
	_, err = st.GetSpace(ctx, spaceID)
	require.ErrorIs(t, err, ErrLifecycleFrozen, "receipts alone must not publish LIVE")
	aggregate, _, err := transitions.CompleteLifecycleRestore(ctx, spaceID)
	require.NoError(t, err)
	require.Equal(t, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_LIVE, aggregate.Phase())
	row, err := st.GetSpace(ctx, spaceID)
	require.NoError(t, err)
	require.NotNil(t, row)
	name := "Restored Space"
	updated, err := st.UpdateSpace(ctx, spaceID, UpdateSpaceInput{Name: &name})
	require.NoError(t, err)
	require.Equal(t, name, updated.Name)
	page, err := st.ListMySpacesPage(ctx, row.OwnerProfileID, "", 10)
	require.NoError(t, err)
	require.Len(t, page.Rows, 1)
	require.Equal(t, spaceID, page.Rows[0].ID)
}

func TestLifecycleScope_AuthorityUncertaintyFailsClosed(t *testing.T) {
	st := lifecycleStoreFixture(t)
	f := newR20ScopeFixture(t, st, "")
	before := r20ScopeSnapshot(t, st, f.binding.SpaceID)
	_, err := st.Pool.Exec(context.Background(), `ALTER TABLE space_lifecycle_aggregates RENAME TO unavailable_lifecycle_authority`)
	require.NoError(t, err)
	for _, tc := range r20OrdinaryScopeCases() {
		t.Run(tc.name, func(t *testing.T) {
			assert.ErrorIs(t, tc.call(st, f), ErrOwnershipScopeUnavailable, "missing authority must not be interpreted as LIVE")
			assert.Equal(t, before, r20ScopeSnapshot(t, st, f.binding.SpaceID))
		})
	}
	shared, err := st.AreCoMembers(context.Background(), f.owner, f.member, nil)
	require.ErrorIs(t, err, ErrOwnershipScopeUnavailable)
	require.False(t, shared)
}

func TestLifecycleScope_OwnershipReservationRejectsEveryNonLivePhase(t *testing.T) {
	st := lifecycleStoreFixture(t)
	for _, phase := range lifecycleFrozenPhases {
		t.Run(phase, func(t *testing.T) {
			f := newR20ScopeFixture(t, st, "")
			seedLifecycleScopePhase(t, st, f.binding.SpaceID, phase)
			before := lifecycleScopeSnapshot(t, st, f.binding.SpaceID)
			_, err := st.ReserveOwnership(context.Background(), f.binding)
			require.ErrorIs(t, err, ErrLifecycleFrozen)
			require.Equal(t, before, lifecycleScopeSnapshot(t, st, f.binding.SpaceID))
		})
	}
}

func TestLifecycleScope_ScheduleReservationRejectsPendingOwnership(t *testing.T) {
	st := lifecycleStoreFixture(t)
	for _, phase := range []string{"reserved", "proof_confirmed", "prepared", "commit_decided", "abort_decided"} {
		t.Run(phase, func(t *testing.T) {
			f := newR20ScopeFixture(t, st, phase)
			request := lifecycleScheduleRequest(f.binding.SpaceID, uuid.New())
			request.ConfirmationName = "Ownership reservation"
			before := lifecycleScopeSnapshot(t, st, f.binding.SpaceID)
			_, err := st.ReserveLifecycleSchedule(context.Background(), f.account, f.currentOwner, 7, request)
			require.ErrorIs(t, err, ErrOwnershipFrozen)
			require.Equal(t, before, lifecycleScopeSnapshot(t, st, f.binding.SpaceID))
		})
	}
}

func TestLifecycleScope_FirstAggregatePersistenceRejectsPendingOwnership(t *testing.T) {
	st := lifecycleStoreFixture(t)
	for _, phase := range []string{"reserved", "proof_confirmed", "prepared", "commit_decided", "abort_decided"} {
		t.Run(phase, func(t *testing.T) {
			f := newR20ScopeFixture(t, st, phase)
			aggregate, err := spacecore.NewLifecycleAggregate(f.binding.SpaceID.String(), uuid.NewString())
			require.NoError(t, err)
			require.NoError(t, aggregate.BeginSchedule(1))
			before := lifecycleScopeSnapshot(t, st, f.binding.SpaceID)
			require.ErrorIs(t, st.PersistLifecycle(context.Background(), aggregate), ErrOwnershipFrozen)
			require.Equal(t, before, lifecycleScopeSnapshot(t, st, f.binding.SpaceID))
		})
	}
}

func TestLifecycleScope_OrdinaryTransactionHoldsLockUntilReadCompletes(t *testing.T) {
	st := lifecycleStoreFixture(t)
	f := newR20ScopeFixture(t, st, "")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	ordinaryReady, finishOrdinary := make(chan struct{}), make(chan struct{})
	ordinaryDone := make(chan error, 1)
	go func() {
		ordinaryDone <- st.withOwnershipScope(ctx, []uuid.UUID{f.binding.SpaceID}, func(scoped *SpaceStore) error {
			close(ordinaryReady)
			select {
			case <-finishOrdinary:
			case <-ctx.Done():
				return ctx.Err()
			}
			_, err := scoped.GetSpace(ctx, f.binding.SpaceID)
			return err
		})
	}()
	select {
	case <-ordinaryReady:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	worker := &SpaceStore{Pool: r22SecondSpacePool(t, ctx, st.Pool, "lifecycle_after_ordinary")}
	reserved := make(chan error, 1)
	go func() {
		request := lifecycleScheduleRequest(f.binding.SpaceID, uuid.New())
		request.ConfirmationName = "Ownership reservation"
		_, err := worker.ReserveLifecycleSchedule(ctx, f.account, f.owner, 7, request)
		reserved <- err
	}()
	r22WaitForSpaceLock(t, ctx, st.Pool, "lifecycle_after_ordinary")
	close(finishOrdinary)
	require.NoError(t, <-ordinaryDone)
	require.NoError(t, <-reserved)
	_, err := st.GetSpace(ctx, f.binding.SpaceID)
	require.ErrorIs(t, err, ErrLifecycleFrozen)
}

func TestLifecycleScope_QueuedReadObservesCommittedScheduleReservation(t *testing.T) {
	st := lifecycleStoreFixture(t)
	f := newR20ScopeFixture(t, st, "")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	blocker, err := st.Pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = blocker.Rollback(context.Background()) }()
	_, err = blocker.Exec(ctx, `SELECT pg_advisory_xact_lock($1::bigint)`, spaceMutationAdvisoryKey(f.binding.SpaceID))
	require.NoError(t, err)
	request := lifecycleScheduleRequest(f.binding.SpaceID, uuid.New())
	request.ConfirmationName = "Ownership reservation"
	reservationWorker := &SpaceStore{Pool: r22SecondSpacePool(t, ctx, st.Pool, "lifecycle_scope_reservation")}
	reserved := make(chan error, 1)
	go func() {
		_, callErr := reservationWorker.ReserveLifecycleSchedule(ctx, f.account, f.owner, 7, request)
		reserved <- callErr
	}()
	r22WaitForSpaceLock(t, ctx, st.Pool, "lifecycle_scope_reservation")
	readWorker := &SpaceStore{Pool: r22SecondSpacePool(t, ctx, st.Pool, "lifecycle_scope_read")}
	type readResult struct {
		row *SpaceRow
		err error
	}
	read := make(chan readResult, 1)
	go func() {
		row, callErr := readWorker.GetSpace(ctx, f.binding.SpaceID)
		read <- readResult{row, callErr}
	}()
	r22WaitForSpaceLock(t, ctx, st.Pool, "lifecycle_scope_read")
	require.NoError(t, blocker.Commit(ctx))
	require.NoError(t, <-reserved)
	result := <-read
	require.ErrorIs(t, result.err, ErrLifecycleFrozen)
	require.Nil(t, result.row)
	aggregate, err := st.LoadLifecycle(ctx, f.binding.SpaceID)
	require.NoError(t, err)
	require.Equal(t, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_SCHEDULE_PENDING, aggregate.Phase())
}
