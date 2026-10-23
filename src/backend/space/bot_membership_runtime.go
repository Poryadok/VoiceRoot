package main

import (
	"context"
	"errors"
	"net"
	"sync"

	"google.golang.org/grpc"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/botmembershipprincipal"
	"voice/backend/space/internal/grpcsvc"
)

// botMembershipRuntime owns one dedicated, authenticated listener. Main must
// supervise Errors and call Stop on every later startup failure and shutdown.
// Construction neither registers nor enables the ordinary Space listener.
type botMembershipRuntime struct {
	listener  net.Listener
	server    *grpc.Server
	principal *botmembershipprincipal.Runtime
	cancel    context.CancelFunc
	once      sync.Once
	done      chan struct{}
	errors    chan error
}

func newBotMembershipRuntime(parent context.Context, config botmembershipprincipal.Config, persistence grpcsvc.BotMembershipStore) (*botMembershipRuntime, error) {
	if parent == nil {
		return nil, errors.New("bot membership parent context is required")
	}
	if err := parent.Err(); err != nil {
		return nil, err
	}
	adapter, err := grpcsvc.NewBotMembershipProtected(persistence)
	if err != nil {
		return nil, err
	}
	// Bind before contacting dependencies; duplicate ports fail without creating
	// a partially owned principal runtime. Do not serve until readiness succeeds.
	listener, err := net.Listen("tcp", config.ListenAddr)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(parent)
	principalRuntime, err := botmembershipprincipal.New(ctx, config)
	if err != nil {
		cancel()
		_ = listener.Close()
		return nil, err
	}
	server := grpc.NewServer(principalRuntime.ServerOptions()...)
	spacev1.RegisterSpaceServiceServer(server, adapter)
	r := &botMembershipRuntime{listener: listener, server: server, principal: principalRuntime, cancel: cancel, done: make(chan struct{}), errors: make(chan error, 1)}
	go func() {
		defer close(r.done)
		defer close(r.errors)
		if err := server.Serve(listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) && ctx.Err() == nil {
			r.errors <- err
		}
		r.closeOwned()
	}()
	go func() {
		select {
		case <-ctx.Done():
			r.closeOwned()
		case <-r.done:
		}
	}()
	return r, nil
}

func (r *botMembershipRuntime) Errors() <-chan error { return r.errors }

func (r *botMembershipRuntime) closeOwned() {
	r.once.Do(func() { r.cancel(); r.server.Stop(); _ = r.listener.Close(); _ = r.principal.Close() })
}

func (r *botMembershipRuntime) Stop() {
	if r == nil {
		return
	}
	r.closeOwned()
	<-r.done
}
