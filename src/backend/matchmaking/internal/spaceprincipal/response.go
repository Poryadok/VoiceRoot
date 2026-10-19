package spaceprincipal

import (
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	matchmakingv1 "voice.app/voice/matchmaking/v1"
	"voice/backend/matchmaking/internal/responseprincipal"
)

type delegatedResponseVerifier interface {
	VerifyResponse(context.Context, *matchmakingv1.RespondToMatchRequest) (context.Context, error)
}

func protectedResponse(v verifier, ctx context.Context, request any, handler grpc.UnaryHandler) (any, error) {
	verify, ok := v.(delegatedResponseVerifier)
	if !ok || v == nil {
		return nil, status.Error(codes.Unavailable, "protected response verifier unavailable")
	}
	req, ok := request.(*matchmakingv1.RespondToMatchRequest)
	if !ok || req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid RespondToMatch request")
	}
	verified, err := verify.VerifyResponse(ctx, req)
	if err != nil {
		return nil, err
	}
	if _, ok := responseprincipal.SubjectFromContext(verified, req); !ok {
		return nil, status.Error(codes.Unauthenticated, "invalid match response binding")
	}
	return handler(verified, request)
}

func (r *Runtime) VerifyResponse(ctx context.Context, req *matchmakingv1.RespondToMatchRequest) (context.Context, error) {
	if r == nil || r.resolver == nil || r.replay == nil || r.config.GatewayJWKSURL == "" {
		return nil, status.Error(codes.Unavailable, "protected response runtime unavailable")
	}
	return (responseprincipal.Verifier{KeyResolver: r.resolver.Resolve, ReplayGuard: r.recordReplay, SessionEpochChecker: r.checkSessionEpoch}).Verify(ctx, req)
}

// NativeOrdinaryUnaryInterceptor is only for the fixed native composition.
// The true pre-cut ordinary responder remains an explicitly separate artifact.
func NativeOrdinaryUnaryInterceptor() grpc.UnaryServerInterceptor {
	other := OrdinaryUnaryInterceptor()
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if info.FullMethod == matchmakingv1.MatchmakingService_RespondToMatch_FullMethodName {
			return nil, status.Error(codes.Unavailable, "protected response RPC unavailable on this listener")
		}
		return other(ctx, req, info, handler)
	}
}
