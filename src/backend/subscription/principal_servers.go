package main

import (
	"context"
	"sync"

	"google.golang.org/grpc"

	"voice/backend/subscription/internal/principalgrpc"
	"voice/backend/subscription/internal/principalruntime"

	subscriptionv1 "voice.app/voice/subscription/v1"
)

func newSubscriptionGRPCServers(shared []grpc.ServerOption, service subscriptionv1.SubscriptionServiceServer, runtime *principalruntime.Runtime) (*grpc.Server, *grpc.Server) {
	ordinaryOptions := append([]grpc.ServerOption{}, shared...)
	ordinaryOptions = append(ordinaryOptions, grpc.ChainUnaryInterceptor(principalgrpc.OrdinaryUnaryInterceptor()))
	ordinary := grpc.NewServer(ordinaryOptions...)
	subscriptionv1.RegisterSubscriptionServiceServer(ordinary, service)
	if runtime == nil {
		return ordinary, nil
	}
	protectedOptions := append([]grpc.ServerOption{}, shared...)
	protectedOptions = append(protectedOptions, runtime.ServerOptions()...)
	protected := grpc.NewServer(protectedOptions...)
	subscriptionv1.RegisterSubscriptionServiceServer(protected, service)
	return ordinary, protected
}

func shutdownSubscriptionServers(ctx context.Context, servers ...*grpc.Server) {
	var waiting sync.WaitGroup
	for _, server := range servers {
		if server != nil {
			waiting.Add(1)
			go func(server *grpc.Server) { defer waiting.Done(); server.GracefulStop() }(server)
		}
	}
	done := make(chan struct{})
	go func() { waiting.Wait(); close(done) }()
	select {
	case <-done:
	case <-ctx.Done():
		for _, server := range servers {
			if server != nil {
				go server.Stop()
			}
		}
	}
}
