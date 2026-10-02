package principalgrpc

import (
	"context"
	"errors"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	callsv1 "voice.app/voice/calls/v1"
	"voice/backend/pkg/principal"
)

type Verifier interface {
	Verify(context.Context, string, string, string, string) (principal.Principal, error)
}

func StrictUnaryInterceptor(verifier Verifier) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if verifier == nil {
			return nil, status.Error(codes.Unavailable, "Space principal verifier unavailable")
		}
		if !isLifecycleMethod(info.FullMethod) {
			return nil, status.Error(codes.PermissionDenied, "method is not allowed on Space principal listener")
		}
		transport, err := principal.IncomingMetadata(ctx)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid principal")
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
			return nil, VerificationStatus(err)
		}
		return handler(principal.WithVerified(ctx, verified), request)
	}
}

func OrdinaryUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if isLifecycleMethod(info.FullMethod) {
			return nil, status.Error(codes.Unavailable, "protected lifecycle method unavailable on ordinary listener")
		}
		return handler(ctx, request)
	}
}

func isLifecycleMethod(method string) bool {
	return method == callsv1.VoiceService_ApplySpaceLifecycleFence_FullMethodName || method == callsv1.VoiceService_PurgeSpace_FullMethodName
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

func VerificationStatus(err error) error {
	if status.Code(err) != codes.Unknown {
		return err
	}
	var unavailable unavailableError
	if errors.As(err, &unavailable) {
		return status.Error(codes.Unavailable, "Space principal verifier unavailable")
	}
	return status.Error(codes.Unauthenticated, "invalid principal")
}
