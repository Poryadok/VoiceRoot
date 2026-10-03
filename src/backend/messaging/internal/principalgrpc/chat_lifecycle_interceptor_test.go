package principalgrpc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"voice/backend/pkg/principal"

	messagingv1 "voice.app/voice/messaging/v1"
)

type chatLifecycleVerifierFunc func(context.Context, string, string, string, string) (principal.Principal, error)

func (f chatLifecycleVerifierFunc) Verify(ctx context.Context, token, method, requestID, hash string) (principal.Principal, error) {
	return f(ctx, token, method, requestID, hash)
}

func TestChatLifecycleInterceptorPinsLookupToExactChatPrincipal(t *testing.T) {
	request := &messagingv1.GetSpacePurgeReceiptRequest{SpaceId: "20000000-0000-4000-8000-000000000001", DeletionOperationId: "20000000-0000-4000-8000-000000000002", PurgeGeneration: 8, SourceScheduleGeneration: 7, MessagingRequestSha256: make([]byte, 32)}
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	method := messagingv1.MessagingService_GetSpacePurgeReceipt_FullMethodName
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer signed", "x-request-id", "lookup-1"))
	called := false
	result, err := ChatLifecycleStrictUnaryInterceptor(chatLifecycleVerifierFunc(func(_ context.Context, token, rpc, id, requestHash string) (principal.Principal, error) {
		require.Equal(t, "signed", token)
		require.Equal(t, method, rpc)
		require.Equal(t, "lookup-1", id)
		require.Equal(t, hash, requestHash)
		return principal.Principal{Kind: "service", Issuer: "chat", Subject: "service:chat", Audience: "messaging", RPC: rpc, RequestID: id, RequestHash: requestHash}, nil
	}))(ctx, request, &grpc.UnaryServerInfo{FullMethod: method}, func(ctx context.Context, got any) (any, error) {
		verified, ok := principal.FromContext(ctx)
		require.True(t, ok)
		require.Equal(t, "service:chat", verified.Subject)
		require.Same(t, request, got)
		called = true
		return "ok", nil
	})
	require.NoError(t, err)
	require.Equal(t, "ok", result)
	require.True(t, called)

	for _, tc := range []struct {
		name string
		principal.Principal
	}{
		{name: "wrong issuer", Principal: principal.Principal{Kind: "service", Issuer: "space", Subject: "service:space", Audience: "messaging", RPC: method, RequestID: "lookup-1", RequestHash: hash}},
		{name: "wrong audience", Principal: principal.Principal{Kind: "service", Issuer: "chat", Subject: "service:chat", Audience: "file", RPC: method, RequestID: "lookup-1", RequestHash: hash}},
		{name: "wrong hash", Principal: principal.Principal{Kind: "service", Issuer: "chat", Subject: "service:chat", Audience: "messaging", RPC: method, RequestID: "lookup-1", RequestHash: "sha256:wrong"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ChatLifecycleStrictUnaryInterceptor(chatLifecycleVerifierFunc(func(context.Context, string, string, string, string) (principal.Principal, error) {
				return tc.Principal, nil
			}))(ctx, request, &grpc.UnaryServerInfo{FullMethod: method}, func(context.Context, any) (any, error) { t.Fatal("invalid principal reached handler"); return nil, nil })
			require.Equal(t, codes.Unauthenticated, status.Code(err))
		})
	}
}

func TestChatLifecycleInterceptorRestrictsListenerMethods(t *testing.T) {
	request := &messagingv1.GetSpacePurgeReceiptRequest{}
	ordinary := ChatLifecycleOrdinaryUnaryInterceptor()
	_, err := ordinary(context.Background(), request, &grpc.UnaryServerInfo{FullMethod: messagingv1.MessagingService_GetSpacePurgeReceipt_FullMethodName}, func(context.Context, any) (any, error) {
		t.Fatal("protected method reached general listener")
		return nil, nil
	})
	require.Equal(t, codes.Unavailable, status.Code(err))
	strict := ChatLifecycleStrictUnaryInterceptor(nil)
	_, err = strict(context.Background(), request, &grpc.UnaryServerInfo{FullMethod: messagingv1.MessagingService_GetSpacePurgeReceipt_FullMethodName}, func(context.Context, any) (any, error) {
		t.Fatal("unverified principal reached handler")
		return nil, nil
	})
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	_, err = strict(context.Background(), request, &grpc.UnaryServerInfo{FullMethod: messagingv1.MessagingService_GetMessages_FullMethodName}, func(context.Context, any) (any, error) {
		t.Fatal("unrelated method reached lifecycle listener")
		return nil, nil
	})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestSpaceLifecycleInterceptorPinsManifestImportToSpacePrincipal(t *testing.T) {
	request := &messagingv1.ImportSpacePurgeManifestPageRequest{ProtocolVersion: 1, SpaceId: "20000000-0000-4000-8000-000000000001", DeletionOperationId: "20000000-0000-4000-8000-000000000002", ScheduleGeneration: 7}
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	method := messagingv1.MessagingService_ImportSpacePurgeManifestPage_FullMethodName
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer signed", "x-request-id", "manifest-1"))
	called := false
	result, err := SpaceLifecycleStrictUnaryInterceptor(chatLifecycleVerifierFunc(func(_ context.Context, token, rpc, id, requestHash string) (principal.Principal, error) {
		require.Equal(t, "signed", token)
		require.Equal(t, method, rpc)
		require.Equal(t, "manifest-1", id)
		require.Equal(t, hash, requestHash)
		return principal.Principal{Kind: "service", Issuer: "space", Subject: "service:space", Audience: "messaging", RPC: rpc, RequestID: id, RequestHash: requestHash}, nil
	}))(ctx, request, &grpc.UnaryServerInfo{FullMethod: method}, func(ctx context.Context, got any) (any, error) {
		verified, ok := principal.FromContext(ctx)
		require.True(t, ok)
		require.Equal(t, "service:space", verified.Subject)
		require.Same(t, request, got)
		called = true
		return "ok", nil
	})
	require.NoError(t, err)
	require.Equal(t, "ok", result)
	require.True(t, called)

	_, err = SpaceLifecycleStrictUnaryInterceptor(chatLifecycleVerifierFunc(func(context.Context, string, string, string, string) (principal.Principal, error) {
		return principal.Principal{Kind: "service", Issuer: "chat", Subject: "service:chat", Audience: "messaging", RPC: method, RequestID: "manifest-1", RequestHash: hash}, nil
	}))(ctx, request, &grpc.UnaryServerInfo{FullMethod: method}, func(context.Context, any) (any, error) {
		t.Fatal("non-Space caller reached handler")
		return nil, nil
	})
	require.Equal(t, codes.Unauthenticated, status.Code(err))

	_, err = SpaceLifecycleOrdinaryUnaryInterceptor()(context.Background(), request, &grpc.UnaryServerInfo{FullMethod: method}, func(context.Context, any) (any, error) {
		t.Fatal("protected method reached general listener")
		return nil, nil
	})
	require.Equal(t, codes.Unavailable, status.Code(err))
}
