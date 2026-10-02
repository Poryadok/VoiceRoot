package spaceprincipalruntime

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
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	notificationv1 "voice.app/voice/notification/v1"
	"voice/backend/notification/internal/principalgrpc"
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/principal"
)

type runtimeTestService struct {
	notificationv1.UnimplementedNotificationServiceServer
	calls atomic.Int32
}

func (s *runtimeTestService) ApplySpaceLifecycleFence(ctx context.Context, _ *notificationv1.ApplySpaceLifecycleFenceRequest) (*notificationv1.ApplySpaceLifecycleFenceResponse, error) {
	if _, ok := principal.FromContext(ctx); !ok {
		return nil, status.Error(codes.Unauthenticated, "missing verified principal")
	}
	s.calls.Add(1)
	return &notificationv1.ApplySpaceLifecycleFenceResponse{}, nil
}

func runtimeTestCA(t *testing.T) (*x509.Certificate, *rsa.PrivateKey, []byte) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	cert := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "lifecycle-test-ca"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	der, err := x509.CreateCertificate(rand.Reader, cert, cert, &key.PublicKey, key)
	require.NoError(t, err)
	cert, err = x509.ParseCertificate(der)
	require.NoError(t, err)
	return cert, key, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func runtimeTestLeaf(t *testing.T, ca *x509.Certificate, caKey *rsa.PrivateKey, client bool) (tls.Certificate, []byte, []byte) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	usage := x509.ExtKeyUsageServerAuth
	if client {
		usage = x509.ExtKeyUsageClientAuth
	}
	cert := &x509.Certificate{SerialNumber: big.NewInt(time.Now().UnixNano()), Subject: pkix.Name{CommonName: "notification-test"}, DNSNames: []string{"notification.test"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{usage}}
	der, err := x509.CreateCertificate(rand.Reader, cert, ca, &key.PublicKey, caKey)
	require.NoError(t, err)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	require.NoError(t, err)
	return pair, certPEM, keyPEM
}

