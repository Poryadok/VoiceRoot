package usercapture

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"math/big"
	"net"
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
	"google.golang.org/grpc/test/bufconn"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/principal"
	"voice/backend/pkg/spaceaccountcapture"
)

type testPKI struct {
	roots          *x509.CertPool
	server, client tls.Certificate
}

func newTestPKI(t *testing.T) testPKI {
	t.Helper()
	now := time.Now()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "capture test CA"}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	der, err := x509.CreateCertificate(rand.Reader, ca, ca, &key.PublicKey, key)
	require.NoError(t, err)
	ca, err = x509.ParseCertificate(der)
	require.NoError(t, err)
	roots := x509.NewCertPool()
	roots.AddCert(ca)
	issue := func(serial int64, usage x509.ExtKeyUsage) tls.Certificate {
		leafKey, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		leaf := &x509.Certificate{SerialNumber: big.NewInt(serial), DNSNames: []string{"capture.user.test"}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{usage}}
		cert, err := x509.CreateCertificate(rand.Reader, leaf, ca, &leafKey.PublicKey, key)
		require.NoError(t, err)
		return tls.Certificate{Certificate: [][]byte{cert}, PrivateKey: leafKey}
	}
	return testPKI{roots: roots, server: issue(2, x509.ExtKeyUsageServerAuth), client: issue(3, x509.ExtKeyUsageClientAuth)}
}

// This is a contract receiver for the client component, not User's real DB
// provider, socialprincipal registration, production listener or lineage proof.
type contractReceiver struct {
	userv1.UnimplementedUserServiceServer
}

func (*contractReceiver) CaptureSpaceMemberAccountBindings(_ context.Context, r *userv1.CaptureSpaceMemberAccountBindingsRequest) (*userv1.CaptureSpaceMemberAccountBindingsResponse, error) {
	if spaceaccountcapture.ValidateRequest(r) != nil {
		return nil, status.Error(codes.InvalidArgument, "request")
	}
	return testResponse(r, 3), nil
}

func testProtectedTransport(t *testing.T, pki testPKI, key *rsa.PublicKey) (func(context.Context, string) (net.Conn, error), func() metadata.MD) {
	t.Helper()
	listener := bufconn.Listen(1 << 20)
	var mu sync.Mutex
	used := map[string]bool{}
	var captured metadata.MD
	var replay principal.ReplayGuard = func(_ context.Context, issuer, jti string, _ time.Time) error {
		mu.Lock()
		defer mu.Unlock()
		id := issuer + ":" + jti
		if used[id] {
			return errors.New("replay")
		}
		used[id] = true
		return nil
	}
	guard := func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if info.FullMethod != userv1.UserService_CaptureSpaceMemberAccountBindings_FullMethodName {
			return nil, status.Error(codes.PermissionDenied, "method")
		}
		md, _ := metadata.FromIncomingContext(ctx)
		_, err := verifyCallWithReplay(metadata.NewOutgoingContext(ctx, md), req.(*userv1.CaptureSpaceMemberAccountBindingsRequest), key, replay)
		if err != nil {
			return nil, status.Error(codes.PermissionDenied, "credential")
		}
		mu.Lock()
		captured = md.Copy()
		mu.Unlock()
		return handler(ctx, req)
	}
	server := grpc.NewServer(grpc.Creds(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{pki.server}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: pki.roots})), grpc.UnaryInterceptor(guard))
	userv1.RegisterUserServiceServer(server, &contractReceiver{})
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { server.Stop(); _ = listener.Close() })
	return func(ctx context.Context, _ string) (net.Conn, error) { return listener.DialContext(ctx) }, func() metadata.MD { mu.Lock(); defer mu.Unlock(); return captured.Copy() }
}

