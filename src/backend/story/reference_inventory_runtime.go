package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"sync"

	"google.golang.org/grpc"
	storyv1 "voice.app/voice/story/v1"
	"voice/backend/pkg/grpcmw"
	"voice/backend/story/internal/inventorygrpc"
)

type referenceInventoryRuntime struct {
	server    *grpc.Server
	listener  net.Listener
	principal *inventorygrpc.Runtime
	errors    <-chan error
	once      sync.Once
	closeErr  error
}

// startReferenceInventory is the sole registration site for the private service.
// The ordinary Story listener registers only StoryService.
func startReferenceInventory(ctx context.Context, cfg inventorygrpc.Config, enabled bool, store any, logger *slog.Logger) (*referenceInventoryRuntime, error) {
	if !enabled {
		return nil, nil
	}
	source, ok := store.(inventorygrpc.Store)
	if !ok || source == nil {
		return nil, errors.New("Story reference inventory Store unavailable")
	}
	auth, err := inventorygrpc.NewRuntime(ctx, cfg)
	if err != nil {
		return nil, err
	}
	lis, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		_ = auth.Close()
		return nil, err
	}
	options := append(grpcmw.ServerOptions(logger), auth.ServerOptions()...)
	server := grpc.NewServer(options...)
	storyv1.RegisterStoryReferenceInventoryServiceServer(server, inventorygrpc.New(source))
	errorsCh := make(chan error, 1)
	r := &referenceInventoryRuntime{server: server, listener: lis, principal: auth, errors: errorsCh}
	go func() { errorsCh <- server.Serve(lis) }()
	logger.Info("story reference inventory grpc listening", slog.String("addr", lis.Addr().String()))
	return r, nil
}

func (r *referenceInventoryRuntime) Close(ctx context.Context) error {
	if r == nil {
		return nil
	}
	r.once.Do(func() {
		done := make(chan struct{})
		go func() { r.server.GracefulStop(); close(done) }()
		select {
		case <-done:
		case <-ctx.Done():
			r.server.Stop()
			<-done
		}
		r.closeErr = r.principal.Close()
	})
	return r.closeErr
}
