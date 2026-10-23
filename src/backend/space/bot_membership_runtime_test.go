package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
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
	"voice/backend/space/internal/botmembershipprincipal"
)

// This sink checks the real private capability; its acknowledgement proves
// transport dispatch only. Actual typed effects belong to the separate real
// migrated database fixture, not this listener component test.
type runtimeBotStore struct{ calls atomic.Int64 }

func (s *runtimeBotStore) AddBotMemberWithInstallation(_ context.Context, proof *botmembershipprincipal.VerifiedInstallation, req *spacev1.AddBotMemberRequest) error {
	if _, err := proof.Check(req); err != nil {
		return err
	}
	s.calls.Add(1)
	return nil
}
func (s *runtimeBotStore) RemoveBotMemberWithInstallation(context.Context, *botmembershipprincipal.VerifiedInstallation, *spacev1.RemoveBotMemberRequest) error {
	s.calls.Add(1)
	return errors.New("unexpected cancellation in listener component test")
}

func botRuntimeTransportFixture(t *testing.T) (botmembershipprincipal.Config, *tls.Config, *principal.Issuer) {
	t.Helper()
	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "owned component CA"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	caDER, err := x509.CreateCertificate(rand.Reader, ca, ca, &caKey.PublicKey, caKey)
	require.NoError(t, err)
	ca, err = x509.ParseCertificate(caDER)
	require.NoError(t, err)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	leaf := &x509.Certificate{SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "owned loopback"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, NotBefore: ca.NotBefore, NotAfter: ca.NotAfter, KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}}
	der, err := x509.CreateCertificate(rand.Reader, leaf, ca, &key.PublicKey, caKey)
	require.NoError(t, err)
	private, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	certPEM, keyPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: private})
	identity, err := tls.X509KeyPair(certPEM, keyPEM)
	require.NoError(t, err)
	roots := x509.NewCertPool()
	roots.AddCert(ca)
	current, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	next, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	jwks := httptest.NewUnstartedServer(principal.JWKSHandlerKeys([]principal.JWKSKey{{KeyID: "current", PublicKey: &current.PublicKey}, {KeyID: "next", PublicKey: &next.PublicKey}}))
	jwks.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{identity}, ClientCAs: roots, ClientAuth: tls.RequireAndVerifyClientCert}
	jwks.StartTLS()
	t.Cleanup(jwks.Close)
	dir := t.TempDir()
	certFile, keyFile, caFile := filepath.Join(dir, "leaf.crt"), filepath.Join(dir, "leaf.key"), filepath.Join(dir, "ca.crt")
	require.NoError(t, os.WriteFile(certFile, certPEM, 0600))
	require.NoError(t, os.WriteFile(keyFile, keyPEM, 0600))
	require.NoError(t, os.WriteFile(caFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}), 0600))
	replay := miniredis.RunT(t)
	config := botmembershipprincipal.Config{ListenAddr: "127.0.0.1:0", CertFile: certFile, KeyFile: keyFile, ClientCAFile: caFile, ReplayAddr: replay.Addr(), JWKSURLs: map[string]string{"bot": jwks.URL}, JWKSCAFile: caFile, JWKSClientCertFile: certFile, JWKSClientKeyFile: keyFile, RefreshAfter: time.Second, HardExpiry: time.Minute, UnknownKIDCooldown: time.Second}
	signer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "bot", KeyID: "current", PrivateKey: current})
	require.NoError(t, err)
	return config, &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, Certificates: []tls.Certificate{identity}, ServerName: "127.0.0.1"}, signer
}

