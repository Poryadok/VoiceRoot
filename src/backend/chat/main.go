package main

import (
	"context"
	"errors"
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
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"

	"voice/backend/chat/internal/chatevents"
	grpcsvc "voice/backend/chat/internal/grpcsvc"
	"voice/backend/chat/internal/store"
	"voice/backend/pkg/grpcclient"
	"voice/backend/pkg/grpcmw"
	"voice/backend/pkg/httpserver"
	voiceprom "voice/backend/pkg/promhttp"

	chatv1 "voice.app/voice/chat/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	rolev1 "voice.app/voice/role/v1"
	userv1 "voice.app/voice/user/v1"
)

const serviceName = "chat"

func main() {
	logger := httpserver.NewLogger(serviceName)
	metricsReg := prometheus.NewRegistry()
	httpAddr := ":8080"
	if v := os.Getenv("LISTEN_ADDR"); v != "" {
		httpAddr = v
	}
	grpcListen := ":9090"
	if v := strings.TrimSpace(os.Getenv("CHAT_GRPC_LISTEN")); v != "" {
		grpcListen = v
	}

	dbURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	var grpcSrv *grpc.Server
	runCtx, runCancel := context.WithCancel(context.Background())
	defer runCancel()
	if dbURL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		pool, err := pgxpool.New(ctx, dbURL)
		cancel()
		if err != nil {
			log.Fatalf("postgres: %v", err)
		}
		defer pool.Close()

		var blocks grpcsvc.AccountBlockChecker
		var friends grpcsvc.ProfileFriendChecker
		if socialAddr := strings.TrimSpace(os.Getenv("SOCIAL_GRPC_ADDR")); socialAddr != "" {
			sconn, err := grpc.NewClient(grpcclient.DialTarget(socialAddr), grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatalf("social grpc: %v", err)
			}
			sconn.Connect()
			waitCtx, waitCancel := context.WithTimeout(context.Background(), grpcclient.DialTimeoutFromEnv())
			for {
				st := sconn.GetState()
				if st == connectivity.Ready {
					break
				}
				if st == connectivity.Shutdown {
					waitCancel()
					_ = sconn.Close()
					log.Fatalf("social grpc: unexpected shutdown")
				}
				if !sconn.WaitForStateChange(waitCtx, st) {
					waitCancel()
					_ = sconn.Close()
					log.Fatalf("social grpc dial: %v", context.Cause(waitCtx))
				}
			}
			waitCancel()
			defer func() { _ = sconn.Close() }()
			blocks = grpcsvc.NewSocialGRPCBlocks(sconn)
			friends = grpcsvc.NewSocialGRPCFriends(sconn)
		}

		var profiles grpcsvc.UserProfileLookup
		var privacy grpcsvc.PrivacyChecker
		var spaceCoMembership grpcsvc.SpaceCoMembershipChecker
		if userAddr := strings.TrimSpace(os.Getenv("USER_GRPC_ADDR")); userAddr != "" {
			uconn, err := grpc.NewClient(grpcclient.DialTarget(userAddr), grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatalf("user grpc: %v", err)
			}
			uconn.Connect()
			waitCtx, waitCancel := context.WithTimeout(context.Background(), grpcclient.DialTimeoutFromEnv())
			for {
				st := uconn.GetState()
				if st == connectivity.Ready {
					break
				}
				if st == connectivity.Shutdown {
					waitCancel()
					_ = uconn.Close()
					log.Fatalf("user grpc: unexpected shutdown")
				}
				if !uconn.WaitForStateChange(waitCtx, st) {
					waitCancel()
					_ = uconn.Close()
					log.Fatalf("user grpc dial: %v", context.Cause(waitCtx))
				}
			}
			waitCancel()
			defer func() { _ = uconn.Close() }()
			userClient := userv1.NewUserServiceClient(uconn)
			profiles = &grpcsvc.UserGRPCProfiles{Client: userClient}
			privacy = &grpcsvc.UserGRPCPrivacy{Client: userClient}
		}
		if spaceAddr := strings.TrimSpace(os.Getenv("SPACE_GRPC_ADDR")); spaceAddr != "" {
			spconn, err := grpc.NewClient(grpcclient.DialTarget(spaceAddr), grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatalf("space grpc: %v", err)
			}
			defer func() { _ = spconn.Close() }()
			spaceCoMembership = grpcsvc.NewSpaceGRPCCoMembership(spconn)
		}

		var roleClient rolev1.RoleServiceClient
		if roleAddr := strings.TrimSpace(os.Getenv("ROLE_GRPC_ADDR")); roleAddr != "" {
			rconn, err := grpc.NewClient(grpcclient.DialTarget(roleAddr), grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatalf("role grpc: %v", err)
			}
			defer func() { _ = rconn.Close() }()
			roleClient = rolev1.NewRoleServiceClient(rconn)
		}

		var listEnrich grpcsvc.ListChatsEnrichment
		var e2ePreKeyGate grpcsvc.E2EPreKeyGate
		if msgAddr := strings.TrimSpace(os.Getenv("MESSAGING_GRPC_ADDR")); msgAddr != "" {
			mconn, err := grpc.NewClient(grpcclient.DialTarget(msgAddr), grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatalf("messaging grpc: %v", err)
			}
			defer func() { _ = mconn.Close() }()
			msgClient := messagingv1.NewMessagingServiceClient(mconn)
			listEnrich = grpcsvc.NewMessagingListEnricher(msgClient)
			e2ePreKeyGate = grpcsvc.NewMessagingE2EPreKeyGate(msgClient)
		}

		dmStore := &store.DMStore{Pool: pool}
		var spaceMembers *store.SpaceMembersStore
		if spaceDB := strings.TrimSpace(os.Getenv("SPACE_DATABASE_URL")); spaceDB != "" {
			sctx, scancel := context.WithTimeout(context.Background(), 15*time.Second)
			spacePool, serr := pgxpool.New(sctx, spaceDB)
			scancel()
			if serr != nil {
				log.Fatalf("space postgres: %v", serr)
			}
			defer spacePool.Close()
			spaceMembers = &store.SpaceMembersStore{Pool: spacePool}
		}
		natsURL := strings.TrimSpace(os.Getenv("NATS_URL"))
		var chatEvents chatevents.Publisher
		if natsURL != "" {
			jsPub, err := chatevents.NewJetStreamPublisher(natsURL)
			if err != nil {
				log.Fatalf("nats jetstream publisher: %v", err)
			}
			defer func() { _ = jsPub.Close() }()
			jsPub.Logger = logger
			chatEvents = jsPub
			go func() {
				if err := runMessageActivityConsumer(runCtx, natsURL, os.Getenv("HOSTNAME"), dmStore, logger); err != nil && !errors.Is(err, context.Canceled) {
					logger.Error("message activity consumer stopped", slog.String("error", err.Error()))
				}
			}()
		}

		lis, err := net.Listen("tcp", grpcListen)
		if err != nil {
			log.Fatalf("grpc listen: %v", err)
		}
		grpcSrv = grpc.NewServer(grpcmw.ServerOptions(logger, grpcmw.WithRegistry(metricsReg))...)
		chatv1.RegisterChatServiceServer(grpcSrv, &grpcsvc.ChatGRPC{
			DM:            dmStore,
			Profiles:      profiles,
			Blocks:        blocks,
			Privacy:           privacy,
			Friends:           friends,
			SpaceCoMembership: spaceCoMembership,
			ListEnrich:    listEnrich,
			E2EPreKeyGate: e2ePreKeyGate,
			ChatEvents:    chatEvents,
			Roles:         roleClient,
			SpaceMembers:  spaceMembers,
			Logger:        logger,
		})
		go func() {
			logger.Info("gRPC listening", slog.String("addr", grpcListen))
			if err := grpcSrv.Serve(lis); err != nil {
				log.Fatalf("grpc serve: %v", err)
			}
		}()
	} else {
		logger.Warn("DATABASE_URL not set; gRPC disabled (health only)")
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
		runCancel()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if grpcSrv != nil {
			grpcSrv.GracefulStop()
		}
		if err := server.Shutdown(ctx); err != nil {
			log.Fatal(err)
		}
	}
}
