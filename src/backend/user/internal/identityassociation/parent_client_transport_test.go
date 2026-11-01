package identityassociation

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"math/big"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	identityv1 "voice.app/voice/identityerasure/v1"
	"voice/backend/pkg/principal"
)

type parentTestPKI struct {
	roots          *x509.CertPool
	caDER          []byte
	server, client tls.Certificate
}

func makeParentTestPKI(t *testing.T) parentTestPKI {
	t.Helper()
	now := time.Now()
	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	caTemplate := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	der, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	require.NoError(t, err)
	ca, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	roots := x509.NewCertPool()
	roots.AddCert(ca)
	issue := func(serial int64, usage x509.ExtKeyUsage) tls.Certificate {
		key, e := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, e)
		leaf := &x509.Certificate{SerialNumber: big.NewInt(serial), DNSNames: []string{"auth.parent.test"}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{usage}}
		certificate, e := x509.CreateCertificate(rand.Reader, leaf, ca, &key.PublicKey, caKey)
		require.NoError(t, e)
		return tls.Certificate{Certificate: [][]byte{certificate}, PrivateKey: key}
	}
	return parentTestPKI{roots: roots, caDER: der, server: issue(2, x509.ExtKeyUsageServerAuth), client: issue(3, x509.ExtKeyUsageClientAuth)}
}

type parentContractServer struct {
	identityv1.UnimplementedAuthUserIdentityAuthorityServiceServer
}

func (*parentContractServer) ResolveAccountHandle(_ context.Context, req *identityv1.ResolveAccountHandleRequest) (*identityv1.ResolveAccountHandleResponse, error) {
	now := time.Now().UTC()
	return &identityv1.ResolveAccountHandleResponse{ProtocolVersion: 1, RequestId: req.RequestId, AccountId: req.AccountId,
		Window: &identityv1.ObservationWindow{ObservedAt: timestamppb.New(now.Add(-time.Second)), ValidUntil: timestamppb.New(now.Add(time.Minute))},
		Lifecycle: &identityv1.AccountLifecycleFact{OwnerHandle: uuid.NewString(), LifecycleRevision: 1, State: identityv1.AccountLifecycleState_ACCOUNT_LIFECYCLE_STATE_ACTIVE,
			Eligibility: identityv1.Eligibility_ELIGIBILITY_ELIGIBLE, Disposition: identityv1.ErasureDisposition_ERASURE_DISPOSITION_NOT_IRREVERSIBLE,
			OwnerCut: &identityv1.OwnerCut{Owner: "auth", StreamId: uuid.NewString(), HistoryGeneration: uuid.NewString(), ThroughRevision: 1, SelectorKeysetRevision: 1, SelectorKeyVersions: []string{"1"}, FactCount: 1, Digest: bytes32(1), Complete: true}}}, nil
}

