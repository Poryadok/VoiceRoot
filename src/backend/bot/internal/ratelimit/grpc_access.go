package ratelimit

import (
	"context"
	"os"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"voice/backend/bot/internal/authctx"
)

const headerInternalGateway = "x-voice-internal"

// GatewayAccessFromEnv returns an interceptor that rejects non-gateway gRPC when BOT_GRPC_GATEWAY_ONLY=true.
func GatewayAccessFromEnv() grpc.UnaryServerInterceptor {
	if !strings.EqualFold(strings.TrimSpace(os.Getenv("BOT_GRPC_GATEWAY_ONLY")), "true") {
		return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
			return handler(ctx, req)
		}
	}
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if isTrustedGRPCCaller(ctx) {
			return handler(ctx, req)
		}
		return nil, status.Error(codes.PermissionDenied, "bot gRPC requires gateway")
	}
}

func isTrustedGRPCCaller(ctx context.Context) bool {
	if internalGateway(ctx) {
		return true
	}
	if token, ok := authctx.BotToken(ctx); ok && token != "" {
		return true
	}
	if _, ok := authctx.AccountID(ctx); ok {
		return true
	}
	return false
}

func internalGateway(ctx context.Context) bool {
	for _, from := range []func(context.Context) (metadata.MD, bool){
		metadata.FromIncomingContext,
		metadata.FromOutgoingContext,
	} {
		if md, ok := from(ctx); ok {
			vals := md.Get(headerInternalGateway)
			if len(vals) > 0 && vals[0] == "true" {
				return true
			}
		}
	}
	return false
}
