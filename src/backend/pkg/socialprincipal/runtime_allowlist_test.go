package socialprincipal

import (
	"context"
	"crypto"
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

func TestRuntimeUserPrivacyListenerUsesFiniteAllowlist(t *testing.T) {
	cert, certPEM, _ := ephemeralTLS(t)
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(certPEM))
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	runtime := &Runtime{target: "user", credentials: credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}}), verifier: &Verifier{Target: "user", Issuers: map[string]bool{"social": true}, Resolve: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Replay: func(context.Context, string, string, time.Time) error { return nil }}}
	lis := bufconn.Listen(1 << 20)
	server := grpc.NewServer(runtime.ServerOptions()...)
	server.RegisterService(&grpc.ServiceDesc{ServiceName: "voice.user.v1.UserService", HandlerType: (*interface{})(nil), Methods: []grpc.MethodDesc{
		allowlistMethod("GetPrivacySettings"), allowlistMethod("GetProfile"), allowlistMethod("ListProfileIDsForAccount"), allowlistMethod("GetProfiles"),
	}}, new(struct{}))
	go func() { _ = server.Serve(lis) }()
	t.Cleanup(server.Stop)
	conn, err := grpc.NewClient("passthrough:///privacy.test", grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: "privacy.test"})))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	for _, method := range []string{"GetPrivacySettings", "GetProfile", "ListProfileIDsForAccount"} {
		fullMethod := "/voice.user.v1.UserService/" + method
		require.NoError(t, invokeSignedUserMethod(conn, key, fullMethod))
	}
	err = invokeSignedUserMethod(conn, key, "/voice.user.v1.UserService/GetProfiles")
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestRuntimeUserMessagingListenerAllowsOnlyScheduledPresence(t *testing.T) {
	serverCert, serverCAPEM, _ := ephemeralTLS(t)
	serverRoots := x509.NewCertPool()
	require.True(t, serverRoots.AppendCertsFromPEM(serverCAPEM))
	clientCA, clientCAKey, clientCAPEM := testMessagingClientCA(t)
	clientCAs := x509.NewCertPool()
	require.True(t, clientCAs.AppendCertsFromPEM(clientCAPEM))
	clientCert := testMessagingClientCertificate(t, clientCA, clientCAKey)
	serverTLS, err := listenerTLSConfig(Config{Capability: "messaging", ClientCAs: clientCAs}, serverCert)
	require.NoError(t, err)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	runtime := &Runtime{target: "user", capability: "messaging", credentials: credentials.NewTLS(serverTLS), verifier: &Verifier{Target: "user", Capability: "messaging", Issuers: map[string]bool{"messaging": true}, Resolve: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Replay: func(context.Context, string, string, time.Time) error { return nil }}}
	lis := bufconn.Listen(1 << 20)
	server := grpc.NewServer(runtime.ServerOptions()...)
	var forbiddenProfileCalls atomic.Int32
	server.RegisterService(&grpc.ServiceDesc{ServiceName: "voice.user.v1.UserService", HandlerType: (*interface{})(nil), Methods: []grpc.MethodDesc{
		allowlistMethod("GetScheduledMessageDispatchPresence"),
		allowlistMethod("GetBulkPresence"),
		allowlistMethod("GetNotificationRoutingPresence"),
		allowlistMethodWithCalls("GetProfile", &forbiddenProfileCalls),
	}}, new(struct{}))
	go func() { _ = server.Serve(lis) }()
	t.Cleanup(server.Stop)
	conn, err := grpc.NewClient("passthrough:///privacy.test", grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: serverRoots, ServerName: "privacy.test", Certificates: []tls.Certificate{clientCert}})))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	method := "/voice.user.v1.UserService/GetScheduledMessageDispatchPresence"
	require.NoError(t, invokeSignedUserMethodWithIssuer(conn, key, "messaging", method))
	for _, denied := range []string{"GetBulkPresence", "GetNotificationRoutingPresence", "GetProfile"} {
		err := invokeSignedUserMethodWithIssuer(conn, key, "messaging", "/voice.user.v1.UserService/"+denied)
		require.Equal(t, codes.PermissionDenied, status.Code(err), denied)
	}
	require.Zero(t, forbiddenProfileCalls.Load(), "the protected listener must reject GetProfile before dispatch")
}

