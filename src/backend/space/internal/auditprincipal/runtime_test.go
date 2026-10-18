package auditprincipal

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
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
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	spacev1 "voice.app/voice/space/v1"
)

func auditTLSMaterial(t *testing.T) (string, string, string, tls.Certificate, *x509.CertPool) {
	t.Helper()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "audit-test-ca"}, IsCA: true, BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageCertSign, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour)}
	caDER, err := x509.CreateCertificate(rand.Reader, ca, ca, &caKey.PublicKey, caKey)
	require.NoError(t, err)
	dir := t.TempDir()
	caFile := filepath.Join(dir, "ca.pem")
	require.NoError(t, os.WriteFile(caFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}), 0600))
	issue := func(name string, serial int64, usage x509.ExtKeyUsage) (string, string, tls.Certificate) {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		require.NoError(t, err)
		leaf := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: name}, DNSNames: []string{"localhost"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
			KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{usage}, NotBefore: ca.NotBefore, NotAfter: ca.NotAfter}
		der, err := x509.CreateCertificate(rand.Reader, leaf, ca, &key.PublicKey, caKey)
		require.NoError(t, err)
		pk, err := x509.MarshalPKCS8PrivateKey(key)
		require.NoError(t, err)
		certFile, keyFile := filepath.Join(dir, name+".pem"), filepath.Join(dir, name+"-key.pem")
		require.NoError(t, os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600))
		require.NoError(t, os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pk}), 0600))
		pair, err := tls.LoadX509KeyPair(certFile, keyFile)
		require.NoError(t, err)
		return certFile, keyFile, pair
	}
	certFile, keyFile, _ := issue("server", 2, x509.ExtKeyUsageServerAuth)
	_, _, client := issue("client", 3, x509.ExtKeyUsageClientAuth)
	parsedCA, err := x509.ParseCertificate(caDER)
	require.NoError(t, err)
	roots := x509.NewCertPool()
	roots.AddCert(parsedCA)
	return certFile, keyFile, caFile, client, roots
}

type runtimeAuditService struct {
	spacev1.UnimplementedSpaceServiceServer
	calls atomic.Int64
}

func (s *runtimeAuditService) AppendAuditEvent(ctx context.Context, req *spacev1.AppendAuditEventRequest) (*spacev1.AppendAuditEventResponse, error) {
	if _, ok := Producer(ctx, req); !ok {
		return nil, status.Error(codes.Unauthenticated, "no verified producer")
	}
	s.calls.Add(1)
	return &spacev1.AppendAuditEventResponse{}, nil
}

func TestAuditRuntimeTLSJWKSThenReplayAndDependencyFailure(t *testing.T) {
	certFile, keyFile, caFile, clientCert, roots := auditTLSMaterial(t)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	peer, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	keys := []map[string]string{}
	for i, k := range []*rsa.PrivateKey{key, peer} {
		kid := "current"
		if i == 1 {
			kid = "next"
		}
		keys = append(keys, map[string]string{"kty": "RSA", "kid": kid, "alg": "RS256", "use": "sig", "n": base64.RawURLEncoding.EncodeToString(k.N.Bytes()), "e": "AQAB"})
	}
	jwks := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"keys": keys}))
	}))
	serverIdentity, err := tls.LoadX509KeyPair(certFile, keyFile)
	require.NoError(t, err)
	jwks.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{serverIdentity}}
	jwks.StartTLS()
	t.Cleanup(jwks.Close)
	redis := miniredis.RunT(t)
	cfg := Config{ListenAddr: "127.0.0.1:0", CertFile: certFile, KeyFile: keyFile, ClientCAFile: caFile, ReplayAddr: redis.Addr(), JWKSCAFile: caFile,
		JWKSURLs: map[string]string{"role": jwks.URL + "/role", "chat": jwks.URL + "/chat"}, RefreshAfter: 30 * time.Second, HardExpiry: 2 * time.Minute, UnknownKIDCooldown: 5 * time.Second}
	runtime, err := New(context.Background(), cfg)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, runtime.Close()) })
	server := grpc.NewServer(runtime.ServerOptions()...)
	t.Cleanup(server.Stop)
	service := &runtimeAuditService{}
	spacev1.RegisterSpaceServiceServer(server, service)
	listener, err := net.Listen("tcp", cfg.ListenAddr)
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })
	go func() { _ = server.Serve(listener) }()
	connect := func(certs []tls.Certificate) spacev1.SpaceServiceClient {
		conn, err := grpc.NewClient(listener.Addr().String(), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: "localhost", Certificates: certs})))
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, conn.Close()) })
		return spacev1.NewSpaceServiceClient(conn)
	}
	client := connect([]tls.Certificate{clientCert})
	request := auditRequest()
	signer := auditSigner(t, "role", key, time.Now())
	ctx := signedContext(t, signer, request, nil)
	md, _ := metadata.FromIncomingContext(ctx)
	ctx = metadata.NewOutgoingContext(context.Background(), md)
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, err = client.AppendAuditEvent(ctx, request)
	require.NoError(t, err)
	require.EqualValues(t, 1, service.calls.Load())
	_, err = client.AppendAuditEvent(ctx, request)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	require.EqualValues(t, 1, service.calls.Load())
	_, err = client.GetSpace(ctx, &spacev1.GetSpaceRequest{})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	withoutCert := connect(nil)
	denied, cancelDenied := context.WithTimeout(ctx, time.Second)
	defer cancelDenied()
	_, err = withoutCert.AppendAuditEvent(denied, request)
	require.Error(t, err)
	require.EqualValues(t, 1, service.calls.Load(), "missing client certificate cannot reach handler")
	redis.Close()
	fresh := signedContext(t, signer, request, nil)
	freshMD, _ := metadata.FromIncomingContext(fresh)
	fresh = metadata.NewOutgoingContext(context.Background(), freshMD)
	fresh, cancelFresh := context.WithTimeout(fresh, 5*time.Second)
	defer cancelFresh()
	_, err = client.AppendAuditEvent(fresh, request)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	require.EqualValues(t, 1, service.calls.Load(), "Redis failure must not fail open")
}
