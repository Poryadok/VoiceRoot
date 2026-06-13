package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	chatv1 "voice.app/voice/chat/v1"
)

type dmPrivacyStub struct {
	friendsOnly map[uuid.UUID]bool
}

func (s dmPrivacyStub) AllowDM(_ context.Context, profileID uuid.UUID) (string, error) {
	if s.friendsOnly[profileID] {
		return "friends", nil
	}
	return "everyone", nil
}

type noFriendsStub struct{}

func (noFriendsStub) AreFriends(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}

// TestCreateDM_FriendsOnlyPrivacy_StrangerDenied documents privacy.md: friends-only DM blocks strangers at CreateDM.
func TestCreateDM_FriendsOnlyPrivacy_StrangerDenied(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)

	accA := uuid.New()
	accB := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	profiles := mapProfileAccounts{profA: accA, profB: accB}

	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil,
		WithPrivacyChecker(dmPrivacyStub{friendsOnly: map[uuid.UUID]bool{profB: true}}),
		WithFriendChecker(noFriendsStub{}),
	)
	t.Cleanup(cleanup)

	ctxA := withAccountProfileCtx(ctx, accA, profA)
	_, err := client.CreateDM(ctxA, &chatv1.CreateDMRequest{OtherProfileId: profB.String()})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}
