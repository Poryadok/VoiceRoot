package principalruntime

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/subscription/internal/principalgrpc"

	subscriptionv1 "voice.app/voice/subscription/v1"
)

func (r *Runtime) ServerOptions() []grpc.ServerOption {
	return []grpc.ServerOption{
		grpc.Creds(r.credentials),
		grpc.ChainUnaryInterceptor(func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
			if !isLifecycleMethod(info.FullMethod) {
				return nil, status.Error(codes.PermissionDenied, "method unavailable on lifecycle listener")
			}
			return handler(ctx, req)
		}, principalgrpc.StrictUnaryInterceptor(r)),
	}
}

func isLifecycleMethod(method string) bool {
	return method == subscriptionv1.SubscriptionService_ApplySpaceLifecycleFence_FullMethodName ||
		method == subscriptionv1.SubscriptionService_PurgeSpace_FullMethodName
}
