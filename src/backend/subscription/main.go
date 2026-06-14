package main

import (
	"context"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"

	grpcsvc "voice/backend/subscription/internal/grpcsvc"
	"voice/backend/subscription/internal/store"
	"voice/backend/pkg/grpcmw"
	"voice/backend/pkg/httpserver"

	subscriptionv1 "voice.app/voice/subscription/v1"
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
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		pool, err := pgxpool.New(ctx, dbURL)
		cancel()
		if err != nil {
			log.Fatalf("postgres: %v", err)
		}
		defer pool.Close()

		if err := runMigrations(pool); err != nil {
			log.Fatalf("migrate: %v", err)
		}

		lis, err := net.Listen("tcp", grpcAddr)
		if err != nil {
			log.Fatalf("grpc listen: %v", err)
		}
		st := &store.SubscriptionStore{Pool: pool}
		grpcSrv = grpc.NewServer(grpcmw.ServerOptions(logger)...)
		subscriptionv1.RegisterSubscriptionServiceServer(grpcSrv, grpcsvc.NewSubscriptionGRPC(st))
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
		Addr:              addr,
		Handler:           httpserver.Wrap(healthHandler(serviceName), logger),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
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

func runMigrations(pool *pgxpool.Pool) error {
	migrationPath := os.Getenv("SUBSCRIPTION_MIGRATION_PATH")
	if migrationPath == "" {
		migrationPath = filepath.Join("migrations", "subscription_db", "000001_init.up.sql")
	}
	sqlBytes, err := os.ReadFile(migrationPath)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err = pool.Exec(ctx, string(sqlBytes))
	return err
}
