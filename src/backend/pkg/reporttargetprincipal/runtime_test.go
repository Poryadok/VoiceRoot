package reporttargetprincipal

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
	"voice/backend/pkg/principal"
)

func targetTLS(t *testing.T) (tls.Certificate, []byte, []byte) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "localhost"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), DNSNames: []string{"localhost"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	require.NoError(t, err)
	return cert, certPEM, keyPEM
}

func TestProtectedRuntimeRealTLSJWKSAndSharedReplay(t *testing.T) {
	for _, target := range []string{"user", "messaging", "space", "story"} {
		t.Run(target, func(t *testing.T) {
			ctx := context.Background()
			cert, certPEM, keyPEM := targetTLS(t)
			dir := t.TempDir()
			certPath, keyPath := filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
			require.NoError(t, os.WriteFile(certPath, certPEM, 0600))
			require.NoError(t, os.WriteFile(keyPath, keyPEM, 0600))
			active, err := rsa.GenerateKey(rand.Reader, 2048)
			require.NoError(t, err)
			next, err := rsa.GenerateKey(rand.Reader, 2048)
			require.NoError(t, err)
			jwks, err := principal.PublicJWKSKeys([]principal.JWKSKey{{KeyID: "active", PublicKey: &active.PublicKey}, {KeyID: "next", PublicKey: &next.PublicKey}})
			require.NoError(t, err)
			var outage atomic.Bool
			publisher := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if outage.Load() {
					w.WriteHeader(503)
					return
				}
				_, _ = w.Write(jwks)
			}))
			publisher.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}}
			publisher.StartTLS()
			defer publisher.Close()
			replay := miniredis.RunT(t)
			cfg := Config{Target: target, ListenAddr: "127.0.0.1:0", TLSCertFile: certPath, TLSKeyFile: keyPath, ClientCAFile: certPath, JWKSCAFile: certPath, JWKSURLs: map[string]string{"moderation": publisher.URL}, ReplayAddr: replay.Addr(), RefreshAfter: 30 * time.Second, HardExpiry: 2 * time.Minute, UnknownKIDCooldown: 5 * time.Second}
			a, err := New(ctx, cfg)
			require.NoError(t, err)
			defer func() { _ = a.Close() }()
			b, err := New(ctx, cfg)
			require.NoError(t, err)
			defer func() { _ = b.Close() }()
			var stores atomic.Int32
			connect := func(runtime *Runtime, certificate *tls.Certificate) *grpc.ClientConn {
				listener, err := net.Listen("tcp", "127.0.0.1:0")
				require.NoError(t, err)
				server := grpc.NewServer(runtime.ServerOptions()...)
				serviceName := "voice." + target + ".v1." + map[string]string{"user": "UserService", "messaging": "MessagingService", "space": "SpaceService", "story": "StoryService"}[target]
				server.RegisterService(&grpc.ServiceDesc{ServiceName: serviceName, HandlerType: (*interface{})(nil), Methods: []grpc.MethodDesc{{MethodName: "ResolveReportTarget", Handler: func(srv any, ctx context.Context, decode func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
					request := new(wrapperspb.StringValue)
					if err := decode(request); err != nil {
						return nil, err
					}
					return interceptor(ctx, request, &grpc.UnaryServerInfo{FullMethod: Method(target)}, func(ctx context.Context, _ any) (any, error) {
						if err := Require(ctx, target, request); err != nil {
							return nil, err
						}
						stores.Add(1)
						return &emptypb.Empty{}, nil
					})
				}}}}, struct{}{})
				go func() { _ = server.Serve(listener) }()
				t.Cleanup(server.Stop)
				roots := x509.NewCertPool()
				require.True(t, roots.AppendCertsFromPEM(certPEM))
				tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: "localhost"}
				if certificate != nil {
					tlsConfig.Certificates = []tls.Certificate{*certificate}
				}
				conn, err := grpc.NewClient(listener.Addr().String(), grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
				require.NoError(t, err)
				t.Cleanup(func() { _ = conn.Close() })
				return conn
			}
			ca, cb := connect(a, &cert), connect(b, &cert)
			request := wrapperspb.String("target")
			signed := func(issuer string, key *rsa.PrivateKey, kid string, body proto.Message) context.Context {
				signer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: issuer, KeyID: kid, PrivateKey: key})
				require.NoError(t, err)
				hash, err := principal.RequestHash(body)
				require.NoError(t, err)
				id := uuid.NewString()
				token, err := signer.IssueService(principal.ServiceInput{Audience: target, RPC: Method(target), RequestID: id, RequestHash: hash})
				require.NoError(t, err)
				return metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id))
			}
			call := func(conn *grpc.ClientConn, ctx context.Context, body proto.Message) error {
				bounded, cancel := context.WithTimeout(ctx, time.Second)
				defer cancel()
				return conn.Invoke(bounded, Method(target), body, new(emptypb.Empty))
			}
			once := signed("moderation", active, "active", request)
			require.NoError(t, call(ca, once, request))
			require.Equal(t, codes.Unauthenticated, status.Code(call(cb, once, request)), "shared replay applies across independent receiver runtimes")
			require.EqualValues(t, 1, stores.Load())
			require.Equal(t, codes.Unauthenticated, status.Code(call(ca, signed("moderation", active, "active", request), wrapperspb.String("changed"))))
			require.Equal(t, codes.Unauthenticated, status.Code(call(ca, signed("space", active, "active", request), request)))
			require.Equal(t, codes.Unauthenticated, status.Code(call(ca, signed("moderation", next, "active", request), request)))
			noCert := connect(a, nil)
			require.Error(t, call(noCert, signed("moderation", active, "active", request), request))
			require.EqualValues(t, 1, stores.Load())
			require.NoError(t, call(ca, signed("moderation", next, "next", request), request))
			// A new receiver has no last-good set: JWKS outage is a dependency denial.
			outage.Store(true)
			cold, err := New(ctx, cfg)
			require.NoError(t, err)
			defer func() { _ = cold.Close() }()
			require.Equal(t, codes.Unavailable, status.Code(call(connect(cold, &cert), signed("moderation", active, "active", request), request)))
			outage.Store(false)
			replay.Close()
			require.Equal(t, codes.Unavailable, status.Code(call(ca, signed("moderation", active, "active", request), request)))
			require.EqualValues(t, 2, stores.Load())
		})
	}
}
