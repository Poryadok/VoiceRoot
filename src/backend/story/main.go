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

	"google.golang.org/grpc"

	"voice/backend/pkg/grpcmw"
	"voice/backend/pkg/httpserver"
	"voice/backend/pkg/postgres"
	"voice/backend/pkg/runtimeconfig"
	"voice/backend/story/internal/clients"
	grpcsvc "voice/backend/story/internal/grpcsvc"
	"voice/backend/story/internal/inventorygrpc"
	"voice/backend/story/internal/jobs"
	"voice/backend/story/internal/privacy"
	"voice/backend/story/internal/store"
	"voice/backend/story/internal/storyevents"

	storyv1 "voice.app/voice/story/v1"
)

const serviceName = "story"

func main() {
	logger := httpserver.NewLogger(serviceName)
	inventoryConfig, inventoryEnabled, err := inventorygrpc.LoadFromEnv(os.LookupEnv)
	if err != nil {
		log.Fatalf("story reference inventory config: %v", err)
	}
	mediaClient, mediaConn, publicJWKS, err := loadSignedStoryMediaClientFromEnv()
	if err != nil {
		log.Fatalf("story media principal config: %v", err)
	}
	if mediaConn != nil {
		defer func() { _ = mediaConn.Close() }()
	}
	jwksHTTPS, jwksHTTPSEnabled, err := newStoryJWKSHTTPSServer(os.LookupEnv, publicJWKS)
	if err != nil {
		log.Fatalf("story HTTPS JWKS config: %v", err)
	}
	var jwksListener net.Listener
	if jwksHTTPSEnabled {
		jwksListener, err = listenStoryJWKSHTTPS(jwksHTTPS)
		if err != nil {
			log.Fatalf("story HTTPS JWKS listen: %v", err)
		}
		defer func() { _ = jwksListener.Close() }()
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
	var inventoryErrors <-chan error
	dbURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if inventoryEnabled && dbURL == "" {
		log.Fatal("story reference inventory requires DATABASE_URL")
	}
	if dbURL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), runtimeconfig.PostgresConnectTimeoutFromEnv())
		pool, err := postgres.NewPool(ctx, dbURL)
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
		inventory, err := startReferenceInventory(serviceCtx, inventoryConfig, inventoryEnabled, st, logger)
		if err != nil {
			log.Fatalf("story reference inventory startup: %v", err)
		}
		if inventory != nil {
			inventoryErrors = inventory.errors
			defer func() {
				ctx, cancel := context.WithTimeout(context.Background(), runtimeconfig.ShutdownTimeoutFromEnv())
				defer cancel()
				if err := inventory.Close(ctx); err != nil {
					logger.Error("story reference inventory shutdown", slog.Any("error", err))
				}
			}()
		}
		svc := grpcsvc.NewStoryGRPC(st)
		if natsURL := strings.TrimSpace(os.Getenv("NATS_URL")); natsURL != "" {
			pub, err := storyevents.NewJetStreamPublisher(natsURL)
			if err != nil {
				log.Fatalf("story.events publisher: %v", err)
			}
			if err := pub.Validate(); err != nil {
				log.Fatalf("story.events bootstrap: %v", err)
			}
			pub.Logger = logger
			svc.Events = pub
			defer func() { _ = pub.Close() }()
		}
		if mediaClient != nil {
			svc.Files = mediaClient
		}
		friendChecker := privacy.NewFriendChecker(logger)
		svc.Friends = friendChecker
		svc.Audience = friendChecker
		svc.FeedAuthors = friendChecker
		_ = clients.WireGRPC(logger, svc)
		var fileDeleter jobs.FileDeleter
		if mediaClient != nil {
			fileDeleter = mediaClient
		}
		grpcSrv = grpc.NewServer(grpcmw.ServerOptions(logger)...)
		registerOrdinaryStoryServices(grpcSrv, svc)
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
	var jwksErrors <-chan error
	if jwksHTTPSEnabled {
		ch := make(chan error, 1)
		jwksErrors = ch
		go func() { ch <- jwksHTTPS.ServeTLS(jwksListener, "", "") }()
	}

	select {
	case err := <-jwksErrors:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("story HTTPS JWKS serve: %v", err)
		}
	case err := <-inventoryErrors:
		if err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			log.Fatalf("story reference inventory serve: %v", err)
		}
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
		if jwksHTTPS != nil {
			if err := jwksHTTPS.Shutdown(ctx); err != nil {
				log.Fatal(err)
			}
		}
		if err := server.Shutdown(ctx); err != nil {
			log.Fatal(err)
		}
	}
}

func registerOrdinaryStoryServices(server grpc.ServiceRegistrar, svc storyv1.StoryServiceServer) {
	storyv1.RegisterStoryServiceServer(server, svc)
}
