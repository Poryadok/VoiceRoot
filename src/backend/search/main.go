package main

import (
	"context"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	grpcsvc "voice/backend/search/internal/grpcsvc"
	"voice/backend/search/internal/deps"
	"voice/backend/search/internal/indexer"
	"voice/backend/search/internal/store"
	"voice/backend/pkg/grpcclient"
	"voice/backend/pkg/grpcmw"
	"voice/backend/pkg/httpserver"
	voiceprom "voice/backend/pkg/promhttp"

	chatv1 "voice.app/voice/chat/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	searchv1 "voice.app/voice/search/v1"
	socialv1 "voice.app/voice/social/v1"
	spacev1 "voice.app/voice/space/v1"
	userv1 "voice.app/voice/user/v1"
)

const serviceName = "search"

func main() {
	logger := httpserver.NewLogger(serviceName)
	metricsReg := prometheus.NewRegistry()
	httpAddr := ":8080"
	if v := os.Getenv("LISTEN_ADDR"); v != "" {
		httpAddr = v
	}
	grpcListen := ":9090"
	if v := strings.TrimSpace(os.Getenv("SEARCH_GRPC_LISTEN")); v != "" {
		grpcListen = v
	}

	dbURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	var grpcSrv *grpc.Server
	rootCtx, rootCancel := context.WithCancel(context.Background())
	defer rootCancel()

	if dbURL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		pool, err := pgxpool.New(ctx, dbURL)
		cancel()
		if err != nil {
			log.Fatalf("postgres: %v", err)
		}
		defer pool.Close()

		msgStore := store.NewMessageSearchStore(pool)
		profileSpaceStore := store.NewProfileSpaceSearchStore(pool)

		svc := &grpcsvc.SearchGRPC{
			Messages: &grpcsvc.MessageStoreAdapter{MessageSearchStore: msgStore},
			Profiles: &grpcsvc.ProfileStoreAdapter{ProfileSpaceSearchStore: profileSpaceStore},
			Spaces:   &grpcsvc.SpaceStoreAdapter{ProfileSpaceSearchStore: profileSpaceStore},
		}

		if conn, err := dialOptional(os.Getenv("MESSAGING_GRPC_ADDR")); err == nil && conn != nil {
			defer func() { _ = conn.Close() }()
			messagingFetcher := &deps.MessagingFetcher{Client: messagingv1.NewMessagingServiceClient(conn)}
			svc.Reindex = &grpcsvc.ChatReindexService{
				Messages: messagingFetcher,
				Store:    msgStore,
			}
		}

		if natsURL := strings.TrimSpace(os.Getenv("NATS_URL")); natsURL != "" {
			instanceID := strings.TrimSpace(os.Getenv("SEARCH_INSTANCE_ID"))
			var messaging indexer.MessagingClient
			if conn, err := dialOptional(os.Getenv("MESSAGING_GRPC_ADDR")); err == nil && conn != nil {
				defer func() { _ = conn.Close() }()
				messaging = &deps.MessagingFetcher{Client: messagingv1.NewMessagingServiceClient(conn)}
			}
			msgIdx := &indexer.MessageIndexer{Store: msgStore, Messaging: messaging}
			go func() {
				if err := indexer.RunMessageEventsConsumer(rootCtx, natsURL, instanceID, msgIdx, logger); err != nil && rootCtx.Err() == nil {
					logger.Warn("message events consumer stopped", slog.Any("error", err))
				}
			}()

			var profileHydrator indexer.ProfileHydrator
			if conn, err := dialOptional(os.Getenv("USER_GRPC_ADDR")); err == nil && conn != nil {
				defer func() { _ = conn.Close() }()
				profileHydrator = &deps.ProfileHydrator{Client: userv1.NewUserServiceClient(conn)}
			}
			profileIdx := &indexer.ProfileIndexer{Store: profileSpaceStore, Profiles: profileHydrator}
			go func() {
				if err := indexer.RunUserEventsConsumer(rootCtx, natsURL, instanceID, profileIdx, logger); err != nil && rootCtx.Err() == nil {
					logger.Warn("user events consumer stopped", slog.Any("error", err))
				}
			}()

			var chatHydrator indexer.ChatHydrator
			var spaceHydrator indexer.SpaceHydrator
			if conn, err := dialOptional(os.Getenv("CHAT_GRPC_ADDR")); err == nil && conn != nil {
				defer func() { _ = conn.Close() }()
				chatHydrator = &deps.ChatHydrator{Client: chatv1.NewChatServiceClient(conn)}
			}
			if conn, err := dialOptional(os.Getenv("SPACE_GRPC_ADDR")); err == nil && conn != nil {
				defer func() { _ = conn.Close() }()
				spaceHydrator = &deps.SpaceHydrator{Client: spacev1.NewSpaceServiceClient(conn)}
			}
			chatSpaceIdx := &indexer.ChatSpaceIndexer{
				Chats:    profileSpaceStore,
				Spaces:   profileSpaceStore,
				ChatAPI:  chatHydrator,
				SpaceAPI: spaceHydrator,
			}
			go func() {
				if err := indexer.RunChatEventsConsumer(rootCtx, natsURL, instanceID, chatSpaceIdx, logger); err != nil && rootCtx.Err() == nil {
					logger.Warn("chat events consumer stopped", slog.Any("error", err))
				}
			}()
		}

		var chatClient chatv1.ChatServiceClient
		if conn, err := dialOptional(os.Getenv("CHAT_GRPC_ADDR")); err == nil && conn != nil {
			defer func() { _ = conn.Close() }()
			chatClient = chatv1.NewChatServiceClient(conn)
			svc.Roles = &deps.ChatReadAccess{Client: chatClient}
		}
		if conn, err := dialOptional(os.Getenv("SOCIAL_GRPC_ADDR")); err == nil && conn != nil {
			defer func() { _ = conn.Close() }()
			svc.Blocks = &deps.SocialBlocks{Client: socialv1.NewSocialServiceClient(conn)}
		}
		chatAccess := &grpcsvc.ProjectionChatAccess{Store: profileSpaceStore}
		if chatClient != nil {
			chatAccess.Accessible = (&deps.ChatMembership{Client: chatClient}).AccessibleChatIDs
		}
		svc.Chats = chatAccess

		lis, err := net.Listen("tcp", grpcListen)
		if err != nil {
			log.Fatalf("grpc listen: %v", err)
		}
		grpcSrv = grpc.NewServer(grpcmw.ServerOptions(logger, grpcmw.WithRegistry(metricsReg))...)
		searchv1.RegisterSearchServiceServer(grpcSrv, svc)
		go func() {
			if err := grpcSrv.Serve(lis); err != nil {
				log.Fatalf("grpc serve: %v", err)
			}
		}()
	}

	server := &http.Server{
		Addr:              httpAddr,
		Handler:           httpserver.Wrap(voiceprom.MountMetricsOnHealth(healthHandler(serviceName), metricsReg), logger),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	errCh := make(chan error, 1)
	logger.Info("listening", slog.String("addr", httpAddr))
	go func() {
		errCh <- server.ListenAndServe()
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	select {
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	case <-stop:
		rootCancel()
		if grpcSrv != nil {
			grpcSrv.GracefulStop()
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			log.Fatal(err)
		}
	}
}

func dialOptional(addr string) (*grpc.ClientConn, error) {
	addr = grpcclient.DialTarget(strings.TrimSpace(addr))
	if addr == "" {
		return nil, nil
	}
	return grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
}
