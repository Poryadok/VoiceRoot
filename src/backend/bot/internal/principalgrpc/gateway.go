package principalgrpc

import (
	"context"
	"errors"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	botv1 "voice.app/voice/bot/v1"
	"voice/backend/pkg/principal"
)

type GatewayUserVerifier struct {
	Keys   principal.KeyResolver
	Replay principal.ReplayGuard
	Epoch  principal.SessionEpochChecker
	Clock  func() time.Time
}

func GatewayLifecycleStreamInterceptor() grpc.StreamServerInterceptor {
	return func(_ any, _ grpc.ServerStream, _ *grpc.StreamServerInfo, _ grpc.StreamHandler) error {
		return status.Error(codes.Unimplemented, "streams are not exposed on the Gateway lifecycle listener")
	}
}

func isGatewayLifecycleMethod(method string) bool {
	return method == botv1.BotService_DeleteBot_FullMethodName || method == botv1.BotService_InstallBotInSpace_FullMethodName || method == botv1.BotService_UninstallBotFromSpace_FullMethodName || method == botv1.BotService_SetChatWhitelist_FullMethodName || method == botv1.BotService_SetBotChatEnabled_FullMethodName
}

func (v GatewayUserVerifier) VerifyGateway(ctx context.Context, token, method, requestID, hash string) (principal.Principal, error) {
	if !isGatewayLifecycleMethod(method) || v.Keys == nil || v.Replay == nil || v.Epoch == nil {
		return principal.Principal{}, status.Error(codes.Unavailable, "Bot initiating authority unavailable")
	}
	return principal.VerifyDelegatedUser(ctx, token, principal.VerifyConfig{ExpectedIssuer: "gateway", ExpectedAudience: "bot", ExpectedRPC: method, ExpectedRequestID: requestID, ExpectedRequestHash: hash, KeyResolver: v.Keys, ReplayGuard: v.Replay, SessionEpochChecker: v.Epoch, Clock: v.Clock})
}

// GatewayLifecycleUnaryInterceptor is used only on the dedicated verified-mTLS
// listener. Raw identity headers never become a delegated initiating identity.
func GatewayLifecycleUnaryInterceptor(verifier GatewayUserVerifier) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if !isGatewayLifecycleMethod(info.FullMethod) {
			return nil, status.Error(codes.Unimplemented, "Bot method is not exposed on the Gateway lifecycle listener")
		}
		remote, ok := peer.FromContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "verified Gateway transport required")
		}
		tlsInfo, ok := remote.AuthInfo.(credentials.TLSInfo)
		if !ok || len(tlsInfo.State.VerifiedChains) == 0 {
			return nil, status.Error(codes.Unauthenticated, "verified Gateway transport required")
		}
		transport, err := principal.IncomingMetadata(ctx)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid Gateway initiating credential")
		}
		message, ok := request.(proto.Message)
		if !ok || message == nil || !message.ProtoReflect().IsValid() || len(message.ProtoReflect().GetUnknown()) != 0 {
			return nil, status.Error(codes.InvalidArgument, "invalid Bot lifecycle request")
		}
		hash, err := principal.RequestHash(message)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid Bot lifecycle request")
		}
		verified, err := verifier.VerifyGateway(ctx, transport.BearerToken, info.FullMethod, transport.RequestID, hash)
		if err != nil {
			if status.Code(err) == codes.Unavailable || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
				return nil, status.Error(codes.Unavailable, "Bot initiating authority unavailable")
			}
			return nil, status.Error(codes.Unauthenticated, "invalid Gateway initiating credential")
		}
		// Auth/JWKS/replay may have waited. Recheck the exact message and expiry at
		// dispatch; no authority is extended by a dependency's latency.
		now := time.Now()
		if verifier.Clock != nil {
			now = verifier.Clock()
		}
		rebound, err := principal.RequestHash(message)
		if err != nil || rebound != hash || !now.Before(verified.ExpiresAt) || ctx.Err() != nil {
			return nil, status.Error(codes.Unauthenticated, "expired or changed initiating request")
		}
		verifiedCtx := principal.WithVerified(ctx, verified)
		if _, err := principal.RequireDelegatedUser(verifiedCtx, "bot", info.FullMethod, message); err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid delegated initiating identity")
		}
		return handler(verifiedCtx, request)
	}
}
