package grpcsvc

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	matchmakingv1 "voice.app/voice/matchmaking/v1"
)

type nativeRouteSpaceProbe struct{ calls int }

func (p *nativeRouteSpaceProbe) EnsureMemberAndMMEnabled(context.Context, uuid.UUID) error {
	p.calls++
	return nil
}

func TestNativeFreshRoutesDenyUnqualifiedMethodsBeforeDependencies(t *testing.T) {
	probe := &nativeRouteSpaceProbe{}
	legacy := MatchmakingGRPC{SpaceQueue: probe}
	s := NewNativeRouteHandler(legacy)
	for _, space := range []string{"invalid", uuid.NewString()} {
		response, err := s.StartSpaceQueue(context.Background(), &matchmakingv1.StartSpaceQueueRequest{SpaceId: space})
		require.Nil(t, response)
		require.Equal(t, codes.Unavailable, status.Code(err))
	}
	require.Zero(t, probe.calls, "native denial precedes even membership lookup")
	for _, decision := range []string{"ACCEPT", "DECLINE", "invalid"} {
		response, err := s.DecideLfpRequest(context.Background(), &matchmakingv1.DecideLfpRequestRequest{Decision: decision})
		require.Nil(t, response)
		require.Equal(t, codes.Unavailable, status.Code(err))
	}
	_, err := legacy.StartSpaceQueue(context.Background(), &matchmakingv1.StartSpaceQueueRequest{SpaceId: "invalid"})
	require.Equal(t, codes.InvalidArgument, status.Code(err), "native construction does not mutate predecessor policy")
	_, err = s.StartSearch(context.Background(), &matchmakingv1.StartSearchRequest{})
	require.Equal(t, codes.Unauthenticated, status.Code(err), "native route policy cannot mint a protected subject")
}

func TestNativeFreshRoutesGRPCDispatchUsesSameDenial(t *testing.T) {
	probe := &nativeRouteSpaceProbe{}
	service := NewNativeRouteHandler(MatchmakingGRPC{SpaceQueue: probe})
	listener := bufconn.Listen(1 << 20)
	server := grpc.NewServer()
	matchmakingv1.RegisterMatchmakingServiceServer(server, service)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { server.Stop(); _ = listener.Close() })
	conn, err := grpc.NewClient("passthrough:///native-route-fixture", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	client := matchmakingv1.NewMatchmakingServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	response, err := client.StartSpaceQueue(ctx, &matchmakingv1.StartSpaceQueueRequest{SpaceId: uuid.NewString()})
	require.Nil(t, response)
	require.Equal(t, codes.Unavailable, status.Code(err))
	lfp, err := client.DecideLfpRequest(ctx, &matchmakingv1.DecideLfpRequestRequest{Decision: "ACCEPT"})
	require.Nil(t, lfp)
	require.Equal(t, codes.Unavailable, status.Code(err))
	require.Zero(t, probe.calls)
}
