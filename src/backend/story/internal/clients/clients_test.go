package clients

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/status"

	chatv1 "voice.app/voice/chat/v1"
	filev1 "voice.app/voice/file/v1"
	"voice/backend/pkg/grpcclient"
	"voice/backend/story/internal/grpcsvc"
)

func TestIsIdempotentDeleteResult_acceptsFileNotFoundAfterRetry(t *testing.T) {
	require.True(t, isIdempotentDeleteResult(status.Error(codes.NotFound, "file not found")))
	require.False(t, isIdempotentDeleteResult(status.Error(codes.Internal, "storage unavailable")))
	require.False(t, isIdempotentDeleteResult(errors.New("transport response lost")))
}

// Exercise the production constructor and generated clients over a real transport.
func TestWireGRPCBreakersIsolateChatAndFileConnections(t *testing.T) {
	chat := &breakerChatServer{err: status.Error(codes.Unavailable, "chat unavailable")}
	file := &breakerFileServer{err: status.Error(codes.Unavailable, "file unavailable")}
	addr := startBreakerUpstream(t, chat, file)
	configureStoryUpstreams(t, addr)
	svc := &grpcsvc.StoryGRPC{}
	deleter := WireGRPC(nil, svc)
	require.NotNil(t, svc.Chat)
	require.NotNil(t, deleter)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	threshold := grpcclient.DefaultBreakerConfig().FailureThreshold
	for i := 0; i < threshold; i++ {
		_, err := svc.Chat.CreateDM(ctx, &chatv1.CreateDMRequest{})
		require.Equal(t, "chat unavailable", status.Convert(err).Message())
	}
	_, err := svc.Chat.CreateDM(ctx, &chatv1.CreateDMRequest{})
	require.Equal(t, codes.Unavailable, status.Code(err))
	assert.Equal(t, int32(threshold), chat.calls.Load(), "OPEN must reject without upstream admission")

	// File is a separate connection, even though both dependencies resolve to one server.
	for i := 0; i < threshold; i++ {
		require.Equal(t, "file unavailable", status.Convert(deleter.DeleteFile(ctx, "media")).Message())
	}
	require.Equal(t, codes.Unavailable, status.Code(deleter.DeleteFile(ctx, "media")))
	assert.Equal(t, int32(threshold), file.calls.Load(), "OPEN must reject without upstream admission")

	// Reconstructing clients starts a fresh connection budget rather than a process-global circuit.
	fresh := &grpcsvc.StoryGRPC{}
	freshDeleter := WireGRPC(nil, fresh)
	_, err = fresh.Chat.CreateDM(ctx, &chatv1.CreateDMRequest{})
	require.Equal(t, "chat unavailable", status.Convert(err).Message())
	require.Equal(t, "file unavailable", status.Convert(freshDeleter.DeleteFile(ctx, "media")).Message())
	require.Equal(t, int32(threshold+1), chat.calls.Load())
	require.Equal(t, int32(threshold+1), file.calls.Load())
}

func TestWireGRPCFileNotFoundRemainsIdempotentAndDoesNotOpen(t *testing.T) {
	file := &breakerFileServer{err: status.Error(codes.NotFound, "already deleted")}
	configureStoryUpstreams(t, startBreakerUpstream(t, &breakerChatServer{}, file))
	deleter := WireGRPC(nil, &grpcsvc.StoryGRPC{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	n := grpcclient.DefaultBreakerConfig().FailureThreshold + 2
	for i := 0; i < n; i++ {
		require.NoError(t, deleter.DeleteFile(ctx, "media"))
	}
	require.Equal(t, int32(n), file.calls.Load(), "outbox retries must still reach File after NotFound")
}

func TestWireGRPCDisablesConfiguredMutationRetries(t *testing.T) {
	chat := &breakerChatServer{err: status.Error(codes.Unavailable, "ambiguous chat commit")}
	file := &breakerFileServer{err: status.Error(codes.Unavailable, "ambiguous file commit")}
	addr := startBreakerUpstream(t, chat, file)
	// Supply an actual resolver retry policy: a single application call must not replay it.
	builder := &breakerResolver{addr: addr}
	resolver.Register(builder)
	configureStoryUpstreams(t, builder.Scheme()+":///upstream")
	svc := &grpcsvc.StoryGRPC{}
	deleter := WireGRPC(nil, svc)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := svc.Chat.CreateDM(ctx, &chatv1.CreateDMRequest{})
	require.Equal(t, codes.Unavailable, status.Code(err))
	require.Equal(t, codes.Unavailable, status.Code(deleter.DeleteFile(ctx, "media")))
	assert.Equal(t, int32(1), chat.calls.Load(), "CreateDM must not replay ambiguous outcomes")
	assert.Equal(t, int32(1), file.calls.Load(), "DeleteFile retry belongs to durable outbox")
}

type breakerChatServer struct {
	chatv1.UnimplementedChatServiceServer
	calls atomic.Int32
	err   error
}

func (s *breakerChatServer) CreateDM(context.Context, *chatv1.CreateDMRequest) (*chatv1.CreateDMResponse, error) {
	s.calls.Add(1)
	return &chatv1.CreateDMResponse{}, s.err
}

type breakerFileServer struct {
	filev1.UnimplementedFileServiceServer
	calls atomic.Int32
	err   error
}

func (s *breakerFileServer) DeleteFile(context.Context, *filev1.DeleteFileRequest) (*filev1.DeleteFileResponse, error) {
	s.calls.Add(1)
	return &filev1.DeleteFileResponse{}, s.err
}

func startBreakerUpstream(t *testing.T, chat *breakerChatServer, file *breakerFileServer) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := grpc.NewServer()
	chatv1.RegisterChatServiceServer(server, chat)
	filev1.RegisterFileServiceServer(server, file)
	go func() { _ = server.Serve(lis) }()
	t.Cleanup(server.Stop)
	return lis.Addr().String()
}

func configureStoryUpstreams(t *testing.T, addr string) {
	t.Helper()
	for _, name := range []string{"NATS_URL", "MESSAGING_GRPC_ADDR", "USER_GRPC_ADDR", "SUBSCRIPTION_GRPC_ADDR"} {
		t.Setenv(name, "")
	}
	t.Setenv("CHAT_GRPC_ADDR", addr)
	t.Setenv("FILE_GRPC_ADDR", addr)
}

type breakerResolver struct{ addr string }

func (*breakerResolver) Scheme() string { return "story-breaker-test" }
func (b *breakerResolver) Build(_ resolver.Target, cc resolver.ClientConn, _ resolver.BuildOptions) (resolver.Resolver, error) {
	config := cc.ParseServiceConfig(`{"methodConfig":[{"name":[{}],"retryPolicy":{"MaxAttempts":3,"InitialBackoff":"0.001s","MaxBackoff":"0.001s","BackoffMultiplier":1,"RetryableStatusCodes":["UNAVAILABLE"]}}]}`)
	err := cc.UpdateState(resolver.State{Addresses: []resolver.Address{{Addr: b.addr}}, ServiceConfig: config})
	return b, err
}
func (*breakerResolver) ResolveNow(resolver.ResolveNowOptions) {}
func (*breakerResolver) Close()                                {}
