package ratelimit_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"voice/backend/bot/internal/ratelimit"
)

func TestRedisGRPCLimiter_sharedAcrossInstances(t *testing.T) {
	mr := miniredis.RunT(t)
	cfg := ratelimit.Config{
		BotAPI: ratelimit.Limit{Max: 2, Window: time.Minute},
	}
	lim1 := ratelimit.NewRedisGRPCLimiterForTest(mr.Addr(), "", cfg)
	lim2 := ratelimit.NewRedisGRPCLimiterForTest(mr.Addr(), "", cfg)

	token := "bot-token-shared"
	method := "/voice.bot.v1.BotService/SendBotMessage"
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-voice-bot-token", token))

	handler := func(ctx context.Context, req any) (any, error) { return "ok", nil }
	info := &grpc.UnaryServerInfo{FullMethod: method}

	_, err := lim1.UnaryServerInterceptor()(ctx, nil, info, handler)
	require.NoError(t, err)
	_, err = lim2.UnaryServerInterceptor()(ctx, nil, info, handler)
	require.NoError(t, err)
	_, err = lim1.UnaryServerInterceptor()(ctx, nil, info, handler)
	require.Error(t, err)
}

func TestRedisGRPCLimiter_failureStopsRuntimeAndRecovery(t *testing.T) {
	for _, method := range []string{"SendBotMessage", "AssignBotRole", "TouchPresence"} {
		t.Run(method, func(t *testing.T) {
			mr := miniredis.RunT(t)
			cfg := ratelimit.Config{
				BotAPI:        ratelimit.Limit{Max: 1, Window: time.Minute},
				BotRoleOps:    ratelimit.Limit{Max: 1, Window: time.Minute},
				TouchPresence: ratelimit.Limit{Max: 1, Window: time.Minute},
			}
			lim := ratelimit.NewRedisGRPCLimiterForTest(mr.Addr(), "", cfg)
			ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-voice-bot-token", "private-token"))
			info := &grpc.UnaryServerInfo{FullMethod: "/voice.bot.v1.BotService/" + method}
			calls := 0
			handler := func(context.Context, any) (any, error) { calls++; return "ok", nil }
			mr.SetError("ERR private Redis diagnostics")
			response, err := lim.UnaryServerInterceptor()(ctx, nil, info, handler)
			require.Equal(t, codes.Unavailable, status.Code(err))
			require.Nil(t, response)
			require.Zero(t, calls)
			require.NotContains(t, err.Error(), "private")
			mr.SetError("")
			response, err = lim.UnaryServerInterceptor()(ctx, nil, info, handler)
			require.NoError(t, err)
			require.Equal(t, "ok", response)
			require.Equal(t, 1, calls)
			_, err = lim.UnaryServerInterceptor()(ctx, nil, info, handler)
			require.Equal(t, codes.ResourceExhausted, status.Code(err))
			require.Equal(t, 1, calls)
		})
	}
}

func TestRedisGRPCLimiter_connectionFailureAndUnrelatedPaths(t *testing.T) {
	mr := miniredis.RunT(t)
	lim := ratelimit.NewRedisGRPCLimiterForTest(mr.Addr(), "", ratelimit.Config{
		BotAPI: ratelimit.Limit{Max: 1, Window: time.Minute},
	})
	mr.Close()
	botCtx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-voice-bot-token", "private-token"))
	for _, tc := range []struct {
		name   string
		ctx    context.Context
		method string
		want   codes.Code
		calls  int
	}{
		{"runtime", botCtx, "/voice.bot.v1.BotService/SendBotMessage", codes.Unavailable, 0},
		{"registry", botCtx, "/voice.bot.v1.BotService/GetBot", codes.OK, 1},
		{"other service", botCtx, "/grpc.health.v1.Health/Check", codes.OK, 1},
		{"without bot token", context.Background(), "/voice.bot.v1.BotService/SendBotMessage", codes.OK, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			_, err := lim.UnaryServerInterceptor()(tc.ctx, nil, &grpc.UnaryServerInfo{FullMethod: tc.method}, func(context.Context, any) (any, error) {
				calls++
				return "ok", nil
			})
			require.Equal(t, tc.want, status.Code(err))
			require.Equal(t, tc.calls, calls)
		})
	}
}
