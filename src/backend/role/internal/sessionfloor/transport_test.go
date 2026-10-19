package sessionfloor

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	authv1 "voice.app/voice/auth/v1"
	"voice/backend/pkg/principal"
)

type signedFloorServer struct {
	authv1.UnimplementedAuthServiceServer
	key   *rsa.PublicKey
	reads atomic.Int64
	floor atomic.Int64
}

func (s *signedFloorServer) GetRoleSessionEpochFloor(ctx context.Context, req *authv1.GetRoleSessionEpochFloorRequest) (*authv1.GetRoleSessionEpochFloorResponse, error) {
	transport, err := principal.IncomingMetadata(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid principal")
	}
	hash, err := principal.RequestHash(req)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	_, err = principal.VerifyService(ctx, transport.BearerToken, principal.VerifyConfig{ExpectedIssuer: "role", ExpectedAudience: "auth", ExpectedRPC: RPC, ExpectedRequestID: transport.RequestID, ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return s.key, nil }})
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid principal")
	}
	s.reads.Add(1)
	return &authv1.GetRoleSessionEpochFloorResponse{SessionEpochFloor: s.floor.Load()}, nil
}

func floorCertificate(t *testing.T, parent *x509.Certificate, signer *ecdsa.PrivateKey, serial int64, usage x509.ExtKeyUsage) ([]byte, []byte, *x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	cert := &x509.Certificate{SerialNumber: big.NewInt(serial), NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), BasicConstraintsValid: true, KeyUsage: x509.KeyUsageDigitalSignature}
	if parent == nil {
		cert.IsCA = true
		cert.KeyUsage |= x509.KeyUsageCertSign
		parent, signer = cert, key
	} else {
		cert.ExtKeyUsage = []x509.ExtKeyUsage{usage}
		if usage == x509.ExtKeyUsageServerAuth {
			cert.DNSNames = []string{"auth.internal"}
		}
	}
	der, err := x509.CreateCertificate(rand.Reader, cert, parent, &key.PublicKey, signer)
	require.NoError(t, err)
	parsed, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	private, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: private}), parsed, key
}

func TestRoleFloorDialUsesMutualTLSAndVerifiedServerIdentity(t *testing.T) {
	caPEM, _, ca, caKey := floorCertificate(t, nil, nil, 1, 0)
	serverPEM, serverKey, _, _ := floorCertificate(t, ca, caKey, 2, x509.ExtKeyUsageServerAuth)
	clientPEM, clientKey, _, _ := floorCertificate(t, ca, caKey, 3, x509.ExtKeyUsageClientAuth)
	serverCertificate, err := tls.X509KeyPair(serverPEM, serverKey)
	require.NoError(t, err)
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(caPEM))
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := grpc.NewServer(grpc.Creds(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{serverCertificate}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: roots})))
	issuer, public := roleFloorIssuer(t)
	implementation := &signedFloorServer{key: public}
	implementation.floor.Store(4)
	authv1.RegisterAuthServiceServer(server, implementation)
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	t.Cleanup(func() { server.Stop(); <-done })
	dir := t.TempDir()
	config := Config{Address: listener.Addr().String(), CAFile: filepath.Join(dir, "ca.pem"), ServerName: "auth.internal", ClientCertFile: filepath.Join(dir, "client.pem"), ClientKeyFile: filepath.Join(dir, "client.key")}
	require.NoError(t, os.WriteFile(config.CAFile, caPEM, 0600))
	require.NoError(t, os.WriteFile(config.ClientCertFile, clientPEM, 0600))
	require.NoError(t, os.WriteFile(config.ClientKeyFile, clientKey, 0600))
	client, conn, err := config.Dial(issuer)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	account := uuid.NewString()
	require.NoError(t, client.RequireCurrent(t.Context(), account, 4), "production Dial must succeed through actual mTLS and signed Role RPC")
	implementation.floor.Store(5)
	require.Equal(t, codes.Unauthenticated, status.Code(client.RequireCurrent(t.Context(), account, 4)))
	require.Equal(t, int64(2), implementation.reads.Load())
	for _, name := range []string{"wrong server", "untrusted client"} {
		t.Run(name, func(t *testing.T) {
			bad := config
			if name == "wrong server" {
				bad.ServerName = "wrong.internal"
			} else {
				pemCert, pemKey, _, _ := floorCertificate(t, nil, nil, 4, 0)
				bad.ClientCertFile, bad.ClientKeyFile = filepath.Join(dir, "wrong.pem"), filepath.Join(dir, "wrong.key")
				require.NoError(t, os.WriteFile(bad.ClientCertFile, pemCert, 0600))
				require.NoError(t, os.WriteFile(bad.ClientKeyFile, pemKey, 0600))
			}
			client, conn, err := bad.Dial(issuer)
			require.NoError(t, err)
			defer conn.Close()
			ctx, cancel := context.WithTimeout(t.Context(), 150*time.Millisecond)
			defer cancel()
			_, err = client.CurrentFloor(ctx, account)
			require.Equal(t, codes.Unavailable, status.Code(err))
			require.Equal(t, int64(2), implementation.reads.Load(), "TLS rejection precedes floor handler")
		})
	}
}
