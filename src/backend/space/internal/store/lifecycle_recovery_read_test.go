package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	spacev1 "voice.app/voice/space/v1"
)

type lifecycleRecoveryReader interface {
	GetLifecycleRecoverySpace(context.Context, uuid.UUID, uuid.UUID) (*spacev1.Space, error)
}

func recoveryReader(t *testing.T, st *SpaceStore) lifecycleRecoveryReader {
	t.Helper()
	reader, ok := any(st).(lifecycleRecoveryReader)
	require.True(t, ok, "SpaceStore must expose a locked recorded-owner recovery projection")
	return reader
}

func TestLifecycleRecoveryRead_Surface(t *testing.T) { recoveryReader(t, &SpaceStore{}) }

func TestLifecycleRecoveryRead_PhaseAndTimestampValidation(t *testing.T) {
	st := lifecycleStoreFixture(t)
	ctx := context.Background()
	scheduled := time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)
	deadline := scheduled.Add(7 * 24 * time.Hour)
	for _, tc := range []struct {
		phase      string
		start, end *time.Time
		want       error
	}{
		{"SCHEDULE_PENDING", nil, nil, nil}, {"FREEZE_PENDING", nil, nil, nil},
		{"SCHEDULED", &scheduled, &deadline, nil}, {"RESTORE_DECIDED", &scheduled, &deadline, nil},
		{"PURGE_DECIDED", &scheduled, &deadline, nil}, {"PURGING", &scheduled, &deadline, nil},
		{"PURGED", &scheduled, &deadline, pgx.ErrNoRows}, {"LIVE", nil, nil, ErrLifecycleStateTransition},
		{"SCHEDULE_PENDING", &scheduled, &deadline, ErrOwnershipScopeUnavailable},
		{"FREEZE_PENDING", &scheduled, &deadline, ErrOwnershipScopeUnavailable},
		{"SCHEDULED", nil, nil, ErrOwnershipScopeUnavailable},
		{"SCHEDULED", &deadline, &scheduled, ErrOwnershipScopeUnavailable},
		{"RESTORE_DECIDED", &scheduled, &scheduled, ErrOwnershipScopeUnavailable},
	} {
		t.Run(tc.phase, func(t *testing.T) {
			f := newR20ScopeFixture(t, st, "")
			_, err := st.Pool.Exec(ctx, `INSERT INTO space_lifecycle_aggregates(space_id,deletion_operation_id,phase,generation,scheduled_at,purge_after) VALUES($1,$2,$3,1,$4,$5)`, f.binding.SpaceID, uuid.New(), tc.phase, tc.start, tc.end)
			require.NoError(t, err)
			result, err := recoveryReader(t, st).GetLifecycleRecoverySpace(ctx, f.binding.SpaceID, f.owner)
			if tc.want != nil {
				require.ErrorIs(t, err, tc.want)
				require.Nil(t, result)
				return
			}
			require.NoError(t, err)
			if tc.start == nil {
				require.Nil(t, result.DeletionScheduledAt)
				require.Nil(t, result.PurgeAfter)
			} else {
				require.Equal(t, tc.start.UTC(), result.DeletionScheduledAt.AsTime())
				require.Equal(t, tc.end.UTC(), result.PurgeAfter.AsTime())
			}
		})
	}
}

func TestLifecycleRecoveryRead_MinimalProjectionAndNoMutation(t *testing.T) {
	f := newLifecycleRestoreFixture(t)
	ctx := context.Background()
	aggregate, err := f.store.LoadLifecycle(ctx, f.space)
	require.NoError(t, err)
	var name string
	require.NoError(t, f.st.Pool.QueryRow(ctx, `SELECT name FROM spaces WHERE id=$1`, f.space).Scan(&name))
	snapshot := aggregate.Snapshot()
	expected := &spacev1.Space{Id: f.space.String(), Name: name, DeletionScheduledAt: timestamppb.New(snapshot.ScheduledAt), PurgeAfter: timestamppb.New(snapshot.PurgeAfter)}
	before := lifecycleScopeSnapshot(t, f.st, f.space)
	got, err := recoveryReader(t, f.st).GetLifecycleRecoverySpace(ctx, f.space, f.actor)
	require.NoError(t, err)
	require.True(t, proto.Equal(expected, got), "only four documented fields may be disclosed: %v", got)
	require.Equal(t, before, lifecycleScopeSnapshot(t, f.st, f.space))
	_, err = f.st.GetSpace(ctx, f.space)
	require.ErrorIs(t, err, ErrLifecycleFrozen)
	f.admit(t, 0)
	got, err = recoveryReader(t, f.st).GetLifecycleRecoverySpace(ctx, f.space, f.actor)
	require.NoError(t, err)
	require.True(t, proto.Equal(expected, got), "partial restore stays minimal")
}

