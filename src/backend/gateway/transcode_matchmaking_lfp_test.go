package main

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	matchmakingv1 "voice.app/voice/matchmaking/v1"
)

type recordingDecideLfpGRPC struct {
	matchmakingv1.UnimplementedMatchmakingServiceServer
	last *matchmakingv1.DecideLfpRequestRequest
}

func (s *recordingDecideLfpGRPC) DecideLfpRequest(_ context.Context, req *matchmakingv1.DecideLfpRequestRequest) (*matchmakingv1.DecideLfpRequestResponse, error) {
	s.last = req
	partyID := uuid.NewString()
	return &matchmakingv1.DecideLfpRequestResponse{
		Status:  "accepted",
		PartyId: &partyID,
	}, nil
}

func TestTranscodeMatchmakingDecideLfpRequest(t *testing.T) {
	t.Parallel()
	grpcRec := &recordingDecideLfpGRPC{}
	conn, cleanup := startBufconnMatchmakingConn(t, grpcRec)
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{matchmaking: matchmakingv1.NewMatchmakingServiceClient(conn)}},
	})
	storyID := uuid.NewString()
	responderID := uuid.NewString()
	body := `{"story_id":"` + storyID + `","responder_profile_id":"` + responderID + `","response_type":"JOIN","decision":"ACCEPT"}`
	rec := performRequest(h, http.MethodPost, "/api/v1/matchmaking/lfp-requests/decide", body, map[string]string{
		"Authorization": "Bearer valid-user-token",
		"Content-Type":  "application/json",
	})
	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, grpcRec.last)
	require.Equal(t, storyID, grpcRec.last.GetStoryId())
	require.Equal(t, responderID, grpcRec.last.GetResponderProfileId())
	require.Equal(t, "JOIN", grpcRec.last.GetResponseType())
	require.Equal(t, "ACCEPT", grpcRec.last.GetDecision())
	require.Contains(t, rec.Body.String(), "accepted")
}
