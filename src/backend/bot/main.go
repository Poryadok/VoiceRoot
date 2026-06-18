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
	"google.golang.org/grpc/credentials/insecure"

	"voice/backend/bot/internal/botevents"
	"voice/backend/bot/internal/dispatch"
	grpcsvc "voice/backend/bot/internal/grpcsvc"
	"voice/backend/bot/internal/ratelimit"
	"voice/backend/bot/internal/store"
	"voice/backend/pkg/grpcclient"
	"voice/backend/pkg/grpcmw"
	"voice/backend/pkg/httpserver"

	botv1 "voice.app/voice/bot/v1"
	chatv1 "voice.app/voice/chat/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	rolev1 "voice.app/voice/role/v1"
	userv1 "voice.app/voice/user/v1"
	spacev1 "voice.app/voice/space/v1"
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

		lis, err := net.Listen("tcp", grpcAddr)
		if err != nil {
			log.Fatalf("grpc listen: %v", err)
		}
		st := &store.BotStore{Pool: pool}
		hub := dispatch.NewHub()
		svc := grpcsvc.NewBotGRPC(st, hub)
		wireDownstream(svc, logger)
		svc.RehydrateDeferred(context.Background())
		startDeferredTTLSweeper(svc, logger)
		if natsURL := strings.TrimSpace(os.Getenv("NATS_URL")); natsURL != "" {
			pub, err := botevents.NewJetStreamPublisher(natsURL)
			if err != nil {
				log.Fatalf("nats bot events: %v", err)
			}
			pub.Logger = logger
			svc.Events = pub
			defer pub.Close()
			logger.Info("bot.events publisher enabled")
		}
		grpcSrv = grpc.NewServer(
			grpc.ChainUnaryInterceptor(
				ratelimit.GatewayAccessFromEnv(),
				ratelimit.ServerLimiterFromEnv().UnaryServerInterceptor(),
				grpcmw.UnaryRecovery(logger),
				grpcmw.UnaryAccessLog(logger),
			),
		)
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

func startDeferredTTLSweeper(svc *grpcsvc.BotGRPC, logger *slog.Logger) {
	interval := 15 * time.Minute
	if v := strings.TrimSpace(os.Getenv("BOT_DEFERRED_SWEEP_INTERVAL")); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			interval = d
		}
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			if err := svc.RunDeferredTTLSweeper(context.Background()); err != nil {
				logger.Warn("deferred TTL sweeper failed", slog.String("error", err.Error()))
			}
		}
	}()
	logger.Info("deferred TTL sweeper started", slog.String("interval", interval.String()))
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
	if addr := grpcclient.DialTarget(strings.TrimSpace(os.Getenv("SPACE_GRPC_ADDR"))); addr != "" {
		conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Fatalf("space grpc: %v", err)
		}
		svc.Space = spacev1.NewSpaceServiceClient(conn)
		logger.Info("space client enabled", slog.String("addr", addr))
	}
}
