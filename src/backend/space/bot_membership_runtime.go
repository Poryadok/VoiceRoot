package main

import (
	"context"
	"google.golang.org/grpc"
	"net"
	"sync"
	"voice/backend/space/internal/botmembershipprincipal"
	"voice/backend/space/internal/grpcsvc"
)

// Assembly owns the enabled/config decision. This helper registers only the
// installation pair; ordinary Space and Role authority methods are absent.
func startBotMembershipServer(ctx context.Context, config botmembershipprincipal.Config, enabled bool, store grpcsvc.BotMembershipStore, shared []grpc.ServerOption) (func(), error) {
	if !enabled {
		return func() {}, nil
	}
	runtime, err := botmembershipprincipal.New(ctx, config)
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
	grpcsvc.RegisterBotMembershipServer(server, &grpcsvc.BotMembershipGRPC{Store: store})
	var once sync.Once
	stop := func() { once.Do(func() { server.Stop(); _ = listener.Close(); _ = runtime.Close() }) }
	go func() { _ = server.Serve(listener); stop() }()
	return stop, nil
}
