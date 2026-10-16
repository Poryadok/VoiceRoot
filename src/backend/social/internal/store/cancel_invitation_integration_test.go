package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestCancelInvitationPendingOwnerAndFreshResend(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startStorePostgres(t, ctx)
	applyStoreMigrations(t, ctx, pool)
	s := &FriendshipStore{Pool: pool}
	sender, target := uuid.New(), uuid.New()
	sa, ta := uuid.New(), uuid.New()
	require.NoError(t, s.SendInvitationChecked(ctx, sender, target, sa, ta))
	var oldRequest, oldEvent uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT friendship_id, event_id FROM friend_request_outbox`).Scan(&oldRequest, &oldEvent))
	require.ErrorIs(t, s.CancelInvitation(ctx, target, sender), ErrFriendshipNotFound)
	require.ErrorIs(t, s.CancelInvitation(ctx, uuid.New(), target), ErrFriendshipNotFound)
	require.NoError(t, s.CancelInvitation(ctx, sender, target))
	require.ErrorIs(t, s.CancelInvitation(ctx, sender, target), ErrFriendshipNotFound)
	incoming, _, err := s.ListFriendRequests(ctx, target)
	require.NoError(t, err)
	require.Empty(t, incoming)
	_, outgoing, err := s.ListFriendRequests(ctx, sender)
	require.NoError(t, err)
	require.Empty(t, outgoing)
	n, err := s.DispatchFriendRequestOutbox(ctx, 1, func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID) error {
		t.Error("cancelled request must not dispatch")
		return nil
	})
	require.NoError(t, err)
	require.Zero(t, n)
	require.NoError(t, s.SendInvitationChecked(ctx, sender, target, sa, ta))
	var newRequest, newEvent uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT friendship_id, event_id FROM friend_request_outbox`).Scan(&newRequest, &newEvent))
	require.NotEqual(t, oldRequest, newRequest)
	require.NotEqual(t, oldEvent, newEvent)
}

func TestCancelInvitationPreservesDecidedRows(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startStorePostgres(t, ctx)
	applyStoreMigrations(t, ctx, pool)
	s := &FriendshipStore{Pool: pool}
	for _, decision := range []string{"accepted", "declined"} {
		t.Run(decision, func(t *testing.T) {
			sender, target := uuid.New(), uuid.New()
			require.NoError(t, s.SendInvitation(ctx, sender, target))
			if decision == "accepted" {
				require.NoError(t, s.AcceptInvitation(ctx, target, sender))
			} else {
				require.NoError(t, s.DeclineInvitation(ctx, target, sender))
			}
			require.ErrorIs(t, s.CancelInvitation(ctx, sender, target), ErrFriendshipNotFound)
			var actual string
			require.NoError(t, pool.QueryRow(ctx, `SELECT status FROM friendships WHERE requester_profile_id=$1 AND target_profile_id=$2`, sender, target).Scan(&actual))
			require.Equal(t, decision, actual)
		})
	}
}

func TestCancelInvitationConcurrentAcceptHasSingleWinner(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	pool := startStorePostgres(t, context.Background())
	applyStoreMigrations(t, context.Background(), pool)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	s := &FriendshipStore{Pool: pool}
	for _, first := range []string{"accept", "cancel"} {
		sender, target := uuid.New(), uuid.New()
		require.NoError(t, s.SendInvitation(ctx, sender, target))
		blocker, err := pool.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = blocker.Rollback(context.Background()) }()
		_, err = blocker.Exec(ctx, `SELECT id FROM friendships WHERE requester_profile_id=$1 AND target_profile_id=$2 FOR UPDATE`, sender, target)
		require.NoError(t, err)
		accepted, cancelled := make(chan error, 1), make(chan error, 1)
		accept := func() { accepted <- s.AcceptInvitation(ctx, target, sender) }
		cancelRequest := func() { cancelled <- s.CancelInvitation(ctx, sender, target) }
		if first == "accept" {
			go accept()
		} else {
			go cancelRequest()
		}
		waiters := func(want int) {
			t.Helper()
			require.Eventually(t, func() bool {
				var n int
				err := pool.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND (query LIKE '%UPDATE friendships%' OR query LIKE '%DELETE FROM friendships%')`).Scan(&n)
				return err == nil && n == want
			}, 5*time.Second, 10*time.Millisecond)
		}
		waiters(1)
		if first == "accept" {
			go cancelRequest()
		} else {
			go accept()
		}
		waiters(2)
		require.NoError(t, blocker.Commit(ctx))
		var a, c error
		select {
		case a = <-accepted:
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		select {
		case c = <-cancelled:
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		if first == "accept" {
			require.NoError(t, a)
			require.ErrorIs(t, c, ErrFriendshipNotFound)
		} else {
			require.NoError(t, c)
			require.ErrorIs(t, a, ErrFriendshipNotFound)
		}
		friends, err := s.AreFriendsAccepted(ctx, sender, target)
		require.NoError(t, err)
		require.Equal(t, a == nil, friends)
		var rows int
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM friendships WHERE requester_profile_id=$1 AND target_profile_id=$2`, sender, target).Scan(&rows))
		if a == nil {
			require.Equal(t, 1, rows)
		} else {
			require.Zero(t, rows)
		}
	}
}

func TestCancelInvitationWaitsForDispatchAndReconcilesPublishedNotice(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	pool := startStorePostgres(t, context.Background())
	applyStoreMigrations(t, context.Background(), pool)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	s := &FriendshipStore{Pool: pool}
	sender, target := uuid.New(), uuid.New()
	require.NoError(t, s.SendInvitationChecked(ctx, sender, target, uuid.New(), uuid.New()))
	publishing, release := make(chan struct{}), make(chan struct{})
	dispatched, cancelled := make(chan error, 1), make(chan error, 1)
	go func() {
		_, err := s.DispatchFriendRequestOutbox(ctx, 1, func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID) error {
			close(publishing)
			select {
			case <-release:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
		dispatched <- err
	}()
	select {
	case <-publishing:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	go func() { cancelled <- s.CancelInvitation(ctx, sender, target) }()
	// Observe the actual FK/outbox lock wait, not a scheduling delay.
	var waiting bool
	require.Eventually(t, func() bool {
		err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE '%DELETE FROM friendships%')`).Scan(&waiting)
		return err == nil && waiting
	}, 5*time.Second, 10*time.Millisecond)
	select {
	case err := <-cancelled:
		t.Fatalf("cancel completed while publisher held outbox: %v", err)
	default:
	}
	close(release)
	select {
	case err := <-dispatched:
		require.NoError(t, err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	select {
	case err := <-cancelled:
		require.NoError(t, err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	// A delivered notice remains possible, but cannot authorize accepting a row
	// that the authoritative request list no longer contains.
	incoming, _, err := s.ListFriendRequests(ctx, target)
	require.NoError(t, err)
	require.Empty(t, incoming)
	require.ErrorIs(t, s.AcceptInvitation(ctx, target, sender), ErrFriendshipNotFound)
	var remaining int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM friend_request_outbox`).Scan(&remaining))
	require.Zero(t, remaining)
}
