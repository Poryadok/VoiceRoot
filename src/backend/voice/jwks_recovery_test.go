package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
	"voice/backend/pkg/socialprincipal"
	"voice/backend/pkg/spacecommunication"
	"voice/backend/voice/internal/principaljwks"
	"voice/backend/voice/internal/s2s"
	"voice/backend/voice/internal/spacemedia"
	"voice/backend/voice/internal/store"
)

type recoverySpaceAuthority struct {
	spacev1.UnimplementedSpaceServiceServer
	calls atomic.Int32
}

func (s *recoverySpaceAuthority) ResolveVoiceRoomAccess(_ context.Context, req *spacev1.ResolveVoiceRoomAccessRequest) (*spacev1.ResolveVoiceRoomAccessResponse, error) {
	s.calls.Add(1)
	return &spacev1.ResolveVoiceRoomAccessResponse{ProtocolVersion: 1, SpaceId: req.GetSpace().GetId(), ProfileId: req.GetProfileId(),
		Active: true, Member: true, Discoverable: true, AccessEpoch: 4, EvaluatedAt: timestamppb.Now(), CommunicationState: spacev1.MemberCommunicationState_MEMBER_COMMUNICATION_STATE_CLEAR}, nil
}

// This regression uses a real persisted COMMITTED/ACTIVE admission, production
// reconciliation, real signed mTLS Space client/verifier and a cold HTTPS JWKS
// cache. Space's domain decision, Role/Auth/media and Redis are explicit test
// fixtures; this is not full Space/Voice/provider acceptance.
func TestNonemptyAdmissionRecoveryNeedsPublishedVoiceJWKS(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL admission recovery integration")
	}
	ctx := t.Context()
	pool := compositionPostgres(t, ctx)
	admissions := spacemedia.NewPostgresAdmissionStore(pool)
	admission := compositionAdmission(t, ctx, admissions)
	redisClient := redis.NewClient(&redis.Options{Addr: miniredis.RunT(t).Addr()})
	t.Cleanup(func() { _ = redisClient.Close() })
	calls := store.NewRedisCallStore(redisClient, uuid.NewString()+":")
	coordinator := &spacemedia.Coordinator{ProjectionGates: calls, Store: calls, Calls: calls, Admissions: admissions,
		SessionEpochChecker: compositionAuthority{}, Grants: compositionAuthority{}, Media: &compositionMedia{}}
	var ready atomic.Bool
	coordinator.OnReadiness = ready.Store
	cfg, caPath, clientCert, clientKey, spaceCert, spaceKey, roots := recoveryTLS(t)
	// Reserve a known currently absent endpoint; the first reconciliation models
	// the legacy root's former publisher-after-recovery ordering.
	reserved, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	cfg.ListenAddr = reserved.Addr().String()
	require.NoError(t, reserved.Close())
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "voice", KeyID: cfg.KeyID, PrivateKey: cfg.SigningKey})
	require.NoError(t, err)
	authority := &recoverySpaceAuthority{}
	withColdSpaceVerifier := func(check func()) {
		t.Helper()
		replay := miniredis.RunT(t)
		runtime, err := socialprincipal.New(ctx, socialprincipal.Config{Target: "space", Capability: "space-voice-communication",
			JWKSURLs:     map[string]string{"voice": "https://" + cfg.ListenAddr + "/internal/v1/principal/jwks.json"},
			RefreshAfter: 30 * time.Second, HardExpiry: 2 * time.Minute, UnknownKIDCooldown: time.Second,
			ReplayAddr: replay.Addr(), JWKSCAFile: caPath, TLSCertFile: spaceCert, TLSKeyFile: spaceKey, ListenAddr: "127.0.0.1:0",
			ClientCAFile: caPath, ClientCAs: roots, JWKSClientCertFile: clientCert, JWKSClientKeyFile: clientKey})
		require.NoError(t, err)
		defer func() { _ = runtime.Close() }()
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		server := grpc.NewServer(runtime.ServerOptions()...)
		spacev1.RegisterSpaceServiceServer(server, authority)
		go func() { _ = server.Serve(listener) }()
		defer server.Stop()
		conn, err := (spacecommunication.ClientConfig{Address: listener.Addr().String(), CAFile: caPath, CertFile: clientCert, KeyFile: clientKey}).Dial()
		require.NoError(t, err)
		defer func() { _ = conn.Close() }()
		coordinator.Access = s2s.NewGRPCVoiceRoomAccessResolver(spacev1.NewSpaceServiceClient(conn), issuer)
		check()
	}
	withColdSpaceVerifier(func() {
		require.Error(t, coordinator.ReconcileAll(ctx), "absent real issuer keys must keep recovery closed")
		require.False(t, ready.Load())
		require.Zero(t, authority.calls.Load(), "no unauthenticated domain decision may run")
	})
	publisher, err := principaljwks.Start(cfg, nil)
	require.NoError(t, err)
	defer func() { _ = publisher.Close() }()
	withColdSpaceVerifier(func() {
		require.NoError(t, coordinator.ReconcileAll(ctx))
		require.True(t, ready.Load())
		require.Positive(t, authority.calls.Load(), "nonempty recovery must cross real authentication")
	})
	recovered, err := calls.GetCall(ctx, admission.RoomID)
	require.NoError(t, err)
	require.Equal(t, admission.Generation, recovered.SpaceMedia[admission.ProfileID.String()].Generation)
	var committed, touched int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM voice_space_media_admissions WHERE operation_id=$1 AND state='COMMITTED' AND participant_state='ACTIVE'`, admission.OperationID).Scan(&committed))
	require.Equal(t, 1, committed)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM voice_roster_outbox WHERE lease_token IS NOT NULL OR delivered_at IS NOT NULL`).Scan(&touched))
	require.Zero(t, touched, "startup key publication cannot activate the native roster relay")
}

func recoveryTLS(t *testing.T) (principaljwks.Config, string, string, string, string, string, *x509.CertPool) {
	t.Helper()
	dir := t.TempDir()
	write := func(name string, data []byte) string {
		path := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(path, data, 0600))
		return path
	}
	key := func() *rsa.PrivateKey {
		k, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		return k
	}
	caKey := key()
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "recovery.test"}, IsCA: true,
		BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour)}
	caDER, err := x509.CreateCertificate(rand.Reader, ca, ca, &caKey.PublicKey, caKey)
	require.NoError(t, err)
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})
	caPath := write("ca.pem", caPEM)
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(caPEM))
	leaf := func(name string, usage x509.ExtKeyUsage) (string, string) {
		k := key()
		certificate := &x509.Certificate{SerialNumber: big.NewInt(time.Now().UnixNano()), Subject: pkix.Name{CommonName: name},
			IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, NotBefore: ca.NotBefore, NotAfter: ca.NotAfter,
			KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment, ExtKeyUsage: []x509.ExtKeyUsage{usage}}
		der, err := x509.CreateCertificate(rand.Reader, certificate, ca, &k.PublicKey, caKey)
		require.NoError(t, err)
		return write(name+".crt", pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})),
			write(name+".key", pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(k)}))
	}
	voiceCert, voiceKey := leaf("voice", x509.ExtKeyUsageServerAuth)
	spaceCert, spaceKey := leaf("space", x509.ExtKeyUsageServerAuth)
	clientCert, clientKey := leaf("voice-client", x509.ExtKeyUsageClientAuth)
	return principaljwks.Config{KeyID: "current", SigningKey: key(), NextKeyID: "next", NextSigningKey: key(), TLSCert: voiceCert, TLSKey: voiceKey},
		caPath, clientCert, clientKey, spaceCert, spaceKey, roots
}
