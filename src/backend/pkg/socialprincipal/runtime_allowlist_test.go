package socialprincipal

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"net"
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

func allowlistMethod(name string) grpc.MethodDesc {
	return grpc.MethodDesc{MethodName: name, Handler: func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
		req := new(wrapperspb.StringValue)
		if err := dec(req); err != nil {
			return nil, err
		}
		handler := func(context.Context, any) (any, error) { return req, nil }
		if interceptor == nil {
			return handler(ctx, req)
		}
		return interceptor(ctx, req, &grpc.UnaryServerInfo{Server: srv, FullMethod: "/voice.user.v1.UserService/" + name}, handler)
	}}
}

func invokeSignedUserMethod(conn *grpc.ClientConn, key *rsa.PrivateKey, method string) error {
	req := wrapperspb.String("profile")
	hash, err := principal.RequestHash(req)
	if err != nil {
		return err
	}
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "social", KeyID: "current", PrivateKey: key})
	if err != nil {
		return err
	}
	token, err := issuer.IssueService(principal.ServiceInput{Audience: "user", RPC: method, RequestID: "request-" + method, RequestHash: hash})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", "request-"+method))
	return conn.Invoke(ctx, method, req, new(wrapperspb.StringValue))
}
