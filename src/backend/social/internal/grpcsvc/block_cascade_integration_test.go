package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	socialv1 "voice.app/voice/social/v1"
)

// TestBlockAccount_RemovesAcceptedFriendship documents block cascade: accepted friendship rows are removed.
func TestBlockAccount_RemovesAcceptedFriendship(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSocialPostgresForTest(t, ctx)
	applySocialMigration(t, ctx, pool)

	accA := uuid.New()
	accB := uuid.New()
	profA := uuid.New()
	profB := uuid.New()

	client, cleanup := startSocialGRPCTestServer(t, pool, func(s *SocialGRPC) {
		s.AccountProfiles = stubAccountProfiles{
			accA: {profA},
			accB: {profB},
		}
	})
	t.Cleanup(cleanup)

	_, err := client.SendFriendInvitation(withITProfileCtx(ctx, profA), &socialv1.SendFriendInvitationRequest{
		TargetProfileId: profB.String(),
	})
	require.NoError(t, err)
	_, err = client.AcceptFriendInvitation(withProfileCtx(ctx, profB), &socialv1.AcceptFriendInvitationRequest{
		RequesterProfileId: profA.String(),
	})
	require.NoError(t, err)

	friends, err := client.ListFriends(withProfileCtx(ctx, profA), &socialv1.ListFriendsRequest{})
	require.NoError(t, err)
	require.Len(t, friends.GetFriendList().GetFriends(), 1)

	_, err = client.BlockAccount(withAccountCtx(ctx, accA), &socialv1.BlockAccountRequest{
		BlockedAccountId: accB.String(),
	})
	require.NoError(t, err)

	friendsAfter, err := client.ListFriends(withProfileCtx(ctx, profA), &socialv1.ListFriendsRequest{})
	require.NoError(t, err)
	require.Empty(t, friendsAfter.GetFriendList().GetFriends())

	af, err := client.AreFriends(ctx, &socialv1.AreFriendsRequest{
		ProfileIdA: profA.String(),
		ProfileIdB: profB.String(),
	})
	require.NoError(t, err)
	require.False(t, af.GetFriends())
}

// TestBlockAccount_ClearsPendingOutgoingFriendRequest documents block cascade: pending outgoing invites are cancelled.
func TestBlockAccount_ClearsPendingOutgoingFriendRequest(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSocialPostgresForTest(t, ctx)
	applySocialMigration(t, ctx, pool)

	accA := uuid.New()
	accB := uuid.New()
	profA := uuid.New()
	profB := uuid.New()

	client, cleanup := startSocialGRPCTestServer(t, pool, func(s *SocialGRPC) {
		s.AccountProfiles = stubAccountProfiles{
			accA: {profA},
			accB: {profB},
		}
	})
	t.Cleanup(cleanup)

	_, err := client.SendFriendInvitation(withITProfileCtx(ctx, profA), &socialv1.SendFriendInvitationRequest{
		TargetProfileId: profB.String(),
	})
	require.NoError(t, err)

	outBefore, err := client.ListFriendRequests(withProfileCtx(ctx, profA), &socialv1.ListFriendRequestsRequest{})
	require.NoError(t, err)
	require.Len(t, outBefore.GetFriendRequestList().GetOutgoing(), 1)

	_, err = client.BlockAccount(withAccountCtx(ctx, accA), &socialv1.BlockAccountRequest{
		BlockedAccountId: accB.String(),
	})
	require.NoError(t, err)

	outAfter, err := client.ListFriendRequests(withProfileCtx(ctx, profA), &socialv1.ListFriendRequestsRequest{})
	require.NoError(t, err)
	require.Empty(t, outAfter.GetFriendRequestList().GetOutgoing())

	inAfter, err := client.ListFriendRequests(withProfileCtx(ctx, profB), &socialv1.ListFriendRequestsRequest{})
	require.NoError(t, err)
	require.Empty(t, inAfter.GetFriendRequestList().GetIncoming())
}

// TestBlockAccount_ClearsPendingIncomingFriendRequest documents block cascade: pending incoming invites are declined/cancelled.
func TestBlockAccount_ClearsPendingIncomingFriendRequest(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSocialPostgresForTest(t, ctx)
	applySocialMigration(t, ctx, pool)

	accA := uuid.New()
	accB := uuid.New()
	profA := uuid.New()
	profB := uuid.New()

	client, cleanup := startSocialGRPCTestServer(t, pool, func(s *SocialGRPC) {
		s.AccountProfiles = stubAccountProfiles{
			accA: {profA},
			accB: {profB},
		}
	})
	t.Cleanup(cleanup)

	_, err := client.SendFriendInvitation(withITProfileCtx(ctx, profB), &socialv1.SendFriendInvitationRequest{
		TargetProfileId: profA.String(),
	})
	require.NoError(t, err)

	inBefore, err := client.ListFriendRequests(withProfileCtx(ctx, profA), &socialv1.ListFriendRequestsRequest{})
	require.NoError(t, err)
	require.Len(t, inBefore.GetFriendRequestList().GetIncoming(), 1)

	_, err = client.BlockAccount(withAccountCtx(ctx, accA), &socialv1.BlockAccountRequest{
		BlockedAccountId: accB.String(),
	})
	require.NoError(t, err)

	inAfter, err := client.ListFriendRequests(withProfileCtx(ctx, profA), &socialv1.ListFriendRequestsRequest{})
	require.NoError(t, err)
	require.Empty(t, inAfter.GetFriendRequestList().GetIncoming())
}
