package spaceprincipal

import (
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	matchmakingv1 "voice.app/voice/matchmaking/v1"
	"voice/backend/matchmaking/internal/searchprincipal"
)

type delegatedSearchVerifier interface {
	VerifySearch(context.Context, *matchmakingv1.StartSearchRequest) (context.Context, error)
}

func ProtectedUnaryInterceptor(v verifier) grpc.UnaryServerInterceptor {
	other := StrictUnaryInterceptor(v)
	return func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if terminalMethod(info.FullMethod) {
			return protectedTerminal(v, ctx, request, info.FullMethod, handler)
		}
		if info.FullMethod == matchmakingv1.MatchmakingService_RespondToMatch_FullMethodName {
			return protectedResponse(v, ctx, request, handler)
		}
		if info.FullMethod != matchmakingv1.MatchmakingService_StartSearch_FullMethodName {
			return other(ctx, request, info, handler)
		}
		verify, ok := v.(delegatedSearchVerifier)
		if !ok || v == nil {
			return nil, status.Error(codes.Unavailable, "protected search verifier unavailable")
		}
		req, ok := request.(*matchmakingv1.StartSearchRequest)
		if !ok || req == nil {
			return nil, status.Error(codes.InvalidArgument, "invalid StartSearch request")
		}
		verifiedCtx, err := verify.VerifySearch(ctx, req)
		if err != nil {
			return nil, err
		}
		if verifiedCtx == nil {
			return nil, status.Error(codes.Unauthenticated, "invalid search principal")
		}
		if _, ok := searchprincipal.SubjectFromContext(verifiedCtx, req); !ok {
			return nil, status.Error(codes.Unauthenticated, "invalid search binding")
		}
		return handler(verifiedCtx, request)
	}
}

func (r *Runtime) VerifySearch(ctx context.Context, req *matchmakingv1.StartSearchRequest) (context.Context, error) {
	if r == nil || r.resolver == nil || r.replay == nil || r.config.GatewayJWKSURL == "" {
		return nil, status.Error(codes.Unavailable, "protected search runtime unavailable")
	}
	return (searchprincipal.Verifier{KeyResolver: r.resolver.Resolve, ReplayGuard: r.recordReplay, SessionEpochChecker: r.checkSessionEpoch}).Verify(ctx, req)
}
