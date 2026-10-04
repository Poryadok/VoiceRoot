package matchsquadprincipal

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/pkg/principal"
)

type verifierFunc func(context.Context, string, string, string, string) (principal.Principal, error)

func (f verifierFunc) Verify(ctx context.Context, token, method, requestID, hash string) (principal.Principal, error) {
	return f(ctx, token, method, requestID, hash)
}

func TestReplayGuardRejectsRepeatedJTIAndKeepsIssuerNamespacesSeparate(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	runtime := &Runtime{replay: client}
	expires := time.Now().Add(30 * time.Second)
	require.NoError(t, runtime.recordReplay(context.Background(), "matchmaking", "jti-1", expires))
	require.Error(t, runtime.recordReplay(context.Background(), "matchmaking", "jti-1", expires))
	require.NoError(t, runtime.recordReplay(context.Background(), "other-issuer", "jti-1", expires))
}

func TestStrictUnaryInterceptorRejectsPublicMethodsAndRawIdentityBeforeHandler(t *testing.T) {
	request := &chatv1.CreateMatchSquadChatRequest{ProtocolVersion: 1, OperationId: uuid.NewString()}
	called := 0
	verified := 0
	var gotToken, gotMethod, gotRequestID, gotHash string
	verifier := verifierFunc(func(_ context.Context, token, method, requestID, hash string) (principal.Principal, error) {
		verified++
		gotToken, gotMethod, gotRequestID, gotHash = token, method, requestID, hash
		return principal.Principal{Kind: "service", Issuer: "matchmaking", Subject: "service:matchmaking", Audience: "chat"}, nil
	})
	interceptor := strictUnaryInterceptor(verifier)
	handler := func(ctx context.Context, _ any) (any, error) { called++; return nil, nil }

	_, err := interceptor(context.Background(), request, &grpc.UnaryServerInfo{FullMethod: "/voice.chat.v1.ChatService/CreateChat"}, handler)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	require.Zero(t, called)
	require.Zero(t, verified)

	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer test-token", "x-request-id", request.GetOperationId(), "x-profile-id", uuid.NewString()))
	_, err = interceptor(ctx, request, &grpc.UnaryServerInfo{FullMethod: createMethod}, handler)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	require.Zero(t, called)
	require.Zero(t, verified)
	valid := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer test-token", "x-request-id", request.GetOperationId()))
	_, err = interceptor(valid, request, &grpc.UnaryServerInfo{FullMethod: createMethod}, handler)
	require.NoError(t, err)
	require.Equal(t, 1, called)
	require.Equal(t, 1, verified)
	require.Equal(t, "test-token", gotToken)
	require.Equal(t, createMethod, gotMethod)
	require.Equal(t, request.GetOperationId(), gotRequestID)
	require.Equal(t, hash, gotHash)
}

func TestStrictUnaryInterceptorRejectsDuplicateCredentialUnknownFieldsAndBindingMismatch(t *testing.T) {
	request := &chatv1.CreateMatchSquadChatRequest{ProtocolVersion: 1, OperationId: uuid.NewString()}
	verified := 0
	interceptor := strictUnaryInterceptor(verifierFunc(func(_ context.Context, _, _, _, _ string) (principal.Principal, error) {
		verified++
		return principal.Principal{}, errors.New("credential rejected")
	}))
	handlerCalls := 0
	handler := func(context.Context, any) (any, error) { handlerCalls++; return nil, nil }
	base := metadata.Pairs("authorization", "Bearer one", "x-request-id", request.GetOperationId())
	duplicate := metadata.Join(base, metadata.Pairs("authorization", "Bearer two"))
	_, err := interceptor(metadata.NewIncomingContext(context.Background(), duplicate), request, &grpc.UnaryServerInfo{FullMethod: createMethod}, handler)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	require.Zero(t, verified)
	require.Zero(t, handlerCalls)

	unknown := proto.Clone(request).(*chatv1.CreateMatchSquadChatRequest)
	unknown.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x01})
	_, err = interceptor(metadata.NewIncomingContext(context.Background(), base), unknown, &grpc.UnaryServerInfo{FullMethod: createMethod}, handler)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	require.Zero(t, verified)
	require.Zero(t, handlerCalls)

	_, err = interceptor(metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer one", "x-request-id", uuid.NewString())), request, &grpc.UnaryServerInfo{FullMethod: createMethod}, handler)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	require.Zero(t, verified)
	require.Zero(t, handlerCalls)

	good := metadata.NewIncomingContext(context.Background(), base)
	_, err = interceptor(good, request, &grpc.UnaryServerInfo{FullMethod: createMethod}, handler)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	require.Equal(t, 1, verified)
	require.Zero(t, handlerCalls)
}

func TestConfigLoadFromEnvIsDisabledOnlyWhenAllKeysAreAbsent(t *testing.T) {
	values := map[string]string{}
	lookup := func(key string) (string, bool) { value, ok := values[key]; return value, ok }
	_, enabled, err := LoadFromEnv(lookup)
	require.NoError(t, err)
	require.False(t, enabled)
	values[envPrefix+"GRPC_LISTEN"] = ""
	_, enabled, err = LoadFromEnv(lookup)
	require.Error(t, err)
	require.True(t, enabled)
	require.ErrorContains(t, err, "must be complete")
}

func TestConfigLoadFromEnvRequiresEverySettingButAllowsEmptyRedisPassword(t *testing.T) {
	values := map[string]string{
		envPrefix + "GRPC_LISTEN":           "127.0.0.1:9000",
		envPrefix + "TLS_CERT_FILE":         "cert",
		envPrefix + "TLS_KEY_FILE":          "key",
		envPrefix + "CLIENT_CA_FILE":        "ca",
		envPrefix + "JWKS_URL":              "https://auth.example/keys",
		envPrefix + "JWKS_CA_FILE":          "jwks-ca",
		envPrefix + "REPLAY_REDIS_ADDR":     "redis:6379",
		envPrefix + "REPLAY_REDIS_PASSWORD": "",
	}
	lookup := func(key string) (string, bool) { value, ok := values[key]; return value, ok }
	_, enabled, err := LoadFromEnv(lookup)
	require.NoError(t, err)
	require.True(t, enabled)
	delete(values, envPrefix+"REPLAY_REDIS_PASSWORD")
	_, enabled, err = LoadFromEnv(lookup)
	require.Error(t, err)
	require.True(t, enabled)
	require.ErrorContains(t, err, "must be complete")
}

func TestConfigValidateRequiresTLSAndTrustedJWKS(t *testing.T) {
	valid := Config{ListenerAddr: "127.0.0.1:9000", TLSCertFile: "cert", TLSKeyFile: "key", ClientCAFile: "ca", JWKSURL: "https://auth.example/.well-known/principal-jwks.json", JWKSCAFile: "jwks-ca", ReplayRedisAddr: "redis:6379"}
	require.NoError(t, valid.validate())
	for _, mutate := range []func(*Config){
		func(c *Config) { c.TLSKeyFile = "" },
		func(c *Config) { c.JWKSURL = "http://auth.example/keys" },
		func(c *Config) { c.JWKSURL = "https://user:pass@auth.example/keys" },
		func(c *Config) { c.ReplayRedisAddr = "" },
	} {
		candidate := valid
		mutate(&candidate)
		require.Error(t, candidate.validate())
	}
}
