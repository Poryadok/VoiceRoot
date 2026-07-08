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
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	grpcsvc "voice/backend/subscription/internal/grpcsvc"
	"voice/backend/subscription/internal/store"
	"voice/backend/pkg/analyticsevents"
	"voice/backend/pkg/grpcclient"
	"voice/backend/pkg/grpcmw"
	"voice/backend/pkg/httpserver"
	"voice/backend/pkg/runtimeconfig"

	subscriptionv1 "voice.app/voice/subscription/v1"
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
	dbURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if dbURL != "" {
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
		if natsURL := strings.TrimSpace(os.Getenv("NATS_URL")); natsURL != "" {
			if pub, err := analyticsevents.NewJetStreamPublisher(natsURL); err == nil {
				svc.Analytics = pub
				logger.Info("analytics telemetry publisher enabled")
			} else {
				logger.Warn("analytics publisher unavailable", slog.Any("error", err))
			}
		}
		if userAddr := grpcclient.DialTarget(strings.TrimSpace(os.Getenv("USER_GRPC_ADDR"))); userAddr != "" {
			conn, err := grpc.NewClient(userAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatalf("user grpc: %v", err)
			}
			defer func() { _ = conn.Close() }()
			svc.UserProfiles = &grpcsvc.UserGRPCProfileDowngrade{Client: userv1.NewUserServiceClient(conn)}
			logger.Info("user profile downgrade client enabled", slog.String("addr", userAddr))
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
		if err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	case <-stop:
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
