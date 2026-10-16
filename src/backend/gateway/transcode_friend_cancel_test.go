package main

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	socialv1 "voice.app/voice/social/v1"
)

type recordingFriendCancel struct {
	socialv1.UnimplementedSocialServiceServer
	target  string
	profile []string
	code    codes.Code
}

func (s *recordingFriendCancel) CancelFriendInvitation(ctx context.Context, req *socialv1.CancelFriendInvitationRequest) (*socialv1.CancelFriendInvitationResponse, error) {
	s.target = req.GetTargetProfileId()
	md, _ := metadata.FromIncomingContext(ctx)
	s.profile = md.Get("x-voice-profile-id")
	if s.code != codes.OK {
		return nil, status.Error(s.code, "pending friend request not found")
	}
	return &socialv1.CancelFriendInvitationResponse{}, nil
}

func TestTranscodeCancelOwnPendingFriendRequest(t *testing.T) {
	svc := &recordingFriendCancel{}
	conn, cleanup := startBufconnSocialConn(t, svc)
	t.Cleanup(cleanup)
	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{"sender-token": {UserID: "account", ProfileID: "sender"}},
		transcoder:  &transcoder{clients: grpcClients{social: socialv1.NewSocialServiceClient(conn)}},
	})
	path := "/api/v1/friends/invitations/target/cancel"
	unauthorized := performRequest(h, http.MethodPost, path, "{}", nil)
	require.Equal(t, http.StatusUnauthorized, unauthorized.Code)
	require.Empty(t, svc.target)
	auth := map[string]string{"Authorization": "Bearer sender-token", "X-Voice-Profile-ID": "forged-sender"}
	response := performRequest(h, http.MethodPost, path, `{"target_profile_id":"forged","requester_profile_id":"other"}`, auth)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	require.Equal(t, "target", svc.target)
	require.Equal(t, []string{"sender"}, svc.profile)
	svc.code = codes.NotFound
	response = performRequest(h, http.MethodPost, path, "{}", auth)
	require.Equal(t, http.StatusNotFound, response.Code)
}