func TestRuntimeUserMessagingListenerRequiresTrustedClientCertificate(t *testing.T) {
	serverCert, serverCAPEM, _ := ephemeralTLS(t)
	serverRoots := x509.NewCertPool()
	require.True(t, serverRoots.AppendCertsFromPEM(serverCAPEM))
	trustedCA, trustedCAKey, trustedCAPEM := testMessagingClientCA(t)
	trustedClientCert := testMessagingClientCertificate(t, trustedCA, trustedCAKey)
	untrustedCA, untrustedCAKey, _ := testMessagingClientCA(t)
	untrustedClientCert := testMessagingClientCertificate(t, untrustedCA, untrustedCAKey)
	clientCAs := x509.NewCertPool()
	require.True(t, clientCAs.AppendCertsFromPEM(trustedCAPEM))
	serverTLS, err := listenerTLSConfig(Config{Capability: "messaging", ClientCAs: clientCAs}, serverCert)
	require.NoError(t, err)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	runtime := &Runtime{
		target: "user", capability: "messaging", credentials: credentials.NewTLS(serverTLS),
		verifier: &Verifier{
			Target: "user", Capability: "messaging", Issuers: map[string]bool{"messaging": true},
			Resolve: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil },
			Replay:  func(context.Context, string, string, time.Time) error { return nil },
		},
	}
	lis := bufconn.Listen(1 << 20)
	server := grpc.NewServer(runtime.ServerOptions()...)
	var handlerCalls atomic.Int32
	server.RegisterService(&grpc.ServiceDesc{ServiceName: "voice.user.v1.UserService", HandlerType: (*interface{})(nil), Methods: []grpc.MethodDesc{
		allowlistMethodWithCalls("GetScheduledMessageDispatchPresence", &handlerCalls),
	}}, new(struct{}))
	go func() { _ = server.Serve(lis) }()
	t.Cleanup(server.Stop)

	dial := func(clientCertificates []tls.Certificate) *grpc.ClientConn {
		t.Helper()
		clientTLS := &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: serverRoots, ServerName: "privacy.test", Certificates: clientCertificates}
		conn, dialErr := grpc.NewClient("passthrough:///privacy.test", grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }), grpc.WithTransportCredentials(credentials.NewTLS(clientTLS)))
		require.NoError(t, dialErr)
		t.Cleanup(func() { _ = conn.Close() })
		return conn
	}
	const method = "/voice.user.v1.UserService/GetScheduledMessageDispatchPresence"
	for name, certificates := range map[string][]tls.Certificate{
		"no client certificate":        nil,
		"untrusted client certificate": {untrustedClientCert},
	} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel, request, proofErr := signedUserRequestWithIssuer(key, "messaging", method)
			require.NoError(t, proofErr)
			defer cancel()
			err := dial(certificates).Invoke(ctx, method, request, new(wrapperspb.StringValue))
			require.Error(t, err, "TLS must reject the connection before principal-authorized dispatch")
			require.Zero(t, handlerCalls.Load(), "transport rejection must happen before the authorized handler")
		})
	}
	t.Run("trusted client certificate and valid principal", func(t *testing.T) {
		err := invokeSignedUserMethodWithIssuer(dial([]tls.Certificate{trustedClientCert}), key, "messaging", method)
		require.NoError(t, err)
		require.EqualValues(t, 1, handlerCalls.Load(), "the valid mTLS principal reaches the handler")
	})
}

