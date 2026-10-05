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

type canCreateDMPrivacyAudience struct {
	audience privacy.Audience
}

func (p canCreateDMPrivacyAudience) AllowDMAudience(context.Context, uuid.UUID) (privacy.Audience, error) {
	return p.audience, nil
}

func (p canCreateDMPrivacyAudience) AllowChatSpaceInvitesAudience(context.Context, uuid.UUID) (privacy.Audience, error) {
	return privacy.EveryoneWithGuests(), nil
}

type canCreateDMSpaceMembership struct {
	allowed  bool
	spaceIDs []string
}

func (s *canCreateDMSpaceMembership) AreCoMembers(_ context.Context, _, _ uuid.UUID, spaceIDs []string) (bool, error) {
	s.spaceIDs = append([]string(nil), spaceIDs...)
	return s.allowed, nil
}

func TestCanCreateDMIsReadOnlyAndReturnsOnlyEffectivePermission(t *testing.T) {
	ctx := context.Background()
	accountA, profileA := uuid.New(), uuid.New()
	accountB, profileB := uuid.New(), uuid.New()
	profiles := mapProfileAccounts{profileA: accountA, profileB: accountB}
	events := &spyChatEvents{}
	client, cleanup := startChatGRPCTestServer(t, nil, profiles, nil, nil,
		WithDMStore(nil), WithChatEventsPublisher(events),
	)
	t.Cleanup(cleanup)
	caller := withAccountProfileCtx(ctx, accountA, profileA)

	allowed, err := client.CanCreateDM(caller, &chatv1.CanCreateDMRequest{OtherProfileId: profileB.String()})
	require.NoError(t, err)
	require.True(t, allowed.GetAllowed())

	blockedClient, blockedCleanup := startChatGRPCTestServer(t, nil, profiles, stubBlocks{blocked: true}, nil, WithDMStore(nil))
	t.Cleanup(blockedCleanup)
	blocked, err := blockedClient.CanCreateDM(caller, &chatv1.CanCreateDMRequest{OtherProfileId: profileB.String()})
	require.NoError(t, err)
	require.False(t, blocked.GetAllowed())

	self, err := client.CanCreateDM(caller, &chatv1.CanCreateDMRequest{OtherProfileId: profileA.String()})
	require.NoError(t, err)
	require.False(t, self.GetAllowed())

	guest, err := client.CanCreateDM(withGuestAccountProfileCtx(ctx, accountA, profileA), &chatv1.CanCreateDMRequest{OtherProfileId: profileB.String()})
	require.NoError(t, err)
	require.False(t, guest.GetAllowed())

	created, joined := events.snapshot()
	require.Empty(t, created)
	require.Empty(t, joined)
}

func TestCanCreateDMDeniesMissingDeletedOrPrivacyRestrictedPeers(t *testing.T) {
	ctx := context.Background()
	accountA, profileA := uuid.New(), uuid.New()
	accountB, profileB := uuid.New(), uuid.New()
	profiles := mapProfileAccounts{profileA: accountA, profileB: accountB}
	caller := withAccountProfileCtx(ctx, accountA, profileA)

	tests := []struct {
		name   string
		client func() (chatv1.ChatServiceClient, func())
		target string
	}{
		{
			name: "missing_profile_is_not_disclosed",
			client: func() (chatv1.ChatServiceClient, func()) {
				return startChatGRPCTestServer(t, nil, mapProfileAccounts{profileA: accountA}, nil, nil, WithDMStore(nil))
			},
			target: uuid.NewString(),
		},
		{
			name: "deleted_profile_is_not_disclosed",
			client: func() (chatv1.ChatServiceClient, func()) {
				return startChatGRPCTestServer(t, nil, profiles, nil, nil,
					WithDMStore(nil), WithAccountDeletedChecker(mapDeletedAccounts{accountB: {}}),
				)
			},
			target: profileB.String(),
		},
		{
			name: "privacy_denial_is_not_disclosed",
			client: func() (chatv1.ChatServiceClient, func()) {
				return startChatGRPCTestServer(t, nil, profiles, nil, nil,
					WithDMStore(nil),
					WithPrivacyChecker(dmPrivacyStub{friendsOnly: map[uuid.UUID]bool{profileB: true}}),
					WithFriendChecker(noFriendsStub{}),
				)
			},
			target: profileB.String(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, cleanup := tt.client()
			t.Cleanup(cleanup)
			response, err := client.CanCreateDM(caller, &chatv1.CanCreateDMRequest{OtherProfileId: tt.target})
			require.NoError(t, err)
			require.False(t, response.GetAllowed())
		})
	}
}

