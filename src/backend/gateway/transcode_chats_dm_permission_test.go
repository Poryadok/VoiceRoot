package main

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	chatv1 "voice.app/voice/chat/v1"
)

type recordingCanCreateDM struct {
	chatv1.UnimplementedChatServiceServer
	profileID string
	allowed   bool
	err       error
}

func (s *recordingCanCreateDM) CanCreateDM(_ context.Context, req *chatv1.CanCreateDMRequest) (*chatv1.CanCreateDMResponse, error) {
	s.profileID = req.GetOtherProfileId()
	if s.err != nil {
		return nil, s.err
	}
	return &chatv1.CanCreateDMResponse{Allowed: s.allowed}, nil
}

func TestTranscodeChatsCanCreateDMReturnsOnlyEffectivePermission(t *testing.T) {
	for _, allowed := range []bool{true, false} {
		t.Run(map[bool]string{true: "allowed", false: "denied"}[allowed], func(t *testing.T) {
			service := &recordingCanCreateDM{allowed: allowed}
			conn, cleanup := startBufconnChatConn(t, service)
			t.Cleanup(cleanup)
			h := newGatewayForContract(t, gatewayTestOptions{
				tokenClaims: map[string]tokenClaims{
					"viewer-token": {UserID: "viewer-account", ProfileID: "viewer-profile"},
				},
				transcoder: &transcoder{clients: grpcClients{chat: chatv1.NewChatServiceClient(conn)}},
			})

			resp := performRequest(h, http.MethodGet, "/api/v1/chats/dm-permission/selected-profile", "", map[string]string{
				"Authorization": "Bearer viewer-token",
			})
			require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
			require.JSONEq(t, map[bool]string{true: `{"allowed":true}`, false: `{"allowed":false}`}[allowed], resp.Body.String())
			require.Equal(t, "selected-profile", service.profileID)
		})
	}
}

func TestTranscodeChatsCanCreateDMPreservesTypedAuthorizationFailure(t *testing.T) {
	service := &recordingCanCreateDM{err: status.Error(codes.Unavailable, "authorization dependency unavailable")}
	conn, cleanup := startBufconnChatConn(t, service)
	t.Cleanup(cleanup)
	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"viewer-token": {UserID: "viewer-account", ProfileID: "viewer-profile"},
		},
		transcoder: &transcoder{clients: grpcClients{chat: chatv1.NewChatServiceClient(conn)}},
	})

	resp := performRequest(h, http.MethodGet, "/api/v1/chats/dm-permission/selected-profile", "", map[string]string{
		"Authorization": "Bearer viewer-token",
	})
	require.Equal(t, http.StatusServiceUnavailable, resp.Code, resp.Body.String())
}
