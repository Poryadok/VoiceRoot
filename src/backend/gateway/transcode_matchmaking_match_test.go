package main

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	matchmakingv1 "voice.app/voice/matchmaking/v1"
)

type recordingMatchmakingMatchGRPC struct {
	recordingMatchmakingSearchGRPC
	lastGet     *matchmakingv1.GetMatchRequest
	lastRespond *matchmakingv1.RespondToMatchRequest
}

func (s *recordingMatchmakingMatchGRPC) GetMatch(_ context.Context, req *matchmakingv1.GetMatchRequest) (*matchmakingv1.GetMatchResponse, error) {
	s.lastGet = req
	return &matchmakingv1.GetMatchResponse{
		Match: &matchmakingv1.Match{
			Id:         req.GetMatchId(),
			GameId:     "game-1",
			Mode:       "Duo",
			Region:     "eu",
			Status:     "pending_accept",
			ProfileIds: []string{"profile-1", "profile-2"},
			CreatedAt:  timestamppb.Now(),
		},
	}, nil
}

func (s *recordingMatchmakingMatchGRPC) RespondToMatch(_ context.Context, req *matchmakingv1.RespondToMatchRequest) (*matchmakingv1.RespondToMatchResponse, error) {
	s.lastRespond = req
	status := "pending_accept"
	sessionStatus := "pending_accept"
	if req.GetAccept() {
		sessionStatus = "matched"
		status = "active"
	}
	return &matchmakingv1.RespondToMatchResponse{
		Match: &matchmakingv1.Match{
			Id:           req.GetMatchId(),
			GameId:       "game-1",
			Mode:         "Duo",
			Region:       "eu",
			Status:       status,
			VoiceRoomId:  matchProtoStr("voice-1"),
			ChatId:       matchProtoStr("chat-1"),
			ProfileIds:   []string{"profile-1", "profile-2"},
			CreatedAt:    timestamppb.Now(),
		},
		SearchSession: &matchmakingv1.SearchSession{
			Id:        "session-1",
			ProfileId: "profile-1",
			GameId:    "game-1",
			Mode:      "Duo",
			Status:    sessionStatus,
			MatchId:   matchProtoStr(req.GetMatchId()),
		},
	}, nil
}

func matchProtoStr(s string) *string { return &s }

func TestTranscodeMatchmakingGetMatch(t *testing.T) {
	t.Parallel()
	grpcRec := &recordingMatchmakingMatchGRPC{}
	conn, cleanup := startBufconnMatchmakingConn(t, grpcRec)
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{matchmaking: matchmakingv1.NewMatchmakingServiceClient(conn)}},
	})
	rec := performRequest(h, http.MethodGet, "/api/v1/matchmaking/matches/match-1", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, grpcRec.lastGet)
	require.Equal(t, "match-1", grpcRec.lastGet.GetMatchId())
	require.Contains(t, rec.Body.String(), "pending_accept")
}

func TestTranscodeMatchmakingRespondToMatch(t *testing.T) {
	t.Parallel()
	grpcRec := &recordingMatchmakingMatchGRPC{}
	conn, cleanup := startBufconnMatchmakingConn(t, grpcRec)
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{matchmaking: matchmakingv1.NewMatchmakingServiceClient(conn)}},
	})
	body := `{"accept":true}`
	rec := performRequest(h, http.MethodPost, "/api/v1/matchmaking/matches/match-1/respond", body, map[string]string{
		"Authorization": "Bearer valid-user-token",
		"Content-Type":  "application/json",
	})
	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, grpcRec.lastRespond)
	require.Equal(t, "match-1", grpcRec.lastRespond.GetMatchId())
	require.True(t, grpcRec.lastRespond.GetAccept())
	require.Contains(t, rec.Body.String(), "active")
}

func TestTranscodeMatchmakingRespondDecline(t *testing.T) {
	t.Parallel()
	grpcRec := &recordingMatchmakingMatchGRPC{}
	conn, cleanup := startBufconnMatchmakingConn(t, grpcRec)
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{matchmaking: matchmakingv1.NewMatchmakingServiceClient(conn)}},
	})
	body := `{"accept":false}`
	rec := performRequest(h, http.MethodPost, "/api/v1/matchmaking/matches/match-1/respond", body, map[string]string{
		"Authorization": "Bearer valid-user-token",
		"Content-Type":  "application/json",
	})
	require.Equal(t, http.StatusOK, rec.Code)
	require.False(t, grpcRec.lastRespond.GetAccept())
}
