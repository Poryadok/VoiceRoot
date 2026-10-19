package main

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"voice/backend/moderation/internal/automod"
	"voice/backend/pkg/integrationtest"
	authorityv1 "voice/backend/pkg/pb/voice/authority/v1"
	automodv1 "voice/backend/pkg/pb/voice/automod/v1"
	"voice/backend/pkg/principal"
	"voice/backend/pkg/socialprincipal"
)

func TestAutoModP0_PrivateListenerNeedsNoPeerOrDomainReadiness(t *testing.T) {
	f := integrationtest.NewSourceTLSFixture(t, authorityv1.AuthorityOwner_AUTHORITY_OWNER_USER)
	directory := t.TempDir()
	clientCert, clientKey := filepath.Join(directory, "client.pem"), filepath.Join(directory, "client.key")
	require.NoError(t, os.WriteFile(clientCert, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: f.ClientCertificate.Certificate[0]}), 0600))
	der, err := x509.MarshalPKCS8PrivateKey(f.ClientCertificate.PrivateKey)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(clientKey, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0600))
	clientCA, err := os.ReadFile(f.Config.ClientCAFile)
	require.NoError(t, err)
	clients := x509.NewCertPool()
	require.True(t, clients.AppendCertsFromPEM(clientCA))
	cfg := autoModAuthorityConfig{Principal: socialprincipal.Config{Target: "moderation", Capability: "automod-evidence", JWKSURLs: map[string]string{"messaging": f.Config.JWKSURL, "auth": f.Config.JWKSURL}, RefreshAfter: 30 * time.Second, HardExpiry: time.Minute, UnknownKIDCooldown: time.Second, ReplayAddr: f.Config.ReplayAddr, JWKSCAFile: f.Config.JWKSCAFile, TLSCertFile: f.Config.TLSCertFile, TLSKeyFile: f.Config.TLSKeyFile, ListenAddr: "127.0.0.1:0", ClientCAFile: f.Config.ClientCAFile, ClientCAs: clients, JWKSClientCertFile: clientCert, JWKSClientKeyFile: clientKey}, AuthAddr: "127.0.0.1:1", AuthServerName: "localhost", AuthCAFile: f.Config.JWKSCAFile, AuthCertFile: clientCert, AuthKeyFile: clientKey}
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "moderation", KeyID: "current", PrivateKey: f.Current})
	require.NoError(t, err)
	g := automod.NewBootGate()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runtime, err := newAutoModAuthorityRuntime(ctx, cfg, automod.NewStore(nil), issuer, g)
	require.NoError(t, err)
	defer runtime.Close()
	require.Same(t, issuer, runtime.Issuer)
	require.Same(t, issuer, runtime.Auth.Issuer)
	connection, err := grpc.NewClient(runtime.listener.Addr().String(), grpc.WithTransportCredentials(f.ClientCredentials()))
	require.NoError(t, err)
	defer func() { _ = connection.Close() }()
	client := automodv1.NewModerationEvidenceServiceClient(connection)
	request := &automodv1.GetEffectiveRestrictionSnapshotRequest{AccountId: uuid.NewString(), SubjectId: uuid.NewString()}
	call, cancelCall := context.WithTimeout(ctx, 2*time.Second)
	defer cancelCall()
	_, err = client.GetEffectiveRestrictionSnapshot(call, request)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	authIssuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "auth", KeyID: "current", PrivateKey: f.Current})
	require.NoError(t, err)
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	requestID := uuid.NewString()
	token, err := authIssuer.IssueService(principal.ServiceInput{Audience: "moderation", RPC: automodv1.ModerationEvidenceService_GetEffectiveRestrictionSnapshot_FullMethodName, RequestID: requestID, RequestHash: hash})
	require.NoError(t, err)
	_, err = client.GetEffectiveRestrictionSnapshot(metadata.NewOutgoingContext(call, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID)), request)
	require.Equal(t, codes.Unavailable, status.Code(err), "private route is discoverable but fresh gate forbids DB/peer effects")
	require.Contains(t, status.Convert(err).Message(), "current-boot")
	bootID, generation := g.Identity()
	for phase := automod.BootLocalIssuer; phase <= automod.BootTerminalImported; phase++ {
		require.NoError(t, g.Advance(automod.BootReceipt{BootID: bootID, Generation: generation, Phase: phase, ProofID: uuid.New()}))
	}
	captured, err := g.AcquirePrivateRepair(ctx)
	require.NoError(t, err)
	cancel()
	select {
	case <-runtime.done:
	case <-time.After(3 * time.Second):
		t.Fatal("owned P0 listener did not stop")
	}
	require.ErrorIs(t, captured(context.Background()), automod.ErrBootClosed)
	partial := cfg
	partial.AuthKeyFile = ""
	_, err = newAutoModAuthorityRuntime(context.Background(), partial, automod.NewStore(nil), issuer, automod.NewBootGate())
	require.Error(t, err)
}

func TestAutoModConfig_OrphanedAuthSettingsNeverEnablePlaintextFallback(t *testing.T) {
	t.Setenv("MODERATION_AUTOMOD_AUTH_GRPC_ADDR", "voice-auth-authority:9093")
	_, err := loadAutoModAuthorityConfig()
	require.Error(t, err)
}
