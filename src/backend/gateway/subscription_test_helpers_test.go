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

	subscriptionv1 "voice.app/voice/subscription/v1"
)

type recordingSubscriptionBackend struct {
	subscriptionv1.UnimplementedSubscriptionServiceServer
	lastGetSubscription *subscriptionv1.GetSubscriptionRequest
}

func (s *recordingSubscriptionBackend) GetSubscription(_ context.Context, req *subscriptionv1.GetSubscriptionRequest) (*subscriptionv1.GetSubscriptionResponse, error) {
	s.lastGetSubscription = req
	return &subscriptionv1.GetSubscriptionResponse{
		Subscription: &subscriptionv1.Subscription{
			AccountId: req.GetAccountId(),
			Plan:      "premium",
			Status:    "active",
		},
	}, nil
}

func startBufconnSubscriptionClient(t *testing.T, impl subscriptionv1.SubscriptionServiceServer) (subscriptionv1.SubscriptionServiceClient, func()) {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	subscriptionv1.RegisterSubscriptionServiceServer(srv, impl)
	go func() { _ = srv.Serve(lis) }()
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	return subscriptionv1.NewSubscriptionServiceClient(conn), func() {
		_ = conn.Close()
		srv.Stop()
		_ = lis.Close()
	}
}

func newSubscriptionContractGateway(t *testing.T, rec *recordingSubscriptionBackend) http.Handler {
	t.Helper()
	subClient, cleanup := startBufconnSubscriptionClient(t, rec)
	t.Cleanup(cleanup)
	return newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: newTranscoderWithSubscription(subClient),
		restUpstreams: map[string]http.Handler{
			"subscription": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNotImplemented)
			}),
		},
	})
}

// newTranscoderWithSubscription extends grpcClients once transcode.go wires subscription namespace.
func newTranscoderWithSubscription(client subscriptionv1.SubscriptionServiceClient) *transcoder {
	clients := grpcClients{}
	setSubscriptionClient(&clients, client)
	return &transcoder{clients: clients}
}

func setSubscriptionClient(clients *grpcClients, client subscriptionv1.SubscriptionServiceClient) {
	clients.subscription = client
}
