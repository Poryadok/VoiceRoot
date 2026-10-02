package principalgrpc

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/pkg/principal"
)

type GameIntegrationVerifier interface {
	Verify(context.Context, string, string, string, string) (principal.Principal, error)
}

func IsGameIntegrationMethod(method string) bool {
	return method == messagingv1.MessagingService_ResolveGameAction_FullMethodName ||
		method == messagingv1.MessagingService_ProjectGameActionResult_FullMethodName ||
		method == messagingv1.MessagingService_PurgeManagedChatContent_FullMethodName
}

// GameIntegrationOrdinaryUnaryInterceptor keeps GIS-only methods off the
// general Messaging listener, even when a caller presents a forged context.
func GameIntegrationOrdinaryUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if IsGameIntegrationMethod(info.FullMethod) {
			return nil, status.Error(codes.Unavailable, "GIS-only method requires the protected Messaging listener")
		}
		return handler(ctx, request)
	}
}

func GameIntegrationStrictUnaryInterceptor(verifier GameIntegrationVerifier) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if !IsGameIntegrationMethod(info.FullMethod) {
			return nil, status.Error(codes.PermissionDenied, "method unavailable on GIS listener")
		}
		transport, err := principal.IncomingMetadata(ctx)
		if err != nil || verifier == nil {
			return nil, status.Error(codes.Unauthenticated, "invalid GIS principal")
		}
		message, ok := request.(proto.Message)
		if !ok || message == nil || !message.ProtoReflect().IsValid() || len(message.ProtoReflect().GetUnknown()) != 0 {
			return nil, status.Error(codes.InvalidArgument, "invalid request")
		}
		hash, err := principal.RequestHash(message)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid request")
		}
		verified, err := verifier.Verify(ctx, transport.BearerToken, info.FullMethod, transport.RequestID, hash)
		if err != nil || verified.Kind != "service" || verified.Issuer != "gameintegration" ||
			verified.Subject != "service:gameintegration" || verified.AccountID != "" || verified.ProfileID != "" || verified.SessionEpoch != 0 {
			return nil, status.Error(codes.Unauthenticated, "invalid GIS principal")
		}
		if verified.Audience != "messaging" || verified.RPC != info.FullMethod || verified.RequestID != transport.RequestID || verified.RequestHash != hash {
			return nil, status.Error(codes.Unauthenticated, "invalid GIS principal binding")
		}
		if requestID := gameIntegrationRequestID(request); requestID != "" && requestID != transport.RequestID {
			return nil, status.Error(codes.Unauthenticated, "invalid GIS request binding")
		}
		return handler(principal.WithVerified(ctx, verified), request)
	}
}

func gameIntegrationRequestID(request any) string {
	if result, ok := request.(*messagingv1.ProjectGameActionResultRequest); ok && result != nil {
		return result.GetOperationId()
	}
	return ""
}
