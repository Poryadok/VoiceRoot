package principalgrpc

import (
	"context"
	"errors"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"voice/backend/pkg/principal"

	subscriptionv1 "voice.app/voice/subscription/v1"
)

type Verifier interface {
	Verify(context.Context, string, string, string, string) (principal.Principal, error)
}

func StrictUnaryInterceptor(verifier Verifier) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if verifier == nil {
			return nil, status.Error(codes.Unavailable, "principal verifier unavailable")
		}
		transport, err := principal.IncomingMetadata(ctx)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid principal")
		}
		message, ok := request.(proto.Message)
		if !ok || message == nil {
			return nil, status.Error(codes.InvalidArgument, "invalid request")
		}
		requestHash, err := principal.RequestHash(message)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid request")
		}
		verified, err := verifier.Verify(ctx, transport.BearerToken, info.FullMethod, transport.RequestID, requestHash)
		if err != nil {
			return nil, verificationStatus(err)
		}
		return handler(principal.WithVerified(ctx, verified), request)
	}
}

func OrdinaryUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		switch info.FullMethod {
		case subscriptionv1.SubscriptionService_ApplySpaceLifecycleFence_FullMethodName,
			subscriptionv1.SubscriptionService_PurgeSpace_FullMethodName:
			return nil, status.Error(codes.Unavailable, "protected method unavailable on ordinary listener")
		default:
			return handler(ctx, request)
		}
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

func verificationStatus(err error) error {
	var unavailable unavailableError
	if errors.As(err, &unavailable) {
		return status.Error(codes.Unavailable, "principal verifier unavailable")
	}
	return status.Error(codes.Unauthenticated, "invalid principal")
}
