package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"google.golang.org/grpc"
	authv1 "voice.app/voice/auth/v1"
	chatv1 "voice.app/voice/chat/v1"
	filev1 "voice.app/voice/file/v1"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/chat/internal/catalogassets"
	"voice/backend/chat/internal/store"
	catalogactorv1 "voice/backend/pkg/pb/voice/catalogactor/v1"
	"voice/backend/pkg/principal"
)

type catalogRuntimeDeps struct {
	Issuer             *principal.Issuer
	Chats              *store.DMStore
	Spaces             *store.SpaceMembersStore
	Roles              rolev1.RoleServiceClient
	NATSURL            string
	HasAccountConsumer bool
	Logger             *slog.Logger
}
type chatCatalogRuntime struct {
	Repository *catalogassets.PostgresRepository
	close      func()
}

func startCatalogAssets(ctx context.Context, cfg catalogAssetConfig, enabled bool, deps catalogRuntimeDeps) (*chatCatalogRuntime, error) {
	if !enabled {
		return nil, nil
	}
	if deps.Issuer == nil || deps.Chats == nil || deps.Chats.Pool == nil || deps.NATSURL == "" || !deps.HasAccountConsumer {
		return nil, errors.New("catalog requires persistence, issuer and account deletion consumer")
	}
	if err := cfg.Listener.Validate(); err != nil {
		return nil, err
	}
	// Validate the provisioned durable before exposing the new listener. Chat has
	// no permission to create/alter that durable or skip its deletion effects.
	nc, err := nats.Connect(deps.NATSURL, nats.Timeout(3*time.Second))
	if err != nil {
		return nil, errors.New("catalog account event transport unavailable")
	}
	js, err := nc.JetStream()
	if err == nil {
		err = validateAccountDeletedDurable(js, accountDeletedDurableName(""))
	}
	nc.Close()
	if err != nil {
		return nil, errors.New("catalog account deletion durable unavailable")
	}
	keys, err := readCatalogActorKeys(cfg.ActorKeysFile)
	if err != nil {
		return nil, err
	}
	var closers []func() error
	failed := true
	defer func() {
		if failed {
			for i := len(closers) - 1; i >= 0; i-- {
				_ = closers[i]()
			}
		}
	}()
	dial := func(owner catalogOwnerConfig) (*grpc.ClientConn, error) {
		conn, err := dialSpacePurgeOwner(owner.Address, owner.CA, owner.Name, owner.Cert, owner.Key)
		if err != nil {
			return nil, errors.New("catalog owner mTLS configuration invalid")
		}
		closers = append(closers, conn.Close)
		call, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		if err := waitForRequiredGRPCReady(call, conn); err != nil {
			return nil, errors.New("catalog owner transport unavailable")
		}
		return conn, nil
	}
	authConn, err := dial(cfg.Auth)
	if err != nil {
		return nil, err
	}
	userConn, err := dial(cfg.User)
	if err != nil {
		return nil, err
	}
	fileConn, err := dial(cfg.File)
	if err != nil {
		return nil, err
	}
	observer := &catalogassets.GRPCActorObserver{Auth: authv1.NewAuthServiceClient(authConn), User: catalogactorv1.NewCatalogActorAuthorityServiceClient(userConn), Issuer: deps.Issuer}
	repo := &catalogassets.PostgresRepository{Pool: deps.Chats.Pool, Keys: keys, Observer: observer}
	check, cancel := context.WithTimeout(ctx, 5*time.Second)
	err = repo.Preflight(check)
	cancel()
	if err != nil {
		return nil, errors.New("catalog durable actor schema or historical keys unavailable")
	}
	ingress, err := catalogassets.NewListenerRuntime(ctx, cfg.Listener, observer.CheckSessionEpoch)
	if err != nil {
		return nil, err
	}
	closers = append(closers, ingress.Close)
	listener, err := net.Listen("tcp", cfg.Listener.Listen)
	if err != nil {
		return nil, errors.New("catalog listener unavailable")
	}
	files := &catalogassets.GRPCFileClient{Client: filev1.NewCatalogAssetServiceClient(fileConn), Issuer: deps.Issuer}
	service := &catalogassets.Service{Store: repo, Files: files, Authority: &catalogassets.NativeDestinationAuthority{Chats: deps.Chats, Spaces: deps.Spaces, Roles: deps.Roles}, SessionEpoch: observer.CheckSessionEpoch}
	// Resolver intentionally remains nil: no live provider or fake product seed
	// is selected by enabling this protected lifecycle/transport boundary.
	server := grpc.NewServer(ingress.ServerOptions()...)
	chatv1.RegisterCatalogAssetServiceServer(server, service)
	workerCtx, stop := context.WithCancel(ctx)
	done := make(chan struct{})
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	go func() {
		defer close(done)
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		var cursor uuid.UUID
		for {
			call, cancel := context.WithTimeout(workerCtx, 15*time.Second)
			next, err := (&catalogassets.Reconciler{Store: repo, Profiles: observer, Files: files}).Step(call, cursor, 100)
			cancel()
			cursor = next
			if err != nil && workerCtx.Err() == nil {
				logger.Warn("catalog reconciliation incomplete")
			}
			select {
			case <-workerCtx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	go func() {
		if err := server.Serve(listener); err != nil && workerCtx.Err() == nil {
			logger.Error("catalog listener stopped")
			stop()
		}
	}()
	failed = false
	return &chatCatalogRuntime{Repository: repo, close: func() {
		stop()
		server.Stop()
		<-done
		for i := len(closers) - 1; i >= 0; i-- {
			_ = closers[i]()
		}
	}}, nil
}
