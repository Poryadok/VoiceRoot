package principalruntime

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
)

type recordingRoleServer struct {
	rolev1.UnimplementedRoleServiceServer
	calls      atomic.Int64
	principals chan principal.Principal
}

func (s *recordingRoleServer) ApplyOwnershipTransfer(ctx context.Context, _ *rolev1.ApplyOwnershipTransferRequest) (*rolev1.ApplyOwnershipTransferResponse, error) {
	s.calls.Add(1)
	p, _ := principal.FromContext(ctx)
	s.principals <- p
	return &rolev1.ApplyOwnershipTransferResponse{}, nil
}
func (s *recordingRoleServer) ListRoles(context.Context, *rolev1.ListRolesRequest) (*rolev1.ListRolesResponse, error) {
	s.calls.Add(1)
	return &rolev1.ListRolesResponse{}, nil
}

func startRuntimeListener(t *testing.T, f runtimeFixture) (string, *recordingRoleServer, *x509.CertPool) {
	t.Helper()
	runtime := startFixtureRuntime(t, f)
	require.NoError(t, runtime.ActivateOwnershipV2Capabilities(OwnershipV2Activation{
		V1Drained: true, RetiredSpaceFence: true,
		SupportedMethods: []string{
			rolev1.RoleService_PrepareOwnershipTransfer_FullMethodName,
			rolev1.RoleService_FinalizeOwnershipTransfer_FullMethodName,
			rolev1.RoleService_AbortOwnershipTransfer_FullMethodName,
		},
	}))
	return startRuntimeListenerWithRuntime(t, f, runtime)
}

func startRuntimeListenerWithRuntime(t *testing.T, f runtimeFixture, runtime *Runtime) (string, *recordingRoleServer, *x509.CertPool) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := grpc.NewServer(runtime.ServerOptions()...)
	recorder := &recordingRoleServer{principals: make(chan principal.Principal, 10)}
	rolev1.RegisterRoleServiceServer(server, recorder)
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	t.Cleanup(func() { server.Stop(); <-done })
	ca, err := os.ReadFile(f.config.TLSCertFile)
	require.NoError(t, err)
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(ca))
	return listener.Addr().String(), recorder, roots
}

func runtimeListenerClient(t *testing.T, address string, transport credentials.TransportCredentials) rolev1.RoleServiceClient {
	t.Helper()
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(transport))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })
	return rolev1.NewRoleServiceClient(conn)
}

