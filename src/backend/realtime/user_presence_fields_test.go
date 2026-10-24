package main

import (
	"context"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"net"
	"testing"
	userv1 "voice.app/voice/user/v1"
)

type presenceFieldsServer struct {
	userv1.UnimplementedUserServiceServer
	requests chan *userv1.UpdatePresenceRequest
}

func (s *presenceFieldsServer) UpdatePresence(_ context.Context, r *userv1.UpdatePresenceRequest) (*userv1.UpdatePresenceResponse, error) {
	s.requests <- r
	return &userv1.UpdatePresenceResponse{}, nil
}

func TestPresenceUpdaterPreservesExplicitClearAndOmission(t *testing.T) {
	lis := bufconn.Listen(1024 * 1024)
	t.Cleanup(func() { _ = lis.Close() })
	fixture := &presenceFieldsServer{requests: make(chan *userv1.UpdatePresenceRequest, 2)}
	server := grpc.NewServer()
	userv1.RegisterUserServiceServer(server, fixture)
	go func() { _ = server.Serve(lis) }()
	t.Cleanup(server.Stop)
	conn, err := grpc.NewClient("passthrough:///fields", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	updater := newGRPCPresenceUpdater(conn)
	require.NoError(t, updater.UpdatePresenceFields(context.Background(), boundaryAccount, boundaryProfile, "online", nil))
	require.Nil(t, (<-fixture.requests).CustomStatus)
	empty := ""
	require.NoError(t, updater.UpdatePresenceFields(context.Background(), boundaryAccount, boundaryProfile, "idle", &empty))
	req := <-fixture.requests
	require.NotNil(t, req.CustomStatus)
	require.Empty(t, *req.CustomStatus)
}
