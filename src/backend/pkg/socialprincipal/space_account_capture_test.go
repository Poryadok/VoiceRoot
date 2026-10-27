package socialprincipal

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
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

// The real runtime performs mTLS, HTTPS JWKS fetch and shared Redis-protocol
// replay checks. Miniredis is explicit; hosted tests retain real Redis coverage.
func TestSpaceAccountCaptureCapabilityIsSeparate(t *testing.T) {
	const capability = "space-account-capture"
	const method = "/voice.user.v1.UserService/CaptureSpaceMemberAccountBindings"
	require.Equal(t, method, Method(capability))
	require.Equal(t, "space", expectedIssuer(capability))
	require.True(t, RequiresMutualTLS(capability))
	require.True(t, AllowsMethod(capability, method))
	for _, other := range []string{"user", "space", "auth", "auth-admission", "catalog", "messaging", "search", "file", "notification"} {
		require.False(t, AllowsMethod(other, method), other)
		require.False(t, AllowsMethod(capability, Method(other)), other)
	}
}

func TestSpaceAccountCaptureConfigFailsClosed(t *testing.T) {
	const prefix = "USER_SPACE_CAPTURE_PRINCIPAL_"
	t.Setenv("S2S_JWKS_URLS_JSON", `{"space":"https://space/jwks","social":"https://social/jwks"}`)
	load := func() (Config, bool, error) {
		return LoadFromEnvWithAudience("user", "space-account-capture", prefix, ":9099")
	}
	_, enabled, err := load()
	require.NoError(t, err)
	require.False(t, enabled)
	_, _, ca := testMessagingClientCA(t)
	path := filepath.Join(t.TempDir(), "ca.pem")
	require.NoError(t, os.WriteFile(path, ca, 0600))
	for name, value := range map[string]string{"TLS_CERT_FILE": "cert", "TLS_KEY_FILE": "key", "REPLAY_REDIS_ADDR": "redis:6379", "CLIENT_CA_FILE": path, "JWKS_CLIENT_CERT_FILE": "client-cert", "JWKS_CLIENT_KEY_FILE": "client-key"} {
		t.Setenv(prefix+name, value)
	}
	cfg, enabled, err := load()
	require.NoError(t, err)
	require.True(t, enabled)
	require.Equal(t, "user", cfg.Target)
	require.Equal(t, "space-account-capture", cfg.Capability)
	require.Equal(t, map[string]string{"space": "https://space/jwks"}, cfg.JWKSURLs)
	for _, name := range []string{"TLS_CERT_FILE", "TLS_KEY_FILE", "REPLAY_REDIS_ADDR", "CLIENT_CA_FILE", "JWKS_CLIENT_CERT_FILE", "JWKS_CLIENT_KEY_FILE"} {
		t.Run(name, func(t *testing.T) {
			t.Setenv(prefix+name, "")
			_, enabled, err := load()
			require.True(t, enabled)
			require.Error(t, err)
		})
	}
	t.Setenv("S2S_JWKS_URLS_JSON", `{"social":"https://social/jwks"}`)
	_, enabled, err = load()
	require.True(t, enabled)
	require.Error(t, err)
}

