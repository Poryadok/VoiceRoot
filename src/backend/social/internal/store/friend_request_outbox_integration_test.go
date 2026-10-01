package store

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestSendInvitationCheckedPersistsRequestOutboxForEverySend(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startStorePostgres(t, ctx)
	applyStoreMigrations(t, ctx, pool)
	s := &FriendshipStore{Pool: pool}
	requester, target := uuid.New(), uuid.New()
	requesterAccount, targetAccount := uuid.New(), uuid.New()

	sendAndRead := func() (uuid.UUID, uuid.UUID) {
		t.Helper()
		require.NoError(t, s.SendInvitationChecked(ctx, requester, target, requesterAccount, targetAccount))
		var requestID, eventID uuid.UUID
		require.NoError(t, pool.QueryRow(ctx, `
SELECT f.id, o.event_id
FROM friendships f
JOIN friend_request_outbox o ON o.friendship_id = f.id
WHERE f.requester_profile_id = $1 AND f.target_profile_id = $2 AND f.status = 'pending'`, requester, target).Scan(&requestID, &eventID))
		require.NotEqual(t, uuid.Nil, eventID)
		return requestID, eventID
	}

	firstRequest, firstEvent := sendAndRead()
	repeatedRequest, repeatedEvent := sendAndRead()
	require.Equal(t, firstRequest, repeatedRequest, "repeated pending send keeps the request identity")
	require.NotEqual(t, firstEvent, repeatedEvent, "each explicit send gets a fresh event identity")

	require.NoError(t, s.DeclineInvitation(ctx, target, requester))
	reopenedRequest, reopenedEvent := sendAndRead()
	require.Equal(t, firstRequest, reopenedRequest, "reopening keeps the persisted request identity")
	require.NotEqual(t, repeatedEvent, reopenedEvent, "reopening creates a fresh event identity")

	var rows int
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM friend_request_outbox WHERE friendship_id = $1`, firstRequest).Scan(&rows))
	require.Equal(t, 1, rows, "repeated sends refresh one durable outbox row")
}

func TestFriendRequestOutboxRetryPreservesEventIdentity(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startStorePostgres(t, ctx)
	applyStoreMigrations(t, ctx, pool)
	s := &FriendshipStore{Pool: pool}
	requester, target := uuid.New(), uuid.New()
	require.NoError(t, s.SendInvitationChecked(ctx, requester, target, uuid.New(), uuid.New()))

	type publishedRequest struct{ requestID, eventID, requester, target uuid.UUID }
	var first, retry publishedRequest
	failed := errors.New("nats unavailable")
	_, err := s.DispatchFriendRequestOutbox(ctx, 1, func(_ context.Context, requestID, eventID, from, to uuid.UUID) error {
		first = publishedRequest{requestID, eventID, from, to}
		return failed
	})
	require.ErrorIs(t, err, failed)
	require.Equal(t, requester, first.requester)
	require.Equal(t, target, first.target)

	n, err := s.DispatchFriendRequestOutbox(ctx, 1, func(_ context.Context, requestID, eventID, from, to uuid.UUID) error {
		retry = publishedRequest{requestID, eventID, from, to}
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, 1, n)
	require.Equal(t, first, retry, "retry must publish the same request and event identities")

	n, err = s.DispatchFriendRequestOutbox(ctx, 1, func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID) error {
		t.Fatal("delivered request must not be published again")
		return nil
	})
	require.NoError(t, err)
	require.Zero(t, n)
}

func TestFriendRequestOutboxSuppressesRequestsDecidedBeforeDispatch(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startStorePostgres(t, ctx)
	applyStoreMigrations(t, ctx, pool)
	s := &FriendshipStore{Pool: pool}

	for _, decision := range []string{"accepted", "declined"} {
		t.Run(decision, func(t *testing.T) {
			requester, target := uuid.New(), uuid.New()
			requesterAccount, targetAccount := uuid.New(), uuid.New()
			require.NoError(t, s.SendInvitationChecked(ctx, requester, target, requesterAccount, targetAccount))
			if decision == "accepted" {
				require.NoError(t, s.AcceptInvitationChecked(ctx, target, requester, targetAccount, requesterAccount))
			} else {
				require.NoError(t, s.DeclineInvitation(ctx, target, requester))
			}

			publishCalled := false
			n, err := s.DispatchFriendRequestOutbox(ctx, 1, func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID) error {
				publishCalled = true
				return nil
			})
			require.NoError(t, err)
			require.Zero(t, n)
			require.False(t, publishCalled, "a request decided before dispatch must not emit a stale notification")

			var pending int
			require.NoError(t, pool.QueryRow(ctx, `
SELECT COUNT(*) FROM friend_request_outbox o
JOIN friendships f ON f.id = o.friendship_id
WHERE o.delivered_at IS NULL AND o.cancelled_at IS NULL AND f.id = (
  SELECT id FROM friendships WHERE requester_profile_id = $1 AND target_profile_id = $2
)`, requester, target).Scan(&pending))
			require.Zero(t, pending, "decided request must no longer be pending in the outbox")
			var cancelledAt *string
			require.NoError(t, pool.QueryRow(ctx, `
SELECT o.cancelled_at::text FROM friend_request_outbox o
JOIN friendships f ON f.id = o.friendship_id
WHERE f.requester_profile_id = $1 AND f.target_profile_id = $2`, requester, target).Scan(&cancelledAt))
			require.NotNil(t, cancelledAt, "decision must cancel the undelivered request event")
		})
	}
}
