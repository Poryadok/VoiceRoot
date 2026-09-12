package main

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	rolev1 "voice.app/voice/role/v1"
)

func (s *principalBootstrapRoleServer) RetireSpace(context.Context, *rolev1.RetireSpaceRequest) (*rolev1.RetireSpaceResponse, error) {
	s.ownershipCalls.Add(1)
	return &rolev1.RetireSpaceResponse{}, nil
}

func TestNewRoleGRPCServers_OrdinaryListenerDeniesRetireSpaceBeforeHandler(t *testing.T) {
	service := &principalBootstrapRoleServer{}
	ordinary, protected := newRoleGRPCServers(nil, service, nil)
	require.NotNil(t, ordinary)
	require.Nil(t, protected)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() { done <- ordinary.Serve(listener) }()
	t.Cleanup(func() { ordinary.Stop(); <-done })
	conn, err := grpc.NewClient(listener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	response, err := rolev1.NewRoleServiceClient(conn).RetireSpace(ctx, &rolev1.RetireSpaceRequest{})
	require.Equal(t, codes.Unavailable, status.Code(err))
	require.Nil(t, response)
	require.Zero(t, service.ownershipCalls.Load(), "ordinary listener must reject retirement before the handler")
}
