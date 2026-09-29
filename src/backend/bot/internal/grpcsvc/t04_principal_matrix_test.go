package grpcsvc_test

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/emptypb"

	botv1 "voice.app/voice/bot/v1"
	"voice/backend/bot/internal/dispatch"
	grpcsvc "voice/backend/bot/internal/grpcsvc"
	"voice/backend/bot/internal/ratelimit"
)

func TestT04PublishGameEventDoesNotAcceptOtherAuthorities(t *testing.T) {
	t.Setenv("BOT_GRPC_GATEWAY_ONLY", "true")
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer(grpc.UnaryInterceptor(ratelimit.GatewayAccessFromEnv()))
	botv1.RegisterBotServiceServer(server, grpcsvc.NewBotGRPC(nil, dispatch.NewHub()))
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { server.Stop() })

	conn, err := grpc.NewClient("passthrough:///t04-bot",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	cases := []struct {
		name string
		md   metadata.MD
	}{
		{name: "Bot token", md: metadata.Pairs("x-voice-bot-token", "bot-token")},
		{name: "account context", md: metadata.Pairs("x-voice-user-id", "00000000-0000-4000-8000-000000000001", "x-voice-profile-id", "00000000-0000-4000-8000-000000000002")},
		{name: "GIS vgi1 credential", md: metadata.Pairs("authorization", "Bearer vgi1_00000000-0000-4000-8000-000000000003_secret")},
		{name: "T11 HTTP workload proof headers", md: metadata.Pairs("x-voice-workload", "gameintegration", "x-voice-audience", "bot", "x-voice-timestamp", "1790000000", "x-voice-nonce", "00000000-0000-4000-8000-000000000004", "x-voice-signature", "not-a-generic-service-token")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := metadata.NewOutgoingContext(context.Background(), tc.md)
			err := conn.Invoke(ctx, "/voice.bot.v1.BotService/PublishGameEvent", &emptypb.Empty{}, &emptypb.Empty{})
			require.Equal(t, codes.Unimplemented, status.Code(err), "no principal type may create authority before T51 is implemented")
		})
	}
}
