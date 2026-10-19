package main

import (
	"context"
	"net"
	"sync"

	"google.golang.org/grpc"
	"voice/backend/space/internal/grpcsvc"
	"voice/backend/space/internal/memberauthorityprincipal"
)

func startMemberAuthorityServer(ctx context.Context, config memberauthorityprincipal.Config, enabled bool, store grpcsvc.MemberAuthorityStore, shared []grpc.ServerOption) (func(), error) {
	if !enabled {
		return func() {}, nil
	}
	runtime, err := memberauthorityprincipal.New(ctx, config)
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
	grpcsvc.RegisterMemberAuthorityServer(server, &grpcsvc.MemberAuthorityGRPC{Store: store})
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
