package main

import (
	"context"
	"net"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	userv1 "voice.app/voice/user/v1"
)

type recordingUserPrivacy struct {
	userv1.UnimplementedUserServiceServer
	lastGet    *userv1.GetPrivacySettingsRequest
	lastUpdate *userv1.UpdatePrivacySettingsRequest
}

func (s *recordingUserPrivacy) GetPrivacySettings(_ context.Context, req *userv1.GetPrivacySettingsRequest) (*userv1.GetPrivacySettingsResponse, error) {
	s.lastGet = req
	return &userv1.GetPrivacySettingsResponse{
		PrivacySettings: &userv1.PrivacySettings{
			ProfileId:           req.GetProfileId(),
			Preset:              "gaming",
			ShowOnline:          "everyone",
			ShowGameStatus:      "everyone",
			ShowMmRating:        "everyone",
			ShowPhone:           "nobody",
			ShowStories:         "everyone",
			AllowDm:             "everyone",
			AllowFriendRequests: "everyone",
		},
	}, nil
}

func (s *recordingUserPrivacy) UpdatePrivacySettings(_ context.Context, req *userv1.UpdatePrivacySettingsRequest) (*userv1.UpdatePrivacySettingsResponse, error) {
	s.lastUpdate = req
	return &userv1.UpdatePrivacySettingsResponse{PrivacySettings: req.GetSettings()}, nil
}

func startBufconnUserPrivacyClient(t *testing.T, impl userv1.UserServiceServer) (userv1.UserServiceClient, func()) {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	userv1.RegisterUserServiceServer(srv, impl)
	go func() { _ = srv.Serve(lis) }()
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	return userv1.NewUserServiceClient(conn), func() {
		_ = conn.Close()
		srv.Stop()
		_ = lis.Close()
	}
}

func newUserPrivacyContractGateway(t *testing.T, rec *recordingUserPrivacy) http.Handler {
	t.Helper()
	userClient, cleanup := startBufconnUserPrivacyClient(t, rec)
	t.Cleanup(cleanup)
	return newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{user: userClient}},
	})
}

// TestTranscodeUsersPrivacy_GetMe documents GET /api/v1/users/me/privacy.
func TestTranscodeUsersPrivacy_GetMe(t *testing.T) {
	t.Parallel()
	rec := &recordingUserPrivacy{}
	h := newUserPrivacyContractGateway(t, rec)

	resp := performRequest(h, http.MethodGet, "/api/v1/users/me/privacy", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusOK, resp.Code)
	require.NotNil(t, rec.lastGet)
	require.Equal(t, "profile-1", rec.lastGet.GetProfileId())
}

// TestTranscodeUsersPrivacy_PatchMe documents PATCH /api/v1/users/me/privacy with preset + allow_dm.
func TestTranscodeUsersPrivacy_PatchMe(t *testing.T) {
	t.Parallel()
	rec := &recordingUserPrivacy{}
	h := newUserPrivacyContractGateway(t, rec)

	body := `{"settings":{"preset":"personal","allow_dm":"friends","show_online":"friends","show_game_status":"friends","show_mm_rating":"friends","show_phone":"nobody","show_stories":"friends","allow_friend_requests":"everyone","allow_guest_dm":false}}`
	resp := performRequest(h, http.MethodPatch, "/api/v1/users/me/privacy", body, map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusOK, resp.Code)
	require.NotNil(t, rec.lastUpdate)
	require.Equal(t, "profile-1", rec.lastUpdate.GetProfileId())
	require.Equal(t, "personal", rec.lastUpdate.GetSettings().GetPreset())
	require.Equal(t, "friends", rec.lastUpdate.GetSettings().GetAllowDm())
}
