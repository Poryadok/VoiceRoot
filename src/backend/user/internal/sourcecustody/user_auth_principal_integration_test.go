package sourcecustody

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	identityv1 "voice.app/voice/identityerasure/v1"
	"voice/backend/pkg/principal"
	"voice/backend/user/internal/identityassociation"
	"voice/backend/user/internal/store"
	"voice/backend/user/internal/testsupport/associationfixture"
)

type userAuthContractServer struct {
	identityv1.UnimplementedAuthUserIdentityAuthorityServiceServer
}

func (*userAuthContractServer) ResolveAccountHandle(_ context.Context, req *identityv1.ResolveAccountHandleRequest) (*identityv1.ResolveAccountHandleResponse, error) {
	now := time.Now().UTC()
	digest := make([]byte, 32)
	digest[0] = 1
	return &identityv1.ResolveAccountHandleResponse{ProtocolVersion: 1, RequestId: req.RequestId, AccountId: req.AccountId,
		Window: &identityv1.ObservationWindow{ObservedAt: timestamppb.New(now.Add(-time.Second)), ValidUntil: timestamppb.New(now.Add(time.Minute))},
		Lifecycle: &identityv1.AccountLifecycleFact{OwnerHandle: uuid.NewString(), LifecycleRevision: 1, State: identityv1.AccountLifecycleState_ACCOUNT_LIFECYCLE_STATE_ACTIVE,
			Eligibility: identityv1.Eligibility_ELIGIBILITY_ELIGIBLE, Disposition: identityv1.ErasureDisposition_ERASURE_DISPOSITION_NOT_IRREVERSIBLE,
			OwnerCut: &identityv1.OwnerCut{Owner: "auth", StreamId: uuid.NewString(), HistoryGeneration: uuid.NewString(), ThroughRevision: 1, SelectorKeysetRevision: 1, SelectorKeyVersions: []string{"1"}, FactCount: 1, Digest: digest, Complete: true}}}, nil
}

func TestUserSignerJWKSAndProtectedAuthParentIntegration(t *testing.T) {
	fixture := newCustodyTransportFixture(t)
	custody, err := OpenClient(context.Background(), fixture.cfg)
	require.NoError(t, err)
	defer func() { require.NoError(t, custody.Close()) }()
	pki := processTLS(t, t.TempDir(), "user-auth")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	jwks, err := startUserJWKS(ctx, "127.0.0.1:0", pki.serverCert, pki.serverKey, custody.JWKS(), true)
	require.NoError(t, err)
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 3*time.Second)
		defer stop()
		require.NoError(t, jwks.Close(cleanup))
	}()
	httpClient := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: pki.roots, ServerName: "localhost"}}, Timeout: 2 * time.Second}
	defer httpClient.CloseIdleConnections()
	resolver := func(ctx context.Context, issuer, kid string) (*rsa.PublicKey, error) {
		if issuer != "user" {
			return nil, errors.New("wrong issuer")
		}
		req, e := http.NewRequestWithContext(ctx, http.MethodGet, "https://"+jwks.Address()+"/.well-known/jwks.json", nil)
		if e != nil {
			return nil, e
		}
		response, e := httpClient.Do(req)
		if e != nil {
			return nil, e
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return nil, errors.New("JWKS unavailable")
		}
		body, e := io.ReadAll(io.LimitReader(response.Body, 65537))
		if e != nil || len(body) > 65536 {
			return nil, errors.New("invalid JWKS")
		}
		keys, e := principal.ParseJWKS(body)
		if e != nil || keys[kid] == nil {
			return nil, errors.New("unknown key")
		}
		return keys[kid], nil
	}
	interceptor := func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		md, _ := metadata.FromIncomingContext(ctx)
		auth, ids := md.Get("authorization"), md.Get("x-request-id")
		req, ok := request.(*identityv1.ResolveAccountHandleRequest)
		if !ok || len(auth) != 1 || len(ids) != 1 || !strings.HasPrefix(auth[0], "Bearer ") || ids[0] != req.RequestId {
			return nil, status.Error(codes.Unauthenticated, "invalid principal")
		}
		hash, e := principal.RequestHash(req)
		if e != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid request")
		}
		_, e = principal.VerifyService(ctx, strings.TrimPrefix(auth[0], "Bearer "), principal.VerifyConfig{ExpectedIssuer: "user", ExpectedAudience: "auth", ExpectedRPC: info.FullMethod, ExpectedRequestID: ids[0], ExpectedRequestHash: hash, KeyResolver: resolver})
		if e != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid principal")
		}
		return handler(ctx, request)
	}
	serverCert, err := tls.LoadX509KeyPair(pki.serverCert, pki.serverKey)
	require.NoError(t, err)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := grpc.NewServer(grpc.Creds(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{serverCert}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: pki.roots})), grpc.UnaryInterceptor(interceptor))
	identityv1.RegisterAuthUserIdentityAuthorityServiceServer(server, &userAuthContractServer{})
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { server.Stop(); _ = listener.Close() })
	dir := t.TempDir()
	certFile, keyFile := filepath.Join(dir, "client.pem"), filepath.Join(dir, "client.key")
	require.NoError(t, os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: pki.client.Certificate[0]}), 0600))
	keyDER, err := x509.MarshalPKCS8PrivateKey(pki.client.PrivateKey)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0600))
	parent, err := identityassociation.OpenAuthParentRuntime(ctx, identityassociation.AuthParentRuntimeConfig{Address: listener.Addr().String(), ServerName: "localhost", CAFile: pki.caFile, CertFile: certFile, KeyFile: keyFile}, custody.Issuer())
	require.NoError(t, err)
	defer func() { require.NoError(t, parent.Close()) }()
	account := uuid.New()
	verifier, err := identityassociation.NewParentVerifier(parent.Client)
	require.NoError(t, err)
	_, err = verifier.Verify(ctx, account)
	require.NoError(t, err)
	t.Run("guarded profile birth", func(t *testing.T) {
		db := associationfixture.New(t, 1)
		receipt, err := associationfixture.ReadInstallerReceipt(db.InstallerReceiptPath)
		require.NoError(t, err)
		keys, err := identityassociation.NewKeyset(identityassociation.KeysetSnapshot{Revision: 1, ActiveVersion: receipt.KeyVersion, Versions: map[uint32][32]byte{receipt.KeyVersion: receipt.KeyFingerprint}}, map[uint32][]byte{receipt.KeyVersion: receipt.UserKey})
		require.NoError(t, err)
		identities, err := identityassociation.NewFreshStore(ctx, db.Pool, keys, identityassociation.FreshSourcePin{HistoryGeneration: receipt.HistoryGeneration, UserSourceGeneration: receipt.UserSourceGeneration})
		require.NoError(t, err)
		require.NoError(t, identities.ActivateGuards(ctx), "test-only local guard activation is not a deployment qualification")
		profiles, err := store.NewProfileStoreWithBirthCapture(db.Pool, identities, verifier)
		require.NoError(t, err)
		owner := db.Accounts[db.Profiles[0]]
		created, err := profiles.CreateSecondaryProfile(ctx, owner, "Protected parent birth", nil, nil)
		require.NoError(t, err)
		var origin string
		require.NoError(t, db.Pool.QueryRow(ctx, `SELECT a.capture_origin FROM profiles p JOIN identity_associations a ON a.association_id=p.identity_association_id WHERE p.id=$1`, created.ID).Scan(&origin))
		require.Equal(t, "BIRTH", origin)
	})
}
