package principalgrpc

import (
	"context"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"testing"
	"time"
	botv1 "voice.app/voice/bot/v1"
	"voice/backend/pkg/principal"
)

type messagingVerifier struct{ lifecycleVerifier }

func (v *messagingVerifier) VerifyMessaging(_ context.Context, _ string, method, id, hash string) (principal.Principal, error) {
	return principal.Principal{Kind: "service", Issuer: "messaging", Subject: "service:messaging", Audience: "bot", RPC: method, RequestID: id, RequestHash: hash, ExpiresAt: time.Now().Add(time.Minute)}, nil
}
func TestMessageSendAuthorityHasExactPrivateBranch(t *testing.T) {
	method := botv1.BotService_CheckBotMessageSendAuthority_FullMethodName
	req := &botv1.CheckBotMessageSendAuthorityRequest{}
	info := &grpc.UnaryServerInfo{FullMethod: method}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer token", "x-request-id", "request"))
	called := false
	handler := func(ctx context.Context, _ any) (any, error) {
		p, ok := principal.FromContext(ctx)
		require.True(t, ok)
		require.Equal(t, "messaging", p.Issuer)
		called = true
		return nil, nil
	}
	_, err := SpaceLifecycleUnaryInterceptor(&messagingVerifier{})(ctx, req, info, handler)
	require.NoError(t, err)
	require.True(t, called)
	called = false
	_, err = OrdinaryUnaryInterceptor()(ctx, req, info, handler)
	require.Equal(t, codes.Unavailable, status.Code(err))
	require.False(t, called)
	_, err = SpaceLifecycleUnaryInterceptor(&lifecycleVerifier{})(ctx, req, info, handler)
	require.Error(t, err)
	require.False(t, called)
}
