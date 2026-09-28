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

const tombstoneMethod = messagingv1.MessagingService_TombstoneGameMessage_FullMethodName

type Verifier interface {
	Verify(context.Context, string, string, string, string) (principal.Principal, error)
}

// TombstoneUnaryInterceptor authenticates only the moderation tombstone RPC;
// all other Messaging methods continue through the service's normal policies.
func TombstoneUnaryInterceptor(verifier Verifier) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if info.FullMethod != tombstoneMethod {
			return handler(ctx, request)
		}
		transport, err := principal.IncomingMetadata(ctx)
		if err != nil || verifier == nil {
			return nil, status.Error(codes.Unauthenticated, "invalid moderation principal")
		}
		message, ok := request.(proto.Message)
		if !ok || message == nil || !message.ProtoReflect().IsValid() || len(message.ProtoReflect().GetUnknown()) != 0 {
			return nil, status.Error(codes.InvalidArgument, "invalid moderation request")
		}
		hash, err := principal.RequestHash(message)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid moderation request")
		}
		verified, err := verifier.Verify(ctx, transport.BearerToken, info.FullMethod, transport.RequestID, hash)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid moderation principal")
		}
		if verified.Kind != "service" || verified.Issuer != "moderation" || verified.Subject != "service:moderation" || verified.AccountID != "" || verified.ProfileID != "" || verified.SessionEpoch != 0 {
			return nil, status.Error(codes.PermissionDenied, "moderation service principal required")
		}
		if verified.Audience != "messaging" || verified.RPC != info.FullMethod || verified.RequestID != transport.RequestID || verified.RequestHash != hash {
			return nil, status.Error(codes.Unauthenticated, "invalid moderation principal binding")
		}
		return handler(principal.WithVerified(ctx, verified), request)
	}
}
