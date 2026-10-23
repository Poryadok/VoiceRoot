package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	authv1 "voice.app/voice/auth/v1"
	"voice/backend/pkg/grpcclient"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/creatorauthority"
	"voice/backend/space/internal/lifecycleprincipal"
	"voice/backend/space/internal/store"
)

type creatorRuntimeConfig struct{ address, caFile, serverName, certFile, keyFile string }

func loadCreatorRuntimeConfig(getenv func(string) string, publicEnabled bool) (creatorRuntimeConfig, bool, error) {
	names := []string{"SPACE_CREATOR_AUTH_GRPC_ADDR", "SPACE_CREATOR_AUTH_TLS_CA_FILE", "SPACE_CREATOR_AUTH_TLS_SERVER_NAME", "SPACE_CREATOR_AUTH_CLIENT_CERT_FILE", "SPACE_CREATOR_AUTH_CLIENT_KEY_FILE"}
	values := make([]string, len(names))
	enabled := false
	for i, name := range names {
		values[i] = getenv(name)
		enabled = enabled || values[i] != ""
	}
	if !enabled {
		return creatorRuntimeConfig{}, false, nil
	}
	for i, value := range values {
		if value == "" || value != strings.TrimSpace(value) {
			return creatorRuntimeConfig{}, false, fmt.Errorf("%s is required without surrounding whitespace", names[i])
		}
	}
	if !publicEnabled || strings.TrimSpace(getenv("DATABASE_URL")) == "" {
		return creatorRuntimeConfig{}, false, errors.New("creator transport requires DATABASE_URL and protected Gateway listener")
	}
	return creatorRuntimeConfig{values[0], values[1], values[2], values[3], values[4]}, true, nil
}

type creatorReplayLedger interface {
	ClaimCreatorPrincipalReplay(context.Context, string, string, time.Time) error
}

// These are the existing owner-specific providers, not legacy Redis callbacks.
func creatorDependencies(client creatorauthority.Client, issuer *principal.Issuer, ledger creatorReplayLedger) (lifecycleprincipal.CreatorDependencies, error) {
	if client == nil || issuer == nil || ledger == nil {
		return lifecycleprincipal.CreatorDependencies{}, errors.New("creator authority dependencies required")
	}
	checker := &creatorauthority.Checker{Client: client, Issuer: issuer}
	return lifecycleprincipal.CreatorDependencies{Epoch: checker.CheckEpoch, Replay: func(ctx context.Context, issuer, jti string, expiry time.Time) error {
		err := ledger.ClaimCreatorPrincipalReplay(ctx, issuer, jti, expiry)
		if errors.Is(err, store.ErrMemberAuthorityUnavailable) {
			return lifecycleprincipal.ErrDependencyUnavailable
		}
		return err
	}}, nil
}

type creatorRuntime struct {
	dependencies lifecycleprincipal.CreatorDependencies
	conn         *grpc.ClientConn
	cancel       context.CancelFunc
	done         chan struct{}
	once         sync.Once
	closeErr     error
}

func newCreatorRuntime(ctx, startupCtx context.Context, c creatorRuntimeConfig, issuer *principal.Issuer, ledger *store.SpaceStore) (*creatorRuntime, error) {
	if ctx == nil || startupCtx == nil || issuer == nil || ledger == nil || ledger.Pool == nil {
		return nil, errors.New("creator runtime requires issuer and durable replay store")
	}
	tlsConfig, err := creatorAuthTLS(c)
	if err != nil {
		return nil, err
	}
	conn, err := grpc.NewClient(grpcclient.DialTarget(c.address), grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	if err != nil {
		return nil, fmt.Errorf("creator Auth transport: %w", err)
	}
	conn.Connect()
	ready, cancel := context.WithTimeout(startupCtx, grpcclient.DialTimeoutFromEnv())
	err = grpcclient.WaitForReady(ready, conn)
	cancel()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("creator Auth readiness: %w", err)
	}
	dependencies, err := creatorDependencies(authv1.NewAuthServiceClient(conn), issuer, ledger)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	workerCtx, stop := context.WithCancel(ctx)
	runtime := &creatorRuntime{dependencies: dependencies, conn: conn, cancel: stop, done: make(chan struct{})}
	go func() {
		defer close(runtime.done)
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-workerCtx.Done():
				return
			case <-ticker.C:
				cleanup, cancel := context.WithTimeout(workerCtx, 2*time.Second)
				_, _ = ledger.CleanupCreatorPrincipalReplays(cleanup, 100)
				cancel()
			}
		}
	}()
	return runtime, nil
}

