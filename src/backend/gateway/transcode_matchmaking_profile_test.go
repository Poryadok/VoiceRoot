package main

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	matchmakingv1 "voice.app/voice/matchmaking/v1"
)

type recordingMatchmakingProfileGRPC struct {
	matchmakingv1.UnimplementedMatchmakingServiceServer
	lastGetMy      *matchmakingv1.GetMyPlayerProfileRequest
	lastGetProfile *matchmakingv1.GetPlayerProfileRequest
	lastUpsert     *matchmakingv1.UpsertPlayerGameEntryRequest
	lastDelete     *matchmakingv1.DeletePlayerGameEntryRequest
}

func (s *recordingMatchmakingProfileGRPC) GetMyPlayerProfile(_ context.Context, req *matchmakingv1.GetMyPlayerProfileRequest) (*matchmakingv1.GetMyPlayerProfileResponse, error) {
	s.lastGetMy = req
	return &matchmakingv1.GetMyPlayerProfileResponse{
		Entries: []*matchmakingv1.PlayerGameEntry{{
			GameId: "game-1",
			Region: "eu",
		}},
	}, nil
}

func (s *recordingMatchmakingProfileGRPC) GetPlayerProfile(_ context.Context, req *matchmakingv1.GetPlayerProfileRequest) (*matchmakingv1.GetPlayerProfileResponse, error) {
	s.lastGetProfile = req
	return &matchmakingv1.GetPlayerProfileResponse{}, nil
}

func (s *recordingMatchmakingProfileGRPC) UpsertPlayerGameEntry(_ context.Context, req *matchmakingv1.UpsertPlayerGameEntryRequest) (*matchmakingv1.UpsertPlayerGameEntryResponse, error) {
	s.lastUpsert = req
	role := req.GetRole()
	return &matchmakingv1.UpsertPlayerGameEntryResponse{
		Entry: &matchmakingv1.PlayerGameEntry{
			GameId: req.GetGameId(),
			Region: req.GetRegion(),
			Role:   &role,
		},
	}, nil
}

func (s *recordingMatchmakingProfileGRPC) DeletePlayerGameEntry(_ context.Context, req *matchmakingv1.DeletePlayerGameEntryRequest) (*matchmakingv1.DeletePlayerGameEntryResponse, error) {
	s.lastDelete = req
	return &matchmakingv1.DeletePlayerGameEntryResponse{}, nil
}

func TestTranscodeMatchmakingGetMyProfile(t *testing.T) {
	t.Parallel()
	grpcRec := &recordingMatchmakingProfileGRPC{}
	conn, cleanup := startBufconnMatchmakingConn(t, grpcRec)
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{matchmaking: matchmakingv1.NewMatchmakingServiceClient(conn)}},
	})
	rec := performRequest(h, http.MethodGet, "/api/v1/matchmaking/profile/me", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, grpcRec.lastGetMy)
}

func TestTranscodeMatchmakingGetPlayerProfile(t *testing.T) {
	t.Parallel()
	grpcRec := &recordingMatchmakingProfileGRPC{}
	conn, cleanup := startBufconnMatchmakingConn(t, grpcRec)
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{matchmaking: matchmakingv1.NewMatchmakingServiceClient(conn)}},
	})
	rec := performRequest(h, http.MethodGet, "/api/v1/matchmaking/profile/other-profile-id", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "other-profile-id", grpcRec.lastGetProfile.GetProfileId())
}

func TestTranscodeMatchmakingUpsertPlayerGameEntry(t *testing.T) {
	t.Parallel()
	grpcRec := &recordingMatchmakingProfileGRPC{}
	conn, cleanup := startBufconnMatchmakingConn(t, grpcRec)
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{matchmaking: matchmakingv1.NewMatchmakingServiceClient(conn)}},
	})
	body := `{"region":"eu","role":"Carry"}`
	rec := performRequest(h, http.MethodPut, "/api/v1/matchmaking/profile/games/game-1", body, map[string]string{
		"Authorization": "Bearer valid-user-token",
		"Content-Type":  "application/json",
	})
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "game-1", grpcRec.lastUpsert.GetGameId())
	require.Equal(t, "eu", grpcRec.lastUpsert.GetRegion())
}

func TestTranscodeMatchmakingDeletePlayerGameEntry(t *testing.T) {
	t.Parallel()
	grpcRec := &recordingMatchmakingProfileGRPC{}
	conn, cleanup := startBufconnMatchmakingConn(t, grpcRec)
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{matchmaking: matchmakingv1.NewMatchmakingServiceClient(conn)}},
	})
	rec := performRequest(h, http.MethodDelete, "/api/v1/matchmaking/profile/games/game-1", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "game-1", grpcRec.lastDelete.GetGameId())
}
