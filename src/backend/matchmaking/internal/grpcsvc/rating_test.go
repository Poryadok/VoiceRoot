package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/matchmaking/internal/store"

	matchmakingv1 "voice.app/voice/matchmaking/v1"
)

func ratingTestServer(t *testing.T, pool *pgxpool.Pool) *MatchmakingGRPC {
	t.Helper()
	srv := matchTestServer(t, pool, &stubSquadProvisioner{})
	srv.Ratings = &store.RatingStore{Pool: pool}
	srv.Bans = &store.BanStore{Pool: pool}
	return srv
}

func activateDuoMatchViaGRPC(t *testing.T, ctx context.Context, srv *MatchmakingGRPC) (matchID string, profileA, profileB uuid.UUID) {
	t.Helper()
	matchID, profileA, profileB = seedPendingDuoMatch(t, ctx, srv)

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
	return matchID, profileA, profileB
}

func TestCompleteMatch_FirstLeaveKeepsActive(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := ratingTestServer(t, pool)
	matchID, profileA, _ := activateDuoMatchViaGRPC(t, ctx, srv)

	resp, err := srv.CompleteMatch(ctxWithProfile(profileA), &matchmakingv1.CompleteMatchRequest{
		MatchId: matchID,
	})
	require.NoError(t, err)
	require.Equal(t, "active", resp.GetMatch().GetStatus())
}

func TestCompleteMatch_AllLeftSetsCompleted(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := ratingTestServer(t, pool)
	matchID, profileA, profileB := activateDuoMatchViaGRPC(t, ctx, srv)

	_, err := srv.CompleteMatch(ctxWithProfile(profileA), &matchmakingv1.CompleteMatchRequest{MatchId: matchID})
	require.NoError(t, err)

	resp, err := srv.CompleteMatch(ctxWithProfile(profileB), &matchmakingv1.CompleteMatchRequest{MatchId: matchID})
	require.NoError(t, err)
	require.Equal(t, "completed", resp.GetMatch().GetStatus())
}

func TestRateMatch_PersistsStarsForTeammate(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := ratingTestServer(t, pool)
	matchID, profileA, profileB := activateDuoMatchViaGRPC(t, ctx, srv)

	_, err := srv.CompleteMatch(ctxWithProfile(profileA), &matchmakingv1.CompleteMatchRequest{MatchId: matchID})
	require.NoError(t, err)
	_, err = srv.CompleteMatch(ctxWithProfile(profileB), &matchmakingv1.CompleteMatchRequest{MatchId: matchID})
	require.NoError(t, err)

	_, err = srv.RateMatch(ctxWithProfile(profileA), &matchmakingv1.RateMatchRequest{
		MatchId:         matchID,
		RatedProfileId:  profileB.String(),
		Stars:           5,
	})
	require.NoError(t, err)
}

func TestRateMatch_DuplicateRejected(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := ratingTestServer(t, pool)
	matchID, profileA, profileB := activateDuoMatchViaGRPC(t, ctx, srv)

	_, err := srv.CompleteMatch(ctxWithProfile(profileA), &matchmakingv1.CompleteMatchRequest{MatchId: matchID})
	require.NoError(t, err)
	_, err = srv.CompleteMatch(ctxWithProfile(profileB), &matchmakingv1.CompleteMatchRequest{MatchId: matchID})
	require.NoError(t, err)

	req := &matchmakingv1.RateMatchRequest{
		MatchId:        matchID,
		RatedProfileId: profileB.String(),
		Stars:          4,
	}
	_, err = srv.RateMatch(ctxWithProfile(profileA), req)
	require.NoError(t, err)

	_, err = srv.RateMatch(ctxWithProfile(profileA), req)
	require.Equal(t, codes.AlreadyExists, status.Code(err))
}

func TestGetPlayerRating_ReturnsAggregate(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := ratingTestServer(t, pool)
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

	resp, err := srv.GetPlayerRating(ctx, &matchmakingv1.GetPlayerRatingRequest{
		ProfileId: profileB.String(),
		GameId:    gameID,
	})
	require.NoError(t, err)
	require.Equal(t, profileB.String(), resp.GetPlayerRating().GetProfileId())
	require.Equal(t, gameID, resp.GetPlayerRating().GetGameId())
	require.Equal(t, 5.0, resp.GetPlayerRating().GetRatingValue())
	require.Equal(t, int32(1), resp.GetPlayerRating().GetGamesPlayed())
}

func TestUnbanFromMM_ClearsPeerBan(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := ratingTestServer(t, pool)
	banner := uuid.New()
	target := uuid.New()

	_, err := srv.BanFromMM(ctxWithProfile(banner), &matchmakingv1.BanFromMMRequest{
		TargetProfileId: target.String(),
	})
	require.NoError(t, err)

	_, err = srv.UnbanFromMM(ctxWithProfile(banner), &matchmakingv1.UnbanFromMMRequest{
		TargetProfileId: target.String(),
	})
	require.NoError(t, err)

	statusResp, err := srv.GetMMBanStatus(ctxWithProfile(banner), &matchmakingv1.GetMMBanStatusRequest{
		TargetProfileId: target.String(),
	})
	require.NoError(t, err)
	require.False(t, statusResp.GetMmBanStatus().GetBanned())
}

func TestBanFromMM_UsesTargetProfileID(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := ratingTestServer(t, pool)
	banner := uuid.New()
	target := uuid.New()

	_, err := srv.BanFromMM(ctxWithProfile(banner), &matchmakingv1.BanFromMMRequest{
		TargetProfileId: target.String(),
	})
	require.NoError(t, err)

	statusResp, err := srv.GetMMBanStatus(ctxWithProfile(banner), &matchmakingv1.GetMMBanStatusRequest{
		TargetProfileId: target.String(),
	})
	require.NoError(t, err)
	require.True(t, statusResp.GetMmBanStatus().GetBanned())
}

func TestRateMatch_ActiveMatchRejected(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := ratingTestServer(t, pool)
	matchID, profileA, profileB := activateDuoMatchViaGRPC(t, ctx, srv)

	_, err := srv.RateMatch(ctxWithProfile(profileA), &matchmakingv1.RateMatchRequest{
		MatchId:        matchID,
		RatedProfileId: profileB.String(),
		Stars:          3,
	})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestRateMatch_NotParticipantDenied(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := ratingTestServer(t, pool)
	matchID, _, profileB := activateDuoMatchViaGRPC(t, ctx, srv)

	_, err := srv.RateMatch(ctxWithProfile(uuid.New()), &matchmakingv1.RateMatchRequest{
		MatchId:        matchID,
		RatedProfileId: profileB.String(),
		Stars:          3,
	})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestCompleteMatch_NotParticipantDenied(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := ratingTestServer(t, pool)
	matchID, _, _ := activateDuoMatchViaGRPC(t, ctx, srv)

	_, err := srv.CompleteMatch(ctxWithProfile(uuid.New()), &matchmakingv1.CompleteMatchRequest{
		MatchId: matchID,
	})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}
