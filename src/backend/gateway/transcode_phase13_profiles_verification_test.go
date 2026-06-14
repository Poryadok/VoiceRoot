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

	authv1 "voice.app/voice/auth/v1"
	subscriptionv1 "voice.app/voice/subscription/v1"
	userv1 "voice.app/voice/user/v1"
)

type recordingPhase13UserGRPC struct {
	userv1.UnimplementedUserServiceServer
	lastCreate *userv1.CreateProfileRequest
}

func (s *recordingPhase13UserGRPC) CreateProfile(_ context.Context, req *userv1.CreateProfileRequest) (*userv1.CreateProfileResponse, error) {
	s.lastCreate = req
	return &userv1.CreateProfileResponse{
		Profile: &userv1.Profile{
			Id:          "profile-new",
			AccountId:   "account-1",
			DisplayName: req.GetDisplayName(),
			IsPrimary:   false,
		},
	}, nil
}

type recordingPhase13SubscriptionGRPC struct {
	subscriptionv1.UnimplementedSubscriptionServiceServer
	lastLimits *subscriptionv1.GetLimitsRequest
}

func (s *recordingPhase13SubscriptionGRPC) ApplyDowngradeProfiles(_ context.Context, req *subscriptionv1.ApplyDowngradeProfilesRequest) (*subscriptionv1.ApplyDowngradeProfilesResponse, error) {
	return &subscriptionv1.ApplyDowngradeProfilesResponse{KeptProfileIds: req.GetKeptProfileIds()}, nil
}

func (s *recordingPhase13SubscriptionGRPC) GetLimits(_ context.Context, req *subscriptionv1.GetLimitsRequest) (*subscriptionv1.GetLimitsResponse, error) {
	s.lastLimits = req
	return &subscriptionv1.GetLimitsResponse{
		Limits: &subscriptionv1.Limits{LimitsJson: `{"max_profiles":2}`},
	}, nil
}

type recordingPhase13AuthGRPC struct {
	authv1.UnimplementedAuthServiceServer
	lastSwitch *authv1.SwitchActiveProfileRequest
}

func (s *recordingPhase13AuthGRPC) SwitchActiveProfile(_ context.Context, req *authv1.SwitchActiveProfileRequest) (*authv1.SwitchActiveProfileResponse, error) {
	s.lastSwitch = req
	return &authv1.SwitchActiveProfileResponse{
		Session: &authv1.AuthSession{
			AccessToken:       "new-token",
			ProfileId:         req.GetProfileId(),
			AccountId:         "account-1",
			ExpiresInSeconds:  900,
		},
	}, nil
}

func startBufconnAuthConn(t *testing.T, impl authv1.AuthServiceServer) (*grpc.ClientConn, func()) {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	authv1.RegisterAuthServiceServer(srv, impl)
	go func() { _ = srv.Serve(lis) }()
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	return conn, func() {
		_ = conn.Close()
		srv.Stop()
		_ = lis.Close()
	}
}

type recordingPhase13UserVerificationGRPC struct {
	userv1.UnimplementedUserServiceServer
}

func (recordingPhase13UserVerificationGRPC) GetVerificationStatus(context.Context, *userv1.GetVerificationStatusRequest) (*userv1.GetVerificationStatusResponse, error) {
	return &userv1.GetVerificationStatusResponse{
		VerificationStatus: &userv1.VerificationStatus{VerificationType: "none"},
	}, nil
}

func (recordingPhase13UserVerificationGRPC) StartOrganizationVerification(_ context.Context, req *userv1.StartOrganizationVerificationRequest) (*userv1.StartOrganizationVerificationResponse, error) {
	return &userv1.StartOrganizationVerificationResponse{
		Domain:    req.GetDomain(),
		TxtRecord: "voice-verify=test",
	}, nil
}

func newPhase13UsersGateway(t *testing.T, user userv1.UserServiceClient) http.Handler {
	t.Helper()
	return newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{user: user}},
	})
}

func newPhase13SubscriptionGateway(t *testing.T, sub subscriptionv1.SubscriptionServiceClient) http.Handler {
	t.Helper()
	return newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{subscription: sub}},
	})
}

func newPhase13AuthRESTGateway(t *testing.T, auth http.Handler) http.Handler {
	t.Helper()
	return newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		restUpstreams: map[string]http.Handler{"auth": auth},
	})
}

// TestTranscodePhase13_CreateProfile documents POST /api/v1/users/profiles → User.CreateProfile.
func TestTranscodePhase13_CreateProfile(t *testing.T) {
	t.Parallel()

	rec := &recordingPhase13UserGRPC{}
	conn, cleanup := startBufconnUserConn(t, rec)
	t.Cleanup(cleanup)
	h := newPhase13UsersGateway(t, userv1.NewUserServiceClient(conn))

	resp := performRequest(h, http.MethodPost, "/api/v1/users/profiles",
		`{"display_name":"Gaming Alt"}`, map[string]string{
			"Authorization": "Bearer valid-user-token",
		})
	require.Equal(t, http.StatusOK, resp.Code, "body=%s", resp.Body.String())
	require.NotNil(t, rec.lastCreate)
	require.Equal(t, "Gaming Alt", rec.lastCreate.GetDisplayName())
}

