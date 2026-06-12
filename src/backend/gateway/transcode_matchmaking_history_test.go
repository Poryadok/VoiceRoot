package main

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	matchmakingv1 "voice.app/voice/matchmaking/v1"
)

type recordingMatchmakingHistoryGRPC struct {
	matchmakingv1.UnimplementedMatchmakingServiceServer
	lastHistory *matchmakingv1.GetMatchHistoryRequest
}

func (s *recordingMatchmakingHistoryGRPC) GetMatchHistory(_ context.Context, req *matchmakingv1.GetMatchHistoryRequest) (*matchmakingv1.GetMatchHistoryResponse, error) {
	s.lastHistory = req
	return &matchmakingv1.GetMatchHistoryResponse{
		MatchList: &matchmakingv1.MatchList{
			Matches: []*matchmakingv1.Match{{
				Id:         "match-1",
				GameId:     "game-1",
				Mode:       "Duo",
				Region:     "eu",
				Status:     "completed",
				ProfileIds: []string{"profile-1", "profile-2"},
				CreatedAt:  timestamppb.Now(),
			}},
			NextCursor: "cursor-2",
		},
	}, nil
}

func TestTranscodeMatchmakingGetMatchHistory(t *testing.T) {
	t.Parallel()
	grpcRec := &recordingMatchmakingHistoryGRPC{}
	conn, cleanup := startBufconnMatchmakingConn(t, grpcRec)
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{matchmaking: matchmakingv1.NewMatchmakingServiceClient(conn)}},
	})
	rec := performRequest(h, http.MethodGet, "/api/v1/matchmaking/profile/me/matches?cursor=c1&page_size=25", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, grpcRec.lastHistory)
	require.Equal(t, "c1", grpcRec.lastHistory.GetPage().GetCursor())
	require.Equal(t, int32(25), grpcRec.lastHistory.GetPage().GetPageSize())
	require.Contains(t, rec.Body.String(), "match-1")
	require.Contains(t, rec.Body.String(), "cursor-2")
}
