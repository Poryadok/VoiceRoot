package chatcreation

import (
	"context"
	"errors"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/pkg/principal"
)

var ErrDependencyUnavailable = errors.New("chat creation authentication dependency unavailable")

type CreateVerifier struct {
	Keys   principal.KeyResolver
	Replay principal.ReplayGuard
	Epoch  principal.SessionEpochChecker
	Clock  func() time.Time
}

func OrdinaryCreateUnary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, next grpc.UnaryHandler) (any, error) {
		if info.FullMethod == chatv1.ChatService_CreateChat_FullMethodName {
			return nil, status.Error(codes.PermissionDenied, "protected Chat creation listener required")
		}
		return next(ctx, req)
	}
}

func (v CreateVerifier) Unary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, body any, info *grpc.UnaryServerInfo, next grpc.UnaryHandler) (any, error) {
		if info.FullMethod != chatv1.ChatService_CreateChat_FullMethodName {
			return nil, status.Error(codes.PermissionDenied, "method unavailable on Chat creation listener")
		}
		deny := status.Error(codes.Unauthenticated, "invalid Chat creation principal")
		if v.Keys == nil || v.Replay == nil || v.Epoch == nil {
			return nil, status.Error(codes.Unavailable, "Chat creation authentication unavailable")
		}
		remote, ok := peer.FromContext(ctx)
		if !ok {
			return nil, deny
		}
		secure, ok := remote.AuthInfo.(credentials.TLSInfo)
		if !ok || len(secure.State.VerifiedChains) == 0 {
			return nil, deny
		}
		req, ok := body.(*chatv1.CreateChatRequest)
		if !ok || req == nil || HasUnknown(req) {
			return nil, status.Error(codes.InvalidArgument, "invalid Chat create request")
		}
		md, err := principal.IncomingMetadata(ctx)
		if err != nil {
			return nil, deny
		}
		if _, err = CanonicalID(md.RequestID); err != nil {
			return nil, deny
		}
		if req.RequestId != "" && req.RequestId != md.RequestID {
			return nil, deny
		}
		hash, err := principal.RequestHash(req)
		if err != nil {
			return nil, deny
		}
		clock := v.Clock
		if clock == nil {
			clock = time.Now
		}
		config := principal.VerifyConfig{ExpectedIssuer: "gateway", ExpectedAudience: "chat", ExpectedRPC: info.FullMethod, ExpectedRequestID: md.RequestID, ExpectedRequestHash: hash, KeyResolver: v.Keys, ReplayGuard: v.Replay, SessionEpochChecker: v.Epoch, Clock: clock}
		p, err := principal.VerifyDelegatedUser(ctx, md.BearerToken, config)
		if errors.Is(err, ErrDependencyUnavailable) {
			return nil, status.Error(codes.Unavailable, "Chat creation authentication unavailable")
		}
		if err != nil {
			config.ExpectedIssuer = "bot"
			p, err = principal.VerifyBotActor(ctx, md.BearerToken, config)
		}
		if errors.Is(err, ErrDependencyUnavailable) {
			return nil, status.Error(codes.Unavailable, "Chat creation authentication unavailable")
		}
		if err != nil || !p.ExpiresAt.After(clock()) {
			return nil, deny
		}
		verified := principal.WithVerified(ctx, p)
		if _, err = Creator(verified, req); err != nil {
			return nil, deny
		}
		return next(verified, req)
	}
}
