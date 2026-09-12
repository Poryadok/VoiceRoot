package principalgrpc

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	commonv1 "voice.app/voice/common/v1"
	searchv1 "voice.app/voice/search/v1"
	"voice/backend/pkg/principal"
)

type verifierFunc func(context.Context, string, string, string, string) (principal.Principal, error)

func (f verifierFunc) Verify(ctx context.Context, token, method, requestID, hash string) (principal.Principal, error) {
	return f(ctx, token, method, requestID, hash)
}

func TestOrdinaryListenerFailsClosedForExactlyProtectedSearchRPCs(t *testing.T) {
	i := OrdinaryUnaryInterceptor()
	called := false
	handler := func(context.Context, any) (any, error) { called = true; return "ok", nil }
	for _, method := range []string{searchv1.SearchService_ApplySpaceLifecycleFence_FullMethodName, searchv1.SearchService_PurgeSpace_FullMethodName} {
		_, err := i(context.Background(), &searchv1.ApplySpaceLifecycleFenceRequest{}, &grpc.UnaryServerInfo{FullMethod: method}, handler)
		require.Equal(t, codes.Unavailable, status.Code(err))
	}
	_, err := i(context.Background(), &searchv1.SearchSpacesRequest{}, &grpc.UnaryServerInfo{FullMethod: searchv1.SearchService_SearchSpaces_FullMethodName}, handler)
	require.NoError(t, err)
	require.True(t, called)
}

func TestStrictInterceptorRejectsRawIdentityAndBindsExactMetadataAndRequest(t *testing.T) {
	req := &searchv1.ApplySpaceLifecycleFenceRequest{Fence: &commonv1.SpaceLifecycleFenceRequest{ProtocolVersion: 1}}
	verifier := verifierFunc(func(_ context.Context, token, method, requestID, hash string) (principal.Principal, error) {
		require.Equal(t, "token", token)
		require.Equal(t, searchv1.SearchService_ApplySpaceLifecycleFence_FullMethodName, method)
		require.Equal(t, "request-1", requestID)
		require.NotEmpty(t, hash)
		return principal.Principal{Kind: "service", Issuer: "space", Subject: "service:space", Audience: "search", RPC: method, RequestID: requestID, RequestHash: hash}, nil
	})
	i := StrictUnaryInterceptor(verifier)
	handler := func(ctx context.Context, _ any) (any, error) {
		_, ok := principal.FromContext(ctx)
		require.True(t, ok)
		return "ok", nil
	}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer token", "x-request-id", "request-1"))
	_, err := i(ctx, req, &grpc.UnaryServerInfo{FullMethod: searchv1.SearchService_ApplySpaceLifecycleFence_FullMethodName}, handler)
	require.NoError(t, err)
	raw := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer token", "x-request-id", "request-1", "x-voice-service-id", "space"))
	_, err = i(raw, req, &grpc.UnaryServerInfo{FullMethod: searchv1.SearchService_ApplySpaceLifecycleFence_FullMethodName}, handler)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	_, err = StrictUnaryInterceptor(verifierFunc(func(context.Context, string, string, string, string) (principal.Principal, error) {
		return principal.Principal{}, Unavailable(errors.New("redis down"))
	}))(ctx, req, &grpc.UnaryServerInfo{FullMethod: searchv1.SearchService_ApplySpaceLifecycleFence_FullMethodName}, handler)
	require.Equal(t, codes.Unavailable, status.Code(err))
}
