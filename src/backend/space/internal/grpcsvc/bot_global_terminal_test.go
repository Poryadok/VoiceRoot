package grpcsvc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/botmembershipprincipal"
	"voice/backend/space/internal/store"
)

func TestMinimalBotTerminal48ProtectedZeroSpace(t *testing.T) {
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixtureWithImage(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"), "postgres:18-alpine")
	db.Run(true, "goto", "46")
	admin := &store.SpaceStore{Pool: db.Pool}
	_, err := admin.TransitionNativeMembershipCatalog46(ctx)
	require.NoError(t, err)
	db.Run(true, "up", "1")
	_, err = admin.TransitionNativeMembershipCatalog47(ctx)
	require.NoError(t, err)
	db.Run(true, "up", "1")
	_, err = admin.TransitionNativeMembershipCatalog48(ctx)
	require.NoError(t, err)
	serving := &store.SpaceStore{Pool: nativeBotServingLogin(t, ctx, db.Pool)}
	certFile, keyFile, caFile, clientCert, roots := nativeBotTLSMaterial(t)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	next, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	keys := []map[string]string{}
	for i, k := range []*rsa.PrivateKey{key, next} {
		kid := "current"
		if i == 1 {
			kid = "next"
		}
		keys = append(keys, map[string]string{"kty": "RSA", "kid": kid, "alg": "RS256", "use": "sig", "n": base64.RawURLEncoding.EncodeToString(k.N.Bytes()), "e": "AQAB"})
	}
	jwks := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": keys})
	}))
	identity, err := tls.LoadX509KeyPair(certFile, keyFile)
	require.NoError(t, err)
	jwks.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{identity}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: roots}
	jwks.StartTLS()
	t.Cleanup(jwks.Close)
	redis, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: testcontainers.ContainerRequest{Image: "redis:7-alpine", ExposedPorts: []string{"6379/tcp"}, WaitingFor: wait.ForLog("Ready to accept connections")}, Started: true})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, redis.Terminate(context.Background())) })
	addr, err := redis.Endpoint(ctx, "")
	require.NoError(t, err)
	dir := t.TempDir()
	clientCertFile, clientKeyFile := filepath.Join(dir, "client.pem"), filepath.Join(dir, "client-key.pem")
	private, err := x509.MarshalPKCS8PrivateKey(clientCert.PrivateKey)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(clientCertFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: clientCert.Certificate[0]}), 0600))
	require.NoError(t, os.WriteFile(clientKeyFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: private}), 0600))
	start := func() (spacev1.SpaceServiceClient, spacev1.SpaceServiceClient, func()) {
		runtime, err := botmembershipprincipal.New(ctx, botmembershipprincipal.Config{ListenAddr: "127.0.0.1:0", CertFile: certFile, KeyFile: keyFile, ClientCAFile: caFile, ReplayAddr: addr, JWKSCAFile: caFile, JWKSClientCertFile: clientCertFile, JWKSClientKeyFile: clientKeyFile, JWKSURLs: map[string]string{"bot": jwks.URL}, RefreshAfter: 100 * time.Millisecond, HardExpiry: 200 * time.Millisecond, UnknownKIDCooldown: 50 * time.Millisecond})
		require.NoError(t, err)
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		server := grpc.NewServer(runtime.ServerOptions()...)
		RegisterBotMembershipServer(server, &BotMembershipGRPC{Store: serving})
		go func() { _ = server.Serve(listener) }()
		connections := []*grpc.ClientConn{}
		connect := func(certs []tls.Certificate) spacev1.SpaceServiceClient {
			c, err := grpc.NewClient(listener.Addr().String(), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: "localhost", Certificates: certs})))
			require.NoError(t, err)
			connections = append(connections, c)
			return spacev1.NewSpaceServiceClient(c)
		}
		a, b := connect([]tls.Certificate{clientCert}), connect(nil)
		return a, b, func() {
			for _, c := range connections {
				require.NoError(t, c.Close())
			}
			server.Stop()
			_ = listener.Close()
			require.NoError(t, runtime.Close())
		}
	}
	client, noCert, stop := start()
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "bot", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	nextIssuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "bot", KeyID: "next", PrivateKey: next})
	require.NoError(t, err)
	r := &spacev1.RegisterBotGlobalTerminalRequest{ProtocolVersion: 1, BotId: uuid.NewString(), DecisionId: uuid.NewString()}
	sign := func(i *principal.Issuer, method string) context.Context {
		h, err := principal.RequestHash(r)
		require.NoError(t, err)
		token, err := i.IssueService(principal.ServiceInput{Audience: "space", RPC: method, RequestID: r.DecisionId, RequestHash: h})
		require.NoError(t, err)
		return metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", r.DecisionId))
	}
	call := func(c spacev1.SpaceServiceClient, x context.Context) (*spacev1.RegisterBotGlobalTerminalResponse, error) {
		x, cancel := context.WithTimeout(x, 3*time.Second)
		defer cancel()
		return c.RegisterBotGlobalTerminal(x, r)
	}
	_, err = call(client, ctx)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	_, err = call(client, sign(issuer, spacev1.SpaceService_RemoveBotMember_FullMethodName))
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	signed := sign(issuer, spacev1.SpaceService_RegisterBotGlobalTerminal_FullMethodName)
	first, err := call(client, signed)
	require.NoError(t, err)
	require.Equal(t, spacev1.BotGlobalTerminalOutcome_BOT_GLOBAL_TERMINAL_OUTCOME_GLOBAL_NATIVE_ADDS_FENCED, first.Receipt.Outcome)
	_, err = call(client, signed)
	require.Equal(t, codes.Unauthenticated, status.Code(err), "actual Redis rejects a reused credential")
	replay, err := call(client, sign(nextIssuer, spacev1.SpaceService_RegisterBotGlobalTerminal_FullMethodName))
	require.NoError(t, err)
	require.True(t, proto.Equal(first, replay), "actual next JWKS key is accepted with exact saved minimal result")
	_, err = call(noCert, sign(issuer, spacev1.SpaceService_RegisterBotGlobalTerminal_FullMethodName))
	require.Error(t, err)
	stop()
	client, _, stop = start()
	defer stop()
	replay, err = call(client, sign(issuer, spacev1.SpaceService_RegisterBotGlobalTerminal_FullMethodName))
	require.NoError(t, err)
	require.True(t, proto.Equal(first, replay), "listener/runtime restart preserves same-DB receipt and real Redis")
	var spaces, legacy, rows int
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM public.spaces),(SELECT count(*) FROM native_membership_guard.bot_terminals),(SELECT count(*) FROM native_membership_guard.bot_terminal_registrations)`).Scan(&spaces, &legacy, &rows))
	require.Zero(t, spaces)
	require.Zero(t, legacy)
	require.Equal(t, 1, rows)
}
