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
	"google.golang.org/grpc/credentials/insecure"

	"voice/backend/bot/internal/dispatch"
	grpcsvc "voice/backend/bot/internal/grpcsvc"
	"voice/backend/bot/internal/store"
	"voice/backend/pkg/grpcclient"
	"voice/backend/pkg/grpcmw"
	"voice/backend/pkg/httpserver"

	botv1 "voice.app/voice/bot/v1"
	chatv1 "voice.app/voice/chat/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	rolev1 "voice.app/voice/role/v1"
	userv1 "voice.app/voice/user/v1"
)

const serviceName = "bot"

func main() {
	logger := httpserver.NewLogger(serviceName)
	addr := ":8080"
	if v := os.Getenv("LISTEN_ADDR"); v != "" {
		addr = v
	}
	grpcAddr := ":9090"
	if v := strings.TrimSpace(os.Getenv("BOT_GRPC_LISTEN")); v != "" {
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
		st := &store.BotStore{Pool: pool}
		hub := dispatch.NewHub()
		svc := grpcsvc.NewBotGRPC(st, hub)
		wireDownstream(svc, logger)
		grpcSrv = grpc.NewServer(grpcmw.ServerOptions(logger)...)
		botv1.RegisterBotServiceServer(grpcSrv, svc)
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

func wireDownstream(svc *grpcsvc.BotGRPC, logger *slog.Logger) {
	if addr := grpcclient.DialTarget(strings.TrimSpace(os.Getenv("MESSAGING_GRPC_ADDR"))); addr != "" {
		conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Fatalf("messaging grpc: %v", err)
		}
		svc.Messaging = messagingv1.NewMessagingServiceClient(conn)
		logger.Info("messaging client enabled", slog.String("addr", addr))
	}
	if addr := grpcclient.DialTarget(strings.TrimSpace(os.Getenv("CHAT_GRPC_ADDR"))); addr != "" {
		conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Fatalf("chat grpc: %v", err)
		}
		svc.Chat = chatv1.NewChatServiceClient(conn)
		logger.Info("chat client enabled", slog.String("addr", addr))
	}
	if addr := grpcclient.DialTarget(strings.TrimSpace(os.Getenv("ROLE_GRPC_ADDR"))); addr != "" {
		conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Fatalf("role grpc: %v", err)
		}
		svc.Role = rolev1.NewRoleServiceClient(conn)
		logger.Info("role client enabled", slog.String("addr", addr))
	}
	if addr := grpcclient.DialTarget(strings.TrimSpace(os.Getenv("USER_GRPC_ADDR"))); addr != "" {
		conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Fatalf("user grpc: %v", err)
		}
		svc.User = userv1.NewUserServiceClient(conn)
		logger.Info("user client enabled", slog.String("addr", addr))
	}
}

func runMigrations(pool *pgxpool.Pool) error {
	migrationPath := os.Getenv("BOT_MIGRATION_PATH")
	if migrationPath == "" {
		migrationPath = filepath.Join("migrations", "bot_db", "000001_init.up.sql")
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
