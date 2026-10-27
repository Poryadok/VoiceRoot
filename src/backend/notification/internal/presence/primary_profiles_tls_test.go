package presence

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
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/principal"
	"voice/backend/pkg/socialprincipal"
)

type primaryTLSServer struct {
	userv1.UnimplementedUserServiceServer
	profile string
}

func (s primaryTLSServer) ResolvePrimaryProfileIDs(ctx context.Context, req *userv1.ResolvePrimaryProfileIDsRequest) (*userv1.ResolvePrimaryProfileIDsResponse, error) {
	if err := socialprincipal.RequireNotificationPrimary(ctx, req); err != nil {
		return nil, err
	}
	return &userv1.ResolvePrimaryProfileIDsResponse{PrimaryProfileIds: map[string]string{req.GetAccountIds()[0]: s.profile}}, nil
}

func TestPrimaryProfilesActualMutualTLSAndRequestBoundCall(t *testing.T) {
	key := testSigningKey(t)
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "primary.test"}, DNSNames: []string{"primary.test"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	dir := t.TempDir()
	certPath, keyPath := filepath.Join(dir, "test.pem"), filepath.Join(dir, "test.key")
	require.NoError(t, os.WriteFile(certPath, certPEM, 0600))
	require.NoError(t, os.WriteFile(keyPath, keyPEM, 0600))
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	require.NoError(t, err)
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(certPEM))
	signer := testSigningKey(t)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "notification", KeyID: "current", PrivateKey: signer})
	require.NoError(t, err)
	verifier := &socialprincipal.Verifier{Target: "user", Capability: "notification-primary", Issuers: map[string]bool{"notification": true}, Resolve: func(context.Context, string, string) (*rsa.PublicKey, error) { return &signer.PublicKey, nil }, Replay: func(context.Context, string, string, time.Time) error { return nil }}
	server := grpc.NewServer(grpc.Creds(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: roots})), grpc.ChainUnaryInterceptor(socialprincipal.StrictUnaryInterceptor(verifier)))
	profile := uuid.NewString()
	userv1.RegisterUserServiceServer(server, primaryTLSServer{profile: profile})
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)
	cfg := PrimaryProfilesConfig{Address: listener.Addr().String(), ServerName: "primary.test", CAFile: certPath, ClientCertFile: certPath, ClientKeyFile: keyPath}
	checker := &GRPCChecker{issuer: issuer}
	reader, err := checker.NewPrimaryProfiles(cfg)
	require.NoError(t, err)
	t.Cleanup(func() { _ = reader.Close() })
	got, err := reader.PrimaryProfileForAccount(context.Background(), uuid.New())
	require.NoError(t, err)
	require.Equal(t, profile, got.String())
	cfg.ServerName = "wrong.test"
	bad, err := checker.NewPrimaryProfiles(cfg)
	require.NoError(t, err)
	defer func() { _ = bad.Close() }()
	_, err = bad.PrimaryProfileForAccount(context.Background(), uuid.New())
	require.Error(t, err, "untrusted server identity cannot choose a primary")
	cfg.ClientCertFile = ""
	_, err = checker.NewPrimaryProfiles(cfg)
	require.Error(t, err, "client workload certificate is mandatory")
}
