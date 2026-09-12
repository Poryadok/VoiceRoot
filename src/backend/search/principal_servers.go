package main

import (
	"context"
	"sync"

	"google.golang.org/grpc"
	searchv1 "voice.app/voice/search/v1"
	"voice/backend/search/internal/principalgrpc"
	"voice/backend/search/internal/principalruntime"
)

func newSearchGRPCServers(shared []grpc.ServerOption, service searchv1.SearchServiceServer, runtime *principalruntime.Runtime) (*grpc.Server, *grpc.Server) {
	ordinaryOptions := append([]grpc.ServerOption{}, shared...)
	ordinaryOptions = append(ordinaryOptions, grpc.ChainUnaryInterceptor(principalgrpc.OrdinaryUnaryInterceptor()))
	ordinary := grpc.NewServer(ordinaryOptions...)
	searchv1.RegisterSearchServiceServer(ordinary, service)
	if runtime == nil {
		return ordinary, nil
	}
	protectedOptions := append([]grpc.ServerOption{}, shared...)
	protectedOptions = append(protectedOptions, runtime.ServerOptions()...)
	protected := grpc.NewServer(protectedOptions...)
	searchv1.RegisterSearchServiceServer(protected, service)
	return ordinary, protected
}

func shutdownSearchServers(ctx context.Context, servers ...*grpc.Server) {
	var wg sync.WaitGroup
	for _, server := range servers {
		if server != nil {
			wg.Add(1)
			go func(s *grpc.Server) { defer wg.Done(); s.GracefulStop() }(server)
		}
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
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
