package principalruntime

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/metadata"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	botv1 "voice.app/voice/bot/v1"
	commonv1 "voice.app/voice/common/v1"
	"voice/backend/bot/internal/principalgrpc"
	"voice/backend/pkg/principal"
)

type sendAuthorityServer struct {
	botv1.UnimplementedBotServiceServer
	calls atomic.Int32
}

func (s *sendAuthorityServer) CheckBotMessageSendAuthority(ctx context.Context, r *botv1.CheckBotMessageSendAuthorityRequest) (*botv1.CheckBotMessageSendAuthorityResponse, error) {
	s.calls.Add(1)
	return &botv1.CheckBotMessageSendAuthorityResponse{Binding: r.Binding, Allowed: true}, nil
}

func TestMessagingAuthorityMTLSJWKSRecoveryAndReplay(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	uri, _ := url.Parse("spiffe://voice.local/phase0/messaging")
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test authority"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, URIs: []*url.URL{uri}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	require.NoError(t, err)
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(certPEM))
	dir := t.TempDir()
	certPath, keyPath := filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
	require.NoError(t, os.WriteFile(certPath, certPEM, 0600))
	require.NoError(t, os.WriteFile(keyPath, keyPEM, 0600))
	var ready atomic.Bool
	provider := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NotEmpty(t, r.TLS.VerifiedChains)
		if !ready.Load() {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		principal.JWKSHandler("test", &key.PublicKey).ServeHTTP(w, r)
	}))
	provider.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: roots}
	provider.StartTLS()
	defer provider.Close()
	cfg := Config{BotSpaceLifecycleListen: "127.0.0.1:0", BotSpaceLifecycleClientCA: certPath, MessagingJWKSURL: provider.URL + "/.well-known/principal-jwks.json", MessagingJWKSCAFile: certPath, MessagingJWKSClientCert: certPath, MessagingJWKSClientKey: keyPath, MessagingURISAN: uri.String()}
	runtime := &Runtime{config: cfg, spaceCertificate: cert}
	require.NoError(t, runtime.initMessagingAuthority())
	defer runtime.Close()
	runtime.spaceResolver = runtime.messagingResolver
	var mu sync.Mutex
	used := map[string]bool{}
	runtime.replayGuard = func(_ context.Context, issuer, jti string, _ time.Time) error {
		mu.Lock()
		defer mu.Unlock()
		if used[issuer+jti] {
			return context.Canceled
		}
		used[issuer+jti] = true
		return nil
	}
	tlsConfig, err := runtime.SpaceLifecycleServerTLSConfig()
	require.NoError(t, err)
	server := grpc.NewServer(grpc.Creds(credentials.NewTLS(tlsConfig)), grpc.UnaryInterceptor(principalgrpc.SpaceLifecycleUnaryInterceptor(runtime)))
	service := &sendAuthorityServer{}
	botv1.RegisterBotServiceServer(server, service)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	go server.Serve(listener)
	defer server.Stop()
	connection, err := grpc.NewClient(listener.Addr().String(), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, Certificates: []tls.Certificate{cert}})))
	require.NoError(t, err)
	defer connection.Close()
	client := botv1.NewBotServiceClient(connection)
	signer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "messaging", KeyID: "test", PrivateKey: key})
	require.NoError(t, err)
	request := &botv1.CheckBotMessageSendAuthorityRequest{Binding: &commonv1.BotMessageSendBinding{ProtocolVersion: 1, BotId: uuid.NewString(), ActorProfileId: uuid.NewString(), SpaceId: uuid.NewString(), InstallationId: uuid.NewString(), InstallOperationId: uuid.NewString(), InstallationGeneration: 1, ChatId: uuid.NewString()}}
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	signed := func() context.Context {
		id := uuid.NewString()
		token, e := signer.IssueService(principal.ServiceInput{Audience: "bot", RPC: messagingSendAuthorityMethod, RequestID: id, RequestHash: hash})
		require.NoError(t, e)
		return metadata.NewOutgoingContext(t.Context(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id))
	}
	_, err = client.CheckBotMessageSendAuthority(signed(), request)
	require.Equal(t, codes.Unavailable, status.Code(err))
	require.Zero(t, service.calls.Load(), "unavailable keys must not reach handler")
	ready.Store(true)
	// Explicit refresh represents the same resolver's recovery, avoiding cooldown sleeps.
	require.NoError(t, runtime.messagingResolver.Refresh(t.Context(), "messaging"))
	call := signed()
	response, err := client.CheckBotMessageSendAuthority(call, request)
	require.NoError(t, err)
	require.True(t, response.Allowed)
	_, err = client.CheckBotMessageSendAuthority(call, request)
	require.Error(t, err)
	require.EqualValues(t, 1, service.calls.Load())
	foreignURI, err := url.Parse("spiffe://voice.local/phase0/foreign")
	require.NoError(t, err)
	foreignTemplate := *template
	foreignTemplate.SerialNumber = big.NewInt(2)
	foreignTemplate.IsCA = false
	foreignTemplate.URIs = []*url.URL{foreignURI}
	foreignDER, err := x509.CreateCertificate(rand.Reader, &foreignTemplate, template, &key.PublicKey, key)
	require.NoError(t, err)
	foreignCert, err := tls.X509KeyPair(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: foreignDER}), keyPEM)
	require.NoError(t, err)
	foreignConnection, err := grpc.NewClient(listener.Addr().String(), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, Certificates: []tls.Certificate{foreignCert}})))
	require.NoError(t, err)
	defer foreignConnection.Close()
	_, err = botv1.NewBotServiceClient(foreignConnection).CheckBotMessageSendAuthority(signed(), request)
	require.Error(t, err)
	require.EqualValues(t, 1, service.calls.Load())
}