func TestAuthParentClientMutualTLSAndSignedRequest(t *testing.T) {
	pki := makeParentTestPKI(t)
	signing, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "user", KeyID: "fixture", PrivateKey: signing})
	require.NoError(t, err)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	var mu sync.Mutex
	used := map[string]bool{}
	var captured metadata.MD
	var capturedRequest *identityv1.ResolveAccountHandleRequest
	replay := func(_ context.Context, issuer, jti string, _ time.Time) error {
		mu.Lock()
		defer mu.Unlock()
		key := issuer + ":" + jti
		if used[key] {
			return errors.New("replayed")
		}
		used[key] = true
		return nil
	}
	interceptor := func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		md, _ := metadata.FromIncomingContext(ctx)
		auth, ids := md.Get("authorization"), md.Get("x-request-id")
		req, ok := request.(*identityv1.ResolveAccountHandleRequest)
		if !ok || info.FullMethod != identityv1.AuthUserIdentityAuthorityService_ResolveAccountHandle_FullMethodName || len(auth) != 1 || len(ids) != 1 || !strings.HasPrefix(auth[0], "Bearer ") || req.RequestId != ids[0] {
			return nil, status.Error(codes.Unauthenticated, "invalid principal")
		}
		hash, hashErr := principal.RequestHash(req)
		if hashErr != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid request")
		}
		_, verifyErr := principal.VerifyService(ctx, strings.TrimPrefix(auth[0], "Bearer "), principal.VerifyConfig{ExpectedIssuer: "user", ExpectedAudience: "auth", ExpectedRPC: info.FullMethod, ExpectedRequestID: ids[0], ExpectedRequestHash: hash, ReplayGuard: replay, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &signing.PublicKey, nil }})
		if verifyErr != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid principal")
		}
		mu.Lock()
		captured, capturedRequest = md.Copy(), proto.Clone(req).(*identityv1.ResolveAccountHandleRequest)
		mu.Unlock()
		return handler(ctx, request)
	}
	server := grpc.NewServer(grpc.Creds(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{pki.server}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: pki.roots})), grpc.UnaryInterceptor(interceptor))
	identityv1.RegisterAuthUserIdentityAuthorityServiceServer(server, &parentContractServer{})
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { server.Stop(); _ = listener.Close() })
	config := AuthParentTransport{Address: listener.Addr().String(), ServerName: "auth.parent.test", Roots: pki.roots, Certificate: pki.client}
	client, conn, err := NewAuthParentClient(config, issuer)
	require.NoError(t, err)
	defer conn.Close()
	_, err = client.ObserveParent(context.Background(), uuid.New())
	require.NoError(t, err)
	mu.Lock()
	firstMD, firstReq := captured.Copy(), proto.Clone(capturedRequest).(*identityv1.ResolveAccountHandleRequest)
	mu.Unlock()
	_, err = identityv1.NewAuthUserIdentityAuthorityServiceClient(conn).ResolveAccountHandle(metadata.NewOutgoingContext(context.Background(), firstMD), firstReq)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	other := makeParentTestPKI(t)
	config.Roots = other.roots
	untrusted, untrustedConn, err := NewAuthParentClient(config, issuer)
	require.NoError(t, err)
	defer untrustedConn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err = untrusted.ObserveParent(ctx, uuid.New())
	require.Error(t, err)
	config.Roots = pki.roots
	config.Certificate = other.client
	wrongClient, wrongClientConn, err := NewAuthParentClient(config, issuer)
	require.NoError(t, err)
	defer wrongClientConn.Close()
	wrongCtx, stop := context.WithTimeout(context.Background(), time.Second)
	defer stop()
	_, err = wrongClient.ObserveParent(wrongCtx, uuid.New())
	require.Error(t, err)
}

func TestAuthParentClientRejectsMissingTransportMaterial(t *testing.T) {
	pki := makeParentTestPKI(t)
	signing, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "user", KeyID: "fixture", PrivateKey: signing})
	require.NoError(t, err)
	valid := AuthParentTransport{Address: "auth.example:443", ServerName: "auth.example", Roots: pki.roots, Certificate: pki.client}
	for name, mutate := range map[string]func(*AuthParentTransport){
		"address":        func(c *AuthParentTransport) { c.Address = "" },
		"hostname":       func(c *AuthParentTransport) { c.ServerName = "" },
		"roots":          func(c *AuthParentTransport) { c.Roots = nil },
		"certificate":    func(c *AuthParentTransport) { c.Certificate.Certificate = nil },
		"private-key":    func(c *AuthParentTransport) { c.Certificate.PrivateKey = nil },
		"mismatched-key": func(c *AuthParentTransport) { c.Certificate.PrivateKey = signing },
	} {
		t.Run(name, func(t *testing.T) {
			config := valid
			mutate(&config)
			client, conn, err := NewAuthParentClient(config, issuer)
			require.Error(t, err)
			require.Nil(t, client)
			require.Nil(t, conn)
		})
	}
	client, conn, err := NewAuthParentClient(valid, nil)
	require.Error(t, err)
	require.Nil(t, client)
	require.Nil(t, conn)
}
