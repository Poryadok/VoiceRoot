package grpcsvc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/principal"
	"voice/backend/pkg/socialprincipal"
	"voice/backend/user/internal/store"
)

// This suite uses the real production TLS/JWKS/verifier/Redis admission and
// actual User handler/database, rather than constructing verified contexts.
func TestProfileDeletionStatesProtectedTransportPostgres(t *testing.T) {
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "deletiontransport", "")
	applyUserPrivacyMigrations(t, ctx, pool)
	id := uuid.New()
	seedScheduledPresenceProfile(t, ctx, pool, uuid.New(), id, false, "transport-profile")
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
	cfg := socialprincipal.Config{Target: "user", Capability: "messaging", JWKSURLs: map[string]string{"messaging": jwks.URL}, RefreshAfter: 30 * time.Second, HardExpiry: 2 * time.Minute, UnknownKIDCooldown: 5 * time.Second, ReplayAddr: replay.Addr(), JWKSCAFile: certPath, TLSCertFile: certPath, TLSKeyFile: keyPath, ListenAddr: "127.0.0.1:0", ClientCAFile: certPath, ClientCAs: roots, JWKSClientCertFile: certPath, JWKSClientKeyFile: keyPath}
	start := func() (userv1.UserServiceClient, string) {
		runtime, err := socialprincipal.New(ctx, cfg)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, runtime.Close()) })
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		server := grpc.NewServer(runtime.ServerOptions()...)
		RegisterMessagingScheduledPresenceServer(server, &UserGRPC{Profiles: store.NewProfileStore(pool)})
		go func() { _ = server.Serve(listener) }()
		t.Cleanup(server.Stop)
		conn, err := grpc.NewClient(listener.Addr().String(), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: "localhost", Certificates: []tls.Certificate{cert}})))
		require.NoError(t, err)
		t.Cleanup(func() { _ = conn.Close() })
		return userv1.NewUserServiceClient(conn), listener.Addr().String()
	}
	first, address := start()
	second, _ := start()
	signer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "messaging", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	req := &userv1.GetProfileDeletionStatesRequest{ProfileIds: []string{id.String(), uuid.NewString()}}
	issue := func(request *userv1.GetProfileDeletionStatesRequest) context.Context {
		hash, err := principal.RequestHash(request)
		require.NoError(t, err)
		requestID := uuid.NewString()
		token, err := signer.IssueService(principal.ServiceInput{Audience: "user", RPC: userv1.UserService_GetProfileDeletionStates_FullMethodName, RequestID: requestID, RequestHash: hash})
		require.NoError(t, err)
		return metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID))
	}
	signed := issue(req)
	response, err := first.GetProfileDeletionStates(signed, req)
	require.NoError(t, err)
	require.Equal(t, userv1.ProfileDeletionState_PROFILE_DELETION_STATE_NOT_DELETED, response.States[0].State)
	require.Equal(t, userv1.ProfileDeletionState_PROFILE_DELETION_STATE_UNKNOWN, response.States[1].State)
	_, err = second.GetProfileDeletionStates(signed, req)
	require.Equal(t, codes.Unauthenticated, status.Code(err), "replay is rejected across independent User runtimes sharing Redis")
	changed := &userv1.GetProfileDeletionStatesRequest{ProfileIds: []string{uuid.NewString()}}
	_, err = first.GetProfileDeletionStates(issue(req), changed)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	_, err = first.GetProfileDeletionStates(metadata.AppendToOutgoingContext(issue(req), "x-internal-caller", "messaging"), req)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	_, err = first.GetProfile(context.Background(), &userv1.GetProfileRequest{})
	require.Equal(t, codes.Unimplemented, status.Code(err), "unrelated profile lookup absent on protected descriptor")
	withoutCert, err := grpc.NewClient(address, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: "localhost"})))
	require.NoError(t, err)
	t.Cleanup(func() { _ = withoutCert.Close() })
	short, cancel := context.WithTimeout(issue(req), 2*time.Second)
	defer cancel()
	_, err = userv1.NewUserServiceClient(withoutCert).GetProfileDeletionStates(short, req)
	require.Error(t, err, "a signed request without a trusted client certificate cannot reach User")
	replay.Close()
	_, err = first.GetProfileDeletionStates(issue(req), req)
	require.Error(t, err, "shared replay outage fails closed")
}

func deletionTransportIdentity(t *testing.T) (tls.Certificate, *x509.CertPool, string, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	cert := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "private-deletion-fixture"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}, DNSNames: []string{"localhost"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}}
	der, err := x509.CreateCertificate(rand.Reader, cert, cert, &key.PublicKey, key)
	require.NoError(t, err)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	identity, err := tls.X509KeyPair(certPEM, keyPEM)
	require.NoError(t, err)
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(certPEM))
	dir := t.TempDir()
	certPath, keyPath := filepath.Join(dir, "identity.crt"), filepath.Join(dir, "identity.key")
	require.NoError(t, os.WriteFile(certPath, certPEM, 0600))
	require.NoError(t, os.WriteFile(keyPath, keyPEM, 0600))
	return identity, roots, certPath, keyPath
}
