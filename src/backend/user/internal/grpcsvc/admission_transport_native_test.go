package grpcsvc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/principal"
	"voice/backend/pkg/socialprincipal"
	"voice/backend/user/internal/store"
)

func proveAdmissionTransport(t *testing.T, pool *pgxpool.Pool, a, p uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	certPath := filepath.Join(repoRoot(t), "src/backend/auth/src/test/resources/principal-tls/server-cert.pem")
	keyPath := filepath.Join(repoRoot(t), "src/backend/auth/src/test/resources/principal-tls/server-key.pem")
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	require.NoError(t, err)
	pem, err := os.ReadFile(certPath)
	require.NoError(t, err)
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(pem))
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	next, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	jwks, err := json.Marshal(map[string]any{"keys": []map[string]string{sdkPrincipalJWK(key, "current"), sdkPrincipalJWK(next, "next")}})
	require.NoError(t, err)
	jwksServer := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(jwks)
	}))
	jwksServer.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}, ClientCAs: roots, ClientAuth: tls.RequireAndVerifyClientCert}
	jwksServer.StartTLS()
	defer jwksServer.Close()
	cfg := socialprincipal.Config{Target: "user", Capability: "auth-admission", JWKSURLs: map[string]string{"auth": strings.Replace(jwksServer.URL, "127.0.0.1", "localhost", 1)}, JWKSCAFile: certPath,
		JWKSClientCertFile: certPath, JWKSClientKeyFile: keyPath, ClientCAFile: certPath, ClientCAs: roots,
		RefreshAfter: 30 * time.Second, HardExpiry: 2 * time.Minute, UnknownKIDCooldown: time.Second, ReplayAddr: os.Getenv("USER_ADMISSION_TEST_REDIS_ADDR"), TLSCertFile: certPath, TLSKeyFile: keyPath, ListenAddr: ":0"}
	require.NotEmpty(t, cfg.ReplayAddr, "native transport needs owned real replay Redis")
	start := func() (userv1.UserServiceClient, func()) {
		runtime, e := socialprincipal.New(ctx, cfg)
		require.NoError(t, e)
		listener := bufconn.Listen(1 << 20)
		server := grpc.NewServer(runtime.ServerOptions()...)
		RegisterAdmissionEligibilityServer(server, store.NewProfileStore(pool))
		info := server.GetServiceInfo()["voice.user.v1.UserService"]
		require.Len(t, info.Methods, 1)
		require.Equal(t, "GetSpaceAdmissionProfileEligibility", info.Methods[0].Name)
		go func() { _ = server.Serve(listener) }()
		conn, e := grpc.NewClient("passthrough:///admission", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }),
			grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: "localhost", Certificates: []tls.Certificate{cert}})))
		require.NoError(t, e)
		return userv1.NewUserServiceClient(conn), func() { _ = conn.Close(); server.Stop(); _ = listener.Close(); require.NoError(t, runtime.Close()) }
	}
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "auth", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	request := &userv1.GetSpaceAdmissionProfileEligibilityRequest{AccountId: a.String(), ProfileId: p.String(), ObservationBindingSha256: make([]byte, 32)}
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	proof := func(rpc string) context.Context {
		id := uuid.NewString()
		token, e := issuer.IssueService(principal.ServiceInput{Audience: "user", RPC: rpc, RequestID: id, RequestHash: hash})
		require.NoError(t, e)
		return metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id))
	}
	client, closeFirst := start()
	callCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	_, err = client.GetSpaceAdmissionProfileEligibility(callCtx, request)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	_, err = client.GetSpaceAdmissionProfileEligibility(proof(userv1.UserService_GetSdkProfileEligibility_FullMethodName), request)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	original := proof(userv1.UserService_GetSpaceAdmissionProfileEligibility_FullMethodName)
	result, err := client.GetSpaceAdmissionProfileEligibility(original, request)
	require.NoError(t, err)
	require.Positive(t, result.ProfileRevision)
	_, err = client.GetSpaceAdmissionProfileEligibility(original, request)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	closeFirst()
	client, closeSecond := start()
	defer closeSecond()
	_, err = client.GetSpaceAdmissionProfileEligibility(original, request)
	require.Equal(t, codes.Unauthenticated, status.Code(err), "replay persists across listener runtime restart")
	_, err = client.GetSpaceAdmissionProfileEligibility(proof(userv1.UserService_GetSpaceAdmissionProfileEligibility_FullMethodName), request)
	require.NoError(t, err)
	_, err = client.GetSdkProfileEligibility(proof(userv1.UserService_GetSpaceAdmissionProfileEligibility_FullMethodName), &userv1.GetSdkProfileEligibilityRequest{})
	require.Equal(t, codes.Unimplemented, status.Code(err))
}
