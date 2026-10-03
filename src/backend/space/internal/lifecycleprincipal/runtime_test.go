package lifecycleprincipal

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
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
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
)

type transportCA struct {
	certificate *x509.Certificate
	key         *rsa.PrivateKey
	pem         []byte
}

func newTransportCA(t *testing.T, serial int64) transportCA {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	template := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: "lifecycle test CA"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	return transportCA{cert, key, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})}
}
func (ca transportCA) leaf(t *testing.T, serial int64, usage x509.ExtKeyUsage) (tls.Certificate, string, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	template := &x509.Certificate{SerialNumber: big.NewInt(serial), NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), BasicConstraintsValid: true, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{usage}, DNSNames: []string{"localhost"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}}
	der, err := x509.CreateCertificate(rand.Reader, template, ca.certificate, &key.PublicKey, ca.key)
	require.NoError(t, err)
	dir := t.TempDir()
	certFile, keyFile := filepath.Join(dir, "leaf.crt"), filepath.Join(dir, "leaf.key")
	require.NoError(t, os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600))
	require.NoError(t, os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}), 0600))
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	require.NoError(t, err)
	return cert, certFile, keyFile
}

type lifecycleTransportServer struct {
	spacev1.UnimplementedSpaceServiceServer
	calls atomic.Int32
}

func (s *lifecycleTransportServer) RestoreSpace(ctx context.Context, _ *spacev1.RestoreSpaceRequest) (*spacev1.RestoreSpaceResponse, error) {
	p, ok := principal.FromContext(ctx)
	if !ok || p.Kind != "delegated_user" || p.Issuer != "gateway" {
		return nil, status.Error(codes.Internal, "verified delegated actor missing")
	}
	s.calls.Add(1)
	return &spacev1.RestoreSpaceResponse{}, nil
}

func TestRealLifecycleTLSJWKSReplayAndRevocation(t *testing.T) {
	serverCA, clientCA, foreignCA := newTransportCA(t, 1), newTransportCA(t, 2), newTransportCA(t, 3)
	serverCert, certFile, keyFile := serverCA.leaf(t, 10, x509.ExtKeyUsageServerAuth)
	clientCert, _, _ := clientCA.leaf(t, 11, x509.ExtKeyUsageClientAuth)
	foreignCert, _, _ := foreignCA.leaf(t, 12, x509.ExtKeyUsageClientAuth)
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(serverCA.pem))
	dir := t.TempDir()
	serverCAFile, clientCAFile := filepath.Join(dir, "server-ca.crt"), filepath.Join(dir, "client-ca.crt")
	require.NoError(t, os.WriteFile(serverCAFile, serverCA.pem, 0600))
	require.NoError(t, os.WriteFile(clientCAFile, clientCA.pem, 0600))
	keys := map[string]*rsa.PrivateKey{}
	for _, kid := range []string{"current", "next"} {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		keys[kid] = key
	}
	document := []map[string]string{}
	for kid, key := range keys {
		document = append(document, map[string]string{"kty": "RSA", "kid": kid, "alg": "RS256", "use": "sig", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": "AQAB"})
	}
	jwks := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": document})
	}))
	jwks.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{serverCert}}
	jwks.StartTLS()
	defer jwks.Close()
	replay := miniredis.RunT(t)
	cfg := Config{Listen: "127.0.0.1:0", CertFile: certFile, KeyFile: keyFile, ClientCAFile: clientCAFile, JWKSURL: jwks.URL, JWKSCAFile: serverCAFile, RedisAddr: replay.Addr()}
	runtime, err := New(context.Background(), cfg)
	require.NoError(t, err)
	defer runtime.Close()
	listener, err := net.Listen("tcp", cfg.Listen)
	require.NoError(t, err)
	service := &lifecycleTransportServer{}
	server := grpc.NewServer(runtime.ServerOptions()...)
	spacev1.RegisterSpaceServiceServer(server, service)
	go func() { _ = server.Serve(listener) }()
	defer server.Stop()
	connect := func(certs []tls.Certificate) spacev1.SpaceServiceClient {
		conn, err := grpc.NewClient(listener.Addr().String(), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, Certificates: certs})))
		require.NoError(t, err)
		t.Cleanup(func() { _ = conn.Close() })
		return spacev1.NewSpaceServiceClient(conn)
	}
	client := connect([]tls.Certificate{clientCert})
	req := &spacev1.RestoreSpaceRequest{SpaceId: uuid.NewString(), OperationId: uuid.NewString()}
	account, profile := uuid.NewString(), uuid.NewString()
	replay.Set("auth:session:min_epoch:"+account, "7")
	sign := func(kid string) context.Context {
		hash, err := principal.RequestHash(req)
		require.NoError(t, err)
		issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: kid, PrivateKey: keys[kid]})
		require.NoError(t, err)
		token, err := issuer.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "space", RPC: spacev1.SpaceService_RestoreSpace_FullMethodName, RequestID: req.OperationId, RequestHash: hash, AccountID: account, ProfileID: profile, SessionEpoch: 7, ClientExpiresAt: time.Now().Add(time.Minute)})
		require.NoError(t, err)
		return metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", req.OperationId))
	}
	for _, kid := range []string{"current", "next"} {
		ctx := sign(kid)
		_, err := client.RestoreSpace(ctx, req)
		require.NoError(t, err)
		_, err = client.RestoreSpace(ctx, req)
		require.Equal(t, codes.Unauthenticated, status.Code(err))
	}
	require.Equal(t, int32(2), service.calls.Load())
	// Exact business retries use fresh credentials; a changed request is denied.
	ctx := sign("current")
	changed := &spacev1.RestoreSpaceRequest{SpaceId: uuid.NewString(), OperationId: req.OperationId}
	_, err = client.RestoreSpace(ctx, changed)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	ctx = metadata.AppendToOutgoingContext(sign("current"), "x-voice-profile-id", profile)
	_, err = client.RestoreSpace(ctx, req)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	_, err = client.CreateSpace(sign("current"), &spacev1.CreateSpaceRequest{Name: "denied"})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	for _, certs := range [][]tls.Certificate{nil, {foreignCert}} {
		ctx, cancel := context.WithTimeout(sign("current"), time.Second)
		_, err = connect(certs).RestoreSpace(ctx, req)
		cancel()
		require.Error(t, err)
	}
	// Another runtime after restart still sees the permanent shared replay mark.
	ctx = sign("current")
	_, err = client.RestoreSpace(ctx, req)
	require.NoError(t, err)
	second, err := New(context.Background(), cfg)
	require.NoError(t, err)
	defer second.Close()
	md, _ := metadata.FromOutgoingContext(ctx)
	_, err = second.verifier.Unary()(metadata.NewIncomingContext(context.Background(), md), req, &grpc.UnaryServerInfo{FullMethod: spacev1.SpaceService_RestoreSpace_FullMethodName}, func(context.Context, any) (any, error) {
		t.Fatal("replay after restart reached handler")
		return nil, nil
	})
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	replay.Set("auth:session:min_epoch:"+account, "8")
	_, err = client.RestoreSpace(sign("current"), req)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	replay.Del("auth:session:min_epoch:" + account)
	_, err = client.RestoreSpace(sign("current"), req)
	require.Equal(t, codes.Unavailable, status.Code(err))
	require.Equal(t, int32(3), service.calls.Load())
	replay.Close()
	_, err = client.RestoreSpace(sign("current"), req)
	require.Equal(t, codes.Unavailable, status.Code(err))
	require.Equal(t, int32(3), service.calls.Load())
}
