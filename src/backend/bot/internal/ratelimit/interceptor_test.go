package ratelimit_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"voice/backend/bot/internal/ratelimit"
)

func TestBotGRPCRateLimit_touchPresenceExhausted(t *testing.T) {
	lim := ratelimit.NewGRPCLimiter(ratelimit.Config{
		TouchPresence: ratelimit.Limit{Max: 2, Window: time.Minute},
	})
	botID := "00000000-0000-0000-0000-000000000001"

	require.True(t, lim.AllowTouchPresence(botID))
	require.True(t, lim.AllowTouchPresence(botID))
	require.False(t, lim.AllowTouchPresence(botID),
		"TouchPresence must be rate limited on direct Bot gRPC after quota (app stack6)")
}

func TestFromEnv_disabled(t *testing.T) {
	t.Setenv("BOT_RATE_LIMIT_DISABLED", "true")
	lim := ratelimit.FromEnv()
	interceptor := lim.UnaryServerInterceptor()
	called := 0
	handler := func(ctx context.Context, req any) (any, error) {
		called++
		return "ok", nil
	}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-voice-bot-token", "tok"))
	_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/voice.bot.v1.BotService/SendBotMessage"}, handler)
	require.NoError(t, err)
	require.Equal(t, 1, called)
}

func TestBotGRPCRateLimit_interceptorResourceExhausted(t *testing.T) {
	lim := ratelimit.NewGRPCLimiter(ratelimit.Config{
		TouchPresence: ratelimit.Limit{Max: 1, Window: time.Minute},
	})
	interceptor := lim.UnaryServerInterceptor()
	called := 0
	handler := func(ctx context.Context, req any) (any, error) {
		called++
		return "ok", nil
	}
	info := &grpc.UnaryServerInfo{FullMethod: "/voice.bot.v1.BotService/TouchPresence"}
	ctx := withBotToken("vb_test")

	_, err := interceptor(ctx, nil, info, handler)
	require.NoError(t, err)
	require.Equal(t, 1, called)

	_, err = interceptor(ctx, nil, info, handler)
	require.Error(t, err)
	require.Contains(t, err.Error(), "rate limit")
	require.Equal(t, 1, called)
}

func TestBotGRPCRateLimit_roleOpsExhausted(t *testing.T) {
	lim := ratelimit.NewGRPCLimiter(ratelimit.Config{
		BotRoleOps: ratelimit.Limit{Max: 1, Window: time.Minute},
	})
	interceptor := lim.UnaryServerInterceptor()
	info := &grpc.UnaryServerInfo{FullMethod: "/voice.bot.v1.BotService/CreateBotRole"}
	ctx := withBotToken("vb_role")

	_, err := interceptor(ctx, nil, info, okHandler)
	require.NoError(t, err)
	_, err = interceptor(ctx, nil, info, okHandler)
	require.Error(t, err)
}

func TestBotGRPCRateLimit_nonRuntimeMethodSkipsLimit(t *testing.T) {
	lim := ratelimit.NewGRPCLimiter(ratelimit.Config{
		BotAPI: ratelimit.Limit{Max: 1, Window: time.Minute},
	})
	interceptor := lim.UnaryServerInterceptor()
	info := &grpc.UnaryServerInfo{FullMethod: "/voice.bot.v1.BotService/ExecuteSlashInteraction"}
	ctx := withBotToken("vb_user")

	for i := 0; i < 3; i++ {
		_, err := interceptor(ctx, nil, info, okHandler)
		require.NoError(t, err)
	}
}

func okHandler(context.Context, any) (any, error) {
	return nil, nil
}

func withBotToken(token string) context.Context {
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-voice-bot-token", token))
}
