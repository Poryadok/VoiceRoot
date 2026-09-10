package principalruntime

import (
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/role/internal/principalgrpc"
)

// ServerOptions secures the dedicated TLS listener and exposes only the v2
// ownership RPCs. The generic Role listener has a separate deny-only interceptor.
func (r *Runtime) ServerOptions() []grpc.ServerOption {
	return []grpc.ServerOption{
		grpc.Creds(r.credentials),
		grpc.ChainUnaryInterceptor(func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
			if !isOwnershipMethod(info.FullMethod) {
				return nil, status.Error(codes.PermissionDenied, "method unavailable on ownership listener")
			}
			return handler(ctx, req)
		}, principalgrpc.StrictUnaryInterceptor(r), func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
			if info.FullMethod == rolev1.RoleService_GetOwnershipTransferCapabilities_FullMethodName && r.ownershipV2CapabilitiesActive.Load() {
				ctx = principalgrpc.WithOwnershipV2CapabilitiesActive(ctx)
			}
			return handler(ctx, req)
		}),
	}
}
