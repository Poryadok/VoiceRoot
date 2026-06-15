package grpcsvc

import (
	"context"
	"net"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	messagingv1 "voice.app/voice/messaging/v1"
)

type stubPreKeyGateMessaging struct {
	messagingv1.UnimplementedMessagingServiceServer
	bundles map[string]string
}

func (s *stubPreKeyGateMessaging) GetPreKeyBundle(_ context.Context, req *messagingv1.GetPreKeyBundleRequest) (*messagingv1.GetPreKeyBundleResponse, error) {
	bundle, ok := s.bundles[req.GetProfileId()]
	if !ok || bundle == "" {
		return nil, status.Error(codes.NotFound, "pre-key bundle not found")
	}
	return &messagingv1.GetPreKeyBundleResponse{Bundle: bundle}, nil
}

func startStubPreKeyMessagingClient(t *testing.T, srv messagingv1.MessagingServiceServer) messagingv1.MessagingServiceClient {
	t.Helper()
	const bufSize = 1 << 20
	lis := bufconn.Listen(bufSize)
	grpcSrv := grpc.NewServer()
	messagingv1.RegisterMessagingServiceServer(grpcSrv, srv)
	go func() { _ = grpcSrv.Serve(lis) }()
	t.Cleanup(grpcSrv.Stop)
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return messagingv1.NewMessagingServiceClient(conn)
}

func TestMessagingE2EPreKeyGate_EnsureAllMembersHavePreKeys(t *testing.T) {
	profWithBundle := uuid.New()
	stub := &stubPreKeyGateMessaging{bundles: map[string]string{
		profWithBundle.String(): "bundle-a",
	}}
	client := startStubPreKeyMessagingClient(t, stub)
	gate := NewMessagingE2EPreKeyGate(client)

	missing := uuid.New()
	err := gate.EnsureAllMembersHavePreKeys(context.Background(), []uuid.UUID{missing})
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))

	require.NoError(t, gate.EnsureAllMembersHavePreKeys(context.Background(), []uuid.UUID{profWithBundle}))
}
