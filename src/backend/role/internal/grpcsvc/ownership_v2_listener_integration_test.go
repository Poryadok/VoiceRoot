package grpcsvc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
	"google.golang.org/protobuf/proto"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
	"voice/backend/role/internal/principalruntime"
)

func TestOwnershipTransferV2_ProtectedListenerMigrationBackedRestart(t *testing.T) {
	if testing.Short() {
		t.Skip("requires isolated PostgreSQL")
	}
	roleStore, cleanup := startRoleStoreTest(t)
	defer cleanup()
	oldOwner, newOwner, spaceID, operationID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, roleStore.BootstrapSpaceRoles(context.Background(), spaceID, oldOwner))
	fixture := newOwnershipV2RuntimeFixture(t)
	intent := &rolev1.OwnershipTransferIntent{
		ProtocolVersion: 2, SpaceId: spaceID.String(), OldOwnerProfileId: oldOwner.String(),
		NewOwnerProfileId: newOwner.String(), OperationId: operationID.String(),
	}
	prepare := &rolev1.PrepareOwnershipTransferRequest{Intent: intent}
	prepare.ProtoReflect().SetUnknown([]byte{0xa8, 0x06, 0x01})

	firstRuntime, err := principalruntime.New(context.Background(), fixture.config)
	require.NoError(t, err)
	firstClient, firstStop := startOwnershipV2ProtectedServer(t, firstRuntime, fixture.roots, &RoleGRPC{Store: roleStore})
	prepareContext, prepareToken := ownershipV2SignedContext(t, fixture.key, rolev1.RoleService_PrepareOwnershipTransfer_FullMethodName, "prepare-before-restart", prepare)
	prepared, err := firstClient.PrepareOwnershipTransfer(prepareContext, prepare)
	require.NoError(t, err)
	require.Equal(t, rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_PREPARED, prepared.GetReceipt().GetState())
	firstStop()
	require.NoError(t, firstRuntime.Close())

	secondRuntime, err := principalruntime.New(context.Background(), fixture.config)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, secondRuntime.Close()) })
	secondClient, secondStop := startOwnershipV2ProtectedServer(t, secondRuntime, fixture.roots, &RoleGRPC{Store: roleStore})
	defer secondStop()
	replayed := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+prepareToken, "x-request-id", "prepare-before-restart"))
	_, err = secondClient.PrepareOwnershipTransfer(replayed, prepare)
	require.Equal(t, codes.Unauthenticated, status.Code(err), "shared Redis replay state must survive Role restart")
	prepareReplayContext, _ := ownershipV2SignedContext(t, fixture.key, rolev1.RoleService_PrepareOwnershipTransfer_FullMethodName, "prepare-after-restart", prepare)
	preparedReplay, err := secondClient.PrepareOwnershipTransfer(prepareReplayContext, prepare)
	require.NoError(t, err)
	require.True(t, proto.Equal(prepared, preparedReplay), "fresh credential must replay the durable prepared receipt")
	requireSoleTerminalOwner(t, roleStore, spaceID, oldOwner)

	finalize := &rolev1.FinalizeOwnershipTransferRequest{Intent: intent}
	finalizeContext, _ := ownershipV2SignedContext(t, fixture.key, rolev1.RoleService_FinalizeOwnershipTransfer_FullMethodName, "finalize-after-restart", finalize)
	finalized, err := secondClient.FinalizeOwnershipTransfer(finalizeContext, finalize)
	require.NoError(t, err)
	require.Equal(t, rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_FINALIZED, finalized.GetReceipt().GetState())
	require.Equal(t, newOwner.String(), finalized.GetReceipt().GetCurrentOwnerProfileId())
	requireSoleTerminalOwner(t, roleStore, spaceID, newOwner)
}

type ownershipV2RuntimeFixture struct {
	config principalruntime.Config
	roots  *x509.CertPool
	key    *rsa.PrivateKey
}

func newOwnershipV2RuntimeFixture(t *testing.T) ownershipV2RuntimeFixture {
	t.Helper()
	current, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	next, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	jwk := func(kid string, key *rsa.PrivateKey) map[string]string {
		return map[string]string{"kid": kid, "kty": "RSA", "use": "sig", "alg": "RS256", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes())}
	}
	jwks := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{jwk("current", current), jwk("next", next)}})
	}))
	t.Cleanup(jwks.Close)
	dir := t.TempDir()
	certFile, keyFile := filepath.Join(dir, "tls.crt"), filepath.Join(dir, "tls.key")
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: jwks.TLS.Certificates[0].Certificate[0]})
	require.NoError(t, os.WriteFile(certFile, certPEM, 0o600))
	keyDER, err := x509.MarshalPKCS8PrivateKey(jwks.TLS.Certificates[0].PrivateKey)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0o600))
	replay := miniredis.RunT(t)
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(certPEM))
	return ownershipV2RuntimeFixture{
		config: principalruntime.Config{
			JWKSURLs: map[string]string{"space": jwks.URL}, RefreshAfter: time.Minute, HardExpiry: 2 * time.Minute,
			UnknownKIDCooldown: time.Second, ReplayAddr: replay.Addr(), JWKSCAFile: certFile,
			TLSCertFile: certFile, TLSKeyFile: keyFile, ListenAddr: "127.0.0.1:0",
		},
		roots: roots,
		key:   current,
	}
}

func startOwnershipV2ProtectedServer(t *testing.T, runtime *principalruntime.Runtime, roots *x509.CertPool, service rolev1.RoleServiceServer) (rolev1.RoleServiceClient, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := grpc.NewServer(runtime.ServerOptions()...)
	rolev1.RegisterRoleServiceServer(server, service)
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	conn, err := grpc.NewClient(listener.Addr().String(), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots})))
	require.NoError(t, err)
	stop := func() {
		require.NoError(t, conn.Close())
		server.Stop()
		<-done
	}
	return rolev1.NewRoleServiceClient(conn), stop
}

func ownershipV2SignedContext(t *testing.T, key *rsa.PrivateKey, rpc, requestID string, req proto.Message) (context.Context, string) {
	t.Helper()
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "space", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	token, err := issuer.IssueService(principal.ServiceInput{Audience: "role", RPC: rpc, RequestID: requestID, RequestHash: hash})
	require.NoError(t, err)
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID))
	return ctx, token
}
