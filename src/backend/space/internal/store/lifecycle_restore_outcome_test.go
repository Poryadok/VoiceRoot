package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	commonv1 "voice.app/voice/common/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/spacecore"
)

type lifecycleRestoreOutcomeStore interface {
	ReserveLifecycleRestore(context.Context, uuid.UUID, uuid.UUID, int64, *spacev1.RestoreSpaceRequest) (*spacecore.LifecycleAggregate, error)
	ReplayLifecycleRestoreOutcome(context.Context, uuid.UUID, uuid.UUID, int64, *spacev1.RestoreSpaceRequest) (*spacev1.RestoreSpaceResponse, error)
}

func requireLifecycleRestoreOutcome(t *testing.T, st *SpaceStore) lifecycleRestoreOutcomeStore {
	t.Helper()
	api, ok := any(st).(lifecycleRestoreOutcomeStore)
	require.True(t, ok, "SpaceStore must admit RESTORE with an immutable operation and replay its saved typed response")
	return api
}

func TestLifecycleRestoreOutcome_TypedSurfaceRequired(t *testing.T) {
	requireLifecycleRestoreOutcome(t, &SpaceStore{})
}

type lifecycleRestoreFixture struct {
	lifecycleOutcomeFixture
	restore *spacev1.RestoreSpaceRequest
}

func newLifecycleRestoreFixture(t *testing.T) lifecycleRestoreFixture {
	t.Helper()
	f := newLifecycleOutcomeFixture(t, len(lifecycleTestParticipants))
	_, _, err := f.store.CompleteLifecycleSchedule(context.Background(), f.space)
	require.NoError(t, err)
	return lifecycleRestoreFixture{f, &spacev1.RestoreSpaceRequest{SpaceId: f.space.String(), OperationId: uuid.NewString()}}
}

func (f lifecycleRestoreFixture) admit(t *testing.T, count int) *spacecore.LifecycleAggregate {
	t.Helper()
	aggregate, err := requireLifecycleRestoreOutcome(t, f.st).ReserveLifecycleRestore(context.Background(), f.account, f.actor, 7, f.restore)
	require.NoError(t, err)
	require.Equal(t, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_RESTORE_DECIDED, aggregate.Phase())
	require.Equal(t, uint64(2), aggregate.Generation())
	for _, participant := range lifecycleTestParticipants[:count] {
		_, err = requireLifecycleTransitions(t, f.st).RecordLifecycleFenceReceipt(context.Background(), lifecycleFenceReceiptFixture(t,
			f.space, uuid.MustParse(f.request.OperationId), participant, 2, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE, lifecycleManifestFixture()))
		require.NoError(t, err, "receipts keep the deletion operation ID, not the separately admitted restore ID")
	}
	return aggregate
}

func (f lifecycleRestoreFixture) evidence(t *testing.T) lifecycleOutcomeEvidence {
	t.Helper()
	copy := f.lifecycleOutcomeFixture
	copy.request = &spacev1.DeleteSpaceRequest{OperationId: f.restore.OperationId}
	return readLifecycleOutcomeEvidence(t, copy)
}

func (f lifecycleRestoreFixture) replay(t *testing.T, st *SpaceStore) *spacev1.RestoreSpaceResponse {
	t.Helper()
	response, err := requireLifecycleRestoreOutcome(t, st).ReplayLifecycleRestoreOutcome(context.Background(), f.account, f.actor, 7, f.restore)
	require.NoError(t, err)
	return response
}

