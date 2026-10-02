package principalgrpc

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"voice/backend/pkg/principal"

	messagingv1 "voice.app/voice/messaging/v1"
)

type ChatLifecycleVerifier interface {
	Verify(context.Context, string, string, string, string) (principal.Principal, error)
}

func IsChatLifecycleMethod(method string) bool {
	return method == messagingv1.MessagingService_GetSpacePurgeReceipt_FullMethodName
}

func IsSpaceLifecycleMethod(method string) bool {
	return method == messagingv1.MessagingService_ApplySpaceLifecycleFence_FullMethodName ||
		method == messagingv1.MessagingService_PurgeSpace_FullMethodName ||
		method == messagingv1.MessagingService_ImportSpacePurgeManifestPage_FullMethodName
}

// ChatLifecycleOrdinaryUnaryInterceptor keeps Chat's read-only lifecycle lookup
// off the general listener so request metadata cannot impersonate a principal.
func ChatLifecycleOrdinaryUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if IsChatLifecycleMethod(info.FullMethod) {
			return nil, status.Error(codes.Unavailable, "Chat lifecycle method requires the protected Messaging listener")
		}
		return handler(ctx, request)
	}
}

func SpaceLifecycleOrdinaryUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if IsSpaceLifecycleMethod(info.FullMethod) {
			return nil, status.Error(codes.Unavailable, "Space lifecycle method requires the protected Messaging listener")
		}
		return handler(ctx, request)
	}
}

func SpaceLifecycleStrictUnaryInterceptor(verifier ChatLifecycleVerifier) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if !IsSpaceLifecycleMethod(info.FullMethod) {
			return nil, status.Error(codes.PermissionDenied, "method unavailable on Space lifecycle listener")
		}
		transport, err := principal.IncomingMetadata(ctx)
		if err != nil || verifier == nil {
			return nil, status.Error(codes.Unauthenticated, "invalid Space principal")
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
		if err != nil || verified.Kind != "service" || verified.Issuer != "space" ||
			verified.Subject != "service:space" || verified.AccountID != "" || verified.ProfileID != "" || verified.SessionEpoch != 0 {
			return nil, status.Error(codes.Unauthenticated, "invalid Space principal")
		}
		if verified.Audience != "messaging" || verified.RPC != info.FullMethod ||
			verified.RequestID != transport.RequestID || verified.RequestHash != hash {
			return nil, status.Error(codes.Unauthenticated, "invalid Space principal binding")
		}
		return handler(principal.WithVerified(ctx, verified), request)
	}
}

func ChatLifecycleStrictUnaryInterceptor(verifier ChatLifecycleVerifier) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if !IsChatLifecycleMethod(info.FullMethod) {
			return nil, status.Error(codes.PermissionDenied, "method unavailable on Chat lifecycle listener")
		}
		transport, err := principal.IncomingMetadata(ctx)
		if err != nil || verifier == nil {
			return nil, status.Error(codes.Unauthenticated, "invalid Chat principal")
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
		if err != nil || verified.Kind != "service" || verified.Issuer != "chat" ||
			verified.Subject != "service:chat" || verified.AccountID != "" || verified.ProfileID != "" || verified.SessionEpoch != 0 {
			return nil, status.Error(codes.Unauthenticated, "invalid Chat principal")
		}
		if verified.Audience != "messaging" || verified.RPC != info.FullMethod ||
			verified.RequestID != transport.RequestID || verified.RequestHash != hash {
			return nil, status.Error(codes.Unauthenticated, "invalid Chat principal binding")
		}
		return handler(principal.WithVerified(ctx, verified), request)
	}
}
