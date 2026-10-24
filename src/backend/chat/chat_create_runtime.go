package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"os"
	"strings"

	"google.golang.org/grpc"
	chatv1 "voice.app/voice/chat/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/chat/internal/chatcreation"
	"voice/backend/chat/internal/chatevents"
	"voice/backend/chat/internal/grpcsvc"
	"voice/backend/chat/internal/store"
	"voice/backend/pkg/principal"
)

// startChatCreation composes protected ingress and the autonomous durable worker.
// Partial configuration never creates an alternate raw ingress.
func startChatCreation(ctx context.Context, cfg chatcreation.CreateConfig, enabled bool, issuer *principal.Issuer, dm *store.DMStore, publisher *chatevents.JetStreamPublisher, logger *slog.Logger) (func(), error) {
	if !enabled {
		return func() {}, nil
	}
	if issuer == nil || dm == nil || dm.Pool == nil || publisher == nil {
		return nil, errors.New("chat creation requires persistence, signing issuer and JetStream publisher")
	}
	if logger == nil {
		logger = slog.Default()
	}
	if _, err := dm.Pool.Exec(ctx, "SELECT operation_id,state,binding_bytes FROM chat_space_composite_operations LIMIT 0; SELECT event_id,eligibility,payload FROM chat_space_composite_outbox LIMIT 0"); err != nil {
		return nil, errors.New("chat creation durable schema unavailable")
	}
	floor, closeFloor, err := newChatCreateAuthCheckerFromEnv(issuer)
	if err != nil {
		return nil, err
	}
	if floor == nil {
		return nil, errors.New("chat creation requires authoritative Auth floor transport")
	}
	failed := true
	defer func() {
		if failed {
			_ = closeFloor()
		}
	}()
	conn, err := dialChatCreationOwnerFromEnv("CHAT_CREATE_SPACE_")
	if err != nil {
		return nil, err
	}
	defer func() {
		if failed {
			_ = conn.Close()
		}
	}()
	auditConn, err := dialChatCreationOwnerFromEnv("CHAT_CREATE_AUDIT_")
	if err != nil {
		return nil, err
	}
	defer func() {
		if failed {
			_ = auditConn.Close()
		}
	}()
	runtime, err := chatcreation.NewCreateRuntime(ctx, cfg, floor.CheckSessionEpoch)
	if err != nil {
		return nil, err
	}
	defer func() {
		if failed {
			_ = runtime.Close()
		}
	}()
	listener, err := net.Listen("tcp", cfg.Listen)
	if err != nil {
		return nil, err
	}
	client := spacev1.NewSpaceServiceClient(conn)
	coordinator := newChatCreationCoordinator(dm, client, spacev1.NewSpaceServiceClient(auditConn), issuer, publisher, logger)
	server := grpc.NewServer(runtime.ServerOptions()...)
	chatv1.RegisterChatServiceServer(server, &grpcsvc.ChatGRPC{DM: dm, Composite: coordinator, ChatEvents: publisher, Logger: logger})
	workerCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { defer close(done); coordinator.Run(workerCtx) }()
	go func() {
		if err := server.Serve(listener); err != nil && workerCtx.Err() == nil {
			logger.Error("Chat creation listener stopped", slog.String("error", err.Error()))
			cancel()
		}
	}()
	failed = false
	return func() {
		cancel()
		server.Stop()
		<-done
		_ = runtime.Close()
		_ = conn.Close()
		_ = auditConn.Close()
		_ = closeFloor()
	}, nil
}

func newChatCreationCoordinator(dm *store.DMStore, composite, audit spacev1.SpaceServiceClient, issuer *principal.Issuer, publisher *chatevents.JetStreamPublisher, logger *slog.Logger) *grpcsvc.SpaceCompositeCoordinator {
	remote := &grpcsvc.SignedSpaceCompositeRemote{Client: composite, Issuer: issuer, Chats: dm}
	return &grpcsvc.SpaceCompositeCoordinator{Store: dm, Remote: remote, Public: remote, Effects: &grpcsvc.SignedSpaceCompositeEffects{Audit: audit, Issuer: issuer, Created: publisher}, Logger: logger}
}

func dialChatCreationOwnerFromEnv(prefix string) (*grpc.ClientConn, error) {
	values := make([]string, 5)
	for i, name := range []string{"GRPC_ADDR", "TLS_CA_FILE", "TLS_SERVER_NAME", "CLIENT_CERT_FILE", "CLIENT_KEY_FILE"} {
		values[i] = strings.TrimSpace(os.Getenv(prefix + name))
		if values[i] == "" {
			return nil, errors.New("complete " + prefix + " mTLS transport required")
		}
	}
	return dialSpacePurgeOwner(values[0], values[1], values[2], values[3], values[4])
}
