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
const applyGameMessageMethod = messagingv1.MessagingService_ApplyGameMessage_FullMethodName
const sendGameEventMethod = messagingv1.MessagingService_SendGameEventMessage_FullMethodName

type Verifier interface {
	Verify(context.Context, string, string, string, string) (principal.Principal, error)
}

// TombstoneUnaryInterceptor authenticates only the moderation tombstone RPC.
func TombstoneUnaryInterceptor(verifier Verifier) grpc.UnaryServerInterceptor {
	return exactServiceUnaryInterceptor(tombstoneMethod, "moderation", verifier)
}

// ApplyGameMessageUnaryInterceptor authenticates the internal Gateway ingress.
func ApplyGameMessageUnaryInterceptor(verifier Verifier) grpc.UnaryServerInterceptor {
	return exactServiceUnaryInterceptor(applyGameMessageMethod, "gateway", verifier)
}

// SendGameEventMessageUnaryInterceptor authenticates the Bot-owned event sender.
func SendGameEventMessageUnaryInterceptor(verifier Verifier) grpc.UnaryServerInterceptor {
	return exactServiceUnaryInterceptor(sendGameEventMethod, "bot", verifier)
}

func exactServiceUnaryInterceptor(method, issuer string, verifier Verifier) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if info.FullMethod != method {
			return handler(ctx, request)
		}
		transport, err := principal.IncomingMetadata(ctx)
		if err != nil || verifier == nil {
			return nil, status.Errorf(codes.Unauthenticated, "invalid %s service principal", issuer)
		}
		message, ok := request.(proto.Message)
		if !ok || message == nil || !message.ProtoReflect().IsValid() || len(message.ProtoReflect().GetUnknown()) != 0 {
			return nil, status.Error(codes.InvalidArgument, "invalid moderation request")
		}
		hash, err := principal.RequestHash(message)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid %s service request", issuer)
		}
		verified, err := verifier.Verify(ctx, transport.BearerToken, info.FullMethod, transport.RequestID, hash)
		if err != nil {
			return nil, status.Errorf(codes.Unauthenticated, "invalid %s service principal", issuer)
		}
		if verified.Kind != "service" || verified.Issuer != issuer || verified.Subject != "service:"+issuer || verified.AccountID != "" || verified.ProfileID != "" || verified.SessionEpoch != 0 {
			return nil, status.Errorf(codes.PermissionDenied, "%s service principal required", issuer)
		}
		if verified.Audience != "messaging" || verified.RPC != info.FullMethod || verified.RequestID != transport.RequestID || verified.RequestHash != hash {
			return nil, status.Errorf(codes.Unauthenticated, "invalid %s service principal binding", issuer)
		}
		return handler(principal.WithVerified(ctx, verified), request)
	}
}