func TestLifecycleRecoveryRead_HidesNonOwnerMissingAndPurged(t *testing.T) {
	f := newLifecycleRestoreFixture(t)
	ctx := context.Background()
	for _, actor := range []uuid.UUID{uuid.New(), uuid.Nil} {
		got, err := recoveryReader(t, f.st).GetLifecycleRecoverySpace(ctx, f.space, actor)
		require.Error(t, err)
		require.Nil(t, got)
		if actor != uuid.Nil {
			require.ErrorIs(t, err, pgx.ErrNoRows)
		}
	}
	got, err := recoveryReader(t, f.st).GetLifecycleRecoverySpace(ctx, uuid.New(), f.actor)
	require.ErrorIs(t, err, pgx.ErrNoRows)
	require.Nil(t, got)
	_, err = f.st.Pool.Exec(ctx, `UPDATE space_lifecycle_aggregates SET phase='PURGED' WHERE space_id=$1`, f.space)
	require.NoError(t, err)
	got, err = recoveryReader(t, f.st).GetLifecycleRecoverySpace(ctx, f.space, f.actor)
	require.ErrorIs(t, err, pgx.ErrNoRows)
	require.Nil(t, got, "purge tombstone must not grant public read authority")
}

func TestLifecycleRecoveryRead_PendingHasNoInventedDeadline(t *testing.T) {
	st := lifecycleStoreFixture(t)
	_, _, _, actor, spaceID, _, _, _ := reserveLifecycleOperationFixture(t, st)
	got, err := recoveryReader(t, st).GetLifecycleRecoverySpace(context.Background(), spaceID, actor)
	require.NoError(t, err)
	require.Nil(t, got.DeletionScheduledAt)
	require.Nil(t, got.PurgeAfter)
	require.NotEmpty(t, got.Id)
	require.NotEmpty(t, got.Name)
}

func TestLifecycleRecoveryRead_AuthorityUncertaintyAndLiveRefused(t *testing.T) {
	st := lifecycleStoreFixture(t)
	f := newR20ScopeFixture(t, st, "")
	reader := recoveryReader(t, st)
	got, err := reader.GetLifecycleRecoverySpace(context.Background(), f.binding.SpaceID, f.owner)
	require.ErrorIs(t, err, ErrLifecycleStateTransition)
	require.Nil(t, got)
	_, err = st.Pool.Exec(context.Background(), `ALTER TABLE space_lifecycle_aggregates RENAME TO unavailable_recovery_authority`)
	require.NoError(t, err)
	got, err = reader.GetLifecycleRecoverySpace(context.Background(), f.binding.SpaceID, f.owner)
	require.ErrorIs(t, err, ErrOwnershipScopeUnavailable)
	require.Nil(t, got)
}

func TestLifecycleRecoveryRead_WaitsForCommittedOwnershipBeforeDisclosure(t *testing.T) {
	f := newLifecycleRestoreFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	tx, err := f.st.Pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(context.Background()) }()
	require.NoError(t, lockLifecycleSpace(ctx, tx, f.space))
	_, err = tx.Exec(ctx, `UPDATE spaces SET owner_profile_id=$2 WHERE id=$1`, f.space, uuid.New())
	require.NoError(t, err)
	worker := &SpaceStore{Pool: r22SecondSpacePool(t, ctx, f.st.Pool, "recovery_read_lock")}
	reader := recoveryReader(t, worker)
	type result struct {
		row *spacev1.Space
		err error
	}
	done := make(chan result, 1)
	go func() {
		row, callErr := reader.GetLifecycleRecoverySpace(ctx, f.space, f.actor)
		done <- result{row, callErr}
	}()
	r22WaitForSpaceLock(t, ctx, f.st.Pool, "recovery_read_lock")
	require.NoError(t, tx.Commit(ctx))
	response := <-done
	require.ErrorIs(t, response.err, pgx.ErrNoRows)
	require.Nil(t, response.row, "must not disclose using pre-lock owner")
}