func TestLifecycleRestoreOutcome_OwnerAdmissionAndImmutablePendingReplay(t *testing.T) {
	f := newLifecycleRestoreFixture(t)
	api := requireLifecycleRestoreOutcome(t, f.st)
	ctx := context.Background()
	before, err := f.store.LoadLifecycle(ctx, f.space)
	require.NoError(t, err)
	_, err = api.ReserveLifecycleRestore(ctx, f.account, uuid.New(), 7, f.restore)
	require.Error(t, err, "only the current owner may admit a new restore")
	after, err := f.store.LoadLifecycle(ctx, f.space)
	require.NoError(t, err)
	require.Equal(t, before.Snapshot(), after.Snapshot())
	admitted := f.admit(t, 0)
	evidence := f.evidence(t)
	require.Equal(t, "RESTORE_PENDING", evidence.state)
	require.Nil(t, evidence.bytes)
	require.Nil(t, evidence.completedAt)
	require.Nil(t, f.replay(t, f.st), "admission must not claim successful restore")
	_, err = f.st.Pool.Exec(ctx, `UPDATE spaces SET name='renamed after admission',owner_profile_id=$2 WHERE id=$1`, f.space, uuid.New())
	require.NoError(t, err)
	replay, err := api.ReserveLifecycleRestore(ctx, f.account, f.actor, 7, f.restore)
	require.NoError(t, err, "exact pending replay precedes mutable owner and name checks")
	require.Equal(t, admitted.Snapshot(), replay.Snapshot())
	changed := proto.Clone(f.restore).(*spacev1.RestoreSpaceRequest)
	changed.OperationId = uuid.NewString()
	_, err = api.ReserveLifecycleRestore(ctx, f.account, f.actor, 7, changed)
	require.Error(t, err, "a second operation cannot replace the existing restore decision")
	require.Equal(t, evidence, f.evidence(t))
}

func TestLifecycleRestoreOutcome_RejectsMalformedAndChangedBindings(t *testing.T) {
	f := newLifecycleRestoreFixture(t)
	f.admit(t, 0)
	api := requireLifecycleRestoreOutcome(t, f.st)
	before, err := f.store.LoadLifecycle(context.Background(), f.space)
	require.NoError(t, err)
	for _, tc := range []struct {
		name           string
		account, actor uuid.UUID
		epoch          int64
		mutate         func(*spacev1.RestoreSpaceRequest)
	}{
		{name: "account mismatch", account: uuid.New(), actor: f.actor, epoch: 7},
		{name: "actor mismatch", account: f.account, actor: uuid.New(), epoch: 7},
		{name: "epoch mismatch", account: f.account, actor: f.actor, epoch: 8},
		{name: "nil account", actor: f.actor, epoch: 7},
		{name: "nil actor", account: f.account, epoch: 7},
		{name: "zero epoch", account: f.account, actor: f.actor},
		{name: "negative epoch", account: f.account, actor: f.actor, epoch: -1},
		{name: "changed space", account: f.account, actor: f.actor, epoch: 7, mutate: func(r *spacev1.RestoreSpaceRequest) { r.SpaceId = uuid.NewString() }},
		{name: "malformed space", account: f.account, actor: f.actor, epoch: 7, mutate: func(r *spacev1.RestoreSpaceRequest) { r.SpaceId = "not-a-uuid" }},
		{name: "nil space", account: f.account, actor: f.actor, epoch: 7, mutate: func(r *spacev1.RestoreSpaceRequest) { r.SpaceId = uuid.Nil.String() }},
		{name: "malformed operation", account: f.account, actor: f.actor, epoch: 7, mutate: func(r *spacev1.RestoreSpaceRequest) { r.OperationId = "invalid" }},
		{name: "deletion method collision", account: f.account, actor: f.actor, epoch: 7, mutate: func(r *spacev1.RestoreSpaceRequest) { r.OperationId = f.request.OperationId }},
		{name: "unknown fields", account: f.account, actor: f.actor, epoch: 7, mutate: func(r *spacev1.RestoreSpaceRequest) {
			r.ProtoReflect().SetUnknown(protowire.AppendVarint(protowire.AppendTag(nil, 100, protowire.VarintType), 1))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := proto.Clone(f.restore).(*spacev1.RestoreSpaceRequest)
			if tc.mutate != nil {
				tc.mutate(request)
			}
			_, err := api.ReserveLifecycleRestore(context.Background(), tc.account, tc.actor, tc.epoch, request)
			require.Error(t, err)
			response, err := api.ReplayLifecycleRestoreOutcome(context.Background(), tc.account, tc.actor, tc.epoch, request)
			require.Error(t, err)
			require.Nil(t, response)
		})
	}
	_, err = api.ReserveLifecycleRestore(context.Background(), f.account, f.actor, 7, nil)
	require.Error(t, err)
	after, err := f.store.LoadLifecycle(context.Background(), f.space)
	require.NoError(t, err)
	require.Equal(t, before.Snapshot(), after.Snapshot())
}

