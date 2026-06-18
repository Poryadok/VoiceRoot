package ratelimit_test

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"voice/backend/bot/internal/ratelimit"
	botv1 "voice.app/voice/bot/v1"
)

func TestGatewayAccessFromEnv_deniesDirectGRPC(t *testing.T) {
	t.Setenv("BOT_GRPC_GATEWAY_ONLY", "true")
	interceptor := ratelimit.GatewayAccessFromEnv()
	handler := func(ctx context.Context, req any) (any, error) {
		return &botv1.GetBotResponse{}, nil
	}
	info := &grpc.UnaryServerInfo{FullMethod: "/voice.bot.v1.BotService/GetBot"}

	_, err := interceptor(context.Background(), nil, info, handler)
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))

	md := metadata.Pairs("x-voice-internal", "true")
	ctx := metadata.NewIncomingContext(context.Background(), md)
	_, err = interceptor(ctx, nil, info, handler)
	require.NoError(t, err)
}

func TestGatewayAccessFromEnv_allowsBotToken(t *testing.T) {
	t.Setenv("BOT_GRPC_GATEWAY_ONLY", "true")
	interceptor := ratelimit.GatewayAccessFromEnv()
	handler := func(ctx context.Context, req any) (any, error) { return "ok", nil }
	info := &grpc.UnaryServerInfo{FullMethod: "/voice.bot.v1.BotService/SendBotMessage"}

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-voice-bot-token", "tok"))
	_, err := interceptor(ctx, nil, info, handler)
	require.NoError(t, err)
}

func TestGatewayAccessFromEnv_disabledByDefault(t *testing.T) {
	_ = os.Unsetenv("BOT_GRPC_GATEWAY_ONLY")
	interceptor := ratelimit.GatewayAccessFromEnv()
	handler := func(ctx context.Context, req any) (any, error) { return "ok", nil }
	info := &grpc.UnaryServerInfo{FullMethod: "/voice.bot.v1.BotService/GetBot"}
	_, err := interceptor(context.Background(), nil, info, handler)
	require.NoError(t, err)
}
