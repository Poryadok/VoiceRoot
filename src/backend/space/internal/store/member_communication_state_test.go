package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestMemberCommunicationSnapshotUsesSpaceClockAndAtomicEpoch(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL Space authority")
	}
	ctx, st := authorityRevisionSpace(t)
	profile := uuid.New()
	space, err := st.CreateSpace(ctx, profile, "communication snapshot", "", "private")
	require.NoError(t, err)
	room := uuid.New()
	_, err = st.Pool.Exec(ctx, `INSERT INTO voice_rooms(id,space_id,name) VALUES($1,$2,'snapshot')`, room, space.ID)
	require.NoError(t, err)
	clear, err := st.ResolveMemberCommunicationState(ctx, space.ID, profile)
	require.NoError(t, err)
	require.Nil(t, clear.TimedOutUntil)
	require.Equal(t, profile, clear.ProfileID)
	require.False(t, clear.EvaluatedAt.IsZero())
	before := r22SpaceEpoch(t, ctx, st, space.ID)
	require.NoError(t, st.SetMemberTimeout(ctx, space.ID, profile, profile, 60, nil))
	require.Equal(t, before+1, r22SpaceEpoch(t, ctx, st, space.ID), "one timeout mutation must produce exactly one revision")
	r22SpaceOutboxRow(t, ctx, st, space.ID, before+1)
	timedOut, err := st.ResolveMemberCommunicationState(ctx, space.ID, profile)
	require.NoError(t, err)
	require.NotNil(t, timedOut.TimedOutUntil)
	require.True(t, timedOut.TimedOutUntil.After(timedOut.EvaluatedAt))
	require.LessOrEqual(t, timedOut.TimedOutUntil.Sub(timedOut.EvaluatedAt), 60*time.Second)
	voice, err := st.ResolveVoiceRoomAccess(ctx, space.ID, room, profile)
	require.NoError(t, err)
	require.Equal(t, timedOut.AccessEpoch, voice.AccessEpoch)
	require.Equal(t, timedOut.TimedOutUntil, voice.TimedOutUntil)
	require.Equal(t, profile, voice.ProfileID)
	_, err = st.ResolveMemberCommunicationState(ctx, space.ID, uuid.New())
	require.ErrorIs(t, err, ErrMemberNotFound)
	_, err = st.Pool.Exec(ctx, `UPDATE space_member_timeouts SET timed_out_until=clock_timestamp() WHERE space_id=$1 AND profile_id=$2`, space.ID, profile)
	require.NoError(t, err)
	expired, err := st.ResolveMemberCommunicationState(ctx, space.ID, profile)
	require.NoError(t, err)
	require.Nil(t, expired.TimedOutUntil, "equal or expired database timeout is CLEAR")
	before = r22SpaceEpoch(t, ctx, st, space.ID)
	require.NoError(t, st.RemoveMemberTimeout(ctx, space.ID, profile, profile))
	require.Equal(t, before+1, r22SpaceEpoch(t, ctx, st, space.ID))
	var audits int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE space_id=$1 AND action IN ('member_timed_out','member_timeout_removed')`, space.ID).Scan(&audits))
	require.Equal(t, 2, audits)
}

func TestMemberCommunicationSnapshotSerializesWithTimeoutMutation(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL Space authority")
	}
	ctx, st := authorityRevisionSpace(t)
	profile := uuid.New()
	space, err := st.CreateSpace(ctx, profile, "serialized timeout", "", "private")
	require.NoError(t, err)
	tx, err := st.Pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(context.Background()) }()
	require.NoError(t, lockAndCheckOwnershipScope(ctx, tx, []uuid.UUID{space.ID}))
	scoped := *st
	scoped.tx = tx
	clear, err := scoped.ResolveMemberCommunicationState(ctx, space.ID, profile)
	require.NoError(t, err)
	blocked, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	require.Error(t, st.SetMemberTimeout(blocked, space.ID, profile, profile, 60, nil), "writer cannot cross a held decision snapshot")
	unchanged, err := scoped.ResolveMemberCommunicationState(ctx, space.ID, profile)
	require.NoError(t, err)
	require.Equal(t, clear.AccessEpoch, unchanged.AccessEpoch)
	require.Nil(t, unchanged.TimedOutUntil)
	require.NoError(t, tx.Commit(ctx))
	require.NoError(t, st.SetMemberTimeout(ctx, space.ID, profile, profile, 60, nil))
	current, err := st.ResolveMemberCommunicationState(ctx, space.ID, profile)
	require.NoError(t, err)
	require.NotNil(t, current.TimedOutUntil)
	require.Greater(t, current.AccessEpoch, clear.AccessEpoch)
}
