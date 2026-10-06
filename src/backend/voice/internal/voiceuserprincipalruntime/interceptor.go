package voiceuserprincipalruntime

import (
	"context"
	"errors"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"voice/backend/pkg/principal"
	"voice/backend/voice/internal/principalgrpc"
)

type Verifier interface {
	Verify(context.Context, string, string, string, string) (principal.Principal, error)
}

func StrictUnaryInterceptor(verifier Verifier) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if verifier == nil {
			return nil, principalgrpc.Unavailable(errors.New("Voice user verifier unavailable"))
		}
		if !AllowsMethod(info.FullMethod) {
			return nil, status.Error(codes.PermissionDenied, "method unavailable on Voice user listener")
		}
		transport, err := principal.IncomingMetadata(ctx)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid delegated principal")
		}
		message, ok := request.(proto.Message)
		if !ok || message == nil {
			return nil, status.Error(codes.InvalidArgument, "invalid request")
		}
		hash, err := principal.RequestHash(message)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid request")
		}
		verified, err := verifier.Verify(ctx, transport.BearerToken, info.FullMethod, transport.RequestID, hash)
		if err != nil {
			return nil, principalgrpc.VerificationStatus(err)
		}
		return handler(principal.WithVerified(ctx, verified), request)
	}
}

func (r *Runtime) ServerOptions() []grpc.ServerOption {
	if r == nil || r.credentials == nil {
		return []grpc.ServerOption{grpc.ChainUnaryInterceptor(StrictUnaryInterceptor(nil))}
	}
	return []grpc.ServerOption{grpc.Creds(r.credentials), grpc.ChainUnaryInterceptor(StrictUnaryInterceptor(r))}
}
