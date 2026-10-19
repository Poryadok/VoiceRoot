package main

import (
	"context"
	"errors"
	"log"
	"net"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/chat/internal/gisprincipal"
	"voice/backend/chat/internal/grpcsvc"
	"voice/backend/chat/internal/store"
)

type chatScopeListenerConfig struct {
	Principal gisprincipal.Config
	Listen    string
}

func loadChatScopeAuthorityConfigs() ([]chatScopeListenerConfig, error) {
	configs := make([]chatScopeListenerConfig, 0, 3)
	for _, issuer := range []string{"role", "space", "messaging"} {
		cfg, listen, enabled, err := gisprincipal.ScopeAuthorityConfigFromEnv(issuer)
		if err != nil {
			return nil, err
		}
		if enabled {
			configs = append(configs, chatScopeListenerConfig{cfg, listen})
		}
	}
	return configs, nil
}

func startChatScopeAuthorityListeners(ctx context.Context, pool *pgxpool.Pool, configs []chatScopeListenerConfig) (func(), error) {
	cleanups := []func(){}
	cleanup := func() {
		for i := len(cleanups) - 1; i >= 0; i-- {
			cleanups[i]()
		}
	}
	if len(configs) > 0 && pool == nil {
		return cleanup, errors.New("chat scope authority requires DATABASE_URL")
	}
	for _, cfg := range configs {
		runtime, err := gisprincipal.New(ctx, cfg.Principal)
		if err != nil {
			cleanup()
			return func() {}, err
		}
		listener, err := net.Listen("tcp", cfg.Listen)
		if err != nil {
			_ = runtime.Close()
			cleanup()
			return func() {}, err
		}
		server := grpc.NewServer(runtime.ServerOptions()...)
		chatv1.RegisterChatScopeAuthorityServiceServer(server, &grpcsvc.SpaceChatScopeAuthorityGRPC{Store: &store.DMStore{Pool: pool}})
		cleanups = append(cleanups, func() { server.Stop(); _ = runtime.Close() })
		go func() {
			if err := server.Serve(listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
				log.Fatalf("Chat scope authority serve: %v", err)
			}
		}()
	}
	return cleanup, nil
}
