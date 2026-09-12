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

	"voice/backend/pkg/analyticsevents"
	"voice/backend/pkg/grpcclient"
	"voice/backend/pkg/grpcmw"
	"voice/backend/pkg/httpserver"
	voiceprom "voice/backend/pkg/promhttp"
	"voice/backend/pkg/runtimeconfig"
	"voice/backend/search/internal/deps"
	grpcsvc "voice/backend/search/internal/grpcsvc"
	"voice/backend/search/internal/indexer"
	"voice/backend/search/internal/principalruntime"
	"voice/backend/search/internal/store"

	chatv1 "voice.app/voice/chat/v1"
	messagingv1 "voice.app/voice/messaging/v1"
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
	principalConfig, principalEnabled, err := principalruntime.LoadFromEnv()
	if err != nil {
		log.Fatalf("principal config: %v", err)
	}
	if principalEnabled && dbURL == "" {
		log.Fatal("principal listener requires DATABASE_URL")
	}
	manifestClient, manifestConn, searchJWKS, err := loadSignedChatManifestClientFromEnv()
	if err != nil {
		log.Fatalf("Chat manifest principal: %v", err)
	}
	if manifestClient != nil && dbURL == "" {
		log.Fatal("Chat manifest principal requires DATABASE_URL")
	}
	if manifestConn != nil {
		defer func() { _ = manifestConn.Close() }()
	}
	var grpcSrv, principalSrv *grpc.Server
	rootCtx, rootCancel := context.WithCancel(context.Background())
	defer rootCancel()

	if dbURL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), runtimeconfig.PostgresConnectTimeoutFromEnv())
		pool, err := pgxpool.New(ctx, dbURL)
		cancel()
		if err != nil {
			log.Fatalf("postgres: %v", err)
		}
		defer pool.Close()
		go func() {
			ticker := time.NewTicker(time.Hour)
			defer ticker.Stop()
			for {
				if err := store.PruneExpiredLifecycleEvidence(rootCtx, pool); err != nil && rootCtx.Err() == nil {
					logger.Warn("lifecycle retention failed", slog.Any("error", err))
				}
				select {
				case <-rootCtx.Done():
					return
				case <-ticker.C:
				}
			}
		}()

		msgStore := store.NewMessageSearchStore(pool)
		profileSpaceStore := store.NewProfileSpaceSearchStore(pool)

		svc := &grpcsvc.SearchGRPC{
			Messages:     &grpcsvc.MessageStoreAdapter{MessageSearchStore: msgStore},
			Profiles:     &grpcsvc.ProfileStoreAdapter{ProfileSpaceSearchStore: profileSpaceStore},
			Spaces:       &grpcsvc.SpaceStoreAdapter{ProfileSpaceSearchStore: profileSpaceStore},
			ChatManifest: manifestClient,
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
			if pub, err := analyticsevents.NewJetStreamPublisher(natsURL); err == nil {
				_ = pub.EnsureStream()
				svc.Analytics = pub
				defer pub.Close()
			} else {
				logger.Warn("analytics publisher disabled", slog.Any("error", err))
			}
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
		var socialClient socialv1.SocialServiceClient
		if conn, err := dialOptional(os.Getenv("SOCIAL_GRPC_ADDR")); err == nil && conn != nil {
			defer func() { _ = conn.Close() }()
			socialClient = socialv1.NewSocialServiceClient(conn)
			svc.Blocks = &deps.SocialBlocks{Client: socialClient}
			svc.Social = &deps.SocialGraph{Client: socialClient}
		}
		if conn, err := dialOptional(os.Getenv("USER_GRPC_ADDR")); err == nil && conn != nil {
			defer func() { _ = conn.Close() }()
			svc.Discoverability = &deps.UserPrivacy{Client: userv1.NewUserServiceClient(conn)}
		}
		if conn, err := dialOptional(os.Getenv("SPACE_GRPC_ADDR")); err == nil && conn != nil {
			defer func() { _ = conn.Close() }()
			svc.SpaceMembers = &deps.SpaceCoMembership{Client: spacev1.NewSpaceServiceClient(conn)}
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
		var principalRuntime *principalruntime.Runtime
		if principalEnabled {
			principalRuntime, err = principalruntime.New(rootCtx, principalConfig)
			if err != nil {
				log.Fatalf("principal runtime: %v", err)
			}
			defer func() { _ = principalRuntime.Close() }()
		}
		grpcSrv, principalSrv = newSearchGRPCServers(grpcmw.ServerOptions(logger, grpcmw.WithRegistry(metricsReg)), svc, principalRuntime)
		if principalSrv != nil {
			protectedListener, err := net.Listen("tcp", principalConfig.ListenAddr)
			if err != nil {
				log.Fatalf("principal grpc listen: %v", err)
			}
			go func() {
				logger.Info("lifecycle gRPC TLS listening", slog.String("addr", principalConfig.ListenAddr))
				if err := principalSrv.Serve(protectedListener); err != nil {
					log.Fatalf("principal grpc serve: %v", err)
				}
			}()
		}
		go func() {
			if err := grpcSrv.Serve(lis); err != nil {
				log.Fatalf("grpc serve: %v", err)
			}
		}()
	}

	server := &http.Server{
		Addr:    httpAddr,
		Handler: httpserver.Wrap(voiceprom.MountMetricsOnHealth(searchHTTPHandler(serviceName, searchJWKS), metricsReg), logger),
	}
	httpserver.ApplyHTTPServerTimeouts(server)
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
		ctx, cancel := context.WithTimeout(context.Background(), runtimeconfig.ShutdownTimeoutFromEnv())
		defer cancel()
		shutdownSearchServers(ctx, grpcSrv, principalSrv)
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