func TestSpaceAccountCaptureRuntimeProtectsExactCapability(t *testing.T) {
	serverCert, serverPEM, serverKey := ephemeralTLS(t)
	ca, caKey, caPEM := testMessagingClientCA(t)
	clientCert := testMessagingClientCertificate(t, ca, caKey)
	roots, clientRoots := x509.NewCertPool(), x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(serverPEM))
	require.True(t, clientRoots.AppendCertsFromPEM(caPEM))
	dir := t.TempDir()
	write := func(name string, data []byte) string {
		path := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(path, data, 0600))
		return path
	}
	serverPath, serverKeyPath := write("server.pem", serverPEM), write("server.key", serverKey)
	caPath := write("client-ca.pem", caPEM)
	clientPath := write("client.pem", pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: clientCert.Certificate[0]}))
	private, err := x509.MarshalPKCS8PrivateKey(clientCert.PrivateKey)
	require.NoError(t, err)
	clientKeyPath := write("client.key", pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: private}))
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	next, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	document := jwksDocument(t, map[string]*rsa.PublicKey{"current": &key.PublicKey, "next": &next.PublicKey})
	var jwksCalls atomic.Int32
	jwks := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { jwksCalls.Add(1); _, _ = w.Write(document) }))
	jwks.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{serverCert}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: clientRoots}
	jwks.StartTLS()
	defer jwks.Close()
	replay := miniredis.RunT(t)
	for _, capability := range []string{"space-account-capture"} {
		t.Run(capability, func(t *testing.T) {
			issuerName := "space"
			cfg := Config{Target: "user", Capability: capability, JWKSURLs: map[string]string{issuerName: jwks.URL}, RefreshAfter: 30 * time.Second, HardExpiry: 2 * time.Minute, UnknownKIDCooldown: time.Second, ReplayAddr: replay.Addr(), JWKSCAFile: serverPath, TLSCertFile: serverPath, TLSKeyFile: serverKeyPath, ListenAddr: "127.0.0.1:0", ClientCAFile: caPath, ClientCAs: clientRoots, JWKSClientCertFile: clientPath, JWKSClientKeyFile: clientKeyPath}
			a, err := New(t.Context(), cfg)
			require.NoError(t, err)
			defer func() { _ = a.Close() }()
			b, err := New(t.Context(), cfg)
			require.NoError(t, err)
			defer func() { _ = b.Close() }()
			var callsA, callsB atomic.Int32
			goodTLS := &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: "privacy.test", Certificates: []tls.Certificate{clientCert}}
			connA := captureRuntimeConnection(t, a, goodTLS, &callsA)
			connB := captureRuntimeConnection(t, b, goodTLS, &callsB)
			method := Method(capability)
			req := wrapperspb.String("bound profile")
			issue := func(issuer, audience, rpc string, subject ...string) (context.Context, context.CancelFunc) {
				hash, err := principal.RequestHash(req)
				require.NoError(t, err)
				signer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: issuer, KeyID: "current", PrivateKey: key})
				require.NoError(t, err)
				token, err := signer.IssueService(principal.ServiceInput{Audience: audience, RPC: rpc, RequestID: "communication-test", RequestHash: hash})
				require.NoError(t, err)
				if len(subject) > 0 {
					parts := strings.Split(token, ".")
					payload, err := base64.RawURLEncoding.DecodeString(parts[1])
					require.NoError(t, err)
					var claims map[string]any
					require.NoError(t, json.Unmarshal(payload, &claims))
					claims["sub"] = subject[0]
					payload, err = json.Marshal(claims)
					require.NoError(t, err)
					unsigned := parts[0] + "." + base64.RawURLEncoding.EncodeToString(payload)
					digest := sha256.Sum256([]byte(unsigned))
					signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
					require.NoError(t, err)
					token = unsigned + "." + base64.RawURLEncoding.EncodeToString(signature)
				}
				ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
				return metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", "communication-test")), cancel
			}
			ctx, cancel := issue(issuerName, "user", method)
			defer cancel()
			require.NoError(t, connA.Invoke(ctx, method, req, new(wrapperspb.StringValue)))
			require.Equal(t, codes.Unauthenticated, status.Code(connB.Invoke(ctx, method, req, new(wrapperspb.StringValue))), "independent receivers must reject one shared replay")
			wrongSubject, stopSubject := issue(issuerName, "user", method, "service:social")
			require.Equal(t, codes.Unauthenticated, status.Code(connB.Invoke(wrongSubject, method, req, new(wrapperspb.StringValue))))
			stopSubject()
			wrongID, stopID := issue(issuerName, "user", method)
			wrongMetadata, ok := metadata.FromOutgoingContext(wrongID)
			require.True(t, ok)
			wrongID = metadata.NewOutgoingContext(wrongID, metadata.Pairs("authorization", wrongMetadata.Get("authorization")[0], "x-request-id", "different-request"))
			require.Equal(t, codes.Unauthenticated, status.Code(connB.Invoke(wrongID, method, req, new(wrapperspb.StringValue))))
			stopID()
			for _, tc := range []struct {
				issuer, audience, rpc string
				body                  *wrapperspb.StringValue
			}{{"social", "user", method, req}, {issuerName, "space", method, req}, {issuerName, "user", method, wrapperspb.String("changed profile")}} {
				ctx, cancel := issue(tc.issuer, tc.audience, tc.rpc)
				err := connB.Invoke(ctx, method, tc.body, new(wrapperspb.StringValue))
				cancel()
				require.Equal(t, codes.Unauthenticated, status.Code(err))
			}
			other := "/voice.user.v1.UserService/GetProfile"
			if other == method {
				other = "/voice.user.v1.UserService/GetPrivacySettings"
			}
			ctx, stop := issue(issuerName, "user", other)
			require.Equal(t, codes.PermissionDenied, status.Code(connA.Invoke(ctx, other, req, new(wrapperspb.StringValue))))
			stop()
			noCertificate := captureRuntimeConnection(t, a, &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: "privacy.test"}, &callsA)
			ctx, stop = issue(issuerName, "user", method)
			require.Error(t, noCertificate.Invoke(ctx, method, req, new(wrapperspb.StringValue)))
			stop()
			require.EqualValues(t, 1, callsA.Load())
			require.Zero(t, callsB.Load())
		})
	}
	require.Positive(t, jwksCalls.Load(), "production runtime must fetch its signing keys over mutual HTTPS")
}

func captureRuntimeConnection(t *testing.T, runtime *Runtime, clientTLS *tls.Config, calls *atomic.Int32) *grpc.ClientConn {
	t.Helper()
	listener := bufconn.Listen(1 << 20)
	server := grpc.NewServer(runtime.ServerOptions()...)
	var methods []grpc.MethodDesc
	for _, name := range []string{"CaptureSpaceMemberAccountBindings", "GetProfile", "GetPrivacySettings"} {
		full := "/voice.user.v1.UserService/" + name
		methods = append(methods, grpc.MethodDesc{MethodName: name, Handler: func(srv any, ctx context.Context, decode func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
			req := new(wrapperspb.StringValue)
			if err := decode(req); err != nil {
				return nil, err
			}
			return interceptor(ctx, req, &grpc.UnaryServerInfo{Server: srv, FullMethod: full}, func(context.Context, any) (any, error) { calls.Add(1); return req, nil })
		}})
	}
	server.RegisterService(&grpc.ServiceDesc{ServiceName: strings.TrimPrefix("/voice.user.v1.UserService", "/"), HandlerType: (*interface{})(nil), Methods: methods}, new(struct{}))
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)
	conn, err := grpc.NewClient("passthrough:///privacy.test", grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return listener.DialContext(ctx) }), grpc.WithTransportCredentials(credentials.NewTLS(clientTLS)))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}
