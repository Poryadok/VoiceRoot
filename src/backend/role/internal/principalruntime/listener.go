package principalruntime

import (
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	rolev1 "voice.app/voice/role/v1"
	authorityv1 "voice/backend/pkg/pb/voice/authority/v1"
	"voice/backend/role/internal/principalgrpc"
)

// ServerOptions secures the dedicated mTLS listener and exposes only trusted
// ownership, retirement, and GIS/Voice game-session grant RPCs.
func (r *Runtime) ServerOptions() []grpc.ServerOption {
	return []grpc.ServerOption{
		grpc.Creds(r.credentials),
		grpc.ChainUnaryInterceptor(func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
			if isAuthoritySourceMethod(info.FullMethod) {
				if r.authoritySourceInterceptor == nil || r.authoritySourceServer == nil {
					return nil, status.Error(codes.PermissionDenied, "authority source inactive")
				}
				return r.authoritySourceInterceptor(ctx, req, info, handler)
			}
			if !isTrustedServiceMethod(info.FullMethod) {
				return nil, status.Error(codes.PermissionDenied, "method unavailable on trusted-service listener")
			}
			return principalgrpc.StrictUnaryInterceptor(r)(ctx, req, info, handler)
		}, func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
			if info.FullMethod == rolev1.RoleService_GetOwnershipTransferCapabilities_FullMethodName && r.ownershipV2CapabilitiesActive.Load() {
				ctx = principalgrpc.WithOwnershipV2CapabilitiesActive(ctx)
			}
			return handler(ctx, req)
		}),
	}
}

func isAuthoritySourceMethod(method string) bool {
	return method == authorityv1.AuthoritySourceService_ReadSnapshot_FullMethodName || method == authorityv1.AuthoritySourceService_ReadRevision_FullMethodName
}

// RegisterAuthoritySource is called only on the dedicated mTLS server, before
// serving. Activation does not expose this service on the legacy listener.
func (r *Runtime) RegisterAuthoritySource(server *grpc.Server) {
	if r != nil && r.authoritySourceServer != nil {
		authorityv1.RegisterAuthoritySourceServiceServer(server, r.authoritySourceServer)
	}
}