func TestBotMembershipRuntimeOwnsProtectedListenerAndShutdown(t *testing.T) {
	config, clientTLS, signer := botRuntimeTransportFixture(t)
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	persistence := &runtimeBotStore{}
	runtime, err := newBotMembershipRuntime(parent, config, persistence)
	require.NoError(t, err)
	t.Cleanup(runtime.Stop)
	address := runtime.listener.Addr().String()
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(credentials.NewTLS(clientTLS)))
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()
	ctx, done := context.WithTimeout(parent, 3*time.Second)
	defer done()
	_, err = spacev1.NewSpaceServiceClient(conn).GetSpace(ctx, &spacev1.GetSpaceRequest{})
	require.Equal(t, codes.PermissionDenied, status.Code(err), "other methods must not reach the normal Space handler")
	_, err = spacev1.NewSpaceServiceClient(conn).AddBotMember(ctx, &spacev1.AddBotMemberRequest{})
	require.Error(t, err, "mTLS alone cannot mint a private installation capability")
	operation, space, profile := uuid.NewString(), uuid.NewString(), uuid.NewString()
	req := &spacev1.AddBotMemberRequest{SpaceId: space, ProfileId: profile, InstallationIntent: &spacev1.BotInstallationIntent{OperationId: operation, BotId: uuid.NewString(), InstallationId: uuid.NewString(), SpaceId: space, ActorProfileId: profile, Generation: 1}}
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	token, err := signer.IssueService(principal.ServiceInput{Audience: "space", RPC: spacev1.SpaceService_AddBotMember_FullMethodName, RequestID: operation, RequestHash: hash})
	require.NoError(t, err)
	_, err = spacev1.NewSpaceServiceClient(conn).AddBotMember(metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", operation)), req)
	require.NoError(t, err, "real mTLS/JWKS/Redis verifier must dispatch its private capability to the dedicated adapter")
	require.EqualValues(t, 1, persistence.calls.Load())
	withoutClient := clientTLS.Clone()
	withoutClient.Certificates = nil
	unauth, err := grpc.NewClient(address, grpc.WithTransportCredentials(credentials.NewTLS(withoutClient)))
	require.NoError(t, err)
	defer func() { _ = unauth.Close() }()
	_, err = spacev1.NewSpaceServiceClient(unauth).GetSpace(ctx, &spacev1.GetSpaceRequest{})
	require.Error(t, err, "client certificate is mandatory")
	require.EqualValues(t, 1, persistence.calls.Load(), "denied calls must not dispatch to persistence")
	cancel()
	select {
	case <-runtime.done:
	case <-time.After(3 * time.Second):
		t.Fatal("owned listener did not stop with parent")
	}
	runtime.Stop()
	runtime.Stop()
	listener, err := net.Listen("tcp", address)
	require.NoError(t, err, "owned port must be released")
	require.NoError(t, listener.Close())
}

func TestBotMembershipRuntimeClosesPreboundListenerWhenDependencyFails(t *testing.T) {
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	address := probe.Addr().String()
	require.NoError(t, probe.Close())
	config := botmembershipprincipal.Config{ListenAddr: address, CertFile: "absent", KeyFile: "absent", ClientCAFile: "absent", ReplayAddr: "127.0.0.1:1", JWKSURLs: map[string]string{"bot": "https://bot.test/jwks"}, RefreshAfter: time.Second, HardExpiry: time.Minute, UnknownKIDCooldown: time.Second}
	runtime, err := newBotMembershipRuntime(context.Background(), config, &runtimeBotStore{})
	require.Error(t, err)
	require.Nil(t, runtime)
	listener, err := net.Listen("tcp", address)
	require.NoError(t, err)
	require.NoError(t, listener.Close())
	_, err = newBotMembershipRuntime(context.Background(), config, nil)
	require.Error(t, err)
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = newBotMembershipRuntime(canceled, config, &runtimeBotStore{})
	require.Error(t, err)
}

func TestBotMembershipRuntimeReportsServeFailureAndClosesOwnedResources(t *testing.T) {
	config, _, _ := botRuntimeTransportFixture(t)
	runtime, err := newBotMembershipRuntime(context.Background(), config, &runtimeBotStore{})
	require.NoError(t, err)
	t.Cleanup(runtime.Stop)
	require.NoError(t, runtime.listener.Close())
	select {
	case err := <-runtime.Errors():
		require.Error(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("serve failure was not supervised")
	}
	runtime.Stop()
}

func TestBotMembershipRuntimeConfigurationIsExplicitAndFailClosed(t *testing.T) {
	names := []string{"SPACE_BOT_MEMBERSHIP_GRPC_LISTEN", "SPACE_BOT_MEMBERSHIP_TLS_CERT_FILE", "SPACE_BOT_MEMBERSHIP_TLS_KEY_FILE", "SPACE_BOT_MEMBERSHIP_CLIENT_CA_FILE", "SPACE_BOT_MEMBERSHIP_REPLAY_REDIS_ADDR", "SPACE_BOT_MEMBERSHIP_REPLAY_REDIS_PASSWORD"}
	for _, name := range names {
		old, present := os.LookupEnv(name)
		require.NoError(t, os.Unsetenv(name))
		t.Cleanup(func() {
			if present {
				_ = os.Setenv(name, old)
			} else {
				_ = os.Unsetenv(name)
			}
		})
	}
	t.Setenv("S2S_JWKS_URLS_JSON", `{"bot":"https://bot.test/jwks"}`)
	_, enabled, err := botmembershipprincipal.LoadFromEnv()
	require.NoError(t, err)
	require.False(t, enabled, "shared JWKS settings cannot implicitly activate this listener")
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			t.Setenv(name, "")
			_, enabled, err := botmembershipprincipal.LoadFromEnv()
			require.True(t, enabled)
			require.Error(t, err, "present but blank or partial owned configuration must fail startup")
		})
	}
}
