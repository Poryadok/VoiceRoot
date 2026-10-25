package main

import (
	"context"
	"crypto/ed25519"
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
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	authv1 "voice.app/voice/auth/v1"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/chat/internal/chatcreation"
	"voice/backend/pkg/principal"
)

type authFloorProtocolServer struct {
	authv1.UnimplementedAuthServiceServer
	floor atomic.Int64
	calls atomic.Int64
}

func (s *authFloorProtocolServer) GetChatSessionEpochFloor(context.Context, *authv1.GetChatSessionEpochFloorRequest) (*authv1.GetChatSessionEpochFloorResponse, error) {
	s.calls.Add(1)
	return &authv1.GetChatSessionEpochFloorResponse{SessionEpochFloor: s.floor.Load()}, nil
}

func authFloorTestPKI(t *testing.T) (string, string, string, string, string) {
	t.Helper()
	dir := t.TempDir()
	pub, key, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test Auth CA"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	der, err := x509.CreateCertificate(rand.Reader, ca, ca, pub, key)
	require.NoError(t, err)
	caFile := filepath.Join(dir, "ca.pem")
	require.NoError(t, os.WriteFile(caFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600))
	leaf := func(name string, serial int64, usage x509.ExtKeyUsage) (string, string) {
		pub, k, e := ed25519.GenerateKey(rand.Reader)
		require.NoError(t, e)
		cert := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: name}, DNSNames: []string{name}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), ExtKeyUsage: []x509.ExtKeyUsage{usage}, KeyUsage: x509.KeyUsageDigitalSignature}
		b, e := x509.CreateCertificate(rand.Reader, cert, ca, pub, key)
		require.NoError(t, e)
		encoded, e := x509.MarshalPKCS8PrivateKey(k)
		require.NoError(t, e)
		cp, kp := filepath.Join(dir, name+".pem"), filepath.Join(dir, name+".key")
		require.NoError(t, os.WriteFile(cp, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: b}), 0600))
		require.NoError(t, os.WriteFile(kp, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encoded}), 0600))
		return cp, kp
	}
	serverCert, serverKey := leaf("auth.fixture", 2, x509.ExtKeyUsageServerAuth)
	clientCert, clientKey := leaf("chat.fixture", 3, x509.ExtKeyUsageClientAuth)
	return caFile, serverCert, serverKey, clientCert, clientKey
}

func TestChatCreateAuthConfigRequiresCompleteDedicatedTransport(t *testing.T) {
	names := []string{"GRPC_ADDR", "TLS_CA_FILE", "TLS_SERVER_NAME", "CLIENT_CERT_FILE", "CLIENT_KEY_FILE"}
	for _, missing := range names {
		t.Run(missing, func(t *testing.T) {
			for _, name := range names {
				t.Setenv("CHAT_CREATE_AUTH_"+name, "placeholder")
			}
			t.Setenv("CHAT_CREATE_AUTH_"+missing, "")
			checker, closeFn, err := newChatCreateAuthCheckerFromEnv(nil)
			require.Error(t, err)
			require.Nil(t, checker)
			require.Nil(t, closeFn)
		})
	}
}