func TestRuntimeMutualTLSJWKSReplayAndOrdinaryDenial(t *testing.T) {
	if testing.Short() {
		t.Skip("requires isolated Redis")
	}
	integrationtest.ConfigureDockerTesting()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	redisContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: testcontainers.ContainerRequest{Image: "redis:7-alpine", ExposedPorts: []string{"6379/tcp"}, WaitingFor: wait.ForListeningPort("6379/tcp")}, Started: true})
	require.NoError(t, err)
	terminated := false
	t.Cleanup(func() {
		if terminated {
			return
		}
		cleanup, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		require.NoError(t, redisContainer.Terminate(cleanup))
	})
	host, err := redisContainer.Host(ctx)
	require.NoError(t, err)
	port, err := redisContainer.MappedPort(ctx, "6379/tcp")
	require.NoError(t, err)
	current, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	next, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	jwks := httptest.NewTLSServer(principal.JWKSHandlerKeys([]principal.JWKSKey{{KeyID: "current", PublicKey: &current.PublicKey}, {KeyID: "next", PublicKey: &next.PublicKey}}))
	t.Cleanup(jwks.Close)
	ca, caKey, caPEM := runtimeTestCA(t)
	_, certPEM, keyPEM := runtimeTestLeaf(t, ca, caKey, false)
	clientCert, _, _ := runtimeTestLeaf(t, ca, caKey, true)
	foreignCA, foreignKey, _ := runtimeTestCA(t)
	foreignCert, _, _ := runtimeTestLeaf(t, foreignCA, foreignKey, true)
	write := func(name string, data []byte) string {
		path := filepath.Join(t.TempDir(), name)
		require.NoError(t, os.WriteFile(path, data, 0600))
		return path
	}
	cfg := Config{ListenAddr: "127.0.0.1:0", JWKSURL: jwks.URL, RefreshAfter: time.Minute, HardExpiry: 2 * time.Minute, UnknownKIDCooldown: time.Second, ReplayAddr: net.JoinHostPort(host, port.Port()), TLSCertFile: write("server.crt", certPEM), TLSKeyFile: write("server.key", keyPEM), ClientCAFile: write("client-ca.crt", caPEM), JWKSCAFile: write("jwks-ca.crt", pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: jwks.Certificate().Raw}))}
	runtime, err := New(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, runtime.Close()) })
	service := &runtimeTestService{}
	listen, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := grpc.NewServer(runtime.ServerOptions()...)
	notificationv1.RegisterNotificationServiceServer(server, service)
	go func() { _ = server.Serve(listen) }()
	t.Cleanup(server.Stop)
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(caPEM))
	connect := func(certs []tls.Certificate) notificationv1.NotificationServiceClient {
		conn, err := grpc.NewClient(listen.Addr().String(), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: "notification.test", Certificates: certs})))
		require.NoError(t, err)
		t.Cleanup(func() { _ = conn.Close() })
		return notificationv1.NewNotificationServiceClient(conn)
	}
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "space", KeyID: "current", PrivateKey: current})
	require.NoError(t, err)
	request := &notificationv1.ApplySpaceLifecycleFenceRequest{}
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	method := notificationv1.NotificationService_ApplySpaceLifecycleFence_FullMethodName
	credential := func(audience, requestHash string) context.Context {
		token, err := issuer.IssueService(principal.ServiceInput{Audience: audience, RPC: method, RequestID: "runtime-test", RequestHash: requestHash})
		require.NoError(t, err)
		return metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", "runtime-test"))
	}
	client := connect([]tls.Certificate{clientCert})
	signed := credential("notification", hash)
	_, err = client.ApplySpaceLifecycleFence(signed, request)
	require.NoError(t, err)
	_, err = client.ApplySpaceLifecycleFence(signed, request)
	require.Equal(t, codes.Unauthenticated, status.Code(err), "Redis rejects replay before dispatch")
	_, err = client.ApplySpaceLifecycleFence(credential("voice", hash), request)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	_, err = client.ApplySpaceLifecycleFence(credential("notification", "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"), request)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	_, err = client.GetQuietHours(ctx, &notificationv1.GetQuietHoursRequest{})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	for name, certs := range map[string][]tls.Certificate{"missing": nil, "foreign": {foreignCert}} {
		t.Run(name, func(t *testing.T) {
			callCtx, cancel := context.WithTimeout(credential("notification", hash), time.Second)
			defer cancel()
			_, err := connect(certs).ApplySpaceLifecycleFence(callCtx, request)
			require.Error(t, err)
		})
	}
	require.EqualValues(t, 1, service.calls.Load())
	ordinaryListener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	ordinary := grpc.NewServer(grpc.ChainUnaryInterceptor(principalgrpc.OrdinaryUnaryInterceptor()))
	notificationv1.RegisterNotificationServiceServer(ordinary, service)
	go func() { _ = ordinary.Serve(ordinaryListener) }()
	t.Cleanup(ordinary.Stop)
	conn, err := grpc.NewClient(ordinaryListener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	_, err = notificationv1.NewNotificationServiceClient(conn).ApplySpaceLifecycleFence(credential("notification", hash), request)
	require.Equal(t, codes.Unavailable, status.Code(err))
	require.EqualValues(t, 1, service.calls.Load())
	broken := cfg
	broken.ClientCAFile = write("bad-ca.crt", []byte("invalid"))
	_, err = New(ctx, broken)
	require.Error(t, err, "invalid client CA must fail startup")
	require.NoError(t, redisContainer.Terminate(ctx))
	terminated = true
	_, err = client.ApplySpaceLifecycleFence(credential("notification", hash), request)
	require.Equal(t, codes.Unavailable, status.Code(err), "missing replay store must fail closed")
	require.EqualValues(t, 1, service.calls.Load())
	_, err = New(ctx, cfg)
	require.Equal(t, codes.Unavailable, status.Code(err), "unavailable Redis must fail startup")
}
