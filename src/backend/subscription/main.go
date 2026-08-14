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
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	grpcsvc "voice/backend/subscription/internal/grpcsvc"
	"voice/backend/subscription/internal/billing"
	"voice/backend/subscription/internal/store"
	"voice/backend/subscription/internal/subscriptionevents"
	"voice/backend/subscription/internal/sweeper"
	"voice/backend/pkg/analyticsevents"
	"voice/backend/pkg/grpcclient"
	"voice/backend/pkg/grpcmw"
	"voice/backend/pkg/httpserver"
	"voice/backend/pkg/runtimeconfig"

	subscriptionv1 "voice.app/voice/subscription/v1"
	spacev1 "voice.app/voice/space/v1"
	userv1 "voice.app/voice/user/v1"
)

const serviceName = "subscription"

func main() {
	logger := httpserver.NewLogger(serviceName)
	addr := ":8080"
	if v := os.Getenv("LISTEN_ADDR"); v != "" {
		addr = v
	}
	grpcAddr := ":9090"
	if v := strings.TrimSpace(os.Getenv("SUBSCRIPTION_GRPC_LISTEN")); v != "" {
		grpcAddr = v
	}

	var grpcSrv *grpc.Server
	runCtx, runCancel := context.WithCancel(context.Background())
	defer runCancel()
	dbURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if dbURL != "" {
		if err := billing.ValidateWebhookSecretConfig(); err != nil {
			log.Fatalf("billing webhook secret: %v", err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), runtimeconfig.PostgresConnectTimeoutFromEnv())
		pool, err := pgxpool.New(ctx, dbURL)
		cancel()
		if err != nil {
			log.Fatalf("postgres: %v", err)
		}
		defer pool.Close()

		lis, err := net.Listen("tcp", grpcAddr)
		if err != nil {
			log.Fatalf("grpc listen: %v", err)
		}
		st := &store.SubscriptionStore{Pool: pool}
		svc := grpcsvc.NewSubscriptionGRPC(st)
		var domainPub *subscriptionevents.JetStreamPublisher
		if natsURL := strings.TrimSpace(os.Getenv("NATS_URL")); natsURL != "" {
			if pub, err := analyticsevents.NewJetStreamPublisher(natsURL); err == nil {
				pub.HashKey = strings.TrimSpace(os.Getenv("ANALYTICS_ID_HASH_KEY"))
				svc.Analytics = pub
				logger.Info("analytics telemetry publisher enabled")
			} else {
				logger.Warn("analytics publisher unavailable", slog.Any("error", err))
			}
			if pub, err := subscriptionevents.NewJetStreamPublisher(natsURL); err == nil {
				domainPub = pub
				svc.DomainEvents = pub
				defer func() { _ = pub.Close() }()
				logger.Info("subscription.events publisher enabled")
			} else {
				logger.Warn("subscription.events publisher unavailable", slog.Any("error", err))
			}
		}
		go runGraceSweeper(runCtx, st, domainPub, logger)
		if userAddr := grpcclient.DialTarget(strings.TrimSpace(os.Getenv("USER_GRPC_ADDR"))); userAddr != "" {
			conn, err := grpc.NewClient(userAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatalf("user grpc: %v", err)
			}
			defer func() { _ = conn.Close() }()
			svc.UserProfiles = &grpcsvc.UserGRPCProfileDowngrade{Client: userv1.NewUserServiceClient(conn)}
			logger.Info("user profile downgrade client enabled", slog.String("addr", userAddr))
		}
		if spaceAddr := grpcclient.DialTarget(strings.TrimSpace(os.Getenv("SPACE_GRPC_ADDR"))); spaceAddr != "" {
			conn, err := grpc.NewClient(spaceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatalf("space grpc: %v", err)
			}
			defer func() { _ = conn.Close() }()
			svc.SpaceEntitlements = spacev1.NewSpaceServiceClient(conn)
			logger.Info("space entitlement sync client enabled", slog.String("addr", spaceAddr))
		}
		grpcSrv = grpc.NewServer(grpcmw.ServerOptions(logger)...)
		subscriptionv1.RegisterSubscriptionServiceServer(grpcSrv, svc)
		go func() {
			logger.Info("gRPC listening", slog.String("addr", grpcAddr))
			if err := grpcSrv.Serve(lis); err != nil {
				log.Fatalf("grpc serve: %v", err)
			}
		}()
	} else {
		logger.Warn("DATABASE_URL not set; gRPC disabled (health only)")
	}

	server := &http.Server{
		Addr:    addr,
		Handler: httpserver.Wrap(healthHandler(serviceName), logger),
	}
	httpserver.ApplyHTTPServerTimeouts(server)
	errCh := make(chan error, 1)
	logger.Info("listening", slog.String("addr", addr))
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

func runGraceSweeper(ctx context.Context, st *store.SubscriptionStore, domainPub *subscriptionevents.JetStreamPublisher, logger *slog.Logger) {
	runner := &sweeper.Runner{Store: st, DomainEvents: domainPub, Logger: logger}
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := runner.RunOnce(ctx); err != nil {
				logger.Error("subscription grace sweeper", slog.String("error", err.Error()))
			}
		}
	}
}