func TestLifecycleRestoreOutcome_BarrierPersistsExactDatabaseProjectionOnce(t *testing.T) {
	f := newLifecycleRestoreFixture(t)
	f.admit(t, len(lifecycleTestParticipants)-1)
	ctx := context.Background()
	transitions := requireLifecycleTransitions(t, f.st)
	_, _, err := transitions.CompleteLifecycleRestore(ctx, f.space)
	require.Error(t, err)
	require.Nil(t, f.replay(t, f.st))
	require.Equal(t, "RESTORE_PENDING", f.evidence(t).state)
	require.Equal(t, []string{"space.deletion_scheduled"}, lifecycleEventTypes(readyLifecycleEvents(t, f.store)))
	_, err = f.st.Pool.Exec(ctx, `UPDATE spaces SET description='durable restored description',icon_url='https://example.test/icon',banner_url='https://example.test/banner',allow_guests=true WHERE id=$1`, f.space)
	require.NoError(t, err)
	_, err = transitions.RecordLifecycleFenceReceipt(ctx, lifecycleFenceReceiptFixture(t, f.space, uuid.MustParse(f.request.OperationId),
		lifecycleTestParticipants[len(lifecycleTestParticipants)-1], 2, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE, lifecycleManifestFixture()))
	require.NoError(t, err)
	completed, event, err := transitions.CompleteLifecycleRestore(ctx, f.space)
	require.NoError(t, err)
	require.Equal(t, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_LIVE, completed.Phase())
	response := f.replay(t, f.st)
	require.NotNil(t, response)
	require.NotNil(t, response.Space)
	row, err := f.st.GetSpace(ctx, f.space)
	require.NoError(t, err)
	require.Equal(t, row.ID.String(), response.Space.Id)
	require.Equal(t, row.Name, response.Space.Name)
	require.Equal(t, row.Description, response.Space.Description)
	require.Equal(t, row.OwnerProfileID.String(), response.Space.OwnerProfileId)
	require.Equal(t, row.Visibility, response.Space.Visibility)
	require.Equal(t, row.MemberCount, response.Space.MemberCount)
	require.Equal(t, *row.IconURL, response.Space.GetIconUrl())
	require.Equal(t, *row.BannerURL, response.Space.GetBannerUrl())
	require.True(t, response.Space.AllowGuests)
	require.True(t, row.CreatedAt.Equal(response.Space.CreatedAt.AsTime()))
	require.True(t, row.UpdatedAt.Equal(response.Space.UpdatedAt.AsTime()))
	require.Nil(t, response.Space.DeletionScheduledAt)
	require.Nil(t, response.Space.PurgeAfter)
	evidence := f.evidence(t)
	require.Equal(t, "COMPLETED", evidence.state)
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(response)
	require.NoError(t, err)
	require.Equal(t, raw, evidence.bytes)
	hash := lifecycleDomainHash("voice.space.v1.RestoreSpaceResponse", raw)
	require.Equal(t, hash[:], evidence.hash)
	require.NotNil(t, evidence.completedAt)
	require.True(t, evidence.completedAt.Equal(event.OccurredAt))
	require.Equal(t, []string{"space.deletion_scheduled", "space.restored"}, lifecycleEventTypes(readyLifecycleEvents(t, f.store)))
	_, repeated, err := transitions.CompleteLifecycleRestore(ctx, f.space)
	require.NoError(t, err)
	require.Equal(t, event, repeated)
	require.Equal(t, evidence, f.evidence(t))
	requireLifecycleOutcomeReplay(t, f.lifecycleOutcomeFixture, f.st)
}

