package main

import (
	"context"
	"google.golang.org/grpc"
	"sync"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/role/internal/principalgrpc"
	"voice/backend/role/internal/principalruntime"
)

func newRoleGRPCServers(shared []grpc.ServerOption, service rolev1.RoleServiceServer, runtime *principalruntime.Runtime) (*grpc.Server, *grpc.Server) {
	legacyOptions := append([]grpc.ServerOption{}, shared...)
	legacyOptions = append(legacyOptions, grpc.ChainUnaryInterceptor(principalgrpc.OwnershipUnaryInterceptor(nil)))
	legacy := grpc.NewServer(legacyOptions...)
	rolev1.RegisterRoleServiceServer(legacy, service)
	if runtime == nil {
		return legacy, nil
	}
	protectedOptions := append([]grpc.ServerOption{}, shared...)
	protectedOptions = append(protectedOptions, runtime.ServerOptions()...)
	protected := grpc.NewServer(protectedOptions...)
	rolev1.RegisterRoleServiceServer(protected, service)
	return legacy, protected
}

func shutdownRoleServers(ctx context.Context, servers ...*grpc.Server) {
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
				// GracefulStop may hold internal locks while an application
				// handler ignores cancellation. Never wait past the budget.
				go server.Stop()
			}
		}
	}
}
