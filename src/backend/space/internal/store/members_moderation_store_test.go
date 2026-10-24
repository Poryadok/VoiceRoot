package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestSpaceModeration_RawBanMemberCannotBypassQualifiedIntent(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}

	owner := uuid.New()
	target := uuid.New()
	space, err := st.CreateSpace(ctx, owner, "Ban store", "", "private")
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `INSERT INTO space_members (space_id, profile_id) VALUES ($1, $2)`, space.ID, target)
	require.NoError(t, err)

	accountID := uuid.New()
	mod := uuid.New()
	require.ErrorIs(t, st.BanMember(ctx, space.ID, accountID, mod, strPtrStore("spam"), &target), ErrAccountBanUnavailable)

	ok, err := st.IsAccountBanned(ctx, space.ID, accountID)
	require.NoError(t, err)
	require.False(t, ok)

	member, err := st.IsSpaceMember(ctx, space.ID, target)
	require.NoError(t, err)
	require.True(t, member)
	var audits int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE space_id=$1 AND action='member_banned'`, space.ID).Scan(&audits))
	require.Zero(t, audits, "raw UUIDs cannot supply the authenticated intent and complete owner capture")
}

func TestSpaceModeration_RawBanAndUnbanRejectBeforeUsingStore(t *testing.T) {
	ctx := context.Background()
	for _, st := range []*SpaceStore{nil, {}} {
		require.ErrorIs(t, st.BanMember(ctx, uuid.New(), uuid.New(), uuid.New(), nil, nil), ErrAccountBanUnavailable)
		require.ErrorIs(t, st.UnbanMember(ctx, uuid.New(), uuid.New(), uuid.New()), ErrAccountBanUnavailable)
	}
}

func TestSpaceModeration_SetMemberTimeout_ActiveUntil(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}

	owner := uuid.New()
	target := uuid.New()
	space, err := st.CreateSpace(ctx, owner, "Timeout", "", "private")
	require.NoError(t, err)

	require.NoError(t, st.SetMemberTimeout(ctx, space.ID, target, owner, 120, nil))
	timedOut, err := st.IsProfileTimedOut(ctx, space.ID, target)
	require.NoError(t, err)
	require.True(t, timedOut)

	require.NoError(t, st.RemoveMemberTimeout(ctx, space.ID, target, owner))
	timedOut, err = st.IsProfileTimedOut(ctx, space.ID, target)
	require.NoError(t, err)
	require.False(t, timedOut)
	_ = time.Now()
}

func strPtrStore(s string) *string { return &s }
