package main

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"net"
	"sync"
	"time"
	authv1 "voice.app/voice/auth/v1"
	botv1 "voice.app/voice/bot/v1"
	"voice/backend/bot/internal/grpcsvc"
	"voice/backend/bot/internal/installprincipal"
	"voice/backend/bot/internal/principalruntime"
	"voice/backend/bot/internal/spacemembership"
	"voice/backend/bot/internal/store"
	"voice/backend/pkg/grpcclient"
)

type installationRuntime struct {
	client   *spacemembership.Client
	auth     *grpc.ClientConn
	admin    *installprincipal.Runtime
	server   *grpc.Server
	listener net.Listener
	once     sync.Once
}

func (r *installationRuntime) Close() {
	if r == nil {
		return
	}
	r.once.Do(func() {
		if r.server != nil {
			r.server.Stop()
		}
		if r.listener != nil {
			_ = r.listener.Close()
		}
		if r.admin != nil {
			_ = r.admin.Close()
		}
		if r.client != nil {
			_ = r.client.Close()
		}
		if r.auth != nil {
			_ = r.auth.Close()
		}
	})
}
func newInstallationRuntime(ctx context.Context, c spacemembership.Config, adminConfig installprincipal.Config, authEndpoint spacemembership.Endpoint, base *principalruntime.Runtime, pool *pgxpool.Pool, service *grpcsvc.BotGRPC) (result *installationRuntime, err error) {
	if base == nil || base.Issuer() == nil || pool == nil || service == nil {
		return nil, errors.New("installation runtime dependencies required")
	}
	r := &installationRuntime{}
	defer func() {
		if err != nil {
			r.Close()
		}
	}()
	r.listener, err = net.Listen("tcp", adminConfig.Listen)
	if err != nil {
		return nil, err
	}
	var valid bool
	err = pool.QueryRow(ctx, `SELECT count(*)=1 AND coalesce(bool_and(m.version=8 AND NOT m.dirty),false) AND EXISTS(SELECT 1 FROM bot_space_membership_source s WHERE s.singleton AND s.source_generation=$1 AND s.issued_pairs=(SELECT count(*) FROM bot_space_membership_heads) AND s.issued_operations=(SELECT count(*) FROM bot_space_membership_operations) AND s.issued_calls=(SELECT count(*) FROM bot_space_membership_calls)) FROM schema_migrations m`, c.SourceGeneration).Scan(&valid)
	if err != nil {
		return nil, err
	}
	if !valid {
		return nil, errors.New("installation journal source does not match retained deployment identity")
	}
	tlsConfig, err := authEndpoint.TLS()
	if err != nil {
		return nil, err
	}
	r.auth, err = grpc.NewClient(grpcclient.DialTarget(authEndpoint.Address), grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	if err != nil {
		return nil, err
	}
	r.auth.Connect()
	if err = grpcclient.WaitForReady(ctx, r.auth); err != nil {
		return nil, err
	}
	checker := &installprincipal.SessionChecker{Client: authv1.NewAuthServiceClient(r.auth), Issuer: base.Issuer()}
	r.admin, err = installprincipal.New(ctx, adminConfig, checker.Check)
	if err != nil {
		return nil, err
	}
	r.client, err = spacemembership.New(ctx, c, base.Issuer())
	if err != nil {
		return nil, err
	}
	r.client.Store = &store.BotStore{Pool: pool}
	service.MembershipCoordinator = &grpcsvc.MembershipProducer{Journal: r.client.Store, Transport: r.client, Effects: r.client, Source: c.SourceGeneration}
	r.server = grpc.NewServer(r.admin.ServerOptions()...)
	botv1.RegisterBotServiceServer(r.server, service)
	return r, nil
}

// Serialize construction with failure cleanup so a concurrent publisher failure
// cannot miss a partially owned runtime or publish it after shutdown.
type installationOwner struct {
	mu      sync.Mutex
	runtime *installationRuntime
	closed  bool
}

func (o *installationOwner) construct(factory func(context.Context) (*installationRuntime, error)) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed {
		return errors.New("installation runtime closed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	r, err := factory(ctx)
	if err != nil {
		if r != nil {
			r.Close()
		}
		return err
	}
	if ctx.Err() != nil {
		r.Close()
		return ctx.Err()
	}
	o.runtime = r
	return nil
}
func (o *installationOwner) start(failures chan<- error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed || o.runtime == nil {
		return
	}
	r := o.runtime
	go func() {
		if err := r.server.Serve(r.listener); err != nil {
			select {
			case failures <- err:
			default:
			}
		}
	}()
}
func (o *installationOwner) Close() {
	o.mu.Lock()
	o.closed = true
	r := o.runtime
	o.runtime = nil
	o.mu.Unlock()
	if r != nil {
		r.Close()
	}
}
