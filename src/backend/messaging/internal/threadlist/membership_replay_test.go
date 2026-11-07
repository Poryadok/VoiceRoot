package threadlist

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestPostgresMembershipBoundedReplayMakesProgress(t *testing.T) {
	pool := membershipDB(t)
	inbox := MembershipInbox{Pool: pool}
	ctx := context.Background()
	first := membershipFixture()
	require.NoError(t, inbox.Track(ctx, first.ChatID, first.Incarnation))
	for offset := int64(2); offset <= 258; offset++ {
		e := membershipFixture()
		e.ChatID, e.Incarnation, e.Offset = first.ChatID, first.Incarnation, offset
		e.Sources[0] = first.Sources[0]
		require.NoError(t, inbox.Apply(ctx, e))
	}
	done, err := inbox.Replay(ctx, first.ChatID, first.Incarnation)
	require.NoError(t, err)
	require.False(t, done, "a gap cannot be declared complete")
	require.NoError(t, inbox.Apply(ctx, first))
	_, err = inbox.Pair(ctx, first.ChatID, first.Incarnation, first.ProfileID, 258)
	require.ErrorIs(t, err, ErrMembershipUnavailable)
	done, err = inbox.Replay(ctx, first.ChatID, first.Incarnation)
	require.NoError(t, err)
	require.True(t, done, "bounded delivery may leave work; durable replay must finish it without another event")
	pair, err := inbox.Pair(ctx, first.ChatID, first.Incarnation, first.ProfileID, 258)
	require.NoError(t, err)
	require.Equal(t, MemberBuilding, pair.Status)
	done, err = inbox.Replay(ctx, first.ChatID, first.Incarnation)
	require.NoError(t, err)
	require.True(t, done)
}

func TestPostgresMembershipBarrierAndConflictAfterPartialDrain(t *testing.T) {
	t.Run("barrier", func(t *testing.T) {
		pool := membershipDB(t)
		inbox := MembershipInbox{Pool: pool}
		ctx := context.Background()
		e := membershipFixture()
		require.NoError(t, inbox.Track(ctx, e.ChatID, e.Incarnation))
		require.NoError(t, inbox.Apply(ctx, e))
		barrier := e
		barrier.EventID, barrier.Offset, barrier.Revision, barrier.Kind = uuid.New(), 2, 2, MemberBarrier
		require.NoError(t, inbox.Apply(ctx, barrier))
		pair, err := inbox.Pair(ctx, e.ChatID, e.Incarnation, e.ProfileID, 2)
		require.NoError(t, err)
		require.Equal(t, MemberRevoked, pair.Status)
		barrier.EventID, barrier.Offset, barrier.Revision, barrier.Kind = uuid.New(), 3, 3, MemberJoined
		require.ErrorIs(t, inbox.Apply(ctx, barrier), ErrMembershipConflict)
	})
	t.Run("conflict after committed prefix", func(t *testing.T) {
		pool := membershipDB(t)
		inbox := MembershipInbox{Pool: pool}
		ctx := context.Background()
		one := membershipFixture()
		two := membershipFixture()
		two.ChatID, two.Incarnation, two.Offset = one.ChatID, one.Incarnation, 2
		two.Sources[0] = one.Sources[0]
		three := one
		three.EventID, three.Offset, three.Revision = uuid.New(), 3, 2 // illegal second join of an active pair
		require.NoError(t, inbox.Track(ctx, one.ChatID, one.Incarnation))
		require.NoError(t, inbox.Apply(ctx, three))
		require.NoError(t, inbox.Apply(ctx, one))
		require.ErrorIs(t, inbox.Apply(ctx, two), ErrMembershipConflict)
		var prefix int64
		var quarantined bool
		require.NoError(t, pool.QueryRow(ctx, `SELECT applied_q,quarantined FROM thread_list_membership_heads WHERE chat_id=$1`, one.ChatID).Scan(&prefix, &quarantined))
		require.EqualValues(t, 2, prefix)
		require.True(t, quarantined)
		_, err := inbox.Pair(ctx, one.ChatID, one.Incarnation, two.ProfileID, 2)
		require.ErrorIs(t, err, ErrMembershipUnavailable)
	})
}

func TestPostgresMembershipMigrationRoundTrip(t *testing.T) {
	pool := membershipDB(t)
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	ctx := context.Background()
	for _, migration := range []string{"000040_thread_membership_baselines.down.sql", "000039_thread_list_membership.down.sql", "000039_thread_list_membership.up.sql", "000040_thread_membership_baselines.up.sql"} {
		raw, err := os.ReadFile(filepath.Join(filepath.Dir(file), "../../../migrations/messaging_db", migration))
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(raw))
		require.NoError(t, err)
	}
	e := membershipFixture()
	inbox := MembershipInbox{Pool: pool}
	require.NoError(t, inbox.Track(ctx, e.ChatID, e.Incarnation))
	require.NoError(t, inbox.Apply(ctx, e))
}