// Real network and TLS, but a CONTROLLED Go Auth protocol receiver, not Java.
func TestChatCreateAuthMTLSAndVerifierBeforeEffectsComponent(t *testing.T) {
	caFile, serverCert, serverKey, clientCert, clientKey := authFloorTestPKI(t)
	roots := x509.NewCertPool()
	b, err := os.ReadFile(caFile)
	require.NoError(t, err)
	require.True(t, roots.AppendCertsFromPEM(b))
	cert, err := tls.LoadX509KeyPair(serverCert, serverKey)
	require.NoError(t, err)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "chat", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	account, id := uuid.NewString(), uuid.NewString()
	var mu sync.Mutex
	seen := map[string]bool{}
	server := grpc.NewServer(grpc.Creds(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}, ClientCAs: roots, ClientAuth: tls.RequireAndVerifyClientCert})), grpc.UnaryInterceptor(func(ctx context.Context, body any, info *grpc.UnaryServerInfo, next grpc.UnaryHandler) (any, error) {
		md, e := principal.IncomingMetadata(ctx)
		if e != nil {
			return nil, status.Error(codes.Unauthenticated, "metadata")
		}
		req, ok := body.(*authv1.GetChatSessionEpochFloorRequest)
		if !ok || req.AccountId != account {
			return nil, status.Error(codes.InvalidArgument, "account")
		}
		hash, e := principal.RequestHash(req)
		if e != nil {
			return nil, e
		}
		_, e = principal.VerifyService(ctx, md.BearerToken, principal.VerifyConfig{ExpectedIssuer: "chat", ExpectedAudience: "auth", ExpectedRPC: info.FullMethod, ExpectedRequestID: id, ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, ReplayGuard: func(_ context.Context, _ string, jti string, _ time.Time) error {
			mu.Lock()
			defer mu.Unlock()
			if seen[jti] {
				return status.Error(codes.Unauthenticated, "replay")
			}
			seen[jti] = true
			return nil
		}})
		if e != nil {
			return nil, status.Error(codes.Unauthenticated, "principal")
		}
		return next(ctx, body)
	}))
	impl := &authFloorProtocolServer{}
	impl.floor.Store(9)
	authv1.RegisterAuthServiceServer(server, impl)
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go func() { _ = server.Serve(lis) }()
	t.Cleanup(server.Stop)
	env := map[string]string{"GRPC_ADDR": lis.Addr().String(), "TLS_CA_FILE": caFile, "TLS_SERVER_NAME": "auth.fixture", "CLIENT_CERT_FILE": clientCert, "CLIENT_KEY_FILE": clientKey}
	for k, v := range env {
		t.Setenv("CHAT_CREATE_AUTH_"+k, v)
	}
	checker, closeFn, err := newChatCreateAuthCheckerFromEnv(issuer)
	require.NoError(t, err)
	require.NotNil(t, checker)
	t.Cleanup(func() { require.NoError(t, closeFn()) })
	_, _, err = newChatCreateAuthCheckerFromEnv(nil)
	require.Error(t, err, "configured transport requires own signer")
	incoming := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer upstream-only", "x-request-id", id))
	require.NoError(t, checker.CheckSessionEpoch(incoming, account, 9))
	require.NoError(t, checker.CheckSessionEpoch(incoming, account, 10))
	require.EqualValues(t, 2, impl.calls.Load())
	// The ingress peer below is an explicitly synthetic verified-peer seam;
	// actual TLS above qualifies only Chat->controlled Auth, not Gateway->Chat.
	gatewayKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	gateway, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "gateway", PrivateKey: gatewayKey})
	require.NoError(t, err)
	req := &chatv1.CreateChatRequest{Type: chatv1.ChatType_CHAT_TYPE_GROUP, SpaceId: proto.String(uuid.NewString()), Name: proto.String("create"), RequestId: id}
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	v := chatcreation.CreateVerifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &gatewayKey.PublicKey, nil }, Replay: func(context.Context, string, string, time.Time) error { return nil }, Epoch: checker.CheckSessionEpoch}
	effects := 0
	invoke := func() error {
		token, e := gateway.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "chat", RPC: chatv1.ChatService_CreateChat_FullMethodName, RequestID: id, RequestHash: hash, AccountID: account, ProfileID: uuid.NewString(), SessionEpoch: 9, AccountType: "regular", ClientExpiresAt: time.Now().Add(time.Minute)})
		require.NoError(t, e)
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id))
		ctx = peer.NewContext(ctx, &peer.Peer{AuthInfo: credentials.TLSInfo{State: tls.ConnectionState{VerifiedChains: [][]*x509.Certificate{{{}}}}}})
		_, e = v.Unary()(ctx, req, &grpc.UnaryServerInfo{FullMethod: chatv1.ChatService_CreateChat_FullMethodName}, func(context.Context, any) (any, error) { effects++; return nil, nil })
		return e
	}
	require.NoError(t, invoke())
	require.Equal(t, 1, effects)
	impl.floor.Store(10)
	require.Equal(t, codes.Unauthenticated, status.Code(invoke()))
	require.Equal(t, 1, effects)
	impl.floor.Store(0)
	require.Equal(t, codes.Unavailable, status.Code(invoke()))
	require.Equal(t, 1, effects)
	otherCA, _, _, _, _ := authFloorTestPKI(t)
	for _, bad := range []struct{ name, value string }{{"TLS_SERVER_NAME", "wrong.fixture"}, {"CLIENT_CERT_FILE", serverCert}, {"TLS_CA_FILE", otherCA}} {
		t.Run(bad.name, func(t *testing.T) {
			t.Setenv("CHAT_CREATE_AUTH_"+bad.name, bad.value)
			if bad.name == "CLIENT_CERT_FILE" {
				t.Setenv("CHAT_CREATE_AUTH_CLIENT_KEY_FILE", serverKey)
			}
			badChecker, done, e := newChatCreateAuthCheckerFromEnv(issuer)
			require.NoError(t, e)
			defer done()
			ctx, cancel := context.WithTimeout(incoming, time.Second)
			defer cancel()
			require.ErrorIs(t, badChecker.CheckSessionEpoch(ctx, account, 10), chatcreation.ErrDependencyUnavailable)
		})
	}
	server.Stop()
	require.Equal(t, codes.Unavailable, status.Code(invoke()))
	require.Equal(t, 1, effects)
	mu.Lock()
	require.GreaterOrEqual(t, len(seen), 5)
	mu.Unlock()
	require.False(t, strings.Contains(lis.Addr().String(), "0.0.0.0"))
}
