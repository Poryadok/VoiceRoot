package reporttargetprincipal

import (
	"context"
	"errors"
	"google.golang.org/grpc"
	"net"
)

// Start binds a dedicated mTLS listener, exposing only the owner's narrow RPC.
// No settings means disabled; any partial configuration is a startup error.
func Start(ctx context.Context, target string, register func(grpc.ServiceRegistrar), onError func(error)) (func(), error) {
	cfg, enabled, err := LoadFromEnv(target)
	if err != nil {
		return nil, err
	}
	if !enabled {
		return func() {}, nil
	}
	runtime, err := New(ctx, cfg)
	if err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		_ = runtime.Close()
		return nil, err
	}
	server := grpc.NewServer(runtime.ServerOptions()...)
	register(server)
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) && onError != nil {
			onError(err)
		}
	}()
	return func() { server.Stop(); _ = listener.Close(); _ = runtime.Close() }, nil
}
