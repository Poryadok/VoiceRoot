package socialprincipal

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/principal"
)

func TestAuthSDKCapability_OnlyTwoUserMethods(t *testing.T) {
	require.Equal(t, "auth", expectedIssuer("auth"))
	require.Equal(t, "/voice.user.v1.UserService/GetSdkProfileEligibility", Method("auth"))
	for _, method := range []string{
		"/voice.user.v1.UserService/GetSdkProfileEligibility",
		"/voice.user.v1.UserService/RecordSdkAuthorTombstone",
	} {
		require.True(t, AllowsMethod("auth", method))
	}
	for _, method := range []string{
		"/voice.user.v1.UserService/GetProfile",
		"/voice.user.v1.UserService/EnsurePrimaryProfile",
		"/voice.user.v1.UserService/DeleteProfile",
		"/voice.user.v1.UserService/ResolveAccountIDForProfile",
	} {
		require.False(t, AllowsMethod("auth", method))
	}
}

func TestAuthSDKCapability_ConfigFailsClosedUntilComplete(t *testing.T) {
	t.Setenv("S2S_JWKS_URLS_JSON", `{"auth":"https://auth.example/jwks"}`)
	_, enabled, err := LoadFromEnvWithAudience("user", "auth", "USER_AUTH_PRINCIPAL_", ":9094")
	require.NoError(t, err)
	require.False(t, enabled)
	t.Setenv("USER_AUTH_PRINCIPAL_TLS_CERT_FILE", "cert.pem")
	_, enabled, err = LoadFromEnvWithAudience("user", "auth", "USER_AUTH_PRINCIPAL_", ":9094")
	require.Error(t, err)
	require.True(t, enabled)
	t.Setenv("USER_AUTH_PRINCIPAL_TLS_KEY_FILE", "key.pem")
	t.Setenv("USER_AUTH_PRINCIPAL_REPLAY_REDIS_ADDR", "redis:6379")
	cfg, enabled, err := LoadFromEnvWithAudience("user", "auth", "USER_AUTH_PRINCIPAL_", ":9094")
	require.NoError(t, err)
	require.True(t, enabled)
	require.Equal(t, "user", cfg.Target)
	require.Equal(t, "auth", cfg.Capability)
	require.Equal(t, ":9094", cfg.ListenAddr)
	require.Equal(t, map[string]string{"auth": "https://auth.example/jwks"}, cfg.JWKSURLs)
}

func TestAuthSDKCapability_SignedPrincipalBindsBothExactMethodsAndBodies(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	verifier := &Verifier{
		Target: "user", Capability: "auth", Issuers: map[string]bool{"auth": true},
		Resolve: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil },
		Replay:  func(context.Context, string, string, time.Time) error { return nil },
	}
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "auth", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	requests := []struct {
		method string
		body   proto.Message
	}{
		{Method("auth"), &userv1.GetSdkProfileEligibilityRequest{AccountId: "account", ProfileId: "profile"}},
		{userv1.UserService_RecordSdkAuthorTombstone_FullMethodName,
			&userv1.RecordSdkAuthorTombstoneRequest{Version: 1, OperationId: "operation", RequestHash: "inner-hash"}},
	}
	for _, tc := range requests {
		t.Run(tc.method, func(t *testing.T) {
			hash, err := principal.RequestHash(tc.body)
			require.NoError(t, err)
			token, err := issuer.IssueService(principal.ServiceInput{
				Audience: "user", RPC: tc.method, RequestID: "request-id", RequestHash: hash,
			})
			require.NoError(t, err)
			ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
				"authorization", "Bearer "+token, "x-request-id", "request-id",
			))
			call := func(body proto.Message, method string) error {
				_, callErr := StrictUnaryInterceptor(verifier)(ctx, body, &grpc.UnaryServerInfo{FullMethod: method},
					func(ctx context.Context, req any) (any, error) {
						verified, ok := principal.FromContext(ctx)
						require.True(t, ok)
						require.Equal(t, "auth", verified.Issuer)
						require.Equal(t, "service:auth", verified.Subject)
						require.Equal(t, "user", verified.Audience)
						require.Equal(t, method, verified.RPC)
						require.Empty(t, verified.AccountID)
						require.Empty(t, verified.ProfileID)
						require.Zero(t, verified.SessionEpoch)
						return nil, nil
					})
				return callErr
			}
			require.NoError(t, call(tc.body, tc.method))
			require.Equal(t, codes.Unauthenticated, status.Code(call(tc.body, userv1.UserService_GetSdkProfileEligibility_FullMethodName+"/wrong")))
			changed := proto.Clone(tc.body)
			proto.Reset(changed)
			require.Equal(t, codes.Unauthenticated, status.Code(call(changed, tc.method)))
		})
	}
}
