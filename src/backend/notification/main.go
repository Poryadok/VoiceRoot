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

	grpcsvc "voice/backend/notification/internal/grpcsvc"
	"voice/backend/notification/internal/apns"
	"voice/backend/notification/internal/dispatch"
	"voice/backend/notification/internal/fcm"
	"voice/backend/notification/internal/store"
	"voice/backend/pkg/grpcmw"
	"voice/backend/pkg/httpserver"

	notificationv1 "voice.app/voice/notification/v1"
)

const serviceName = "notification"

func main() {
	logger := httpserver.NewLogger(serviceName)
	httpAddr := ":8080"
	if v := os.Getenv("LISTEN_ADDR"); v != "" {
		httpAddr = v
	}
	grpcListen := ":9090"
	if v := strings.TrimSpace(os.Getenv("NOTIFICATION_GRPC_LISTEN")); v != "" {
		grpcListen = v
	}

	dbURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	var grpcSrv *grpc.Server
	if dbURL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		pool, err := pgxpool.New(ctx, dbURL)
		cancel()
		if err != nil {
			log.Fatalf("postgres: %v", err)
		}
		defer pool.Close()

		tokenStore := &store.DeviceTokenStore{Pool: pool}
		fcmSender := fcm.Sender(&fcm.NoopSender{Logger: logger})
		if strings.TrimSpace(os.Getenv("FCM_CREDENTIALS_JSON")) != "" {
			logger.Info("FCM credentials configured; using noop until HTTP sender is enabled")
		}
		apnsSender := apns.Sender(&apns.NoopSender{Logger: logger})
		if cfg, ok := apns.ConfigFromEnv(); ok {
			httpSender, err := apns.NewHTTPSender(cfg)
			if err != nil {
				logger.Warn("APNs credentials invalid; using noop sender", slog.Any("error", err))
			} else {
				apnsSender = httpSender
				logger.Info("APNs HTTP sender enabled", slog.Bool("production", cfg.Production))
			}
		}
		pusher := &dispatch.PushDispatcher{FCM: fcmSender, APNs: apnsSender}

		if natsURL := strings.TrimSpace(os.Getenv("NATS_URL")); natsURL != "" {
			instanceID := strings.TrimSpace(os.Getenv("HOSTNAME"))
			go func() {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				if err := runMatchmakingEventsConsumer(ctx, natsURL, instanceID, tokenStore, pusher, logger); err != nil && logger != nil {
					logger.Error("matchmaking.events consumer exited", slog.Any("error", err))
				}
			}()
		}

		lis, err := net.Listen("tcp", grpcListen)
		if err != nil {
			log.Fatalf("grpc listen: %v", err)
		}
		grpcSrv = grpc.NewServer(grpcmw.ServerOptions(logger)...)
		notificationv1.RegisterNotificationServiceServer(grpcSrv, &grpcsvc.NotificationGRPC{
			Tokens: tokenStore,
			Pusher: pusher,
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
		Handler:           httpserver.Wrap(healthHandler(serviceName), logger),
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
