package main

import (
	"context"
	"errors"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
	"voice/backend/pkg/socialprincipal"
	"voice/backend/space/internal/authctx"
)

func loadCommunicationConfigs() ([]socialprincipal.Config, error) {
	var configs []socialprincipal.Config
	for _, listener := range []struct{ capability, prefix, address string }{
		{"space-voice-communication", "SPACE_VOICE_COMMUNICATION_", ":9094"},
		{"space-messaging-communication", "SPACE_MESSAGING_COMMUNICATION_", ":9095"},
	} {
		cfg, enabled, err := socialprincipal.LoadFromEnvWithAudience("space", listener.capability, listener.prefix, listener.address)
		if err != nil {
			return nil, err
		}
		if enabled {
			configs = append(configs, cfg)
		}
	}
	return configs, nil
}

// Communication RPCs have no ordinary-listener bearer or forwarded-header
// fallback, including when their protected listeners are absent.
func ordinaryCommunicationUnary() grpc.UnaryServerInterceptor {
	return authctx.OrdinaryCommunicationUnaryInterceptor()
}

func communicationIdentityUnary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		verified, ok := principal.FromContext(ctx)
		if !ok || verified.Kind != "service" || verified.Audience != "space" || verified.RPC != info.FullMethod {
			return nil, status.Error(codes.Unauthenticated, "verified communication principal required")
		}
		return handler(authctx.WithVerifiedServiceIdentity(ctx, authctx.ServiceIdentity(verified.Issuer)), req)
	}
}

func startCommunicationServers(ctx context.Context, configs []socialprincipal.Config, svc spacev1.SpaceServiceServer, shared []grpc.ServerOption) (func(), error) {
	var cleanups []func()
	cleanup := func() {
		for i := len(cleanups) - 1; i >= 0; i-- {
			cleanups[i]()
		}
	}
	for _, cfg := range configs {
		runtime, err := socialprincipal.New(ctx, cfg)
		if err != nil {
			cleanup()
			return nil, err
		}
		cleanups = append(cleanups, func() { _ = runtime.Close() })
		listener, err := net.Listen("tcp", cfg.ListenAddr)
		if err != nil {
			cleanup()
			return nil, err
		}
		opts := append([]grpc.ServerOption{}, shared...)
		opts = append(opts, runtime.ServerOptions()...)
		opts = append(opts, grpc.ChainUnaryInterceptor(communicationIdentityUnary()))
		server := grpc.NewServer(opts...)
		spacev1.RegisterSpaceServiceServer(server, svc)
		cleanups = append(cleanups, server.Stop)
		go func() {
			if err := server.Serve(listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
				panic(err)
			}
		}()
	}
	return cleanup, nil
}
