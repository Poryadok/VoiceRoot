package socialprincipal

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"voice/backend/pkg/principal"
)

func ephemeralTLS(t *testing.T) (tls.Certificate, []byte, []byte) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	cert := &x509.Certificate{SerialNumber: big.NewInt(time.Now().UnixNano()), Subject: pkix.Name{CommonName: "privacy.test"}, DNSNames: []string{"privacy.test"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, cert, cert, &key.PublicKey, key)
	require.NoError(t, err)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	require.NoError(t, err)
	return pair, certPEM, keyPEM
}
func testPrivacyConnection(t *testing.T, r *Runtime, roots *x509.CertPool, calls, intercepted *atomic.Int32) *grpc.ClientConn {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	options := r.ServerOptions()
	options = append(options, grpc.ChainUnaryInterceptor(func(ctx context.Context, req any, info *grpc.UnaryServerInfo, next grpc.UnaryHandler) (any, error) {
		intercepted.Add(1)
		return next(ctx, req)
	}))
	server := grpc.NewServer(options...)
	service, method := "voice.user.v1.UserService", "GetPrivacySettings"
	if r.target == "space" {
		service, method = "voice.space.v1.SpaceService", "AreCoMembers"
	}
	server.RegisterService(&grpc.ServiceDesc{ServiceName: service, HandlerType: (*interface{})(nil), Methods: []grpc.MethodDesc{{MethodName: method, Handler: func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
		req := new(wrapperspb.StringValue)
		if err := dec(req); err != nil {
			return nil, err
		}
		handler := func(ctx context.Context, request any) (any, error) {
			if err := RequireSocial(ctx, r.target, request.(*wrapperspb.StringValue)); err != nil {
				return nil, err
			}
			calls.Add(1)
			return request, nil
		}
		if interceptor == nil {
			return handler(ctx, req)
		}
		return interceptor(ctx, req, &grpc.UnaryServerInfo{Server: srv, FullMethod: Method(r.target)}, handler)
	}}}}, new(struct{}))
	go func() { _ = server.Serve(lis) }()
	t.Cleanup(server.Stop)
	conn, err := grpc.NewClient("passthrough:///privacy.test", grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: "privacy.test"})))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func TestTLSPrivacyListenerBufconn(t *testing.T) {
	cert, certPEM, _ := ephemeralTLS(t)
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(certPEM))
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	for _, target := range []string{"user", "space"} {
		t.Run(target, func(t *testing.T) {
			r := &Runtime{target: target, credentials: credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}}), verifier: &Verifier{Target: target, Issuers: map[string]bool{"social": true}, Resolve: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Replay: func(context.Context, string, string, time.Time) error { return nil }}}
			var calls, intercepted atomic.Int32
			conn := testPrivacyConnection(t, r, roots, &calls, &intercepted)
			req := wrapperspb.String("profile")
			hash, err := principal.RequestHash(req)
			require.NoError(t, err)
			issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "social", KeyID: "current", PrivateKey: key})
			require.NoError(t, err)
			token, err := issuer.IssueService(principal.ServiceInput{Audience: target, RPC: Method(target), RequestID: "request", RequestHash: hash})
			require.NoError(t, err)
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", "request"))
			require.NoError(t, conn.Invoke(ctx, Method(target), req, new(wrapperspb.StringValue)))
			require.EqualValues(t, 1, calls.Load())
			err = conn.Invoke(ctx, "/voice.user.v1.UserService/GetProfile", req, new(wrapperspb.StringValue))
			require.Equal(t, codes.Unimplemented, status.Code(err))
			require.EqualValues(t, 1, calls.Load())
			var wrongCalls, wrongIntercepted atomic.Int32
			wrong := testPrivacyConnection(t, r, x509.NewCertPool(), &wrongCalls, &wrongIntercepted)
			err = wrong.Invoke(ctx, Method(target), req, new(wrapperspb.StringValue))
			require.Error(t, err)
			require.Zero(t, wrongCalls.Load())
			require.Zero(t, wrongIntercepted.Load())
		})
	}
}
