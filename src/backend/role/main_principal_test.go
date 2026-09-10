package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
	"voice/backend/role/internal/principalruntime"
)

type principalBootstrapRoleServer struct {
	rolev1.UnimplementedRoleServiceServer
	ownershipCalls atomic.Int64
	listCalls      atomic.Int64
}

func (s *principalBootstrapRoleServer) ApplyOwnershipTransfer(context.Context, *rolev1.ApplyOwnershipTransferRequest) (*rolev1.ApplyOwnershipTransferResponse, error) {
	s.ownershipCalls.Add(1)
	return &rolev1.ApplyOwnershipTransferResponse{}, nil
}
func (s *principalBootstrapRoleServer) CompensateOwnershipTransfer(context.Context, *rolev1.CompensateOwnershipTransferRequest) (*rolev1.CompensateOwnershipTransferResponse, error) {
	s.ownershipCalls.Add(1)
	return &rolev1.CompensateOwnershipTransferResponse{}, nil
}
func (s *principalBootstrapRoleServer) GetOwnershipTransferCapabilities(context.Context, *rolev1.GetOwnershipTransferCapabilitiesRequest) (*rolev1.GetOwnershipTransferCapabilitiesResponse, error) {
	s.ownershipCalls.Add(1)
	return &rolev1.GetOwnershipTransferCapabilitiesResponse{}, nil
}
func (s *principalBootstrapRoleServer) PrepareOwnershipTransfer(context.Context, *rolev1.PrepareOwnershipTransferRequest) (*rolev1.PrepareOwnershipTransferResponse, error) {
	s.ownershipCalls.Add(1)
	return &rolev1.PrepareOwnershipTransferResponse{}, nil
}
func (s *principalBootstrapRoleServer) FinalizeOwnershipTransfer(context.Context, *rolev1.FinalizeOwnershipTransferRequest) (*rolev1.FinalizeOwnershipTransferResponse, error) {
	s.ownershipCalls.Add(1)
	return &rolev1.FinalizeOwnershipTransferResponse{}, nil
}
func (s *principalBootstrapRoleServer) AbortOwnershipTransfer(context.Context, *rolev1.AbortOwnershipTransferRequest) (*rolev1.AbortOwnershipTransferResponse, error) {
	s.ownershipCalls.Add(1)
	return &rolev1.AbortOwnershipTransferResponse{}, nil
}
func (s *principalBootstrapRoleServer) ListRoles(context.Context, *rolev1.ListRolesRequest) (*rolev1.ListRolesResponse, error) {
	s.listCalls.Add(1)
	return &rolev1.ListRolesResponse{}, nil
}

// This exercises the server factory used by main, including real registration
// and transport. The legacy listener must never expose ownership without runtime.
func TestNewRoleGRPCServersLegacyDeniesOwnershipWhenRuntimeAbsent(t *testing.T) {
	service := &principalBootstrapRoleServer{}
	legacy, protected := newRoleGRPCServers(nil, service, nil)
	require.NotNil(t, legacy)
	require.Nil(t, protected)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() { done <- legacy.Serve(listener) }()
	t.Cleanup(func() { legacy.Stop(); <-done })
	conn, err := grpc.NewClient(listener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })
	client := rolev1.NewRoleServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	ownershipCalls := []func() error{
		func() error {
			_, err := client.GetOwnershipTransferCapabilities(ctx, &rolev1.GetOwnershipTransferCapabilitiesRequest{})
			return err
		},
		func() error {
			_, err := client.PrepareOwnershipTransfer(ctx, &rolev1.PrepareOwnershipTransferRequest{})
			return err
		},
		func() error {
			_, err := client.FinalizeOwnershipTransfer(ctx, &rolev1.FinalizeOwnershipTransferRequest{})
			return err
		},
		func() error {
			_, err := client.AbortOwnershipTransfer(ctx, &rolev1.AbortOwnershipTransferRequest{})
			return err
		},
		func() error {
			_, err := client.ApplyOwnershipTransfer(ctx, &rolev1.ApplyOwnershipTransferRequest{})
			return err
		},
		func() error {
			_, err := client.CompensateOwnershipTransfer(ctx, &rolev1.CompensateOwnershipTransferRequest{})
			return err
		},
	}
	for _, call := range ownershipCalls {
		require.Equal(t, codes.Unavailable, status.Code(call()))
	}
	require.Zero(t, service.ownershipCalls.Load())
	_, err = client.ListRoles(ctx, &rolev1.ListRolesRequest{})
	require.NoError(t, err)
	require.Equal(t, int64(1), service.listCalls.Load())
}