func TestRuntimeListenerAuthenticatesBoundV2OwnershipAndRejectsBeforeHandler(t *testing.T) {
	f := newRuntimeFixture(t, 2)
	address, recorder, roots := startRuntimeListener(t, f)
	client := runtimeListenerClient(t, address, credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}))
	request := &rolev1.PrepareOwnershipTransferRequest{Intent: &rolev1.OwnershipTransferIntent{ProtocolVersion: 2, SpaceId: "space-1", OldOwnerProfileId: "owner-1", NewOwnerProfileId: "owner-2", OperationId: "operation-1"}}
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	method := rolev1.RoleService_PrepareOwnershipTransfer_FullMethodName
	token := issueRuntimeToken(t, f.key, "space", "current", "role", method, "listener-valid", hash)
	md := metadata.Pairs("authorization", "Bearer "+token, "x-request-id", "listener-valid")
	ctx, cancel := context.WithTimeout(metadata.NewOutgoingContext(context.Background(), md), 2*time.Second)
	defer cancel()
	_, err = client.PrepareOwnershipTransfer(ctx, request)
	require.NoError(t, err)
	require.Equal(t, int64(1), recorder.calls.Load())
	verified := <-recorder.principals
	require.Equal(t, "service:space", verified.Subject)
	require.Equal(t, hash, verified.RequestHash)
	require.Equal(t, method, verified.RPC)
	require.Empty(t, verified.AccountID)
	require.Empty(t, verified.ProfileID)
	require.Zero(t, verified.SessionEpoch)
	_, err = client.PrepareOwnershipTransfer(ctx, request)
	require.Equal(t, codes.Unauthenticated, status.Code(err), "replay must stop before handler")
	require.Equal(t, int64(1), recorder.calls.Load())
	for _, name := range []string{"duplicate_authorization", "duplicate_request_id", "raw_identity", "wrong_issuer", "wrong_audience", "wrong_rpc", "wrong_hash", "unknown_kid", "session_epoch"} {
		t.Run(name, func(t *testing.T) {
			issuer, kid, audience, signedRPC := "space", "current", "role", method
			signedHash := hash
			switch name {
			case "wrong_issuer":
				issuer = "gateway"
			case "wrong_audience":
				audience = "space"
			case "wrong_rpc":
				signedRPC = rolev1.RoleService_AbortOwnershipTransfer_FullMethodName
			case "wrong_hash":
				signedHash = runtimeHash
			case "unknown_kid":
				kid = "unpublished"
			}
			token := issueRuntimeToken(t, f.key, issuer, kid, audience, signedRPC, name, signedHash)
			if name == "session_epoch" {
				token = withSignedRuntimeClaim(t, token, f.key, "session_epoch")
			}
			md := metadata.Pairs("authorization", "Bearer "+token, "x-request-id", name)
			if name == "duplicate_authorization" {
				md.Append("authorization", "Bearer "+token)
			}
			if name == "duplicate_request_id" {
				md.Append("x-request-id", "other")
			}
			if name == "raw_identity" {
				md.Set("x-voice-profile-id", "forged")
			}
			ctx, cancel := context.WithTimeout(metadata.NewOutgoingContext(context.Background(), md), 2*time.Second)
			defer cancel()
			_, err := client.PrepareOwnershipTransfer(ctx, request)
			require.Equal(t, codes.Unauthenticated, status.Code(err))
			require.Equal(t, int64(1), recorder.calls.Load(), "invalid credentials reached handler")
		})
	}
	listRequest := &rolev1.ListRolesRequest{}
	listHash, err := principal.RequestHash(listRequest)
	require.NoError(t, err)
	listToken := issueRuntimeToken(t, f.key, "space", "current", "role", rolev1.RoleService_ListRoles_FullMethodName, "list-request", listHash)
	listCtx, listCancel := context.WithTimeout(metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+listToken, "x-request-id", "list-request")), 2*time.Second)
	defer listCancel()
	_, err = client.ListRoles(listCtx, listRequest)
	require.Equal(t, codes.PermissionDenied, status.Code(err), "the dedicated listener must deny nonownership RPCs")
	require.Equal(t, int64(1), recorder.calls.Load())
}

func TestRuntimeListenerRequiresTLSAndVerifiedServerIdentity(t *testing.T) {
	f := newRuntimeFixture(t, 2)
	address, recorder, roots := startRuntimeListener(t, f)
	cases := []struct {
		name      string
		transport credentials.TransportCredentials
	}{
		{"plaintext", insecure.NewCredentials()},
		{"untrusted_ca", credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: x509.NewCertPool()})},
		{"wrong_server_identity", credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: "wrong.role.internal"})},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := runtimeListenerClient(t, address, tc.transport)
			request := &rolev1.PrepareOwnershipTransferRequest{Intent: &rolev1.OwnershipTransferIntent{ProtocolVersion: 2, SpaceId: "space-1", OperationId: "transport-check"}}
			hash, err := principal.RequestHash(request)
			require.NoError(t, err)
			token := issueRuntimeToken(t, f.key, "space", "current", "role", rolev1.RoleService_PrepareOwnershipTransfer_FullMethodName, tc.name, hash)
			ctx, cancel := context.WithTimeout(metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", tc.name)), time.Second)
			defer cancel()
			_, err = client.PrepareOwnershipTransfer(ctx, request)
			require.Error(t, err)
			require.Contains(t, []codes.Code{codes.Unavailable, codes.DeadlineExceeded}, status.Code(err), "failure must occur in transport, before auth or handler")
			require.Zero(t, recorder.calls.Load())
		})
	}
}