// TestTranscodePhase13_SwitchProfile documents POST /api/v1/auth/switch-profile → Auth session with new profile_id.
func TestTranscodePhase13_SwitchProfile(t *testing.T) {
	t.Parallel()

	rec := &recordingPhase13AuthGRPC{}
	conn, cleanup := startBufconnAuthConn(t, rec)
	t.Cleanup(cleanup)
	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{auth: authv1.NewAuthServiceClient(conn)}},
	})

	resp := performRequest(h, http.MethodPost, "/api/v1/auth/switch-profile",
		`{"profile_id":"profile-alt-2"}`, map[string]string{
			"Authorization": "Bearer valid-user-token",
		})
	require.Equal(t, http.StatusOK, resp.Code, "body=%s", resp.Body.String())
	require.NotNil(t, rec.lastSwitch)
	require.Equal(t, "profile-alt-2", rec.lastSwitch.GetProfileId())
}

// TestTranscodePhase13_SubscriptionLimits documents GET /api/v1/subscription/limits → Subscription.GetLimits.
func TestTranscodePhase13_SubscriptionLimits(t *testing.T) {
	t.Parallel()

	rec := &recordingPhase13SubscriptionGRPC{}
	subClient, cleanup := startBufconnSubscriptionClient(t, rec)
	t.Cleanup(cleanup)
	h := newPhase13SubscriptionGateway(t, subClient)

	resp := performRequest(h, http.MethodGet, "/api/v1/subscription/limits", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusOK, resp.Code, "body=%s", resp.Body.String())
	require.NotNil(t, rec.lastLimits)
	require.Equal(t, "account-1", rec.lastLimits.GetAccountId())
}

// TestTranscodePhase13_DowngradeProfiles documents POST /api/v1/subscription/downgrade/profiles.
func TestTranscodePhase13_DowngradeProfiles(t *testing.T) {
	t.Parallel()

	rec := &recordingPhase13SubscriptionGRPC{}
	subClient, cleanup := startBufconnSubscriptionClient(t, rec)
	t.Cleanup(cleanup)
	h := newPhase13SubscriptionGateway(t, subClient)

	resp := performRequest(h, http.MethodPost, "/api/v1/subscription/downgrade/profiles",
		`{"kept_profile_ids":["profile-1","profile-2"]}`, map[string]string{
			"Authorization": "Bearer valid-user-token",
		})
	require.Equal(t, http.StatusOK, resp.Code, "body=%s", resp.Body.String())
}

// TestTranscodePhase13_GetVerification documents GET /api/v1/users/me/verification.
func TestTranscodePhase13_GetVerification(t *testing.T) {
	t.Parallel()

	rec := &recordingPhase13UserVerificationGRPC{}
	conn, cleanup := startBufconnUserConn(t, rec)
	t.Cleanup(cleanup)
	h := newPhase13UsersGateway(t, userv1.NewUserServiceClient(conn))

	resp := performRequest(h, http.MethodGet, "/api/v1/users/me/verification", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusOK, resp.Code, "body=%s", resp.Body.String())
}

// TestTranscodePhase13_OrgDNSVerificationStart documents org DNS verification route wiring.
func TestTranscodePhase13_OrgDNSVerificationStart(t *testing.T) {
	t.Parallel()

	rec := &recordingPhase13UserVerificationGRPC{}
	conn, cleanup := startBufconnUserConn(t, rec)
	t.Cleanup(cleanup)
	h := newPhase13UsersGateway(t, userv1.NewUserServiceClient(conn))

	resp := performRequest(h, http.MethodPost, "/api/v1/users/me/verification/organization",
		`{"domain":"riotgames.com"}`, map[string]string{
			"Authorization": "Bearer valid-user-token",
		})
	require.Equal(t, http.StatusOK, resp.Code, "body=%s", resp.Body.String())
}

// TestTranscodePhase13_LinkedAccountsList documents GET /api/v1/auth/linked-accounts.
func TestTranscodePhase13_LinkedAccountsList(t *testing.T) {
	t.Parallel()

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{}},
	})

	resp := performRequest(h, http.MethodGet, "/api/v1/auth/linked-accounts", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusOK, resp.Code, "body=%s", resp.Body.String())
}

// TestTranscodePhase13_LinkedAccountOAuthStart documents Twitch OAuth link initiation route.
func TestTranscodePhase13_LinkedAccountOAuthStart(t *testing.T) {
	t.Parallel()

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{}},
	})

	resp := performRequest(h, http.MethodPost, "/api/v1/auth/linked-accounts/twitch/link",
		`{"redirect_uri":"https://app.voice.test/oauth/twitch"}`, map[string]string{
			"Authorization": "Bearer valid-user-token",
		})
	require.Equal(t, http.StatusOK, resp.Code, "body=%s", resp.Body.String())
}
