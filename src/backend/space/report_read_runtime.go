package main

import (
	"context"
	"net"
	"sync"

	"google.golang.org/grpc"
	"voice/backend/space/internal/grpcsvc"
	"voice/backend/space/internal/reportprincipal"
)

func startReportReadServer(ctx context.Context, config reportprincipal.Config, enabled bool, store grpcsvc.ReportReadStore, shared []grpc.ServerOption) (func(), error) {
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
	grpcsvc.RegisterReportReadServer(server, &grpcsvc.ReportReadGRPC{Store: store})
	var once sync.Once
	stop := func() {
		once.Do(func() {
			server.Stop()
			_ = listener.Close()
			_ = runtime.Close()
		})
	}
	go func() {
		_ = server.Serve(listener)
		stop()
	}()
	return stop, nil
}
