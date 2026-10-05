package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/timestamppb"

	matchmakingv1 "voice.app/voice/matchmaking/v1"
	"voice/backend/pkg/principal"
)

type recordingMatchmakingRatingGRPC struct {
	recordingMatchmakingMatchGRPC
	lastComplete     *matchmakingv1.CompleteMatchRequest
	lastRate         *matchmakingv1.RateMatchRequest
	lastPlayerRating *matchmakingv1.GetPlayerRatingRequest
	lastBan          *matchmakingv1.BanFromMMRequest
	lastBanStatus    *matchmakingv1.GetMMBanStatusRequest
	banStatusBanned  bool
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

type verifyingMatchmakingCompleteClient struct {
	matchmakingv1.MatchmakingServiceClient
	verify func(context.Context, *matchmakingv1.CompleteMatchRequest)
	calls  int
}

func (c *verifyingMatchmakingCompleteClient) CompleteMatch(ctx context.Context, req *matchmakingv1.CompleteMatchRequest, _ ...grpc.CallOption) (*matchmakingv1.CompleteMatchResponse, error) {
	c.calls++
	if c.verify != nil {
		c.verify(ctx, req)
	}
	return &matchmakingv1.CompleteMatchResponse{Match: &matchmakingv1.Match{Id: req.GetMatchId(), Status: "completed"}}, nil
}

func (s *recordingMatchmakingRatingGRPC) RateMatch(_ context.Context, req *matchmakingv1.RateMatchRequest) (*matchmakingv1.RateMatchResponse, error) {
	s.lastRate = req
	return &matchmakingv1.RateMatchResponse{}, nil
}

func (s *recordingMatchmakingRatingGRPC) GetPlayerRating(_ context.Context, req *matchmakingv1.GetPlayerRatingRequest) (*matchmakingv1.GetPlayerRatingResponse, error) {
	s.lastPlayerRating = req
	return &matchmakingv1.GetPlayerRatingResponse{
		PlayerRating: &matchmakingv1.PlayerRating{
			ProfileId:   req.GetProfileId(),
			GameId:      req.GetGameId(),
			RatingValue: 4.5,
			GamesPlayed: 3,
		},
	}, nil
}

func (s *recordingMatchmakingRatingGRPC) BanFromMM(_ context.Context, req *matchmakingv1.BanFromMMRequest) (*matchmakingv1.BanFromMMResponse, error) {
	s.lastBan = req
	s.banStatusBanned = true
	return &matchmakingv1.BanFromMMResponse{}, nil
}

func (s *recordingMatchmakingRatingGRPC) GetMMBanStatus(_ context.Context, req *matchmakingv1.GetMMBanStatusRequest) (*matchmakingv1.GetMMBanStatusResponse, error) {
	s.lastBanStatus = req
	return &matchmakingv1.GetMMBanStatusResponse{
		MmBanStatus: &matchmakingv1.MMBanStatus{Banned: s.banStatusBanned},
	}, nil
}

func TestTranscodeMatchmakingCompleteMatch(t *testing.T) {
	t.Parallel()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	accountID, profileID := "10000000-0000-4000-8000-000000000002", "10000000-0000-4000-8000-000000000003"
	completeClient := &verifyingMatchmakingCompleteClient{}
	completeClient.verify = func(ctx context.Context, req *matchmakingv1.CompleteMatchRequest) {
		md, ok := metadata.FromOutgoingContext(ctx)
		require.True(t, ok)
		require.Empty(t, md.Get("x-voice-user-id"))
		require.Empty(t, md.Get("x-voice-profile-id"))
		require.Empty(t, md.Get("x-voice-session-epoch"))
		require.Len(t, md.Get("authorization"), 1)
		require.True(t, strings.HasPrefix(md.Get("authorization")[0], "Bearer "))
		require.Equal(t, req.GetOperationId(), md.Get("x-request-id")[0])
		hash, err := principal.RequestHash(req)
		require.NoError(t, err)
		verified, err := principal.VerifyDelegatedUser(context.Background(), strings.TrimPrefix(md.Get("authorization")[0], "Bearer "), principal.VerifyConfig{
			ExpectedIssuer: "gateway", ExpectedAudience: "matchmaking", ExpectedRPC: matchmakingv1.MatchmakingService_CompleteMatch_FullMethodName,
			ExpectedRequestID: req.GetOperationId(), ExpectedRequestHash: hash,
			KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil },
			ReplayGuard: func(context.Context, string, string, time.Time) error { return nil },
			SessionEpochChecker: func(_ context.Context, account string, epoch int64) error {
				require.Equal(t, accountID, account)
				require.Equal(t, int64(8), epoch)
				return nil
			},
		})
		require.NoError(t, err)
		require.Equal(t, accountID, verified.AccountID)
		require.Equal(t, profileID, verified.ProfileID)
		require.NotEmpty(t, verified.JWTID)
	}
	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: accountID, ProfileID: profileID, SessionEpoch: 8, AccountType: "regular", ExpiresAt: time.Now().Add(time.Minute)},
		},
		transcoder: &transcoder{clients: grpcClients{matchmakingComplete: completeClient}, matchmakingCompleteIssuer: issuer},
	})
	operationID := "10000000-0000-4000-8000-000000000001"
	rec := performRequest(h, http.MethodPost, "/api/v1/matchmaking/matches/match-1/complete", `{"operationId":"`+operationID+`"}`, map[string]string{
		"Authorization": "Bearer valid-user-token",
		"Content-Type":  "application/json",
	})
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, 1, completeClient.calls)
	require.Contains(t, rec.Body.String(), "completed")
}

func TestTranscodeMatchmakingCompleteMatchFailsClosedWithoutProtectedUpstream(t *testing.T) {
	t.Parallel()
	ordinaryClient := &verifyingMatchmakingCompleteClient{}
	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "10000000-0000-4000-8000-000000000002", ProfileID: "10000000-0000-4000-8000-000000000003", SessionEpoch: 8, AccountType: "regular", ExpiresAt: time.Now().Add(time.Minute)},
		},
		transcoder: &transcoder{clients: grpcClients{matchmaking: ordinaryClient}},
	})
	rec := performRequest(h, http.MethodPost, "/api/v1/matchmaking/matches/10000000-0000-4000-8000-000000000004/complete", `{"operationId":"10000000-0000-4000-8000-000000000001"}`, map[string]string{
		"Authorization": "Bearer valid-user-token",
		"Content-Type":  "application/json",
	})
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	require.Zero(t, ordinaryClient.calls, "the ordinary matchmaking connection must never receive CompleteMatch")
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

func TestTranscodeMatchmakingSkipRating(t *testing.T) {
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
	rec := performRequest(h, http.MethodPost, "/api/v1/matchmaking/matches/match-1/rate", `{"ratedProfileId":"profile-2","skip":true}`, map[string]string{
		"Authorization": "Bearer valid-user-token",
		"Content-Type":  "application/json",
	})
	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, grpcRec.lastRate)
	require.True(t, grpcRec.lastRate.GetSkip())
	require.Zero(t, grpcRec.lastRate.GetStars())
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

	statusRec := performRequest(h, http.MethodGet, "/api/v1/matchmaking/bans/profile-2", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusOK, statusRec.Code)
	require.NotNil(t, grpcRec.lastBanStatus)
	require.Equal(t, "profile-2", grpcRec.lastBanStatus.GetTargetProfileId())
	require.Contains(t, statusRec.Body.String(), `"banned":true`)
}
