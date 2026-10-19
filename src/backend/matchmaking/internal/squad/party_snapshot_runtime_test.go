package squad

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	callsv1 "voice.app/voice/calls/v1"
	"voice/backend/matchmaking/internal/partysource"
	"voice/backend/pkg/principal"
)

var snapshotProfileNames = []string{
	"MATCHMAKING_VOICE_PARTY_SNAPSHOT_GRPC_ADDR", "MATCHMAKING_VOICE_PARTY_SNAPSHOT_CA_FILE",
	"MATCHMAKING_VOICE_PARTY_SNAPSHOT_CLIENT_CERT_FILE", "MATCHMAKING_VOICE_PARTY_SNAPSHOT_CLIENT_KEY_FILE",
	"MATCHMAKING_VOICE_PARTY_SNAPSHOT_SERVER_NAME", "MATCHMAKING_PRINCIPAL_SIGNING_KEYS_DIR", "MATCHMAKING_PRINCIPAL_ACTIVE_KID",
}

func TestPartySnapshotRuntimeProfileIsCompleteOrAbsent(t *testing.T) {
	client, closeClient, configured, err := LoadProtectedPartySnapshotReader(func(name string) string {
		if name == "MATCHMAKING_PRINCIPAL_SIGNING_KEYS_DIR" {
			return "existing-provider-keys"
		}
		return ""
	})
	require.NoError(t, err)
	require.False(t, configured)
	require.Nil(t, client)
	require.NoError(t, closeClient())
	for _, missing := range snapshotProfileNames {
		t.Run(missing, func(t *testing.T) {
			client, _, configured, err := LoadProtectedPartySnapshotReader(func(name string) string {
				if name == missing {
					return ""
				}
				for _, field := range snapshotProfileNames {
					if name == field {
						return "fixture-value"
					}
				}
				return ""
			})
			require.True(t, configured)
			require.Error(t, err)
			require.Nil(t, client)
		})
	}
}

type snapshotRuntimeServer struct {
	callsv1.UnimplementedVoiceServiceServer
	t     *testing.T
	keys  map[string]*rsa.PublicKey
	calls atomic.Int32
	mu    sync.Mutex
	seen  map[string]bool
}

func (s *snapshotRuntimeServer) GetMatchmakingPartySnapshot(ctx context.Context, req *callsv1.GetMatchmakingPartySnapshotRequest) (*callsv1.GetMatchmakingPartySnapshotResponse, error) {
	t := s.t
	transportPeer, ok := peer.FromContext(ctx)
	require.True(t, ok)
	info, ok := transportPeer.AuthInfo.(credentials.TLSInfo)
	require.True(t, ok)
	require.NotEmpty(t, info.State.VerifiedChains)
	require.Equal(t, "matchmaking-snapshot-client", info.State.PeerCertificates[0].Subject.CommonName)
	md, _ := metadata.FromIncomingContext(ctx)
	require.Empty(t, md.Get("x-voice-profile-id"))
	require.Empty(t, md.Get("x-voice-user-id"))
	transport, err := principal.IncomingMetadata(ctx)
	require.NoError(t, err)
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	verified, err := principal.VerifyService(ctx, transport.BearerToken, principal.VerifyConfig{ExpectedIssuer: "matchmaking", ExpectedAudience: "voice", ExpectedRPC: partysource.SnapshotMethod, ExpectedRequestID: req.SearchOperationId, ExpectedRequestHash: hash,
		KeyResolver: func(_ context.Context, issuer, kid string) (*rsa.PublicKey, error) {
			require.Equal(t, "matchmaking", issuer)
			return s.keys[kid], nil
		},
		ReplayGuard: func(_ context.Context, _ string, id string, _ time.Time) error {
			s.mu.Lock()
			defer s.mu.Unlock()
			require.False(t, s.seen[id], "each snapshot read needs a fresh service credential")
			s.seen[id] = true
			return nil
		},
	})
	require.NoError(t, err)
	require.Equal(t, "service:matchmaking", verified.Subject)
	require.Empty(t, verified.AccountID)
	require.Empty(t, verified.ProfileID)
	require.Zero(t, verified.SessionEpoch)
	s.calls.Add(1)
	return &callsv1.GetMatchmakingPartySnapshotResponse{ProtocolVersion: 1, Kind: callsv1.MatchmakingPartySnapshotKind_MATCHMAKING_PARTY_SNAPSHOT_KIND_SOLO, MemberProfileIds: []string{req.InitiatorProfileId}, MemberAuthorityVersion: 1, MemberAuthorities: []*callsv1.MatchmakingPartyMemberAuthority{{ProfileId: req.InitiatorProfileId, AccountId: req.InitiatorAccountId, AdmittedSessionEpoch: req.InitiatorSessionEpoch}}, SourceAuthoritySha256: append([]byte{2}, make([]byte, 31)...)}, nil
}

