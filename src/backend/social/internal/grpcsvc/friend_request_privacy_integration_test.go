package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/pkg/privacy"

	socialv1 "voice.app/voice/social/v1"
)

type friendRequestPrivacyStub struct {
	nobody map[uuid.UUID]bool
}

func (s friendRequestPrivacyStub) AllowFriendRequestsAudience(_ context.Context, profileID uuid.UUID) (privacy.Audience, error) {
	if s.nobody[profileID] {
		return privacy.Nobody(), nil
	}
	return privacy.EveryoneWithGuests(), nil
}

func withFriendRequestPrivacy(p FriendRequestPrivacyChecker) socialServerOption {
	return func(s *SocialGRPC) { s.Privacy = p }
}

// TestSendFriendInvitation_AllowFriendRequestsNobody_PermissionDenied documents privacy.md / FR-03.
func TestSendFriendInvitation_AllowFriendRequestsNobody_PermissionDenied(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSocialPostgresForTest(t, ctx)
	applySocialMigration(t, ctx, pool)

	caller := uuid.New()
	target := uuid.New()

	client, cleanup := startSocialGRPCTestServer(t, pool,
		withFriendRequestPrivacy(friendRequestPrivacyStub{nobody: map[uuid.UUID]bool{target: true}}),
	)
	t.Cleanup(cleanup)

	_, err := client.SendFriendInvitation(withProfileCtx(ctx, caller), &socialv1.SendFriendInvitationRequest{
		TargetProfileId: target.String(),
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}
