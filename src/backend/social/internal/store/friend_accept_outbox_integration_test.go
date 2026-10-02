package store

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestFriendAcceptanceOutboxRetriesPublishWithoutLosingFriendship(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startStorePostgres(t, ctx)
	applyStoreMigrations(t, ctx, pool)
	s := &FriendshipStore{Pool: pool}
	a, b := uuid.New(), uuid.New()
	require.NoError(t, s.SendInvitation(ctx, a, b))
	require.NoError(t, s.AcceptInvitation(ctx, b, a))

	var published int
	failed := errors.New("nats unavailable")
	_, err := s.DispatchAcceptedFriendOutbox(ctx, 10, func(_ context.Context, requester, target uuid.UUID) error {
		require.Equal(t, a, requester)
		require.Equal(t, b, target)
		return failed
	})
	require.ErrorIs(t, err, failed)
	friends, err := s.AreFriendsAccepted(ctx, a, b)
	require.NoError(t, err)
	require.True(t, friends, "accepted friendship must survive a publish outage")

	n, err := s.DispatchAcceptedFriendOutbox(ctx, 10, func(_ context.Context, requester, target uuid.UUID) error {
		published++
		require.Equal(t, a, requester)
		require.Equal(t, b, target)
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, 1, n)
	require.Equal(t, 1, published)
	n, err = s.DispatchAcceptedFriendOutbox(ctx, 10, func(context.Context, uuid.UUID, uuid.UUID) error {
		published++
		return nil
	})
	require.NoError(t, err)
	require.Zero(t, n)
	require.Equal(t, 1, published, "acknowledged event must not be republished")
}
