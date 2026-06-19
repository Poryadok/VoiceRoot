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

type dmPrivacyStub struct {
	friendsOnly map[uuid.UUID]bool
	fofOnly     map[uuid.UUID]bool
}

func (s dmPrivacyStub) AllowDMAudience(_ context.Context, profileID uuid.UUID) (privacy.Audience, error) {
	if s.fofOnly[profileID] {
		return privacy.FriendsAndFoF(), nil
	}
	if s.friendsOnly[profileID] {
		return privacy.FriendsOnly(), nil
	}
	return privacy.EveryoneWithGuests(), nil
}

func (s dmPrivacyStub) AllowChatSpaceInvitesAudience(_ context.Context, _ uuid.UUID) (privacy.Audience, error) {
	return privacy.EveryoneWithGuests(), nil
}

type noFriendsStub struct{}

func (noFriendsStub) AreFriends(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}

func (noFriendsStub) AreFriendsOfFriends(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}

type fofFriendStub struct {
	friends map[string]bool
	fof     map[string]bool
}

func pairKey(a, b uuid.UUID) string {
	if a.String() < b.String() {
		return a.String() + ":" + b.String()
	}
	return b.String() + ":" + a.String()
}

func (s fofFriendStub) AreFriends(_ context.Context, a, b uuid.UUID) (bool, error) {
	if s.friends == nil {
		return false, nil
	}
	return s.friends[pairKey(a, b)], nil
}

func (s fofFriendStub) AreFriendsOfFriends(_ context.Context, a, b uuid.UUID) (bool, error) {
	if s.fof == nil {
		return false, nil
	}
	return s.fof[pairKey(a, b)], nil
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

// TestCreateDM_FriendsOfFriendsPrivacy_StrangerDenied documents FoF-only DM blocks strangers.
func TestCreateDM_FriendsOfFriendsPrivacy_StrangerDenied(t *testing.T) {
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
		WithPrivacyChecker(dmPrivacyStub{fofOnly: map[uuid.UUID]bool{profB: true}}),
		WithFriendChecker(fofFriendStub{}),
	)
	t.Cleanup(cleanup)

	ctxA := withAccountProfileCtx(ctx, accA, profA)
	_, err := client.CreateDM(ctxA, &chatv1.CreateDMRequest{OtherProfileId: profB.String()})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

// TestCreateDM_FriendsOfFriendsPrivacy_FoFAllowed documents FoF-only DM allows friends-of-friends.
func TestCreateDM_FriendsOfFriendsPrivacy_FoFAllowed(t *testing.T) {
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
		WithPrivacyChecker(dmPrivacyStub{fofOnly: map[uuid.UUID]bool{profB: true}}),
		WithFriendChecker(fofFriendStub{fof: map[string]bool{pairKey(profA, profB): true}}),
	)
	t.Cleanup(cleanup)

	ctxA := withAccountProfileCtx(ctx, accA, profA)
	resp, err := client.CreateDM(ctxA, &chatv1.CreateDMRequest{OtherProfileId: profB.String()})
	require.NoError(t, err)
	require.NotEmpty(t, resp.GetChat().GetId())
}
