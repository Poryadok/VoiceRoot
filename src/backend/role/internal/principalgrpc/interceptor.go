package principalgrpc

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

type ownershipV2CapabilitiesContextKey struct{}

// WithOwnershipV2CapabilitiesActive records a server-owned activation decision.
// Incoming metadata cannot construct this context value.
func WithOwnershipV2CapabilitiesActive(ctx context.Context) context.Context {
	return context.WithValue(ctx, ownershipV2CapabilitiesContextKey{}, true)
}

func OwnershipV2CapabilitiesActive(ctx context.Context) bool {
	active, _ := ctx.Value(ownershipV2CapabilitiesContextKey{}).(bool)
	return active
}

// StrictUnaryInterceptor authenticates a protected RPC before its handler.
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

type unavailableError struct{ err error }

func (e unavailableError) Error() string { return e.err.Error() }
func (e unavailableError) Unwrap() error { return e.err }
func Unavailable(err error) error {
	if err == nil {
		err = errors.New("unavailable")
	}
	return unavailableError{err}
}
func verificationStatus(err error) error {
	var unavailable unavailableError
	if errors.As(err, &unavailable) {
		return status.Error(codes.Unavailable, "principal verifier unavailable")
	}
	return status.Error(codes.Unauthenticated, "invalid principal")
}

// OwnershipUnaryInterceptor drains every ownership protocol from the ordinary
// listener without consuming protected verifier dependencies.
func OwnershipUnaryInterceptor(_ Verifier) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		switch info.FullMethod {
		case "/voice.role.v1.RoleService/GetOwnershipTransferCapabilities",
			"/voice.role.v1.RoleService/PrepareOwnershipTransfer",
			"/voice.role.v1.RoleService/FinalizeOwnershipTransfer",
			"/voice.role.v1.RoleService/AbortOwnershipTransfer",
			"/voice.role.v1.RoleService/ApplyOwnershipTransfer",
			"/voice.role.v1.RoleService/CompensateOwnershipTransfer":
			return nil, status.Error(codes.Unavailable, "ownership method unavailable on ordinary listener")
		default:
			return handler(ctx, request)
		}
	}
}
