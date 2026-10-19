package main

import (
	"context"
	"google.golang.org/grpc"
	"net"
	"sync"
	"voice/backend/chat/internal/grpcsvc"
	"voice/backend/chat/internal/reportprincipal"
)

func startReportPolicyServer(ctx context.Context, config reportprincipal.Config, enabled bool, store grpcsvc.ReportPolicyStore, shared []grpc.ServerOption) (func(), error) {
	if !enabled {
		return func() {}, nil
	}
	runtime, err := reportprincipal.New(ctx, config)
	if err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp", config.ListenAddr)
	if err != nil {
		_ = runtime.Close()
		return nil, err
	}
	options := append([]grpc.ServerOption{}, shared...)
	options = append(options, runtime.ServerOptions()...)
	server := grpc.NewServer(options...)
	grpcsvc.RegisterReportPolicyServer(server, &grpcsvc.ReportPolicyGRPC{Store: store})
	var once sync.Once
	stop := func() { once.Do(func() { server.Stop(); _ = listener.Close(); _ = runtime.Close() }) }
	go func() { _ = server.Serve(listener); stop() }()
	return stop, nil
}
