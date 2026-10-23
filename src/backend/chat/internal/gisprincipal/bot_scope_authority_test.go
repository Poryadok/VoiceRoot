package gisprincipal

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/chat/internal/grpcsvc"
	"voice/backend/chat/internal/store"
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/principal"
)

func TestBotScopeAuthorityConfigIsSeparateAndMethodExclusive(t *testing.T) {
	const method = "/voice.chat.v1.ChatScopeAuthorityService/GetBotChatScopeAuthority"
	t.Setenv("CHAT_SCOPE_BOT_GRPC_LISTEN", ":12345")
	_, _, enabled, err := ScopeAuthorityConfigFromEnv("bot")
	require.True(t, enabled)
	require.Error(t, err, "partial Bot configuration must fail closed")
	for _, key := range []string{"TLS_CERT_FILE", "TLS_KEY_FILE", "CLIENT_CA_FILE", "JWKS_URL", "JWKS_CA_FILE", "REPLAY_REDIS_ADDR"} {
		t.Setenv("CHAT_SCOPE_BOT_"+key, "fixture")
	}
	cfg, _, enabled, err := ScopeAuthorityConfigFromEnv("bot")
	require.NoError(t, err)
	require.True(t, enabled)
	require.Equal(t, "bot", cfg.Issuer)
	require.Equal(t, []string{method}, cfg.AllowedMethods)
	_, err = configuredMethods("bot", cfg.AllowedMethods)
	require.NoError(t, err)
	_, err = configuredMethods("bot", []string{ScopeAuthorityMethod})
	require.Error(t, err, "Bot cannot borrow the Role/Space read")
	for _, issuer := range []string{"role", "space", "search", Issuer, "realtime"} {
		_, err := configuredMethods(issuer, []string{method})
		require.Error(t, err, issuer+" cannot borrow the Bot read")
	}
}

