package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/matchmaking/internal/store"

	commonv1 "voice.app/voice/common/v1"
	matchmakingv1 "voice.app/voice/matchmaking/v1"
)

func TestGetMatchHistory_Unauthenticated(t *testing.T) {
	t.Parallel()
	srv := &MatchmakingGRPC{Matches: &store.MatchStore{}}
	_, err := srv.GetMatchHistory(context.Background(), &matchmakingv1.GetMatchHistoryRequest{})
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestGetMatchHistory_InvalidProfileID(t *testing.T) {
	t.Parallel()
	srv := &MatchmakingGRPC{Matches: &store.MatchStore{}}
	profileA := uuid.New()
	_, err := srv.GetMatchHistory(ctxWithProfile(profileA), &matchmakingv1.GetMatchHistoryRequest{
		ProfileId: "not-a-uuid",
	})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestGetMatchHistory_OtherProfileDenied(t *testing.T) {
	t.Parallel()
	srv := &MatchmakingGRPC{Matches: &store.MatchStore{}}
	profileA := uuid.New()
	_, err := srv.GetMatchHistory(ctxWithProfile(profileA), &matchmakingv1.GetMatchHistoryRequest{
		ProfileId: uuid.New().String(),
	})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestGetMatchHistory_ReturnsActiveMatches(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := matchTestServer(t, pool, &stubSquadProvisioner{})
	matchID, profileA, profileB := seedPendingDuoMatch(t, ctx, srv)

	_, err := srv.RespondToMatch(ctxWithProfile(profileA), &matchmakingv1.RespondToMatchRequest{
		MatchId: matchID,
		Accept:  true,
	})
	require.NoError(t, err)
	_, err = srv.RespondToMatch(ctxWithProfile(profileB), &matchmakingv1.RespondToMatchRequest{
		MatchId: matchID,
		Accept:  true,
	})
	require.NoError(t, err)

	resp, err := srv.GetMatchHistory(ctxWithProfile(profileA), &matchmakingv1.GetMatchHistoryRequest{
		Page: &commonv1.CursorPageRequest{PageSize: 10},
	})
	require.NoError(t, err)
	require.Len(t, resp.GetMatchList().GetMatches(), 1)
	got := resp.GetMatchList().GetMatches()[0]
	require.Equal(t, matchID, got.GetId())
	require.Equal(t, "active", got.GetStatus())
	require.Len(t, got.GetProfileIds(), 2)
}

func TestGetMatchHistory_ExcludesPendingForNonParticipant(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := matchTestServer(t, pool, &stubSquadProvisioner{})
	_, _, _ = seedPendingDuoMatch(t, ctx, srv)

	resp, err := srv.GetMatchHistory(ctxWithProfile(uuid.New()), &matchmakingv1.GetMatchHistoryRequest{
		Page: &commonv1.CursorPageRequest{PageSize: 10},
	})
	require.NoError(t, err)
	require.Empty(t, resp.GetMatchList().GetMatches())
}
