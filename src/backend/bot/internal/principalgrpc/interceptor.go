package principalgrpc

import (
	"context"
	"errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	botv1 "voice.app/voice/bot/v1"
	"voice/backend/pkg/principal"
)

type Verifier interface {
	Verify(context.Context, string, string, string, string) (principal.Principal, error)
}

// SpaceVerifier selects Space's issuer and key resolver independently of the
// Game Integration verifier on the same runtime.
type SpaceVerifier interface {
	VerifySpace(context.Context, string, string, string, string) (principal.Principal, error)
}

func Unavailable(err error) error {
	if err == nil {
		return status.Error(codes.Unavailable, "principal authority unavailable")
	}
	return status.Error(codes.Unavailable, "principal authority unavailable")
}

func isSpaceLifecycleMethod(method string) bool {
	return method == botv1.BotService_ApplySpaceLifecycleFence_FullMethodName || method == botv1.BotService_PurgeSpace_FullMethodName
}

func isProtectedServiceMethod(method string) bool {
	return method == botv1.BotService_PublishGameEvent_FullMethodName || isSpaceLifecycleMethod(method)
}

// OrdinaryUnaryInterceptor keeps Space lifecycle administration off the Bot's
// regular service listener, which does not require the dedicated mTLS identity.
func OrdinaryUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if isProtectedServiceMethod(info.FullMethod) {
			return nil, status.Error(codes.Unavailable, "Bot protected service listener is unavailable")
		}
		return handler(ctx, request)
	}
}

// SpaceLifecycleUnaryInterceptor accepts only the two owner lifecycle methods
// and binds the signed service principal to exact protobuf request bytes.
func SpaceLifecycleUnaryInterceptor(verifier SpaceVerifier) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if !isSpaceLifecycleMethod(info.FullMethod) {
			return nil, status.Error(codes.Unimplemented, "Bot method is not exposed on the Space lifecycle listener")
		}
		transport, err := principal.IncomingMetadata(ctx)
		if err != nil || verifier == nil {
			return nil, status.Error(codes.Unauthenticated, "invalid Space service principal")
		}
		message, ok := request.(proto.Message)
		if !ok || message == nil || len(message.ProtoReflect().GetUnknown()) != 0 {
			return nil, status.Error(codes.InvalidArgument, "invalid Space lifecycle request")
		}
		hash, err := principal.RequestHash(message)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid Space lifecycle request")
		}
		verified, err := verifier.VerifySpace(ctx, transport.BearerToken, info.FullMethod, transport.RequestID, hash)
		if err != nil {
			if status.Code(err) == codes.Unavailable || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
				return nil, status.Error(codes.Unavailable, "Space principal authority unavailable")
			}
			return nil, status.Error(codes.Unauthenticated, "invalid Space service principal")
		}
		if verified.Kind != "service" || verified.Issuer != "space" || verified.Subject != "service:space" || verified.Audience != "bot" || verified.RPC != info.FullMethod || verified.RequestID != transport.RequestID || verified.RequestHash != hash || verified.AccountID != "" || verified.ProfileID != "" || verified.SessionEpoch != 0 {
			return nil, status.Error(codes.PermissionDenied, "Space service principal required")
		}
		return handler(principal.WithVerified(ctx, verified), request)
	}
}

func GameEventUnaryInterceptor(verifier Verifier) grpc.UnaryServerInterceptor {
	const method = botv1.BotService_PublishGameEvent_FullMethodName
	return func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if info.FullMethod != method {
			return nil, status.Error(codes.Unimplemented, "Bot service method is not exposed on this listener")
		}
		transport, err := principal.IncomingMetadata(ctx)
		if err != nil || verifier == nil {
			return nil, status.Error(codes.Unauthenticated, "invalid Game Integration service principal")
		}
		message, ok := request.(proto.Message)
		if !ok || message == nil || len(message.ProtoReflect().GetUnknown()) != 0 {
			return nil, status.Error(codes.InvalidArgument, "invalid game event request")
		}
		hash, err := principal.RequestHash(message)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid game event request")
		}
		verified, err := verifier.Verify(ctx, transport.BearerToken, info.FullMethod, transport.RequestID, hash)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid Game Integration service principal")
		}
		if verified.Kind != "service" || verified.Issuer != "gameintegration" || verified.Subject != "service:gameintegration" || verified.Audience != "bot" || verified.RPC != info.FullMethod || verified.RequestID != transport.RequestID || verified.RequestHash != hash || verified.AccountID != "" || verified.ProfileID != "" || verified.SessionEpoch != 0 {
			return nil, status.Error(codes.PermissionDenied, "Game Integration service principal required")
		}
		return handler(principal.WithVerified(ctx, verified), request)
	}
}
