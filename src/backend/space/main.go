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

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"

	"voice/backend/pkg/grpcclient"
	"voice/backend/pkg/grpcmw"
	"voice/backend/pkg/httpserver"
	voiceprom "voice/backend/pkg/promhttp"
	"voice/backend/pkg/runtimeconfig"
	grpcsvc "voice/backend/space/internal/grpcsvc"
	"voice/backend/space/internal/s2s"
	"voice/backend/space/internal/spaceevents"
	"voice/backend/space/internal/store"
	"voice/backend/space/internal/subscriptionconsume"

	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	userv1 "voice.app/voice/user/v1"
)

const (
	serviceName                         = "space"
	spaceMutationLockPoolMaxConns int32 = 8
)

func main() {
	logger := httpserver.NewLogger(serviceName)
	metricsReg := prometheus.NewRegistry()
	runCtx, runCancel := context.WithCancel(context.Background())
	defer runCancel()
	httpAddr := ":8080"
	if v := os.Getenv("LISTEN_ADDR"); v != "" {
		httpAddr = v
	}
	grpcListen := ":9090"
	if v := strings.TrimSpace(os.Getenv("SPACE_GRPC_LISTEN")); v != "" {
		grpcListen = v
	}

	dbURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	var grpcSrv *grpc.Server
	if dbURL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), runtimeconfig.PostgresConnectTimeoutFromEnv())
		pool, err := pgxpool.New(ctx, dbURL)
		if err != nil {
			cancel()
			log.Fatalf("postgres: %v", err)
		}
		defer pool.Close()

		lockPoolConfig, err := pgxpool.ParseConfig(dbURL)
		if err != nil {
			cancel()
			log.Fatalf("postgres mutation lock config: %v", err)
		}
		// Advisory leases pin sessions, so they use an independently bounded
		// pool and cannot exhaust connections needed by SpaceStore queries. Its
		// DSN must use direct PostgreSQL or session pooling, not transaction mode.
		lockPoolConfig.MinConns = 0
		lockPoolConfig.MaxConns = spaceMutationLockPoolMaxConns
		mutationLockPool, err := pgxpool.NewWithConfig(ctx, lockPoolConfig)
		cancel()
		if err != nil {
			log.Fatalf("postgres mutation lock pool: %v", err)
		}
		defer mutationLockPool.Close()

		spaceStore := &store.SpaceStore{Pool: pool}
		natsURL := strings.TrimSpace(os.Getenv("NATS_URL"))
		var spaceEvents spaceevents.Publisher
		if natsURL != "" {
			jsPub, err := spaceevents.NewJetStreamPublisher(natsURL)
			if err != nil {
				log.Fatalf("nats jetstream publisher: %v", err)
			}
			defer func() { _ = jsPub.Close() }()
			jsPub.Logger = logger
			spaceEvents = jsPub
		}

		lis, err := net.Listen("tcp", grpcListen)
		if err != nil {
			log.Fatalf("grpc listen: %v", err)
		}
		var roleClient rolev1.RoleServiceClient
		if roleAddr := strings.TrimSpace(os.Getenv("ROLE_GRPC_ADDR")); roleAddr != "" {
			rconn, err := grpc.NewClient(grpcclient.DialTarget(roleAddr), grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatalf("role grpc dial: %v", err)
			}
			defer func() { _ = rconn.Close() }()
			roleClient = rolev1.NewRoleServiceClient(rconn)
		}

		grpcSrv = grpc.NewServer(grpcmw.ServerOptions(logger, grpcmw.WithRegistry(metricsReg))...)
		spaceSvc := &grpcsvc.SpaceGRPC{
			Store:             spaceStore,
			SpaceEvents:       spaceEvents,
			Roles:             roleClient,
			MutationLocker:    store.NewSpaceMutationLocker(mutationLockPool),
			SpaceCoMembership: &grpcsvc.StoreCoMembership{Store: spaceStore},
			Logger:            logger,
		}
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
			spaceSvc.Privacy = &s2s.GRPCUserPrivacy{Client: userClient}
			spaceSvc.ProfileAccounts = s2s.NewGRPCUserProfiles(uconn)
		}
		if socialAddr := strings.TrimSpace(os.Getenv("SOCIAL_GRPC_ADDR")); socialAddr != "" {
			sconn, err := grpc.NewClient(grpcclient.DialTarget(socialAddr), grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatalf("social grpc: %v", err)
			}
			defer func() { _ = sconn.Close() }()
			spaceSvc.Friends = s2s.NewGRPCSocialFriends(sconn)
			spaceSvc.Blocks = s2s.NewGRPCSocialBlocks(sconn)
		}
		if chatAddr := strings.TrimSpace(os.Getenv("CHAT_GRPC_ADDR")); chatAddr != "" {
			cconn, err := grpc.NewClient(grpcclient.DialTarget(chatAddr), grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatalf("chat grpc: %v", err)
			}
			defer func() { _ = cconn.Close() }()
			spaceSvc.Chats = grpcsvc.NewGRPCChatLookup(cconn)
		}
		if natsURL != "" {
			entitlements := &subscriptionconsume.SpaceStoreEntitlement{Store: spaceStore}
			go func() {
				if err := subscriptionconsume.Run(runCtx, natsURL, "space_subscription_entitlement", entitlements); err != nil && runCtx.Err() == nil {
					logger.Error("space subscription consumer stopped", slog.String("error", err.Error()))
				}
			}()
			logger.Info("space subscription entitlement consumer enabled")
		}
		spacev1.RegisterSpaceServiceServer(grpcSrv, spaceSvc)
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
		Addr:    httpAddr,
		Handler: httpserver.Wrap(voiceprom.MountMetricsOnHealth(healthHandler(serviceName), metricsReg), logger),
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
		runCancel()
		if err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	case <-stop:
		runCancel()
		ctx, cancel := context.WithTimeout(context.Background(), runtimeconfig.ShutdownTimeoutFromEnv())
		defer cancel()
		if grpcSrv != nil {
			grpcSrv.GracefulStop()
		}
		if err := server.Shutdown(ctx); err != nil {
			log.Fatal(err)
		}
	}
}