func TestTransportMutualTLSAndProtectedRequestReplay(t *testing.T) {
	issuer, key := testIssuer(t, "space")
	pki := newTestPKI(t)
	dialer, captured := testProtectedTransport(t, pki, key)
	config := TransportConfig{Address: "passthrough:///capture", ServerName: "capture.user.test", Roots: pki.roots, Certificate: pki.client}
	client, conn, err := newTransport(config, issuer, dialer)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	r := testRequest()
	out, err := client.CaptureSpaceMemberAccountBindings(context.Background(), r)
	require.NoError(t, err)
	require.NoError(t, spaceaccountcapture.ValidateResponse(r, out))
	// Fresh client calls get fresh credentials; replaying the first exact signed
	// transport credential through the real generated RPC is rejected.
	first := captured()
	out, err = client.CaptureSpaceMemberAccountBindings(context.Background(), r)
	require.NoError(t, err)
	require.NotNil(t, out)
	replayCtx, cancel := context.WithTimeout(metadata.NewOutgoingContext(context.Background(), first), time.Second)
	defer cancel()
	_, err = userv1.NewUserServiceClient(conn).CaptureSpaceMemberAccountBindings(replayCtx, r)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	altered := testRequest()
	altered.Purpose = 3
	_, err = userv1.NewUserServiceClient(conn).CaptureSpaceMemberAccountBindings(replayCtx, altered)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestTransportFailsClosedForMissingInputs(t *testing.T) {
	issuer, _ := testIssuer(t, "space")
	pki := newTestPKI(t)
	valid := TransportConfig{Address: "user:443", ServerName: "capture.user.test", Roots: pki.roots, Certificate: pki.client}
	for name, mutate := range map[string]func(*TransportConfig){"address": func(c *TransportConfig) { c.Address = "" }, "whitespace address": func(c *TransportConfig) { c.Address = " user:443" }, "server identity": func(c *TransportConfig) { c.ServerName = "" }, "trust": func(c *TransportConfig) { c.Roots = nil }, "certificate": func(c *TransportConfig) { c.Certificate.Certificate = nil }, "private key": func(c *TransportConfig) { c.Certificate.PrivateKey = nil }} {
		t.Run(name, func(t *testing.T) {
			config := valid
			mutate(&config)
			c, conn, err := New(config, issuer)
			require.ErrorIs(t, err, ErrUnavailable)
			require.Nil(t, c)
			require.Nil(t, conn)
		})
	}
	c, conn, err := New(valid, nil)
	require.ErrorIs(t, err, ErrUnavailable)
	require.Nil(t, c)
	require.Nil(t, conn)
}

func TestTransportRejectsEmptyExplicitRoots(t *testing.T) {
	issuer, _ := testIssuer(t, "space")
	pki := newTestPKI(t)
	c, conn, err := New(TransportConfig{Address: "user:443", ServerName: "capture.user.test", Roots: x509.NewCertPool(), Certificate: pki.client}, issuer)
	require.ErrorIs(t, err, ErrUnavailable)
	require.Nil(t, c)
	require.Nil(t, conn)
}

func TestTransportAcceptsExplicitCallerSystemRoots(t *testing.T) {
	issuer, _ := testIssuer(t, "space")
	pki := newTestPKI(t)
	roots, err := x509.SystemCertPool()
	require.NoError(t, err)
	require.NotNil(t, roots)
	require.False(t, roots.Equal(x509.NewCertPool()), "this fixture requires actual nonempty system trust")
	// The caller acquires this pool. New does not acquire system trust itself;
	// constructor acceptance is not a successful TLS/owner-authority proof.
	c, conn, err := New(TransportConfig{Address: "user:443", ServerName: "capture.user.test", Roots: roots, Certificate: pki.client}, issuer)
	require.NoError(t, err)
	require.NotNil(t, c)
	require.NotNil(t, conn)
	require.NoError(t, conn.Close())
}

func TestTransportRejectsWrongServerTrustAndIssuer(t *testing.T) {
	issuer, key := testIssuer(t, "space")
	wrongIssuer, _ := testIssuer(t, "role")
	pki := newTestPKI(t)
	foreign := newTestPKI(t)
	for _, name := range []string{"server name", "trust", "issuer"} {
		t.Run(name, func(t *testing.T) {
			dialer, _ := testProtectedTransport(t, pki, key)
			config := TransportConfig{Address: "passthrough:///capture", ServerName: "capture.user.test", Roots: pki.roots, Certificate: pki.client}
			signer := issuer
			if name == "server name" {
				config.ServerName = "wrong.user.test"
			}
			if name == "trust" {
				config.Roots = foreign.roots
			}
			if name == "issuer" {
				signer = wrongIssuer
			}
			c, conn, err := newTransport(config, signer, dialer)
			require.NoError(t, err)
			defer conn.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer cancel()
			out, err := c.CaptureSpaceMemberAccountBindings(ctx, testRequest())
			require.True(t, errors.Is(err, ErrUnavailable))
			require.Nil(t, out)
		})
	}
}

func TestProtectedContractReceiverRejectsFreshForeignBindings(t *testing.T) {
	issuer, key := testIssuer(t, "space")
	pki := newTestPKI(t)
	dialer, _ := testProtectedTransport(t, pki, key)
	_, conn, err := newTransport(TransportConfig{Address: "passthrough:///capture", ServerName: "capture.user.test", Roots: pki.roots, Certificate: pki.client}, issuer, dialer)
	require.NoError(t, err)
	defer conn.Close()
	for _, name := range []string{"audience", "method", "body"} {
		t.Run(name, func(t *testing.T) {
			r := testRequest()
			hash, err := principal.RequestHash(r)
			require.NoError(t, err)
			input := principal.ServiceInput{Audience: "user", RPC: userv1.UserService_CaptureSpaceMemberAccountBindings_FullMethodName, RequestID: uuid.NewString(), RequestHash: hash}
			switch name {
			case "audience":
				input.Audience = "role"
			case "method":
				input.RPC = "/voice.user.v1.UserService/GetProfile"
			case "body":
				r.Purpose = userv1.SpaceAccountCapturePurpose_SPACE_ACCOUNT_CAPTURE_PURPOSE_COMMUNITY_PUBLICATION
			}
			token, err := issuer.IssueService(input)
			require.NoError(t, err)
			ctx, cancel := context.WithTimeout(metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", input.RequestID)), time.Second)
			defer cancel()
			_, err = userv1.NewUserServiceClient(conn).CaptureSpaceMemberAccountBindings(ctx, r)
			require.Equal(t, codes.PermissionDenied, status.Code(err))
		})
	}
}

func TestMutualTLSReceiverRejectsUncertifiedClient(t *testing.T) {
	_, key := testIssuer(t, "space")
	pki := newTestPKI(t)
	dialer, _ := testProtectedTransport(t, pki, key)
	// An ordinary TLS client with the correct server trust still cannot enter
	// the protected listener without a client certificate.
	conn, err := grpc.NewClient("passthrough:///capture", grpc.WithContextDialer(dialer), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: pki.roots, ServerName: "capture.user.test"})), grpc.WithDisableRetry())
	require.NoError(t, err)
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	out, err := userv1.NewUserServiceClient(conn).CaptureSpaceMemberAccountBindings(ctx, testRequest())
	require.Error(t, err)
	require.Nil(t, out)
}