func TestMessagingJWKSHTTPClientUsesDedicatedClientIdentity(t *testing.T) {
	roots := x509.NewCertPool()
	withoutIdentity := Config{Capability: "messaging"}
	_, err := jwksHTTPClientTLSConfig(withoutIdentity, roots)
	require.Error(t, err, "the Messaging JWKS fetch must fail closed without its dedicated identity")

	dir := t.TempDir()
	certPath := filepath.Join(dir, "client.crt")
	keyPath := filepath.Join(dir, "client.key")
	require.NoError(t, os.WriteFile(certPath, []byte("invalid certificate"), 0600))
	require.NoError(t, os.WriteFile(keyPath, []byte("invalid private key"), 0600))
	malformedIdentity := Config{Capability: "messaging", JWKSClientCertFile: certPath, JWKSClientKeyFile: keyPath}
	_, err = jwksHTTPClientTLSConfig(malformedIdentity, roots)
	require.Error(t, err, "malformed client identity must fail closed")

	_, certPEM, keyPEM := ephemeralTLS(t)
	require.NoError(t, os.WriteFile(certPath, certPEM, 0600))
	require.NoError(t, os.WriteFile(keyPath, keyPEM, 0600))
	validIdentity := Config{Capability: "messaging", JWKSClientCertFile: certPath, JWKSClientKeyFile: keyPath}
	clientTLS, err := jwksHTTPClientTLSConfig(validIdentity, roots)
	require.NoError(t, err)
	require.Len(t, clientTLS.Certificates, 1)
	require.Same(t, roots, clientTLS.RootCAs, "peer verification continues using the configured server roots")

	otherCapability := Config{Capability: "social", JWKSClientCertFile: certPath, JWKSClientKeyFile: keyPath}
	otherTLS, err := jwksHTTPClientTLSConfig(otherCapability, roots)
	require.NoError(t, err)
	require.Empty(t, otherTLS.Certificates, "the User mTLS client identity is scoped to Messaging JWKS only")
}

func testMessagingClientCA(t *testing.T) (*x509.Certificate, crypto.Signer, []byte) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	now := time.Now()
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(now.UnixNano()),
		Subject:               pkix.Name{CommonName: "messaging-client-test-ca"},
		NotBefore:             now.Add(-time.Minute),
		NotAfter:              now.Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	certificate, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	return certificate, key, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func testMessagingClientCertificate(t *testing.T, ca *x509.Certificate, caKey crypto.Signer) tls.Certificate {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: big.NewInt(now.UnixNano()),
		Subject:      pkix.Name{CommonName: "messaging-client.test"},
		NotBefore:    now.Add(-time.Minute),
		NotAfter:     now.Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, ca, &key.PublicKey, caKey)
	require.NoError(t, err)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	require.NoError(t, err)
	return pair
}

func allowlistMethod(name string) grpc.MethodDesc {
	return allowlistMethodWithCalls(name, nil)
}

func allowlistMethodWithCalls(name string, calls *atomic.Int32) grpc.MethodDesc {
	return grpc.MethodDesc{MethodName: name, Handler: func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
		req := new(wrapperspb.StringValue)
		if err := dec(req); err != nil {
			return nil, err
		}
		handler := func(context.Context, any) (any, error) {
			if calls != nil {
				calls.Add(1)
			}
			return req, nil
		}
		if interceptor == nil {
			return handler(ctx, req)
		}
		return interceptor(ctx, req, &grpc.UnaryServerInfo{Server: srv, FullMethod: "/voice.user.v1.UserService/" + name}, handler)
	}}
}

func invokeSignedUserMethod(conn *grpc.ClientConn, key *rsa.PrivateKey, method string) error {
	return invokeSignedUserMethodWithIssuer(conn, key, "social", method)
}

func invokeSignedUserMethodWithIssuer(conn *grpc.ClientConn, key *rsa.PrivateKey, issuerName, method string) error {
	ctx, cancel, req, err := signedUserRequestWithIssuer(key, issuerName, method)
	if err != nil {
		return err
	}
	defer cancel()
	return conn.Invoke(ctx, method, req, new(wrapperspb.StringValue))
}

func signedUserRequestWithIssuer(key *rsa.PrivateKey, issuerName, method string) (context.Context, context.CancelFunc, *wrapperspb.StringValue, error) {
	req := wrapperspb.String("profile")
	hash, err := principal.RequestHash(req)
	if err != nil {
		return nil, nil, nil, err
	}
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: issuerName, KeyID: "current", PrivateKey: key})
	if err != nil {
		return nil, nil, nil, err
	}
	token, err := issuer.IssueService(principal.ServiceInput{Audience: "user", RPC: method, RequestID: "request-" + method, RequestHash: hash})
	if err != nil {
		return nil, nil, nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", "request-"+method))
	return ctx, cancel, req, nil
}