func TestLifecycleRestoreOutcome_SavedResponseSurvivesRenameOwnerChangeRemovalAndRestart(t *testing.T) {
	f := newLifecycleRestoreFixture(t)
	f.admit(t, len(lifecycleTestParticipants))
	ctx := context.Background()
	_, _, err := requireLifecycleTransitions(t, f.st).CompleteLifecycleRestore(ctx, f.space)
	require.NoError(t, err)
	saved := f.replay(t, f.st)
	require.NotNil(t, saved)
	_, err = f.st.Pool.Exec(ctx, `UPDATE spaces SET name='later name',owner_profile_id=$2 WHERE id=$1`, f.space, uuid.New())
	require.NoError(t, err)
	require.True(t, proto.Equal(saved, f.replay(t, f.st)), "replay must not reconstruct the response from mutable rows")
	_, err = f.st.Pool.Exec(ctx, `DELETE FROM space_lifecycle_aggregates WHERE space_id=$1`, f.space)
	require.NoError(t, err)
	_, err = f.st.Pool.Exec(ctx, `DELETE FROM spaces WHERE id=$1`, f.space)
	require.NoError(t, err)
	restarted := &SpaceStore{Pool: r22SecondSpacePool(t, ctx, f.st.Pool, "restore_outcome_restart")}
	require.True(t, proto.Equal(saved, f.replay(t, restarted)))
}

func TestLifecycleRestoreOutcome_FailureRollsBackLiveOutcomeAndReadyEvent(t *testing.T) {
	f := newLifecycleRestoreFixture(t)
	f.admit(t, len(lifecycleTestParticipants))
	ctx := context.Background()
	before, err := f.store.LoadLifecycle(ctx, f.space)
	require.NoError(t, err)
	_, err = f.st.Pool.Exec(ctx, `CREATE FUNCTION reject_restore_outcome() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN IF NEW.method='RESTORE' AND NEW.state='COMPLETED' THEN RAISE EXCEPTION 'injected restore failure'; END IF; RETURN NEW; END $$;
		CREATE TRIGGER reject_restore_outcome BEFORE UPDATE ON space_lifecycle_operations FOR EACH ROW EXECUTE FUNCTION reject_restore_outcome()`)
	require.NoError(t, err)
	_, _, err = requireLifecycleTransitions(t, f.st).CompleteLifecycleRestore(ctx, f.space)
	require.ErrorContains(t, err, "injected restore failure")
	after, err := f.store.LoadLifecycle(ctx, f.space)
	require.NoError(t, err)
	require.Equal(t, before.Snapshot(), after.Snapshot())
	require.Nil(t, f.replay(t, f.st))
	require.Equal(t, "RESTORE_PENDING", f.evidence(t).state)
	require.Equal(t, []string{"space.deletion_scheduled"}, lifecycleEventTypes(readyLifecycleEvents(t, f.store)))
	_, err = f.st.Pool.Exec(ctx, `DROP TRIGGER reject_restore_outcome ON space_lifecycle_operations`)
	require.NoError(t, err)
	_, _, err = requireLifecycleTransitions(t, f.st).CompleteLifecycleRestore(ctx, f.space)
	require.NoError(t, err)
	require.NotNil(t, f.replay(t, f.st))
}

