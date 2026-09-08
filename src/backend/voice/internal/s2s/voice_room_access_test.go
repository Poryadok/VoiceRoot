package s2s_test

import (
	"context"
	"net"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"

	"voice/backend/voice/internal/grpcsvc"
	"voice/backend/voice/internal/s2s"

	spacev1 "voice.app/voice/space/v1"
)

const voiceRoomAccessBufSize = 1024 * 1024

type recordingSpaceResolverServer struct {
	spacev1.UnimplementedSpaceServiceServer
	request       *spacev1.ResolveVoiceRoomAccessRequest
	metadata      metadata.MD
	result        *spacev1.ResolveVoiceRoomAccessResponse
	resolverError error
	calls         int
}

func (s *recordingSpaceResolverServer) ResolveVoiceRoomAccess(ctx context.Context, req *spacev1.ResolveVoiceRoomAccessRequest) (*spacev1.ResolveVoiceRoomAccessResponse, error) {
	s.calls++
	s.request = req
	s.metadata, _ = metadata.FromIncomingContext(ctx)
	if s.resolverError != nil {
		return nil, s.resolverError
	}
	return s.result, nil
}

// spaceResolverAuthInterceptor mirrors Space authctx's resolver-only boundary:
// exactly one dedicated bearer credential creates the trusted service identity.
// It intentionally does not accept profile or other forwarded user metadata.
func spaceResolverAuthInterceptor(expectedToken string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if info.FullMethod != spacev1.SpaceService_ResolveVoiceRoomAccess_FullMethodName {
			return handler(ctx, req)
		}
		md, _ := metadata.FromIncomingContext(ctx)
		if strings.TrimSpace(expectedToken) == "" || len(md.Get("authorization")) != 1 || md.Get("authorization")[0] != "Bearer "+strings.TrimSpace(expectedToken) {
			return nil, status.Error(codes.Unauthenticated, "verified Voice service identity required")
		}
		return handler(ctx, req)
	}
}

func startVoiceResolverServer(t *testing.T, expectedToken string, spaceServer *recordingSpaceResolverServer) spacev1.SpaceServiceClient {
	t.Helper()
	listener := bufconn.Listen(voiceRoomAccessBufSize)
	server := grpc.NewServer(grpc.UnaryInterceptor(spaceResolverAuthInterceptor(expectedToken)))
	spacev1.RegisterSpaceServiceServer(server, spaceServer)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)

	conn, err := grpc.NewClient("passthrough:///space-resolver", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })
	return spacev1.NewSpaceServiceClient(conn)
}

// TestGRPCVoiceRoomAccessResolver_UsesDedicatedBearerForGeneratedCanonicalRPC
// specifies the Voice-owned adapter factory. The adapter must send its own
// configured bearer through the generated RPC; it must not forward or reuse a
// caller's incoming authorization/profile metadata, and it cannot fall back to
// ListMembers.
func TestGRPCVoiceRoomAccessResolver_UsesDedicatedBearerForGeneratedCanonicalRPC(t *testing.T) {
	trustedToken := "trusted-voice-service-token"
	spaceID := uuid.NewString()
	voiceRoomID := uuid.NewString()
	profileID := uuid.NewString()
	spaceServer := &recordingSpaceResolverServer{result: &spacev1.ResolveVoiceRoomAccessResponse{
		SpaceId: spaceID,
		Member:  true,
		Active:  true,
	}}
	resolver := s2s.NewGRPCVoiceRoomAccessResolver(startVoiceResolverServer(t, trustedToken, spaceServer), trustedToken)
	var _ grpcsvc.AuthoritativeVoiceRoomAccessResolver = resolver

	forgedCtx := metadata.NewIncomingContext(t.Context(), metadata.Pairs(
		"authorization", "Bearer attacker-controlled-token",
		"x-voice-profile-id", uuid.NewString(),
	))
	access, err := resolver.ResolveVoiceRoomAccess(forgedCtx, voiceRoomID, profileID)

	require.NoError(t, err)
	require.True(t, proto.Equal(&spacev1.ResolveVoiceRoomAccessRequest{VoiceRoomId: voiceRoomID, ProfileId: profileID}, spaceServer.request))
	require.Equal(t, []string{"Bearer " + trustedToken}, spaceServer.metadata.Get("authorization"))
	require.Empty(t, spaceServer.metadata.Get("x-voice-profile-id"), "caller metadata cannot cross the S2S authentication boundary")
	require.Equal(t, grpcsvc.CanonicalVoiceRoomAccess{SpaceID: spaceID, Member: true, Active: true}, access)
}

func TestGRPCVoiceRoomAccessResolver_BlankTokenFailsBeforeGeneratedRPC(t *testing.T) {
	for _, token := range []string{"", " \t "} {
		t.Run("blank configured token", func(t *testing.T) {
			spaceServer := &recordingSpaceResolverServer{result: &spacev1.ResolveVoiceRoomAccessResponse{SpaceId: uuid.NewString(), Member: true, Active: true}}
			resolver := s2s.NewGRPCVoiceRoomAccessResolver(startVoiceResolverServer(t, "trusted-voice-service-token", spaceServer), token)

			access, err := resolver.ResolveVoiceRoomAccess(t.Context(), uuid.NewString(), uuid.NewString())

			require.Equal(t, codes.FailedPrecondition, status.Code(err))
			require.Equal(t, grpcsvc.CanonicalVoiceRoomAccess{}, access)
			require.Zero(t, spaceServer.calls, "blank configuration must fail locally before an S2S request")
		})
	}
}

func TestGRPCVoiceRoomAccessResolver_PropagatesSpaceUnavailable(t *testing.T) {
	spaceServer := &recordingSpaceResolverServer{resolverError: status.Error(codes.Unavailable, "space database unavailable")}
	resolver := s2s.NewGRPCVoiceRoomAccessResolver(startVoiceResolverServer(t, "trusted-voice-service-token", spaceServer), "trusted-voice-service-token")

	access, err := resolver.ResolveVoiceRoomAccess(t.Context(), uuid.NewString(), uuid.NewString())

	require.Equal(t, codes.Unavailable, status.Code(err))
	require.Equal(t, grpcsvc.CanonicalVoiceRoomAccess{}, access)
	require.Equal(t, 1, spaceServer.calls)
}