func creatorAuthTLS(c creatorRuntimeConfig) (*tls.Config, error) {
	if c.address == "" || c.serverName == "" {
		return nil, errors.New("creator Auth endpoint and server name required")
	}
	ca, err := os.ReadFile(c.caFile)
	if err != nil {
		return nil, fmt.Errorf("creator Auth CA: %w", err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(ca) {
		return nil, errors.New("creator Auth CA contains no certificates")
	}
	identity, err := tls.LoadX509KeyPair(c.certFile, c.keyFile)
	if err != nil {
		return nil, fmt.Errorf("creator Auth client identity: %w", err)
	}
	return &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: c.serverName, Certificates: []tls.Certificate{identity}}, nil
}

func (r *creatorRuntime) Close() error {
	if r == nil {
		return nil
	}
	r.once.Do(func() { r.cancel(); <-r.done; r.closeErr = r.conn.Close() })
	return r.closeErr
}

type creatorRuntimeCloser interface{ Close() error }

// Fatal logging skips defers. Cancel/join/close the new owned runtime first,
// while preserving the original failure even if cleanup itself reports an error.
func failCreatorRuntime(owned creatorRuntimeCloser, cause error, terminate func(error)) {
	if owned != nil {
		_ = owned.Close()
	}
	terminate(cause)
}

// This owner exists before any external failure callback. Construction and
// registration share one bounded critical section, so a callback cannot miss a
// returned runtime. Factory/worker/publication callbacks must never call this
// owner's Close synchronously (the factory holds mu; the worker would self-join).
type creatorRuntimeOwner struct {
	mu      sync.Mutex
	closed  bool
	runtime creatorRuntimeCloser
}

func (o *creatorRuntimeOwner) construct(ctx context.Context, factory func(context.Context) (*creatorRuntime, error)) (*creatorRuntime, error) {
	if o == nil || ctx == nil || factory == nil {
		return nil, errors.New("creator runtime owner unavailable")
	}
	o.mu.Lock()
	if o.closed || o.runtime != nil {
		o.mu.Unlock()
		return nil, errors.New("creator runtime owner closed or already registered")
	}
	startup, cancel := context.WithTimeout(ctx, 5*time.Second)
	runtime, err := factory(startup)
	expired := startup.Err()
	cancel()
	o.runtime = runtime
	if err == nil && expired != nil {
		err = expired
	}
	if err == nil && runtime == nil {
		err = errors.New("creator runtime constructor returned no runtime")
	}
	if err != nil {
		o.closed = true
		o.mu.Unlock()
		_ = runtime.Close()
		return nil, err
	}
	o.mu.Unlock()
	return runtime, nil
}

// Publication is local/nonblocking (dependency assignment, server registration,
// goroutine launch), never network work. A closed owner cannot publish a server.
func (o *creatorRuntimeOwner) publish(commit func()) error {
	if o == nil || commit == nil {
		return errors.New("creator runtime publication unavailable")
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed {
		return errors.New("creator runtime owner closed")
	}
	commit()
	return nil
}

func (o *creatorRuntimeOwner) Close() error {
	if o == nil {
		return nil
	}
	o.mu.Lock()
	o.closed = true
	runtime := o.runtime
	o.mu.Unlock()
	// Do not short-circuit on closed: every concurrent caller must wait for the
	// actual once-guarded cancel/join/connection cleanup to finish before Fatal.
	if runtime == nil {
		return nil
	}
	return runtime.Close()
}
