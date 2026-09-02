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

type allowFriendRequestsPrivacyStub struct {
	disabled map[uuid.UUID]bool
}

func (s allowFriendRequestsPrivacyStub) AllowFriendRequestsAudience(_ context.Context, profileID uuid.UUID) (privacy.Audience, error) {
	if s.disabled[profileID] {
		return privacy.Nobody(), nil
	}
	return privacy.EveryoneWithGuests(), nil
}

func withAllowFriendRequestsPrivacy(p FriendRequestPrivacyChecker) socialServerOption {
	return func(s *SocialGRPC) { s.Privacy = p }
}

// TestSendFriendInvitation_AllowFriendRequestsFalse_PermissionDenied documents privacy.md / FR-03:
// stranger cannot send friend invite when target has allow_friend_requests=false.
func TestSendFriendInvitation_AllowFriendRequestsFalse_PermissionDenied(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSocialPostgresForTest(t, ctx)
	applySocialMigration(t, ctx, pool)

	caller := uuid.New()
	target := uuid.New()

	client, cleanup := startSocialGRPCTestServer(t, pool,
		withAllowFriendRequestsPrivacy(allowFriendRequestsPrivacyStub{disabled: map[uuid.UUID]bool{target: true}}),
	)
	t.Cleanup(cleanup)

	_, err := client.SendFriendInvitation(withITProfileCtx(ctx, caller), &socialv1.SendFriendInvitationRequest{
		TargetProfileId: target.String(),
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}