func TestNewRoleGRPCServers_ConfiguredRuntimeEnforcesListenerMatrix(t *testing.T) {
	runtime, roots, key := newMainPrincipalRuntime(t)
	service := &principalBootstrapRoleServer{}
	legacy, protected := newRoleGRPCServers(nil, service, runtime)
	require.NotNil(t, legacy)
	require.NotNil(t, protected)

	legacyListener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	protectedListener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	legacyDone, protectedDone := make(chan error, 1), make(chan error, 1)
	go func() { legacyDone <- legacy.Serve(legacyListener) }()
	go func() { protectedDone <- protected.Serve(protectedListener) }()
	t.Cleanup(func() {
		legacy.Stop()
		protected.Stop()
		<-legacyDone
		<-protectedDone
	})
	legacyConn, err := grpc.NewClient(legacyListener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, legacyConn.Close()) })
	protectedConn, err := grpc.NewClient(protectedListener.Addr().String(), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots})))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, protectedConn.Close()) })
	legacyClient := rolev1.NewRoleServiceClient(legacyConn)
	protectedClient := rolev1.NewRoleServiceClient(protectedConn)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	legacyCalls := []func() error{
		func() error {
			_, err := legacyClient.GetOwnershipTransferCapabilities(ctx, &rolev1.GetOwnershipTransferCapabilitiesRequest{})
			return err
		},
		func() error {
			_, err := legacyClient.PrepareOwnershipTransfer(ctx, &rolev1.PrepareOwnershipTransferRequest{})
			return err
		},
		func() error {
			_, err := legacyClient.FinalizeOwnershipTransfer(ctx, &rolev1.FinalizeOwnershipTransferRequest{})
			return err
		},
		func() error {
			_, err := legacyClient.AbortOwnershipTransfer(ctx, &rolev1.AbortOwnershipTransferRequest{})
			return err
		},
		func() error {
			_, err := legacyClient.ApplyOwnershipTransfer(ctx, &rolev1.ApplyOwnershipTransferRequest{})
			return err
		},
		func() error {
			_, err := legacyClient.CompensateOwnershipTransfer(ctx, &rolev1.CompensateOwnershipTransferRequest{})
			return err
		},
	}
	for _, call := range legacyCalls {
		require.Equal(t, codes.Unavailable, status.Code(call()))
	}

	_, err = protectedClient.ApplyOwnershipTransfer(ctx, &rolev1.ApplyOwnershipTransferRequest{})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	_, err = protectedClient.CompensateOwnershipTransfer(ctx, &rolev1.CompensateOwnershipTransferRequest{})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	_, err = protectedClient.ListRoles(ctx, &rolev1.ListRolesRequest{})
	require.Equal(t, codes.PermissionDenied, status.Code(err))

	v2Calls := []struct {
		method string
		req    proto.Message
		call   func(context.Context) error
	}{
		{rolev1.RoleService_GetOwnershipTransferCapabilities_FullMethodName, &rolev1.GetOwnershipTransferCapabilitiesRequest{}, func(callCtx context.Context) error {
			_, err := protectedClient.GetOwnershipTransferCapabilities(callCtx, &rolev1.GetOwnershipTransferCapabilitiesRequest{})
			return err
		}},
		{rolev1.RoleService_PrepareOwnershipTransfer_FullMethodName, &rolev1.PrepareOwnershipTransferRequest{}, func(callCtx context.Context) error {
			_, err := protectedClient.PrepareOwnershipTransfer(callCtx, &rolev1.PrepareOwnershipTransferRequest{})
			return err
		}},
		{rolev1.RoleService_FinalizeOwnershipTransfer_FullMethodName, &rolev1.FinalizeOwnershipTransferRequest{}, func(callCtx context.Context) error {
			_, err := protectedClient.FinalizeOwnershipTransfer(callCtx, &rolev1.FinalizeOwnershipTransferRequest{})
			return err
		}},
		{rolev1.RoleService_AbortOwnershipTransfer_FullMethodName, &rolev1.AbortOwnershipTransferRequest{}, func(callCtx context.Context) error {
			_, err := protectedClient.AbortOwnershipTransfer(callCtx, &rolev1.AbortOwnershipTransferRequest{})
			return err
		}},
	}
	for index, call := range v2Calls {
		hash, err := principal.RequestHash(call.req)
		require.NoError(t, err)
		requestID := "factory-v2-" + string(rune('a'+index))
		token := mainPrincipalToken(t, key, call.method, requestID, hash)
		callCtx := metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID))
		require.NoError(t, call.call(callCtx))
	}
	require.Equal(t, int64(len(v2Calls)), service.ownershipCalls.Load())
}

