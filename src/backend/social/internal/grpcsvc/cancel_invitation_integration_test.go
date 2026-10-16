package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	socialv1 "voice.app/voice/social/v1"
)

func TestCancelFriendInvitationAuthenticatedSenderOnly(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSocialPostgresForTest(t, ctx)
	applySocialMigration(t, ctx, pool)
	client, cleanup := startSocialGRPCTestServer(t, pool)
	t.Cleanup(cleanup)
	sender, target := uuid.New(), uuid.New()
	_, err := client.SendFriendInvitation(withITProfileCtx(ctx, sender), &socialv1.SendFriendInvitationRequest{TargetProfileId: target.String()})
	require.NoError(t, err)
	_, err = client.CancelFriendInvitation(ctx, &socialv1.CancelFriendInvitationRequest{TargetProfileId: target.String()})
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	_, err = client.CancelFriendInvitation(withITProfileCtx(ctx, sender), &socialv1.CancelFriendInvitationRequest{TargetProfileId: "bad-id"})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	_, err = client.CancelFriendInvitation(withITProfileCtx(ctx, target), &socialv1.CancelFriendInvitationRequest{TargetProfileId: sender.String()})
	require.Equal(t, codes.NotFound, status.Code(err))
	_, err = client.CancelFriendInvitation(withITProfileCtx(ctx, uuid.New()), &socialv1.CancelFriendInvitationRequest{TargetProfileId: target.String()})
	require.Equal(t, codes.NotFound, status.Code(err))
	_, err = client.CancelFriendInvitation(withITProfileCtx(ctx, sender), &socialv1.CancelFriendInvitationRequest{TargetProfileId: target.String()})
	require.NoError(t, err)
	_, err = client.CancelFriendInvitation(withITProfileCtx(ctx, sender), &socialv1.CancelFriendInvitationRequest{TargetProfileId: target.String()})
	require.Equal(t, codes.NotFound, status.Code(err))
	requests, err := client.ListFriendRequests(withITProfileCtx(ctx, target), &socialv1.ListFriendRequestsRequest{})
	require.NoError(t, err)
	require.Empty(t, requests.GetFriendRequestList().GetIncoming())
}