func TestBotScopeAuthorityRealNativeTLSJWKSRedisAndRetiredHead(t *testing.T) {
	if testing.Short() {
		t.Skip("real private Chat PostgreSQL/Redis/TLS")
	}
	ctx := t.Context()
	pool := integrationtest.StartPostgres(t, ctx, "bot-chat-scope", "")
	paths, err := filepath.Glob("../../../migrations/chat_db/*.up.sql")
	require.NoError(t, err)
	require.NotEmpty(t, paths)
	for _, path := range paths {
		body, err := os.ReadFile(path)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(body))
		require.NoError(t, err, path)
	}
	native := &store.DMStore{Pool: pool}
	space := uuid.New()
	row, err := native.CreateSpaceChannelChat(ctx, uuid.New(), space, "Bot native scope", nil)
	require.NoError(t, err)
	redis, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: testcontainers.ContainerRequest{Image: "redis:7.4-alpine", ExposedPorts: []string{"6379/tcp"}, WaitingFor: wait.ForListeningPort("6379/tcp")}, Started: true})
	require.NoError(t, err)
	t.Cleanup(func() { _ = redis.Terminate(context.Background()) })
	redisAddress, err := redis.Endpoint(ctx, "")
	require.NoError(t, err)
	ca, clients := newManifestTestCA(t, 247301), newManifestTestCA(t, 247302)
	serverCert, certPath, keyPath := ca.leaf(t, 247303, x509.ExtKeyUsageServerAuth)
	clientCert, _, _ := clients.leaf(t, 247304, x509.ExtKeyUsageClientAuth)
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(ca.pem))
	caPath, clientCAPath := filepath.Join(t.TempDir(), "ca.pem"), filepath.Join(t.TempDir(), "client-ca.pem")
	require.NoError(t, os.WriteFile(caPath, ca.pem, 0600))
	require.NoError(t, os.WriteFile(clientCAPath, clients.pem, 0600))
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	next, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	keys := []map[string]string{}
	for kid, key := range map[string]*rsa.PrivateKey{"current": key, "next": next} {
		keys = append(keys, map[string]string{"kty": "RSA", "kid": kid, "use": "sig", "alg": "RS256", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": "AQAB"})
	}
	jwks := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": keys})
	}))
	jwks.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{serverCert}}
	jwks.StartTLS()
	defer jwks.Close()
	runtime, err := New(ctx, Config{Issuer: "bot", Audience: Audience, AllowedMethods: []string{BotScopeAuthorityMethod}, TLSCertFile: certPath, TLSKeyFile: keyPath, ClientCAFile: clientCAPath, JWKSURL: jwks.URL, JWKSCAFile: caPath, ReplayAddr: redisAddress})
	require.NoError(t, err)
	defer runtime.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := grpc.NewServer(runtime.ServerOptions()...)
	chatv1.RegisterChatScopeAuthorityServiceServer(server, &grpcsvc.BotChatScopeAuthorityGRPC{Store: native})
	go func() { _ = server.Serve(listener) }()
	defer server.Stop()
	conn, err := grpc.NewClient(listener.Addr().String(), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: "localhost", Certificates: []tls.Certificate{clientCert}})))
	require.NoError(t, err)
	defer conn.Close()
	client := chatv1.NewChatScopeAuthorityServiceClient(conn)
	req := &chatv1.GetBotChatScopeAuthorityRequest{ProtocolVersion: 1, ChatId: row.ID.String(), ExpectedSpaceId: space.String(), OperationId: uuid.NewString()}
	sign := func(issuer, audience, method string) string {
		signer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: issuer, KeyID: "current", PrivateKey: key})
		require.NoError(t, err)
		hash, err := principal.RequestHash(req)
		require.NoError(t, err)
		token, err := signer.IssueService(principal.ServiceInput{Audience: audience, RPC: method, RequestID: req.OperationId, RequestHash: hash})
		require.NoError(t, err)
		return token
	}
	call := func(token string, extra ...string) (*chatv1.GetBotChatScopeAuthorityResponse, error) {
		callCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		pairs := append([]string{"authorization", "Bearer " + token, "x-request-id", req.OperationId}, extra...)
		return client.GetBotChatScopeAuthority(metadata.NewOutgoingContext(callCtx, metadata.Pairs(pairs...)), req)
	}
	token := sign("bot", "chat", BotScopeAuthorityMethod)
	response, err := call(token)
	require.NoError(t, err)
	require.True(t, response.Scope.Exists)
	require.True(t, response.Scope.Live)
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	require.Equal(t, hash, "sha256:"+hex.EncodeToString(response.RequestSha256))
	require.Equal(t, req.OperationId, response.Scope.OperationId)
	_, err = call(token)
	require.Equal(t, codes.Unauthenticated, status.Code(err), "real Redis rejects credential replay")
	for _, issuer := range []string{"role", "space", "gateway", "gameintegration"} {
		_, err = call(sign(issuer, "chat", BotScopeAuthorityMethod))
		require.Equal(t, codes.Unauthenticated, status.Code(err), issuer)
	}
	_, err = call(sign("bot", "role", BotScopeAuthorityMethod))
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	_, err = call(sign("bot", "chat", ScopeAuthorityMethod))
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	_, err = call(sign("bot", "chat", BotScopeAuthorityMethod), "x-profile-id", uuid.NewString())
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	token = sign("bot", "chat", BotScopeAuthorityMethod)
	req.ExpectedSpaceId = uuid.NewString()
	_, err = call(token)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	req.ExpectedSpaceId = space.String()
	req.ProtoReflect().SetUnknown([]byte{0x98, 0x06, 0x01})
	_, err = call(sign("bot", "chat", BotScopeAuthorityMethod))
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	req.ProtoReflect().SetUnknown(nil)
	req.ExpectedSpaceId = uuid.NewString()
	_, err = call(sign("bot", "chat", BotScopeAuthorityMethod))
	require.Equal(t, codes.NotFound, status.Code(err))
	req.ExpectedSpaceId = space.String()
	_, err = pool.Exec(ctx, `DELETE FROM chats WHERE id=$1`, row.ID)
	require.NoError(t, err)
	retired, err := call(sign("bot", "chat", BotScopeAuthorityMethod))
	require.NoError(t, err)
	require.False(t, retired.Scope.Exists)
	require.False(t, retired.Scope.Live)
	require.Equal(t, response.Scope.ChatScopeIncarnation, retired.Scope.ChatScopeIncarnation)
	require.Greater(t, retired.Scope.ChatAuthorityRevision, response.Scope.ChatAuthorityRevision)
	require.Equal(t, chatv1.ChatType_CHAT_TYPE_UNSPECIFIED, retired.Scope.ChatType)
}
