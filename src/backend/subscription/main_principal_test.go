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

	subscriptionv1 "voice.app/voice/subscription/v1"
)

type correctionPrincipalSubscriptionServer struct {
	subscriptionv1.UnimplementedSubscriptionServiceServer
	lifecycleCalls atomic.Int64
	publicCalls    atomic.Int64
}

func (s *correctionPrincipalSubscriptionServer) ApplySpaceLifecycleFence(context.Context, *subscriptionv1.ApplySpaceLifecycleFenceRequest) (*subscriptionv1.ApplySpaceLifecycleFenceResponse, error) {
	s.lifecycleCalls.Add(1)
	return &subscriptionv1.ApplySpaceLifecycleFenceResponse{}, nil
}

func (s *correctionPrincipalSubscriptionServer) PurgeSpace(context.Context, *subscriptionv1.PurgeSpaceRequest) (*subscriptionv1.PurgeSpaceResponse, error) {
	s.lifecycleCalls.Add(1)
	return &subscriptionv1.PurgeSpaceResponse{}, nil
}

func (s *correctionPrincipalSubscriptionServer) GetSubscription(context.Context, *subscriptionv1.GetSubscriptionRequest) (*subscriptionv1.GetSubscriptionResponse, error) {
	s.publicCalls.Add(1)
	return &subscriptionv1.GetSubscriptionResponse{}, nil
}

func TestR23CorrectionOrdinaryListenerDrainsProtectedLifecycleRPCs(t *testing.T) {
	service := &correctionPrincipalSubscriptionServer{}
	ordinary, protected := newSubscriptionGRPCServers(nil, service, nil)
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
	client := subscriptionv1.NewSubscriptionServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err = client.ApplySpaceLifecycleFence(ctx, &subscriptionv1.ApplySpaceLifecycleFenceRequest{})
	require.Equal(t, codes.Unavailable, status.Code(err))
	_, err = client.PurgeSpace(ctx, &subscriptionv1.PurgeSpaceRequest{})
	require.Equal(t, codes.Unavailable, status.Code(err))
	require.Zero(t, service.lifecycleCalls.Load())
	_, err = client.GetSubscription(ctx, &subscriptionv1.GetSubscriptionRequest{})
	require.NoError(t, err)
	require.Equal(t, int64(1), service.publicCalls.Load())
}
