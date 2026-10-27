package grpcsvc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"net"
	"net/http/httptest"
	"testing"
	"time"
	userv1 "voice.app/voice/user/v1"
	catalogv1 "voice/backend/pkg/pb/voice/catalogactor/v1"
	"voice/backend/pkg/principal"
	"voice/backend/pkg/socialprincipal"
)

// Real TLS/JWKS/signatures/shared Redis admission, without requiring PostgreSQL.
// Valid requests reach the unavailable store; rejected requests cannot reach it.
func TestCatalogActorProtectedTransport(t *testing.T) {
	ctx := context.Background()
	cert, roots, certPath, keyPath := deletionTransportIdentity(t)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	next, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	jwks := httptest.NewUnstartedServer(principal.JWKSHandlerKeys([]principal.JWKSKey{{KeyID: "current", PublicKey: &key.PublicKey}, {KeyID: "next", PublicKey: &next.PublicKey}}))
	jwks.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: roots}
	jwks.StartTLS()
	t.Cleanup(jwks.Close)
	replay := miniredis.RunT(t)
	cfg := socialprincipal.Config{Target: "user", Capability: "catalog", JWKSURLs: map[string]string{"chat": jwks.URL, "file": jwks.URL}, RefreshAfter: 30 * time.Second, HardExpiry: 2 * time.Minute, UnknownKIDCooldown: 5 * time.Second, ReplayAddr: replay.Addr(), JWKSCAFile: certPath, TLSCertFile: certPath, TLSKeyFile: keyPath, ListenAddr: "127.0.0.1:0", ClientCAFile: certPath, ClientCAs: roots, JWKSClientCertFile: certPath, JWKSClientKeyFile: keyPath}
	start := func() (*grpc.ClientConn, string) {
		runtime, err := socialprincipal.New(ctx, cfg)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, runtime.Close()) })
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		server := grpc.NewServer(runtime.ServerOptions()...)
		RegisterCatalogActorServer(server, nil)
		require.Len(t, server.GetServiceInfo(), 1)
		require.Contains(t, server.GetServiceInfo(), "voice.catalogactor.v1.CatalogActorAuthorityService")
		go func() { _ = server.Serve(listener) }()
		t.Cleanup(server.Stop)
		conn, err := grpc.NewClient(listener.Addr().String(), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: "localhost", Certificates: []tls.Certificate{cert}})))
		require.NoError(t, err)
		t.Cleanup(func() { _ = conn.Close() })
		return conn, listener.Addr().String()
	}
	conn, address := start()
	second, _ := start()
	client := catalogv1.NewCatalogActorAuthorityServiceClient(conn)
	req := catalogRequest()
	issue := func(issuer string, request *catalogv1.GetCatalogActorStatesRequest) context.Context {
		signer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: issuer, KeyID: "current", PrivateKey: key})
		require.NoError(t, err)
		hash, err := principal.RequestHash(request)
		require.NoError(t, err)
		rid := uuid.NewString()
		token, err := signer.IssueService(principal.ServiceInput{Audience: "user", RPC: catalogv1.CatalogActorAuthorityService_GetCatalogActorStates_FullMethodName, RequestID: rid, RequestHash: hash})
		require.NoError(t, err)
		return metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", rid))
	}
	for _, issuer := range []string{"chat", "file"} {
		_, err = client.GetCatalogActorStates(issue(issuer, req), req)
		require.Equal(t, codes.Unavailable, status.Code(err), "valid principal reaches unavailable store")
	}
	signed := issue("chat", req)
	_, err = client.GetCatalogActorStates(signed, req)
	require.Equal(t, codes.Unavailable, status.Code(err))
	_, err = catalogv1.NewCatalogActorAuthorityServiceClient(second).GetCatalogActorStates(signed, req)
	require.Equal(t, codes.Unauthenticated, status.Code(err), "replay rejected across replicas")
	_, err = client.GetCatalogActorStates(issue("messaging", req), req)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	_, err = client.GetCatalogActorStates(ctx, req)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	_, err = client.GetCatalogActorStates(issue("chat", req), catalogRequest())
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	_, err = client.GetCatalogActorStates(metadata.AppendToOutgoingContext(issue("chat", req), "x-voice-internal-caller", "chat"), req)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	_, err = userv1.NewUserServiceClient(conn).GetProfileDeletionStates(ctx, &userv1.GetProfileDeletionStatesRequest{})
	require.Equal(t, codes.Unimplemented, status.Code(err))
	plain, err := grpc.NewClient(address, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: "localhost"})))
	require.NoError(t, err)
	t.Cleanup(func() { _ = plain.Close() })
	short, cancel := context.WithTimeout(issue("chat", req), time.Second)
	defer cancel()
	_, err = catalogv1.NewCatalogActorAuthorityServiceClient(plain).GetCatalogActorStates(short, req)
	require.Error(t, err, "signed credential needs mTLS identity")
	ordinary := grpc.NewServer()
	userv1.RegisterUserServiceServer(ordinary, &UserGRPC{})
	require.NotContains(t, ordinary.GetServiceInfo(), "voice.catalogactor.v1.CatalogActorAuthorityService")
	ordinary.Stop()
	replay.Close()
	_, err = client.GetCatalogActorStates(issue("file", req), req)
	require.Equal(t, codes.Unavailable, status.Code(err))
}
