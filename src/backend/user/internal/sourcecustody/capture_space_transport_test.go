package sourcecustody

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"errors"
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
	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/principal"
	"voice/backend/user/internal/grpcsvc"
)

// This is a protected transport fixture, not Space startup or source qualification.
func spaceCaptureTransportFixture(t *testing.T, authority grpcsvc.SpaceAccountCaptureAuthority) (userv1.UserServiceClient, *principal.Issuer) {
	t.Helper()
	pki := processTLS(t, t.TempDir(), "space-capture")
	signing, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "space", KeyID: "fixture", PrivateKey: signing})
	require.NoError(t, err)
	var mu sync.Mutex
	seen := map[string]bool{}
	replay := func(_ context.Context, issuer, id string, _ time.Time) error {
		mu.Lock()
		defer mu.Unlock()
		key := issuer + ":" + id
		if seen[key] {
			return errors.New("replay")
		}
		seen[key] = true
		return nil
	}
	interceptor := func(ctx context.Context, input any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		request, ok := input.(*userv1.CaptureSpaceMemberAccountBindingsRequest)
		md, _ := metadata.FromIncomingContext(ctx)
		auth, ids := md.Get("authorization"), md.Get("x-request-id")
		if !ok || len(auth) != 1 || len(ids) != 1 || !strings.HasPrefix(auth[0], "Bearer ") {
			return nil, status.Error(codes.Unauthenticated, "invalid principal")
		}
		hash, e := principal.RequestHash(request)
		if e != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid request")
		}
		verified, e := principal.VerifyService(ctx, strings.TrimPrefix(auth[0], "Bearer "), principal.VerifyConfig{ExpectedIssuer: "space", ExpectedAudience: "user", ExpectedRPC: info.FullMethod, ExpectedRequestID: ids[0], ExpectedRequestHash: hash, ReplayGuard: replay, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &signing.PublicKey, nil }})
		if e != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid principal")
		}
		return handler(principal.WithVerified(ctx, verified), input)
	}
	cert, err := tls.LoadX509KeyPair(pki.serverCert, pki.serverKey)
	require.NoError(t, err)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := grpc.NewServer(grpc.Creds(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: pki.roots})), grpc.UnaryInterceptor(interceptor))
	grpcsvc.RegisterSpaceAccountCaptureServer(server, authority)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { server.Stop(); _ = listener.Close() })
	conn, err := grpc.NewClient(listener.Addr().String(), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: pki.roots, ServerName: "localhost", Certificates: []tls.Certificate{pki.client}})), grpc.WithDisableRetry())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })
	return userv1.NewUserServiceClient(conn), issuer
}

func signedSpaceCapture(ctx context.Context, client userv1.UserServiceClient, issuer *principal.Issuer, request *userv1.CaptureSpaceMemberAccountBindingsRequest) (*userv1.CaptureSpaceMemberAccountBindingsResponse, error) {
	id := uuid.NewString()
	hash, err := principal.RequestHash(request)
	if err != nil {
		return nil, err
	}
	token, err := issuer.IssueService(principal.ServiceInput{Audience: "user", RPC: userv1.UserService_CaptureSpaceMemberAccountBindings_FullMethodName, RequestID: id, RequestHash: hash})
	if err != nil {
		return nil, err
	}
	return client.CaptureSpaceMemberAccountBindings(metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id)), request)
}
