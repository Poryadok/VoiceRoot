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

	grpcsvc "voice/backend/moderation/internal/grpcsvc"
	"voice/backend/moderation/internal/store"
	"voice/backend/pkg/grpcmw"
	"voice/backend/pkg/httpserver"

	moderationv1 "voice.app/voice/moderation/v1"
)

const serviceName = "moderation"

func main() {
	logger := httpserver.NewLogger(serviceName)
	addr := ":8080"
	if v := os.Getenv("LISTEN_ADDR"); v != "" {
		addr = v
	}
	grpcAddr := ":9090"
	if v := strings.TrimSpace(os.Getenv("MODERATION_GRPC_LISTEN")); v != "" {
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

		lis, err := net.Listen("tcp", grpcAddr)
		if err != nil {
			log.Fatalf("grpc listen: %v", err)
		}
		grpcSrv = grpc.NewServer(grpcmw.ServerOptions(logger)...)
		moderationv1.RegisterModerationServiceServer(grpcSrv, &grpcsvc.ModerationGRPC{
			Reports: &store.ReportStore{Pool: pool},
		})
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
