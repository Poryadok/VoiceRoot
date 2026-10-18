package main

import (
	"context"
	"net"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/status"

	userv1 "voice.app/voice/user/v1"
)

type breakerUserGRPC struct {
	userv1.UnimplementedUserServiceServer
	profileCalls  atomic.Int64
	listCalls     atomic.Int64
	mutationCalls atomic.Int64
}

func (s *breakerUserGRPC) GetProfile(context.Context, *userv1.GetProfileRequest) (*userv1.GetProfileResponse, error) {
	s.profileCalls.Add(1)
	return nil, status.Error(codes.Unavailable, "fixture unavailable")
}

func (s *breakerUserGRPC) ListMyProfiles(context.Context, *userv1.ListMyProfilesRequest) (*userv1.ListMyProfilesResponse, error) {
	s.listCalls.Add(1)
	return &userv1.ListMyProfilesResponse{}, nil
}

func (s *breakerUserGRPC) UpdatePresence(context.Context, *userv1.UpdatePresenceRequest) (*userv1.UpdatePresenceResponse, error) {
	s.mutationCalls.Add(1)
	return nil, status.Error(codes.Unavailable, "ambiguous committed mutation")
}

type breakerRetryResolver struct{ address string }

func (b breakerRetryResolver) Scheme() string { return "be040retry" }
func (b breakerRetryResolver) Build(_ resolver.Target, cc resolver.ClientConn, _ resolver.BuildOptions) (resolver.Resolver, error) {
	config := cc.ParseServiceConfig(`{"methodConfig":[{"name":[{"service":"voice.user.v1.UserService","method":"UpdatePresence"}],"retryPolicy":{"MaxAttempts":4,"InitialBackoff":"0.001s","MaxBackoff":"0.001s","BackoffMultiplier":1,"RetryableStatusCodes":["UNAVAILABLE"]}}]}`)
	if err := cc.UpdateState(resolver.State{Addresses: []resolver.Address{{Addr: b.address}}, ServiceConfig: config}); err != nil {
		return nil, err
	}
	return b, nil
}
func (breakerRetryResolver) ResolveNow(resolver.ResolveNowOptions) {}
func (breakerRetryResolver) Close()                                {}

func configuredBreakerGatewayClients(t *testing.T) (*grpcClients, *breakerUserGRPC) {
	t.Helper()
	for _, namespace := range []string{"USERS", "FRIENDS", "CHATS", "MESSAGES", "VOICE", "FILES", "SPACES", "ROLES", "NOTIFICATIONS", "MATCHMAKING", "SEARCH", "MODERATION", "SUBSCRIPTION", "BOTS", "STORIES", "AUTH", "ANALYTICS"} {
		t.Setenv("GATEWAY_"+namespace+"_GRPC_ADDR", "")
	}
	t.Setenv("GATEWAY_GRPC_UPSTREAMS_JSON", "{}")
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := grpc.NewServer()
	backend := &breakerUserGRPC{}
	userv1.RegisterUserServiceServer(server, backend)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { server.Stop(); _ = listener.Close() })
	resolver.Register(breakerRetryResolver{address: listener.Addr().String()})
	t.Setenv("GATEWAY_USERS_GRPC_ADDR", "be040retry:///fixture")
	clients := grpcClientsFromEnv(nil)
	require.NotNil(t, clients)
	require.NotNil(t, clients.user)
	t.Cleanup(clients.close)
	return clients, backend
}

func TestGatewayOrdinaryClientsApplyBreakerToRealHTTPRoute(t *testing.T) {
	clients, backend := configuredBreakerGatewayClients(t)
	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{"breaker-token": {UserID: "account-1", ProfileID: "profile-1"}},
		transcoder:  newTranscoder(clients),
	})
	headers := map[string]string{"Authorization": "Bearer breaker-token"}
	for range 5 {
		response := performRequest(h, http.MethodGet, "/api/v1/users/me", "", headers)
		require.Equal(t, http.StatusServiceUnavailable, response.Code, response.Body.String())
	}
	require.EqualValues(t, 5, backend.profileCalls.Load())
	response := performRequest(h, http.MethodGet, "/api/v1/users/me", "", headers)
	require.Equal(t, http.StatusServiceUnavailable, response.Code, response.Body.String())
	require.EqualValues(t, 5, backend.profileCalls.Load(), "OPEN must reject before the real upstream server")
	response = performRequest(h, http.MethodGet, "/api/v1/users/profiles", "", headers)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	require.EqualValues(t, 1, backend.listCalls.Load(), "healthy method stays usable on the same ClientConn")
}

func TestGatewayOrdinaryClientsDisableConfiguredMutationRetry(t *testing.T) {
	clients, backend := configuredBreakerGatewayClients(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err := clients.user.UpdatePresence(ctx, &userv1.UpdatePresenceRequest{})
	require.Equal(t, codes.Unavailable, status.Code(err))
	require.EqualValues(t, 1, backend.mutationCalls.Load(), "service-config retry must not replay a received mutation")
}
