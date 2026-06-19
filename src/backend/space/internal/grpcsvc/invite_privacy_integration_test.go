package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/privacy"
)

type spaceInvitePrivacyStub struct {
	friendsOnly map[uuid.UUID]bool
}

func (s spaceInvitePrivacyStub) AllowChatSpaceInvitesAudience(_ context.Context, profileID uuid.UUID) (privacy.Audience, error) {
	if s.friendsOnly[profileID] {
		return privacy.FriendsOnly(), nil
	}
	return privacy.EveryoneWithGuests(), nil
}

type spaceNoFriendsStub struct{}

func (spaceNoFriendsStub) AreFriends(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}

func (spaceNoFriendsStub) AreFriendsOfFriends(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}

// TestJoinByInvite_FriendsOnlyInvitePrivacy_StrangerDenied documents privacy.md: joiner with friends-only invite privacy rejects invite from non-friend space owner.
func TestJoinByInvite_FriendsOnlyInvitePrivacy_StrangerDenied(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	ownerAccount, ownerProfile := uuid.New(), uuid.New()
	joinerAccount, joinerProfile := uuid.New(), uuid.New()
	ownerCtx := withAccountProfileCtx(context.Background(), ownerAccount, ownerProfile)
	joinerCtx := withAccountProfileCtx(context.Background(), joinerAccount, joinerProfile)

	pool := startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)
	client, cleanup := startSpaceGRPCTestServer(t, pool,
		withInvitePrivacy(spaceInvitePrivacyStub{friendsOnly: map[uuid.UUID]bool{joinerProfile: true}}),
		withInviteFriends(spaceNoFriendsStub{}),
	)
	t.Cleanup(cleanup)

	created, err := client.CreateSpace(ownerCtx, &spacev1.CreateSpaceRequest{Name: "Invite privacy"})
	require.NoError(t, err)
	spaceID := created.GetSpace().GetId()

	inv, err := client.CreateInvite(ownerCtx, &spacev1.CreateInviteRequest{SpaceId: spaceID})
	require.NoError(t, err)

	_, err = client.JoinByInvite(joinerCtx, &spacev1.JoinByInviteRequest{Code: inv.GetInvite().GetCode()})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}
