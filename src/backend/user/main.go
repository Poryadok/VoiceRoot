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
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"

	grpcsvc "voice/backend/user/internal/grpcsvc"
	"voice/backend/pkg/grpcclient"
	"voice/backend/pkg/grpcmw"
	"voice/backend/pkg/httpserver"
	"voice/backend/user/internal/r2avatar"
	"voice/backend/user/internal/store"

	userv1 "voice.app/voice/user/v1"
)

const serviceName = "user"

func main() {
	logger := httpserver.NewLogger(serviceName)
	httpAddr := ":8080"
	if v := os.Getenv("LISTEN_ADDR"); v != "" {
		httpAddr = v
	}
	grpcAddr := ":9090"
	if v := os.Getenv("USER_GRPC_ADDR"); v != "" {
		grpcAddr = v
	}

	dbURL := os.Getenv("DATABASE_URL")
	var pool *pgxpool.Pool
	if dbURL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		p, err := pgxpool.New(ctx, dbURL)
		cancel()
		if err != nil {
			log.Fatalf("postgres: %v", err)
		}
		pool = p
		defer pool.Close()
	}

	if pool != nil {
		var blocks grpcsvc.AccountBlockChecker
		if socialAddr := strings.TrimSpace(os.Getenv("SOCIAL_GRPC_ADDR")); socialAddr != "" {
			sconn, err := grpc.NewClient(grpcclient.DialTarget(socialAddr),
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			)
			if err != nil {
				log.Fatalf("social grpc: %v", err)
			}
			sconn.Connect()
			waitCtx, waitCancel := context.WithTimeout(context.Background(), 30*time.Second)
			for {
				st := sconn.GetState()
				if st == connectivity.Ready {
					break
				}
				if st == connectivity.Shutdown {
					waitCancel()
					_ = sconn.Close()
					log.Fatalf("social grpc: unexpected shutdown")
				}
				if !sconn.WaitForStateChange(waitCtx, st) {
					waitCancel()
					_ = sconn.Close()
					log.Fatalf("social grpc dial: %v", context.Cause(waitCtx))
				}
			}
			waitCancel()
			defer func() { _ = sconn.Close() }()
			blocks = grpcsvc.NewSocialGRPCBlocks(sconn)
		}

		var presence *store.PresenceStore
		if redisAddr := strings.TrimSpace(os.Getenv("USER_REDIS_ADDR")); redisAddr != "" {
			rdb := redis.NewClient(&redis.Options{Addr: redisAddr})
			pctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err := rdb.Ping(pctx).Err()
			cancel()
			if err != nil {
				log.Fatalf("redis ping: %v", err)
			}
			presence = store.NewPresenceStore(rdb)
			defer func() { _ = rdb.Close() }()
		}

		var avatarPresigner grpcsvc.AvatarPresigner
		avatarPublicBase := ""
		r2cfg := r2avatar.EnvConfigFromOSEnv()
		if r2cfg.Endpoint != "" {
			p, err := r2avatar.NewS3R2PutPresigner(r2cfg)
			if err != nil {
				log.Printf("user: USER_R2_* set but presigner init failed (avatar upload disabled): %v", err)
			} else {
				avatarPresigner = p
				avatarPublicBase = strings.TrimSpace(r2cfg.PublicBaseURL)
			}
		}

		lis, err := net.Listen("tcp", grpcAddr)
		if err != nil {
			log.Fatalf("grpc listen: %v", err)
		}
		srv := grpc.NewServer(grpcmw.ServerOptions(logger)...)
		userv1.RegisterUserServiceServer(srv, &grpcsvc.UserGRPC{
			Profiles:            store.NewProfileStore(pool),
			Presence:            presence,
			Blocks:              blocks,
			AvatarPresigner:     avatarPresigner,
			AvatarPublicBaseURL: avatarPublicBase,
		})
		go func() {
			logger.Info("gRPC listening", slog.String("addr", grpcAddr))
			if err := srv.Serve(lis); err != nil {
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
		if err := server.Shutdown(ctx); err != nil {
			log.Fatal(err)
		}
	}
}
