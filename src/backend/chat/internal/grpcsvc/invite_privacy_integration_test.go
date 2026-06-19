package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/pkg/privacy"
)

// invitePrivacyChecker documents Phase 11 ChatGRPC.Privacy extension (privacy.md: allow_chat_space_invites).
type invitePrivacyChecker interface {
	PrivacyChecker
	AllowChatSpaceInvitesAudience(ctx context.Context, profileID uuid.UUID) (privacy.Audience, error)
}

type invitePrivacyStub struct {
	friendsOnly map[uuid.UUID]bool
}

func (s invitePrivacyStub) AllowDMAudience(_ context.Context, profileID uuid.UUID) (privacy.Audience, error) {
	return privacy.EveryoneWithGuests(), nil
}

func (s invitePrivacyStub) AllowChatSpaceInvitesAudience(_ context.Context, profileID uuid.UUID) (privacy.Audience, error) {
	if s.friendsOnly[profileID] {
		return privacy.FriendsOnly(), nil
	}
	return privacy.EveryoneWithGuests(), nil
}

// TestAddMembers_FriendsOnlyInvitePrivacy_StrangerDenied documents privacy.md: strangers cannot add a profile with friends-only invite privacy.
func TestAddMembers_FriendsOnlyInvitePrivacy_StrangerDenied(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)

	profiles := profileMap(uuid.New(), uuid.New(), uuid.New(), uuid.New())
	ids := make([]uuid.UUID, 0, 4)
	for id := range profiles {
		ids = append(ids, id)
	}
	owner, memberA, memberB, target := ids[0], ids[1], ids[2], ids[3]

	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil,
		WithPrivacyChecker(invitePrivacyStub{friendsOnly: map[uuid.UUID]bool{target: true}}),
		WithFriendChecker(noFriendsStub{}),
	)
	t.Cleanup(cleanup)

	chat := createStandaloneGroup(t, client, profiles, owner, "Invite privacy", memberA, memberB)

	_, err := client.AddMembers(ctxFor(t, profiles, memberA), &chatv1.AddMembersRequest{
		ChatId:     chat.GetId(),
		ProfileIds: []string{target.String()},
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}
