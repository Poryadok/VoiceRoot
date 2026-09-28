package principalgrpc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/pkg/principal"
)

type verifierFunc func(context.Context, string, string, string, string) (principal.Principal, error)

func (f verifierFunc) Verify(ctx context.Context, token, method, requestID, hash string) (principal.Principal, error) {
	return f(ctx, token, method, requestID, hash)
}

func TestTombstoneInterceptorVerifiesExactModerationPrincipalAndRequest(t *testing.T) {
	req := &messagingv1.TombstoneGameMessageRequest{ActionId: "action"}
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	verified := principal.Principal{Kind: "service", Issuer: "moderation", Subject: "service:moderation", Audience: "messaging", RPC: tombstoneMethod, RequestID: "req-1", RequestHash: hash}
	verifier := verifierFunc(func(_ context.Context, token, method, requestID, gotHash string) (principal.Principal, error) {
		require.Equal(t, "signed-token", token)
		require.Equal(t, tombstoneMethod, method)
		require.Equal(t, "req-1", requestID)
		require.Equal(t, hash, gotHash)
		return verified, nil
	})
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer signed-token", "x-request-id", "req-1"))
	called := false
	response, err := TombstoneUnaryInterceptor(verifier)(ctx, req, &grpc.UnaryServerInfo{FullMethod: tombstoneMethod}, func(ctx context.Context, _ any) (any, error) {
		called = true
		got, ok := principal.FromContext(ctx)
		require.True(t, ok)
		require.Equal(t, verified, got)
		return "ok", nil
	})
	require.NoError(t, err)
	require.Equal(t, "ok", response)
	require.True(t, called)
}

func TestTombstoneInterceptorFailsClosedForMissingOrWrongCredential(t *testing.T) {
	req := &messagingv1.TombstoneGameMessageRequest{ActionId: "action"}
	for _, test := range []struct {
		name     string
		ctx      context.Context
		verifier Verifier
		want     codes.Code
	}{
		{name: "missing metadata", ctx: context.Background(), verifier: verifierFunc(func(context.Context, string, string, string, string) (principal.Principal, error) {
			return principal.Principal{}, nil
		}), want: codes.Unauthenticated},
		{name: "missing verifier", ctx: metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer signed", "x-request-id", "r")), want: codes.Unauthenticated},
		{name: "wrong principal", ctx: metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer signed", "x-request-id", "r")), verifier: verifierFunc(func(context.Context, string, string, string, string) (principal.Principal, error) {
			return principal.Principal{Kind: "service", Issuer: "gateway", Subject: "service:gateway", Audience: "messaging", RPC: tombstoneMethod, RequestID: "r", RequestHash: "sha256:"}, nil
		}), want: codes.PermissionDenied},
	} {
		t.Run(test.name, func(t *testing.T) {
			called := false
			_, err := TombstoneUnaryInterceptor(test.verifier)(test.ctx, req, &grpc.UnaryServerInfo{FullMethod: tombstoneMethod}, func(context.Context, any) (any, error) { called = true; return nil, nil })
			require.Equal(t, test.want, status.Code(err))
			require.False(t, called)
		})
	}
}