func newMainPrincipalRuntime(t *testing.T) (*principalruntime.Runtime, *x509.CertPool, *rsa.PrivateKey) {
	t.Helper()
	current, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	next, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	jwk := func(kid string, key *rsa.PrivateKey) map[string]string {
		return map[string]string{"kid": kid, "kty": "RSA", "use": "sig", "alg": "RS256", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes())}
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{jwk("current", current), jwk("next", next)}})
	}))
	t.Cleanup(server.Close)
	dir := t.TempDir()
	certFile, keyFile := filepath.Join(dir, "tls.crt"), filepath.Join(dir, "tls.key")
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.TLS.Certificates[0].Certificate[0]})
	require.NoError(t, os.WriteFile(certFile, certPEM, 0o600))
	keyDER, err := x509.MarshalPKCS8PrivateKey(server.TLS.Certificates[0].PrivateKey)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0o600))
	replay := miniredis.RunT(t)
	runtime, err := principalruntime.New(context.Background(), principalruntime.Config{
		JWKSURLs: map[string]string{"space": server.URL}, RefreshAfter: time.Minute, HardExpiry: 2 * time.Minute,
		UnknownKIDCooldown: time.Second, ReplayAddr: replay.Addr(), JWKSCAFile: certFile,
		TLSCertFile: certFile, TLSKeyFile: keyFile, ListenAddr: "127.0.0.1:0",
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, runtime.Close()) })
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(certPEM))
	return runtime, roots, current
}

func mainPrincipalToken(t *testing.T, key *rsa.PrivateKey, rpc, requestID, hash string) string {
	t.Helper()
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "space", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	token, err := issuer.IssueService(principal.ServiceInput{Audience: "role", RPC: rpc, RequestID: requestID, RequestHash: hash})
	require.NoError(t, err)
	return token
}

type blockingShutdownRoleServer struct {
	rolev1.UnimplementedRoleServiceServer
	entered chan struct{}
}

func (s *blockingShutdownRoleServer) ListRoles(ctx context.Context, _ *rolev1.ListRolesRequest) (*rolev1.ListRolesResponse, error) {
	close(s.entered)
	<-ctx.Done()
	return nil, status.FromContextError(ctx.Err()).Err()
}

func TestShutdownRoleServersBoundsInFlightRPC(t *testing.T) {
	service := &blockingShutdownRoleServer{entered: make(chan struct{})}
	server := grpc.NewServer()
	rolev1.RegisterRoleServiceServer(server, service)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()
	t.Cleanup(func() { server.Stop(); <-served })
	conn, err := grpc.NewClient(listener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })
	// No client deadline: shutdown itself must release an otherwise stuck RPC.
	rpcCtx, rpcCancel := context.WithCancel(context.Background())
	defer rpcCancel()
	rpcDone := make(chan error, 1)
	go func() {
		_, err := rolev1.NewRoleServiceClient(conn).ListRoles(rpcCtx, &rolev1.ListRolesRequest{})
		rpcDone <- err
	}()
	select {
	case <-service.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("RPC did not enter handler")
	}
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer shutdownCancel()
	shutdownDone := make(chan struct{})
	go func() { shutdownRoleServers(shutdownCtx, server); close(shutdownDone) }()
	select {
	case <-shutdownDone:
	case <-time.After(time.Second):
		t.Fatal("shutdown hung on in-flight RPC beyond deadline")
	}
	select {
	case err := <-rpcDone:
		require.Error(t, err)
		require.Contains(t, []codes.Code{codes.Unavailable, codes.Canceled}, status.Code(err))
	case <-time.After(time.Second):
		t.Fatal("shutdown did not release in-flight RPC")
	}
}

type ignoringContextRoleServer struct {
	rolev1.UnimplementedRoleServiceServer
	entered chan struct{}
	release chan struct{}
}

func (s *ignoringContextRoleServer) ListRoles(context.Context, *rolev1.ListRolesRequest) (*rolev1.ListRolesResponse, error) {
	close(s.entered)
	<-s.release
	return &rolev1.ListRolesResponse{}, nil
}

func TestShutdownRoleServersBoundsHandlerIgnoringCancellation(t *testing.T) {
	service := &ignoringContextRoleServer{entered: make(chan struct{}), release: make(chan struct{})}
	// Release only after the bounded-return assertion, including failure cleanup.
	defer close(service.release)
	server := grpc.NewServer()
	rolev1.RegisterRoleServiceServer(server, service)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()
	t.Cleanup(func() { server.Stop(); <-served })
	conn, err := grpc.NewClient(listener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })
	rpcCtx, rpcCancel := context.WithCancel(context.Background())
	defer rpcCancel()
	rpcDone := make(chan error, 1)
	go func() {
		_, err := rolev1.NewRoleServiceClient(conn).ListRoles(rpcCtx, &rolev1.ListRolesRequest{})
		rpcDone <- err
	}()
	select {
	case <-service.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("RPC did not enter handler")
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	done := make(chan struct{})
	go func() { shutdownRoleServers(shutdownCtx, server); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("shutdown waited for a handler ignoring cancellation")
	}
	// The release channel is still open here; returning must not depend on the
	// handler cooperating with cancellation or completing application work.
	select {
	case err := <-rpcDone:
		require.Error(t, err)
	case <-time.After(time.Second):
		t.Fatal("forced shutdown did not terminate client RPC")
	}
}
