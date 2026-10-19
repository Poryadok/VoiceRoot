package spaceprincipal

import (
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	matchmakingv1 "voice.app/voice/matchmaking/v1"
	"voice/backend/matchmaking/internal/terminalprincipal"
)

type delegatedTerminalVerifier interface {
	VerifyTerminal(context.Context, proto.Message, string) (context.Context, error)
}

func terminalMethod(method string) bool {
	switch method {
	case matchmakingv1.MatchmakingService_GetMatch_FullMethodName, matchmakingv1.MatchmakingService_GetSearchStatus_FullMethodName, matchmakingv1.MatchmakingService_GetMatchHistory_FullMethodName, matchmakingv1.MatchmakingService_CancelSearch_FullMethodName:
		return true
	default:
		return false
	}
}
func protectedTerminal(v verifier, ctx context.Context, request any, method string, handler grpc.UnaryHandler) (any, error) {
	verify, ok := v.(delegatedTerminalVerifier)
	if !ok || v == nil {
		return nil, status.Error(codes.Unavailable, "protected terminal verifier unavailable")
	}
	message, ok := request.(proto.Message)
	if !ok {
		return nil, status.Error(codes.InvalidArgument, "invalid terminal request")
	}
	verified, err := verify.VerifyTerminal(ctx, message, method)
	if err != nil {
		return nil, err
	}
	if _, ok := terminalprincipal.SubjectFromContext(verified, message, method); !ok {
		return nil, status.Error(codes.Unauthenticated, "invalid terminal binding")
	}
	return handler(verified, request)
}
func (r *Runtime) VerifyTerminal(ctx context.Context, request proto.Message, method string) (context.Context, error) {
	if r == nil || r.resolver == nil || r.replay == nil || r.config.GatewayJWKSURL == "" {
		return nil, status.Error(codes.Unavailable, "protected terminal runtime unavailable")
	}
	return (terminalprincipal.Verifier{KeyResolver: r.resolver.Resolve, ReplayGuard: r.recordReplay, SessionEpochChecker: r.checkSessionEpoch}).Verify(ctx, request, method)
}
