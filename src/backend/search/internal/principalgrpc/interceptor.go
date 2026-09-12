package principalgrpc

import (
	"context"
	"errors"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	searchv1 "voice.app/voice/search/v1"
	"voice/backend/pkg/principal"
)

type Verifier interface {
	Verify(context.Context, string, string, string, string) (principal.Principal, error)
}

func IsLifecycleMethod(method string) bool {
	return method == searchv1.SearchService_ApplySpaceLifecycleFence_FullMethodName || method == searchv1.SearchService_PurgeSpace_FullMethodName
}

// OrdinaryUnaryInterceptor keeps protected lifecycle authority off the normal listener.
func OrdinaryUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if IsLifecycleMethod(info.FullMethod) {
			return nil, status.Error(codes.Unavailable, "protected method unavailable on ordinary listener")
		}
		return handler(ctx, request)
	}
}

// StrictUnaryInterceptor authenticates the exact protected request before dispatch.
func StrictUnaryInterceptor(verifier Verifier) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if !IsLifecycleMethod(info.FullMethod) {
			return nil, status.Error(codes.PermissionDenied, "method unavailable on lifecycle listener")
		}
		if verifier == nil {
			return nil, status.Error(codes.Unavailable, "principal verifier unavailable")
		}
		transport, err := principal.IncomingMetadata(ctx)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid principal")
		}
		message, ok := request.(proto.Message)
		if !ok || message == nil || len(message.ProtoReflect().GetUnknown()) != 0 {
			return nil, status.Error(codes.InvalidArgument, "invalid request")
		}
		requestHash, err := principal.RequestHash(message)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid request")
		}
		verified, err := verifier.Verify(ctx, transport.BearerToken, info.FullMethod, transport.RequestID, requestHash)
		if err != nil {
			var unavailable unavailableError
			if errors.As(err, &unavailable) {
				return nil, status.Error(codes.Unavailable, "principal verifier unavailable")
			}
			return nil, status.Error(codes.Unauthenticated, "invalid principal")
		}
		return handler(principal.WithVerified(ctx, verified), request)
	}
}

type unavailableError struct{ err error }

func (e unavailableError) Error() string { return e.err.Error() }
func (e unavailableError) Unwrap() error { return e.err }
func Unavailable(err error) error {
	if err == nil {
		err = errors.New("unavailable")
	}
	return unavailableError{err: err}
}
