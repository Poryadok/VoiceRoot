package main

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	matchmakingv1 "voice.app/voice/matchmaking/v1"
)

type recordingMatchmakingRatingGRPC struct {
	recordingMatchmakingMatchGRPC
	lastComplete      *matchmakingv1.CompleteMatchRequest
	lastRate          *matchmakingv1.RateMatchRequest
	lastPlayerRating  *matchmakingv1.GetPlayerRatingRequest
	lastBan           *matchmakingv1.BanFromMMRequest
}

func (s *recordingMatchmakingRatingGRPC) CompleteMatch(_ context.Context, req *matchmakingv1.CompleteMatchRequest) (*matchmakingv1.CompleteMatchResponse, error) {
	s.lastComplete = req
	return &matchmakingv1.CompleteMatchResponse{
		Match: &matchmakingv1.Match{
			Id:         req.GetMatchId(),
			GameId:     "game-1",
			Mode:       "Duo",
			Region:     "eu",
			Status:     "completed",
			ProfileIds: []string{"profile-1", "profile-2"},
			CreatedAt:  timestamppb.Now(),
		},
	}, nil
}

func (s *recordingMatchmakingRatingGRPC) RateMatch(_ context.Context, req *matchmakingv1.RateMatchRequest) (*matchmakingv1.RateMatchResponse, error) {
	s.lastRate = req
	return &matchmakingv1.RateMatchResponse{}, nil
}

func (s *recordingMatchmakingRatingGRPC) GetPlayerRating(_ context.Context, req *matchmakingv1.GetPlayerRatingRequest) (*matchmakingv1.GetPlayerRatingResponse, error) {
	s.lastPlayerRating = req
	return &matchmakingv1.GetPlayerRatingResponse{
		PlayerRating: &matchmakingv1.PlayerRating{
			ProfileId:    req.GetProfileId(),
			GameId:       req.GetGameId(),
			RatingValue:  4.5,
			GamesPlayed:  3,
		},
	}, nil
}

func (s *recordingMatchmakingRatingGRPC) BanFromMM(_ context.Context, req *matchmakingv1.BanFromMMRequest) (*matchmakingv1.BanFromMMResponse, error) {
	s.lastBan = req
	return &matchmakingv1.BanFromMMResponse{}, nil
}

func TestTranscodeMatchmakingCompleteMatch(t *testing.T) {
	t.Parallel()
	grpcRec := &recordingMatchmakingRatingGRPC{}
	conn, cleanup := startBufconnMatchmakingConn(t, grpcRec)
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{matchmaking: matchmakingv1.NewMatchmakingServiceClient(conn)}},
	})
	rec := performRequest(h, http.MethodPost, "/api/v1/matchmaking/matches/match-1/complete", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, grpcRec.lastComplete)
	require.Equal(t, "match-1", grpcRec.lastComplete.GetMatchId())
	require.Contains(t, rec.Body.String(), "completed")
}

func TestTranscodeMatchmakingRateMatch(t *testing.T) {
	t.Parallel()
	grpcRec := &recordingMatchmakingRatingGRPC{}
	conn, cleanup := startBufconnMatchmakingConn(t, grpcRec)
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{matchmaking: matchmakingv1.NewMatchmakingServiceClient(conn)}},
	})
	body := `{"ratedProfileId":"profile-2","stars":5}`
	rec := performRequest(h, http.MethodPost, "/api/v1/matchmaking/matches/match-1/rate", body, map[string]string{
		"Authorization": "Bearer valid-user-token",
		"Content-Type":  "application/json",
	})
	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, grpcRec.lastRate)
	require.Equal(t, "match-1", grpcRec.lastRate.GetMatchId())
	require.Equal(t, "profile-2", grpcRec.lastRate.GetRatedProfileId())
	require.Equal(t, int32(5), grpcRec.lastRate.GetStars())
}

func TestTranscodeMatchmakingGetPlayerRating(t *testing.T) {
	t.Parallel()
	grpcRec := &recordingMatchmakingRatingGRPC{}
	conn, cleanup := startBufconnMatchmakingConn(t, grpcRec)
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{matchmaking: matchmakingv1.NewMatchmakingServiceClient(conn)}},
	})
	rec := performRequest(h, http.MethodGet, "/api/v1/matchmaking/players/profile-2/rating?game_id=game-1", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, grpcRec.lastPlayerRating)
	require.Equal(t, "profile-2", grpcRec.lastPlayerRating.GetProfileId())
	require.Equal(t, "game-1", grpcRec.lastPlayerRating.GetGameId())
	require.Contains(t, rec.Body.String(), "4.5")
}

func TestTranscodeMatchmakingBanFromMM(t *testing.T) {
	t.Parallel()
	grpcRec := &recordingMatchmakingRatingGRPC{}
	conn, cleanup := startBufconnMatchmakingConn(t, grpcRec)
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{matchmaking: matchmakingv1.NewMatchmakingServiceClient(conn)}},
	})
	body := `{"targetProfileId":"profile-2","reason":"toxic"}`
	rec := performRequest(h, http.MethodPost, "/api/v1/matchmaking/bans", body, map[string]string{
		"Authorization": "Bearer valid-user-token",
		"Content-Type":  "application/json",
	})
	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, grpcRec.lastBan)
	require.Equal(t, "profile-2", grpcRec.lastBan.GetTargetProfileId())
}
