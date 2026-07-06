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

	"voice/backend/story/internal/clients"
	grpcsvc "voice/backend/story/internal/grpcsvc"
	"voice/backend/story/internal/jobs"
	"voice/backend/story/internal/privacy"
	"voice/backend/story/internal/store"
	"voice/backend/pkg/grpcmw"
	"voice/backend/pkg/httpserver"
	"voice/backend/pkg/runtimeconfig"

	storyv1 "voice.app/voice/story/v1"
)

const serviceName = "story"

func main() {
	logger := httpserver.NewLogger(serviceName)
	addr := ":8080"
	if v := os.Getenv("LISTEN_ADDR"); v != "" {
		addr = v
	}
	grpcAddr := ":9090"
	if v := strings.TrimSpace(os.Getenv("STORY_GRPC_LISTEN")); v != "" {
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
		st := &store.StoryStore{Pool: pool}
		svc := grpcsvc.NewStoryGRPC(st)
		friendChecker := privacy.NewFriendChecker(logger)
		svc.Friends = friendChecker
		svc.Audience = friendChecker
		svc.FeedAuthors = friendChecker
		fileDeleter := clients.WireGRPC(logger, svc)
		grpcSrv = grpc.NewServer(grpcmw.ServerOptions(logger)...)
		storyv1.RegisterStoryServiceServer(grpcSrv, svc)
		go func() {
			if err := grpcSrv.Serve(lis); err != nil {
				log.Fatalf("grpc serve: %v", err)
			}
		}()
		logger.Info("story grpc listening", slog.String("addr", grpcAddr))

		jobs.StartExpiryWorker(context.Background(), st, svc.Events, logger)
		jobs.StartArchivePurgeWorker(context.Background(), st, fileDeleter, logger)
	}

	mux := healthHandler(serviceName)
	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}
	httpserver.ApplyHTTPServerTimeouts(server)
	errCh := make(chan error, 1)
	logger.Info("story http listening", slog.String("addr", addr))
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
		if grpcSrv != nil {
			grpcSrv.GracefulStop()
		}
		ctx, cancel := context.WithTimeout(context.Background(), runtimeconfig.ShutdownTimeoutFromEnv())
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			log.Fatal(err)
		}
	}
}