func TestCanCreateDMAppliesFriendFoFAndSpaceAudienceChecks(t *testing.T) {
	ctx := context.Background()
	accountA, profileA := uuid.New(), uuid.New()
	accountB, profileB := uuid.New(), uuid.New()
	profiles := mapProfileAccounts{profileA: accountA, profileB: accountB}
	caller := withAccountProfileCtx(ctx, accountA, profileA)
	pair := pairKey(profileA, profileB)

	friendClient, friendCleanup := startChatGRPCTestServer(t, nil, profiles, nil, nil,
		WithDMStore(nil),
		WithPrivacyChecker(canCreateDMPrivacyAudience{audience: privacy.FriendsOnly()}),
		WithFriendChecker(fofFriendStub{friends: map[string]bool{pair: true}}),
	)
	t.Cleanup(friendCleanup)
	friend, err := friendClient.CanCreateDM(caller, &chatv1.CanCreateDMRequest{OtherProfileId: profileB.String()})
	require.NoError(t, err)
	require.True(t, friend.GetAllowed())

	fofClient, fofCleanup := startChatGRPCTestServer(t, nil, profiles, nil, nil,
		WithDMStore(nil),
		WithPrivacyChecker(canCreateDMPrivacyAudience{audience: privacy.FriendsAndFoF()}),
		WithFriendChecker(fofFriendStub{fof: map[string]bool{pair: true}}),
	)
	t.Cleanup(fofCleanup)
	fof, err := fofClient.CanCreateDM(caller, &chatv1.CanCreateDMRequest{OtherProfileId: profileB.String()})
	require.NoError(t, err)
	require.True(t, fof.GetAllowed())

	spaceMembership := &canCreateDMSpaceMembership{allowed: true}
	spaceAudience := privacy.Audience{SpaceMembers: true, SpaceIDs: []string{"space-1"}}
	spaceClient, spaceCleanup := startChatGRPCTestServer(t, nil, profiles, nil, nil,
		WithDMStore(nil),
		WithPrivacyChecker(canCreateDMPrivacyAudience{audience: spaceAudience}),
		func(service *ChatGRPC) { service.SpaceCoMembership = spaceMembership },
	)
	t.Cleanup(spaceCleanup)
	space, err := spaceClient.CanCreateDM(caller, &chatv1.CanCreateDMRequest{OtherProfileId: profileB.String()})
	require.NoError(t, err)
	require.True(t, space.GetAllowed())
	require.Equal(t, []string{"space-1"}, spaceMembership.spaceIDs)
}

func TestCanCreateDMKeepsAuthenticationValidationAndDependencyFailuresTyped(t *testing.T) {
	ctx := context.Background()
	accountA, profileA := uuid.New(), uuid.New()
	accountB, profileB := uuid.New(), uuid.New()
	profiles := mapProfileAccounts{profileA: accountA, profileB: accountB}
	client, cleanup := startChatGRPCTestServer(t, nil, profiles, nil, nil,
		WithDMStore(nil), WithBlockChecker(nil),
	)
	t.Cleanup(cleanup)

	_, err := client.CanCreateDM(ctx, &chatv1.CanCreateDMRequest{OtherProfileId: profileB.String()})
	require.Equal(t, codes.Unauthenticated, status.Code(err))

	_, err = client.CanCreateDM(withAccountProfileCtx(ctx, accountA, profileA), &chatv1.CanCreateDMRequest{OtherProfileId: "not-a-uuid"})
	require.Equal(t, codes.InvalidArgument, status.Code(err))

	_, err = client.CanCreateDM(withAccountProfileCtx(ctx, accountA, profileA), &chatv1.CanCreateDMRequest{OtherProfileId: profileB.String()})
	require.Equal(t, codes.Unavailable, status.Code(err), "missing block authority must fail closed")

	withPrivacy, privacyCleanup := startChatGRPCTestServer(t, nil, profiles, nil, nil,
		WithDMStore(nil), WithPrivacyChecker(unavailableDMPrivacyChecker{}),
	)
	t.Cleanup(privacyCleanup)
	_, err = withPrivacy.CanCreateDM(withAccountProfileCtx(ctx, accountA, profileA), &chatv1.CanCreateDMRequest{OtherProfileId: profileB.String()})
	require.Equal(t, codes.Unavailable, status.Code(err), "privacy service failure must remain an operational error")
}
