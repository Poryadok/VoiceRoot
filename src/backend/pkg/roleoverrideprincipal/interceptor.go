package roleoverrideprincipal

import (
	"context"
	"errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"voice/backend/pkg/principal"
)

type Verifier interface {
	Verify(context.Context, string, string, string, string) (principal.Principal, error)
}

var ErrUnavailable = errors.New("role target principal dependency unavailable")

func StrictUnaryInterceptor(target string, verifier Verifier) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if !allowsMethod(target, info.FullMethod) {
			return nil, status.Error(codes.PermissionDenied, "method unavailable on target listener")
		}
		transport, err := principal.IncomingMetadata(ctx)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid principal")
		}
		message, ok := req.(proto.Message)
		if !ok || message == nil || !message.ProtoReflect().IsValid() || len(message.ProtoReflect().GetUnknown()) != 0 {
			return nil, status.Error(codes.InvalidArgument, "invalid target request")
		}
		if _, err := CanonicalID(transport.RequestID); err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid principal binding")
		}
		hash, err := principal.RequestHash(message)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid target request")
		}
		if verifier == nil {
			return nil, status.Error(codes.Unavailable, "target verifier unavailable")
		}
		p, err := verifier.Verify(ctx, transport.BearerToken, info.FullMethod, transport.RequestID, hash)
		if err != nil {
			if errors.Is(err, ErrUnavailable) {
				return nil, status.Error(codes.Unavailable, "target verifier unavailable")
			}
			return nil, status.Error(codes.Unauthenticated, "invalid principal")
		}
		verified := principal.WithVerified(ctx, p)
		if err := requireMethod(verified, target, info.FullMethod, message); err != nil {
			return nil, err
		}
		return handler(verified, req)
	}
}

func OrdinaryUnaryInterceptor(target string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if allowsMethod(target, info.FullMethod) {
			return nil, status.Error(codes.Unavailable, "protected target listener required")
		}
		return handler(ctx, req)
	}
}
