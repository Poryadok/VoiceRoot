package ratelimit_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

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
