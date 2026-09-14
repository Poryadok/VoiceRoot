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

	"voice/backend/pkg/grpcmw"
	"voice/backend/pkg/httpserver"
	"voice/backend/pkg/runtimeconfig"
	"voice/backend/story/internal/clients"
	grpcsvc "voice/backend/story/internal/grpcsvc"
	"voice/backend/story/internal/jobs"
	"voice/backend/story/internal/privacy"
	"voice/backend/story/internal/store"

	storyv1 "voice.app/voice/story/v1"
)

const serviceName = "story"

func main() {
	logger := httpserver.NewLogger(serviceName)
	mediaClient, mediaConn, publicJWKS, err := loadSignedStoryMediaClientFromEnv()
	if err != nil {
		log.Fatalf("story media principal config: %v", err)
	}
	if mediaConn != nil {
		defer func() { _ = mediaConn.Close() }()
	}
	serviceCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
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
		if mediaClient != nil {
			svc.Files = mediaClient
		}
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

		jobs.StartExpiryWorker(serviceCtx, st, svc.Events, logger)
		jobs.StartArchivePurgeWorker(serviceCtx, st, fileDeleter, logger)
	}

	mux := http.NewServeMux()
	mux.Handle("/", healthHandler(serviceName))
	if publicJWKS != nil {
		mux.HandleFunc("/.well-known/jwks.json", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				w.Header().Set("Allow", http.MethodGet)
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(publicJWKS)
		})
	}
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

	select {
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	case <-serviceCtx.Done():
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
