package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"voice/backend/pkg/privacy"

	matchmakingv1 "voice.app/voice/matchmaking/v1"
)

type mmRatingPrivacyStub struct {
	audience privacy.Audience
}

func (s mmRatingPrivacyStub) ShowMmRatingAudience(context.Context, uuid.UUID) (privacy.Audience, error) {
	return s.audience, nil
}

func ctxWithGuestProfile(profileID uuid.UUID) context.Context {
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		"x-voice-profile-id", profileID.String(),
		"x-voice-account-type", "guest",
	))
}

func TestGetPlayerRating_GuestViewerDeniedWhenGuestAudienceExcluded(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := ratingTestServer(t, pool)
	srv.RatingPrivacy = mmRatingPrivacyStub{audience: privacy.FriendsOnly()}

	matchID, profileA, profileB := activateDuoMatchViaGRPC(t, ctx, srv)
	_, err := srv.CompleteMatch(ctxWithProfile(profileA), &matchmakingv1.CompleteMatchRequest{MatchId: matchID})
	require.NoError(t, err)
	_, err = srv.CompleteMatch(ctxWithProfile(profileB), &matchmakingv1.CompleteMatchRequest{MatchId: matchID})
	require.NoError(t, err)
	_, err = srv.RateMatch(ctxWithProfile(profileA), &matchmakingv1.RateMatchRequest{
		MatchId:        matchID,
		RatedProfileId: profileB.String(),
		Stars:          5,
	})
	require.NoError(t, err)

	got, err := srv.GetMatch(ctxWithProfile(profileB), &matchmakingv1.GetMatchRequest{MatchId: matchID})
	require.NoError(t, err)
	gameID := got.GetMatch().GetGameId()

	_, err = srv.GetPlayerRating(ctxWithGuestProfile(uuid.New()), &matchmakingv1.GetPlayerRatingRequest{
		ProfileId: profileB.String(),
		GameId:    gameID,
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}