func TestPartySnapshotRuntimeUsesMutualTLSAndExactFreshServicePrincipal(t *testing.T) {
	dir := t.TempDir()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "snapshot fixture CA"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature}
	caDER, err := x509.CreateCertificate(rand.Reader, ca, ca, &caKey.PublicKey, caKey)
	require.NoError(t, err)
	caFile := filepath.Join(dir, "ca.pem")
	require.NoError(t, os.WriteFile(caFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}), 0o600))
	issue := func(serial int64, name string, usages []x509.ExtKeyUsage) (string, string) {
		key, keyErr := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		require.NoError(t, keyErr)
		cert := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: name}, DNSNames: []string{name}, NotBefore: ca.NotBefore, NotAfter: ca.NotAfter, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: usages}
		der, certErr := x509.CreateCertificate(rand.Reader, cert, ca, &key.PublicKey, caKey)
		require.NoError(t, certErr)
		keyDER, keyErr := x509.MarshalPKCS8PrivateKey(key)
		require.NoError(t, keyErr)
		certFile, keyFile := filepath.Join(dir, name+".crt"), filepath.Join(dir, name+".key")
		require.NoError(t, os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600))
		require.NoError(t, os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0o600))
		return certFile, keyFile
	}
	serverCert, serverKey := issue(2, "voice-snapshot", []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth})
	clientCert, clientKey := issue(3, "matchmaking-snapshot-client", []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth})
	certificate, err := tls.LoadX509KeyPair(serverCert, serverKey)
	require.NoError(t, err)
	parsedCA, err := x509.ParseCertificate(caDER)
	require.NoError(t, err)
	roots := x509.NewCertPool()
	roots.AddCert(parsedCA)
	keysDir := t.TempDir()
	writePrincipalRotationKeys(t, keysDir)
	keys, err := loadMatchmakingSigningKeys(keysDir, "current")
	require.NoError(t, err)
	service := &snapshotRuntimeServer{t: t, keys: map[string]*rsa.PublicKey{"current": &keys["current"].PublicKey}, seen: make(map[string]bool)}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := grpc.NewServer(grpc.Creds(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{certificate}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: roots})))
	callsv1.RegisterVoiceServiceServer(server, service)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { server.Stop(); _ = listener.Close() })
	values := map[string]string{snapshotProfileNames[0]: listener.Addr().String(), snapshotProfileNames[1]: caFile, snapshotProfileNames[2]: clientCert, snapshotProfileNames[3]: clientKey, snapshotProfileNames[4]: "voice-snapshot", snapshotProfileNames[5]: keysDir, snapshotProfileNames[6]: "current"}
	client, closeClient, configured, err := LoadProtectedPartySnapshotReader(func(name string) string { return values[name] })
	require.NoError(t, err)
	require.True(t, configured)
	require.NotNil(t, client)
	t.Cleanup(func() { require.NoError(t, closeClient()) })
	subject := partysource.Subject{SearchOperationID: uuid.New(), AccountID: uuid.New(), ProfileID: uuid.New(), SessionEpoch: 9}
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "Bearer untrusted", "x-voice-profile-id", uuid.NewString()))
	for i := 0; i < 2; i++ {
		cut, callErr := client.Snapshot(ctx, subject)
		require.NoError(t, callErr)
		require.Equal(t, "SOLO", cut.Kind)
		require.Equal(t, subject.AccountID, cut.MemberAuthorities[0].AccountID)
	}
	require.EqualValues(t, 2, service.calls.Load())
	values[snapshotProfileNames[4]] = "wrong-voice-name"
	wrong, closeWrong, _, err := LoadProtectedPartySnapshotReader(func(name string) string { return values[name] })
	require.NoError(t, err)
	defer closeWrong()
	bounded, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err = wrong.Snapshot(bounded, subject)
	require.Error(t, err)
	require.EqualValues(t, 2, service.calls.Load(), "wrong server identity cannot reach the application")
	values[snapshotProfileNames[4]] = "voice-snapshot"
	values[snapshotProfileNames[2]], values[snapshotProfileNames[3]] = serverCert, serverKey
	wrongClient, closeWrongClient, _, err := LoadProtectedPartySnapshotReader(func(name string) string { return values[name] })
	require.NoError(t, err)
	defer closeWrongClient()
	clientBounded, clientCancel := context.WithTimeout(context.Background(), time.Second)
	defer clientCancel()
	_, err = wrongClient.Snapshot(clientBounded, subject)
	require.Error(t, err)
	require.EqualValues(t, 2, service.calls.Load(), "a server-only certificate cannot authenticate a client")
}
