package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// userStartup owns only User's startup transports. Reachable HTTP/2 is not
// application admission: all ordinary and dedicated RPC chains retain this
// deny gate until every configured startup prerequisite has completed.
type userStartup struct {
	ctx       context.Context
	cancel    context.CancelCauseFunc
	mu        sync.RWMutex
	ready     bool
	servers   []*grpc.Server
	listeners []net.Listener
}

func newUserStartup(parent context.Context) *userStartup {
	ctx, cancel := context.WithCancelCause(parent)
	return &userStartup{ctx: ctx, cancel: cancel}
}

func (o *userStartup) admitted() bool {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.ready && o.ctx.Err() == nil
}

func (o *userStartup) Release() error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.ctx.Err() != nil {
		return context.Cause(o.ctx)
	}
	o.ready = true
	return nil
}

func (o *userStartup) fail(err error) {
	if err == nil {
		err = errors.New("User gRPC listener stopped")
	}
	o.mu.Lock()
	o.ready = false
	o.cancel(err)
	o.mu.Unlock()
}

func (o *userStartup) unary(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	if !o.admitted() {
		return nil, status.Error(codes.Unavailable, "user startup unavailable")
	}
	return handler(ctx, req)
}

func (o *userStartup) stream(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	if !o.admitted() {
		return status.Error(codes.Unavailable, "user startup unavailable")
	}
	return handler(srv, stream)
}

func (o *userStartup) options() []grpc.ServerOption {
	return []grpc.ServerOption{grpc.ChainUnaryInterceptor(o.unary), grpc.ChainStreamInterceptor(o.stream)}
}

func (o *userStartup) http(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !o.admitted() {
			http.Error(w, "user startup unavailable", http.StatusServiceUnavailable)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type startupListener struct {
	net.Listener
	entered chan struct{}
	once    sync.Once
}

func (l *startupListener) Accept() (net.Conn, error) {
	l.once.Do(func() { close(l.entered) })
	return l.Listener.Accept()
}

// Serve waits until grpc.Server has entered its actual accept loop, rather than
// treating goroutine creation as listener startup. Any Serve failure cancels the
// peer wait and permanently revokes admission for this process incarnation.
func (o *userStartup) Serve(server *grpc.Server, listener net.Listener) error {
	o.mu.Lock()
	if o.ctx.Err() != nil {
		o.mu.Unlock()
		_ = listener.Close()
		return context.Cause(o.ctx)
	}
	o.servers = append(o.servers, server)
	o.listeners = append(o.listeners, listener)
	o.mu.Unlock()
	lis := &startupListener{Listener: listener, entered: make(chan struct{})}
	go func() { o.fail(server.Serve(lis)) }()
	select {
	case <-lis.entered:
		return context.Cause(o.ctx)
	case <-o.ctx.Done():
		return context.Cause(o.ctx)
	}
}

func (o *userStartup) Close() {
	o.fail(context.Canceled)
	o.mu.RLock()
	servers := append([]*grpc.Server(nil), o.servers...)
	listeners := append([]net.Listener(nil), o.listeners...)
	o.mu.RUnlock()
	for _, server := range servers {
		server.Stop()
	}
	for _, listener := range listeners {
		_ = listener.Close()
	}
}
