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
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"

	grpcsvc "voice/backend/matchmaking/internal/grpcsvc"
	"voice/backend/matchmaking/internal/mmevents"
	"voice/backend/matchmaking/internal/queue"
	"voice/backend/matchmaking/internal/store"
	"voice/backend/pkg/grpcmw"
	"voice/backend/pkg/httpserver"

	matchmakingv1 "voice.app/voice/matchmaking/v1"
)

const serviceName = "matchmaking"

func main() {
	logger := httpserver.NewLogger(serviceName)
	httpAddr := ":8080"
	if v := os.Getenv("LISTEN_ADDR"); v != "" {
		httpAddr = v
	}
	grpcListen := ":9090"
	if v := strings.TrimSpace(os.Getenv("MATCHMAKING_GRPC_LISTEN")); v != "" {
		grpcListen = v
	}

	dbURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	var grpcSrv *grpc.Server
	var redisQueue *queue.RedisQueue
	if dbURL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		pool, err := pgxpool.New(ctx, dbURL)
		cancel()
		if err != nil {
			log.Fatalf("postgres: %v", err)
		}
		defer pool.Close()

		gameStore := &store.GameStore{Pool: pool}
		profileStore := &store.ProfileGamesStore{Pool: pool}
		sessionStore := &store.SessionStore{Pool: pool}

		var events mmevents.Publisher = mmevents.NoopPublisher{}
		if natsURL := strings.TrimSpace(os.Getenv("NATS_URL")); natsURL != "" {
			pub, err := mmevents.NewJetStreamPublisher(natsURL)
			if err != nil {
				logger.Warn("NATS publisher disabled", slog.Any("error", err))
			} else {
				events = pub
				defer func() { _ = pub.Close() }()
			}
		}

		if redisAddr := strings.TrimSpace(os.Getenv("MATCHMAKING_REDIS_ADDR")); redisAddr != "" {
			rdb := redis.NewClient(&redis.Options{
				Addr:     redisAddr,
				Password: strings.TrimSpace(os.Getenv("MATCHMAKING_REDIS_PASSWORD")),
			})
			redisQueue = &queue.RedisQueue{Client: rdb, Prefix: "mm"}
		}

		lis, err := net.Listen("tcp", grpcListen)
		if err != nil {
			log.Fatalf("grpc listen: %v", err)
		}
		grpcSrv = grpc.NewServer(grpcmw.ServerOptions(logger)...)
		matchmakingv1.RegisterMatchmakingServiceServer(grpcSrv, &grpcsvc.MatchmakingGRPC{
			Games:        gameStore,
			ProfileGames: profileStore,
			Sessions:     sessionStore,
			Queue:        redisQueue,
			Events:       events,
			Logger:       logger,
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

	health := healthHandler(serviceName)
	if redisQueue != nil {
		health = healthWithRedis(health, redisQueue)
	}

	server := &http.Server{
		Addr:              httpAddr,
		Handler:           httpserver.Wrap(health, logger),
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