func (s *recordingRoleServer) CompensateOwnershipTransfer(ctx context.Context, _ *rolev1.CompensateOwnershipTransferRequest) (*rolev1.CompensateOwnershipTransferResponse, error) {
	s.calls.Add(1)
	p, _ := principal.FromContext(ctx)
	s.principals <- p
	return &rolev1.CompensateOwnershipTransferResponse{}, nil
}

func TestRuntimeListenerAuthenticatesBoundV2Abort(t *testing.T) {
	f := newRuntimeFixture(t, 2)
	address, recorder, roots := startRuntimeListener(t, f)
	client := runtimeListenerClient(t, address, credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}))
	request := &rolev1.AbortOwnershipTransferRequest{Intent: &rolev1.OwnershipTransferIntent{ProtocolVersion: 2}}
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	method := rolev1.RoleService_AbortOwnershipTransfer_FullMethodName
	token := issueRuntimeToken(t, f.next, "space", "next", "role", method, "compensation", hash)
	ctx, cancel := context.WithTimeout(metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", "compensation")), 2*time.Second)
	defer cancel()
	_, err = client.AbortOwnershipTransfer(ctx, request)
	require.NoError(t, err)
	require.Equal(t, int64(1), recorder.calls.Load())
	verified := <-recorder.principals
	require.Equal(t, "service:space", verified.Subject)
	require.Equal(t, "role", verified.Audience)
	require.Equal(t, method, verified.RPC)
	require.Equal(t, hash, verified.RequestHash)
	require.Equal(t, "compensation", verified.RequestID)
	require.Empty(t, verified.AccountID)
	require.Empty(t, verified.ProfileID)
	require.Zero(t, verified.SessionEpoch)
}

func TestRuntimeListenerDistinguishesUnavailableJWKSFromUntrustedCredentials(t *testing.T) {
	for _, tc := range []struct {
		name string
		want codes.Code
	}{
		{"unavailable", codes.Unavailable},
		{"malformed", codes.Unauthenticated},
		{"incomplete", codes.Unauthenticated},
		{"untrusted_tls", codes.Unauthenticated},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newRuntimeFixture(t, 2)
			switch tc.name {
			case "unavailable":
				f.jwks.set(http.StatusServiceUnavailable, nil)
			case "malformed":
				f.jwks.set(http.StatusOK, []byte(`{"keys":`))
			case "incomplete":
				f.jwks.set(http.StatusOK, runtimeJWKSDocument(t, map[string]*rsa.PrivateKey{"current": f.key}))
			case "untrusted_tls":
				f.config.JWKSCAFile = ""
			}
			runtime, err := New(context.Background(), f.config)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, runtime.Close()) })
			// Deliberately do not prime: dependency failures must be classified when
			// no complete trusted last-good set can authenticate the credential.
			address, recorder, roots := startRuntimeListenerWithRuntime(t, f, runtime)
			client := runtimeListenerClient(t, address, credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}))
			request := &rolev1.PrepareOwnershipTransferRequest{Intent: &rolev1.OwnershipTransferIntent{ProtocolVersion: 2, SpaceId: "space-1"}}
			hash, err := principal.RequestHash(request)
			require.NoError(t, err)
			token := issueRuntimeToken(t, f.key, "space", "current", "role", rolev1.RoleService_PrepareOwnershipTransfer_FullMethodName, tc.name, hash)
			ctx, cancel := context.WithTimeout(metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", tc.name)), 2*time.Second)
			defer cancel()
			_, err = client.PrepareOwnershipTransfer(ctx, request)
			require.Equal(t, tc.want, status.Code(err))
			if tc.name == "unavailable" {
				// A failed verification does not consume replay, and issuer
				// cooldown must preserve the dependency's Unavailable category.
				_, err = client.PrepareOwnershipTransfer(ctx, request)
				require.Equal(t, codes.Unavailable, status.Code(err))
			}
			require.Zero(t, recorder.calls.Load())
			require.Empty(t, recorder.principals)
		})
	}
}
