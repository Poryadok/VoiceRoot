package principalgrpc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	notificationv1 "voice.app/voice/notification/v1"
	"voice/backend/pkg/principal"
)

type verifierFunc func(context.Context, string, string, string, string) (principal.Principal, error)

func (f verifierFunc) Verify(ctx context.Context, token, rpc, requestID, hash string) (principal.Principal, error) {
	return f(ctx, token, rpc, requestID, hash)
}

func TestOrdinaryListenerRejectsLifecycleRPCBeforeDispatch(t *testing.T) {
	called := false
	_, err := OrdinaryUnaryInterceptor()(context.Background(), &notificationv1.PurgeSpaceRequest{}, &grpc.UnaryServerInfo{FullMethod: notificationv1.NotificationService_PurgeSpace_FullMethodName}, func(context.Context, any) (any, error) {
		called = true
		return nil, nil
	})
	require.False(t, called)
	require.Equal(t, codes.Unavailable, status.Code(err))
}

func TestStrictListenerVerifiesExactMethodRequestAndPrincipal(t *testing.T) {
	request := &notificationv1.ApplySpaceLifecycleFenceRequest{}
	expectedHash, err := principal.RequestHash(request)
	require.NoError(t, err)
	var attached principal.Principal
	verifier := verifierFunc(func(_ context.Context, token, rpc, requestID, hash string) (principal.Principal, error) {
		require.Equal(t, "signed-token", token)
		require.Equal(t, notificationv1.NotificationService_ApplySpaceLifecycleFence_FullMethodName, rpc)
		require.Equal(t, "lifecycle-request", requestID)
		require.Equal(t, expectedHash, hash)
		return principal.Principal{Kind: "service", Issuer: "space", Subject: "service:space", Audience: "notification", RPC: rpc, RequestID: requestID, RequestHash: hash}, nil
	})
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer signed-token", "x-request-id", "lifecycle-request"))
	_, err = StrictUnaryInterceptor(verifier)(ctx, request, &grpc.UnaryServerInfo{FullMethod: notificationv1.NotificationService_ApplySpaceLifecycleFence_FullMethodName}, func(ctx context.Context, _ any) (any, error) {
		attached, _ = principal.FromContext(ctx)
		return nil, nil
	})
	require.NoError(t, err)
	require.Equal(t, "notification", attached.Audience)
}

func TestStrictListenerRejectsNonLifecycleMethod(t *testing.T) {
	_, err := StrictUnaryInterceptor(verifierFunc(func(context.Context, string, string, string, string) (principal.Principal, error) {
		return principal.Principal{}, nil
	}))(
		context.Background(), &notificationv1.ApplySpaceLifecycleFenceRequest{}, &grpc.UnaryServerInfo{FullMethod: "/voice.notification.v1.NotificationService/SendNotification"}, func(context.Context, any) (any, error) { return nil, nil })
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}
