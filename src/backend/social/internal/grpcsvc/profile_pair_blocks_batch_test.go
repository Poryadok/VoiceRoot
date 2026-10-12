package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/social/internal/store"

	socialv1 "voice.app/voice/social/v1"
)

type batchProfileAccountMap map[uuid.UUID]uuid.UUID

func (m batchProfileAccountMap) AccountIDByProfileID(_ context.Context, profileID uuid.UUID) (uuid.UUID, error) {
	accountID, ok := m[profileID]
	if !ok {
		return uuid.Nil, status.Error(codes.NotFound, "profile not found")
	}
	return accountID, nil
}

func (m batchProfileAccountMap) AccountIDsByProfileIDs(_ context.Context, profileIDs []uuid.UUID) (map[uuid.UUID]uuid.UUID, error) {
	out := make(map[uuid.UUID]uuid.UUID, len(profileIDs))
	for _, profileID := range profileIDs {
		if accountID, ok := m[profileID]; ok {
			out[profileID] = accountID
		}
	}
	return out, nil
}

func TestIsProfilePairsBlocked_BatchesExactDirectionalDecisions(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSocialPostgresForTest(t, ctx)
	applySocialMigration(t, ctx, pool)

	viewerProfile, blockedProfile, neutralProfile := uuid.New(), uuid.New(), uuid.New()
	viewerAccount, blockedAccount, neutralAccount := uuid.New(), uuid.New(), uuid.New()
	client, cleanup := startSocialGRPCTestServer(t, pool, func(s *SocialGRPC) {
		s.ProfileAccounts = batchProfileAccountMap{
			viewerProfile: viewerAccount, blockedProfile: blockedAccount, neutralProfile: neutralAccount,
		}
		s.AccountProfiles = stubAccountProfiles{
			viewerAccount: {viewerProfile}, blockedAccount: {blockedProfile}, neutralAccount: {neutralProfile},
		}
	})
	t.Cleanup(cleanup)
	_, err := client.BlockAccount(withAccountCtx(ctx, viewerAccount), &socialv1.BlockAccountRequest{BlockedAccountId: blockedAccount.String()})
	require.NoError(t, err)

	got, err := client.IsProfilePairsBlocked(ctx, &socialv1.IsProfilePairsBlockedRequest{
		ViewerProfileId: viewerProfile.String(),
		OtherProfileIds: []string{blockedProfile.String(), neutralProfile.String(), blockedProfile.String()},
	})
	require.NoError(t, err)
	require.Len(t, got.GetResults(), 3)
	require.Equal(t, blockedProfile.String(), got.GetResults()[0].GetOtherProfileId())
	require.True(t, got.GetResults()[0].GetBlocked())
	require.Equal(t, neutralProfile.String(), got.GetResults()[1].GetOtherProfileId())
	require.False(t, got.GetResults()[1].GetBlocked())
	require.True(t, got.GetResults()[2].GetBlocked(), "duplicate candidates retain result alignment")

	reverse, err := client.IsProfilePairsBlocked(ctx, &socialv1.IsProfilePairsBlockedRequest{
		ViewerProfileId: blockedProfile.String(), OtherProfileIds: []string{viewerProfile.String()},
	})
	require.NoError(t, err)
	require.False(t, reverse.GetResults()[0].GetBlocked(), "block result remains directional")
}

func TestIsProfilePairsBlocked_RejectsOversizedOrUnresolvableBatch(t *testing.T) {
	ctx := context.Background()
	viewerProfile := uuid.New()
	svc := &SocialGRPC{Blocks: &store.BlockStore{}, ProfileAccounts: batchProfileAccountMap{}}
	tooMany := make([]string, maxProfilePairBlockBatch+1)
	for i := range tooMany {
		tooMany[i] = uuid.NewString()
	}
	_, err := svc.IsProfilePairsBlocked(ctx, &socialv1.IsProfilePairsBlockedRequest{ViewerProfileId: viewerProfile.String(), OtherProfileIds: tooMany})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	_, err = svc.IsProfilePairsBlocked(ctx, &socialv1.IsProfilePairsBlockedRequest{ViewerProfileId: viewerProfile.String(), OtherProfileIds: []string{uuid.NewString()}})
	require.Equal(t, codes.Unavailable, status.Code(err), "missing profile ownership is fail-closed")
}
