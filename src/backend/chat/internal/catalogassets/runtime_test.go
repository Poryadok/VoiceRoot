package catalogassets

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
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/pkg/principal"
)

type catalogTransportFixture struct {
	chatv1.UnimplementedCatalogAssetServiceServer
	calls atomic.Int32
}

type transportCA struct {
	certificate *x509.Certificate
	key         *rsa.PrivateKey
	pem         []byte
}

func newTransportCA(t *testing.T, serial int64) transportCA {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	template := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: "catalog test CA"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature}
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

func (s *catalogTransportFixture) AuthorizeCatalogDestination(context.Context, *chatv1.AuthorizeCatalogDestinationRequest) (*chatv1.AuthorizeCatalogDestinationResponse, error) {
	s.calls.Add(1)
	return &chatv1.AuthorizeCatalogDestinationResponse{}, nil
}

func TestCatalogRealMTLSJWKSAndSharedReplay(t *testing.T) {
	serverCA, clientCA, foreignCA := newTransportCA(t, 101), newTransportCA(t, 102), newTransportCA(t, 103)
	serverCert, certFile, keyFile := serverCA.leaf(t, 110, x509.ExtKeyUsageServerAuth)
	clientCert, clientCertFile, clientKeyFile := clientCA.leaf(t, 111, x509.ExtKeyUsageClientAuth)
	foreignCert, _, _ := foreignCA.leaf(t, 112, x509.ExtKeyUsageClientAuth)
	dir := t.TempDir()
	serverCAFile, clientCAFile := filepath.Join(dir, "server.crt"), filepath.Join(dir, "client.crt")
	require.NoError(t, os.WriteFile(serverCAFile, serverCA.pem, 0600))
	require.NoError(t, os.WriteFile(clientCAFile, clientCA.pem, 0600))
	roots, clients := x509.NewCertPool(), x509.NewCertPool()
	roots.AppendCertsFromPEM(serverCA.pem)
	clients.AppendCertsFromPEM(clientCA.pem)
	keys := map[string]*rsa.PrivateKey{}
	document := []map[string]string{}
	for _, kid := range []string{"current", "next"} {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		keys[kid] = key
		document = append(document, map[string]string{"kty": "RSA", "kid": kid, "alg": "RS256", "use": "sig", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": "AQAB"})
	}
	jwks := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": document})
	}))
	jwks.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{serverCert}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: clients}
	jwks.StartTLS()
	defer jwks.Close()
	replay := miniredis.RunT(t)
	cfg := ListenerConfig{Listen: "127.0.0.1:0", CertFile: certFile, KeyFile: keyFile, ClientCAFile: clientCAFile, GatewayJWKS: jwks.URL + "/gateway", MessagingJWKS: jwks.URL + "/messaging", FileJWKS: jwks.URL + "/file", JWKSCAFile: serverCAFile, JWKSClientCertFile: clientCertFile, JWKSClientKeyFile: clientKeyFile, RedisAddr: replay.Addr()}
	runtime, err := NewListenerRuntime(context.Background(), cfg, func(context.Context, string, int64) error { return nil })
	require.NoError(t, err)
	defer runtime.Close()
	listener, err := net.Listen("tcp", cfg.Listen)
	require.NoError(t, err)
	fixture := &catalogTransportFixture{}
	server := grpc.NewServer(runtime.ServerOptions()...)
	chatv1.RegisterCatalogAssetServiceServer(server, fixture)
	go func() { _ = server.Serve(listener) }()
	defer server.Stop()
	connect := func(certs []tls.Certificate) chatv1.CatalogAssetServiceClient {
		conn, err := grpc.NewClient(listener.Addr().String(), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, Certificates: certs})))
		require.NoError(t, err)
		t.Cleanup(func() { _ = conn.Close() })
		return chatv1.NewCatalogAssetServiceClient(conn)
	}
	client := connect([]tls.Certificate{clientCert})
	req := &chatv1.AuthorizeCatalogDestinationRequest{ProtocolVersion: 1, ActorAccountId: uuid.NewString(), ActorProfileId: uuid.NewString(), DestinationChat: &chatv1.ChatRef{Id: uuid.NewString()}}
	sign := func(kid, issuerName string) context.Context {
		issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: issuerName, KeyID: kid, PrivateKey: keys[kid]})
		require.NoError(t, err)
		hash, err := principal.RequestHash(req)
		require.NoError(t, err)
		rid := uuid.NewString()
		token, err := issuer.IssueService(principal.ServiceInput{Audience: "chat", RPC: chatv1.CatalogAssetService_AuthorizeCatalogDestination_FullMethodName, RequestID: rid, RequestHash: hash})
		require.NoError(t, err)
		return metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", rid))
	}
	for _, kid := range []string{"current", "next"} {
		ctx := sign(kid, "file")
		_, err = client.AuthorizeCatalogDestination(ctx, req)
		require.NoError(t, err)
		_, err = client.AuthorizeCatalogDestination(ctx, req)
		require.Equal(t, codes.Unauthenticated, status.Code(err))
	}
	_, err = client.AuthorizeCatalogDestination(sign("current", "messaging"), req)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	for _, certs := range [][]tls.Certificate{nil, {foreignCert}} {
		ctx, cancel := context.WithTimeout(sign("current", "file"), time.Second)
		_, err = connect(certs).AuthorizeCatalogDestination(ctx, req)
		cancel()
		require.Error(t, err)
	}
	require.Equal(t, int32(2), fixture.calls.Load())
	replay.Close()
	_, err = client.AuthorizeCatalogDestination(sign("current", "file"), req)
	require.Equal(t, codes.Unavailable, status.Code(err))
	require.Equal(t, int32(2), fixture.calls.Load())
}