func TestLifecycleRestoreOutcome_ExpiryUsesCompletionAndNeverRenews(t *testing.T) {
	f := newLifecycleRestoreFixture(t)
	f.admit(t, len(lifecycleTestParticipants))
	ctx := context.Background()
	_, err := f.st.Pool.Exec(ctx, `UPDATE space_lifecycle_operations SET created_at=clock_timestamp()-interval '60 days' WHERE operation_id=$1`, f.restore.OperationId)
	require.NoError(t, err)
	require.Nil(t, f.replay(t, f.st), "nonterminal operations do not expire after 30 days")
	_, _, err = requireLifecycleTransitions(t, f.st).CompleteLifecycleRestore(ctx, f.space)
	require.NoError(t, err)
	_, err = f.st.Pool.Exec(ctx, `UPDATE space_lifecycle_operations SET completed_at=clock_timestamp()-interval '29 days' WHERE operation_id=$1`, f.restore.OperationId)
	require.NoError(t, err)
	before := f.evidence(t)
	require.NotNil(t, f.replay(t, f.st))
	require.Equal(t, before, f.evidence(t))
	_, err = f.st.Pool.Exec(ctx, `UPDATE space_lifecycle_operations SET completed_at=clock_timestamp()-interval '30 days' WHERE operation_id=$1`, f.restore.OperationId)
	require.NoError(t, err)
	expired := f.evidence(t)
	api := requireLifecycleRestoreOutcome(t, f.st)
	response, err := api.ReplayLifecycleRestoreOutcome(ctx, f.account, f.actor, 7, f.restore)
	require.ErrorIs(t, err, ErrLifecycleOutcomeExpired)
	require.Nil(t, response)
	_, err = api.ReserveLifecycleRestore(ctx, f.account, f.actor, 7, f.restore)
	require.ErrorIs(t, err, ErrLifecycleOutcomeExpired)
	require.Equal(t, expired, f.evidence(t))
}

func TestLifecycleRestoreOutcome_ExpiredAdmissionCommitsPurgeAfterSharedLock(t *testing.T) {
	f := newLifecycleRestoreFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	blocker, err := f.st.Pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = blocker.Rollback(context.Background()) }()
	_, err = blocker.Exec(ctx, `SELECT pg_advisory_xact_lock($1::bigint)`, spaceMutationAdvisoryKey(f.space))
	require.NoError(t, err)
	worker := requireLifecycleRestoreOutcome(t, &SpaceStore{Pool: r22SecondSpacePool(t, ctx, f.st.Pool, "restore_admission_clock")})
	result := make(chan error, 1)
	go func() {
		_, callErr := worker.ReserveLifecycleRestore(ctx, f.account, f.actor, 7, f.restore)
		result <- callErr
	}()
	r22WaitForSpaceLock(t, ctx, f.st.Pool, "restore_admission_clock")
	// Expire the window while admission is blocked: a pre-lock read of phase or
	// deadline would incorrectly authorize restore after this transaction commits.
	_, err = blocker.Exec(ctx, `UPDATE space_lifecycle_aggregates SET scheduled_at=clock_timestamp()-interval '7 days',purge_after=clock_timestamp() WHERE space_id=$1`, f.space)
	require.NoError(t, err)
	require.NoError(t, blocker.Commit(ctx))
	require.ErrorIs(t, <-result, ErrLifecycleStateTransition)
	aggregate, err := f.store.LoadLifecycle(ctx, f.space)
	require.NoError(t, err)
	require.Equal(t, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGE_DECIDED, aggregate.Phase())
	require.Equal(t, uint64(2), aggregate.Generation())
	require.Nil(t, f.replay(t, f.st))
	_, _, err = requireLifecycleTransitions(t, f.st).CompleteLifecycleRestore(ctx, f.space)
	require.Error(t, err)
}

func TestLifecycleRestoreOutcome_GenericSnapshotCannotManufactureSuccess(t *testing.T) {
	f := newLifecycleRestoreFixture(t)
	f.admit(t, 0)
	ctx := context.Background()
	aggregate, err := f.store.LoadLifecycle(ctx, f.space)
	require.NoError(t, err)
	snapshot := aggregate.Snapshot()
	snapshot.Phase = spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_LIVE
	snapshot.FenceReceipts = nil
	snapshot.RestoreEvent = nil
	fabricated, err := spacecore.RestoreLifecycleAggregate(snapshot)
	require.NoError(t, err)
	_ = f.store.PersistLifecycle(ctx, fabricated)
	require.Nil(t, f.replay(t, f.st))
	_, _, err = requireLifecycleTransitions(t, f.st).CompleteLifecycleRestore(ctx, f.space)
	require.Error(t, err, "generic LIVE must not replace the ten accepted LIVE receipts")
	require.Equal(t, "RESTORE_PENDING", f.evidence(t).state)
	require.Equal(t, []string{"space.deletion_scheduled"}, lifecycleEventTypes(readyLifecycleEvents(t, f.store)))
}
