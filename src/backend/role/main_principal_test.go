package main

import (
	"context"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	rolev1 "voice.app/voice/role/v1"
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
	_, err = client.ApplyOwnershipTransfer(ctx, &rolev1.ApplyOwnershipTransferRequest{})
	require.Equal(t, codes.Unavailable, status.Code(err))
	_, err = client.CompensateOwnershipTransfer(ctx, &rolev1.CompensateOwnershipTransferRequest{})
	require.Equal(t, codes.Unavailable, status.Code(err))
	require.Zero(t, service.ownershipCalls.Load())
	_, err = client.ListRoles(ctx, &rolev1.ListRolesRequest{})
	require.NoError(t, err)
	require.Equal(t, int64(1), service.listCalls.Load())
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
