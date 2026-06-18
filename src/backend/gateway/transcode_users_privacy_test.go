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
			ShowOnline:          &userv1.PrivacyAudience{Friends: true, FriendsOfFriends: true, SpaceMembers: true, IncludeGuests: true},
			ShowGameStatus:      &userv1.PrivacyAudience{Friends: true, FriendsOfFriends: true, SpaceMembers: true, IncludeGuests: true},
			ShowMmRating:        &userv1.PrivacyAudience{Friends: true, FriendsOfFriends: true, SpaceMembers: true, IncludeGuests: true},
			ShowPhone:           &userv1.PrivacyAudience{},
			ShowStories:         &userv1.PrivacyAudience{Friends: true, FriendsOfFriends: true, SpaceMembers: true, IncludeGuests: true},
			AllowDm:             &userv1.PrivacyAudience{Friends: true, FriendsOfFriends: true, SpaceMembers: true, IncludeGuests: true},
			AllowFriendRequests: &userv1.PrivacyAudience{Friends: true, FriendsOfFriends: true, SpaceMembers: true, IncludeGuests: true},
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

// TestTranscodeUsersPrivacy_PatchMe documents PATCH /api/v1/users/me/privacy with multiselect audiences.
func TestTranscodeUsersPrivacy_PatchMe(t *testing.T) {
	t.Parallel()
	rec := &recordingUserPrivacy{}
	h := newUserPrivacyContractGateway(t, rec)

	body := `{"settings":{"preset":"personal","allow_dm":{"friends":true},"show_online":{"friends":true},"show_game_status":{"friends":true},"show_mm_rating":{"friends":true},"show_phone":{"friends":false,"friends_of_friends":false,"space_members":false,"include_guests":false},"show_stories":{"friends":true},"allow_friend_requests":{"friends":true,"friends_of_friends":true,"space_members":true,"include_guests":true},"allow_guest_dm":false}}`
	resp := performRequest(h, http.MethodPatch, "/api/v1/users/me/privacy", body, map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusOK, resp.Code)
	require.NotNil(t, rec.lastUpdate)
	require.Equal(t, "profile-1", rec.lastUpdate.GetProfileId())
	require.Equal(t, "personal", rec.lastUpdate.GetSettings().GetPreset())
	require.True(t, rec.lastUpdate.GetSettings().GetAllowDm().GetFriends())
}
