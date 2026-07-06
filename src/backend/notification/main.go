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
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"

	grpcsvc "voice/backend/notification/internal/grpcsvc"
	"voice/backend/notification/internal/apns"
	"voice/backend/notification/internal/chatmembers"
	"voice/backend/notification/internal/dispatch"
	"voice/backend/notification/internal/fcm"
	"voice/backend/notification/internal/grouping"
	"voice/backend/notification/internal/presence"
	"voice/backend/notification/internal/pushenrich"
	"voice/backend/notification/internal/store"
	"voice/backend/pkg/grpcmw"
	"voice/backend/pkg/httpserver"
	"voice/backend/pkg/runtimeconfig"
	voiceprom "voice/backend/pkg/promhttp"

	notificationv1 "voice.app/voice/notification/v1"
)

const serviceName = "notification"

func main() {
	logger := httpserver.NewLogger(serviceName)
	metricsReg := prometheus.NewRegistry()
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
		ctx, cancel := context.WithTimeout(context.Background(), runtimeconfig.PostgresConnectTimeoutFromEnv())
		pool, err := pgxpool.New(ctx, dbURL)
		cancel()
		if err != nil {
			log.Fatalf("postgres: %v", err)
		}
		defer pool.Close()

		tokenStore := &store.DeviceTokenStore{Pool: pool}
		fcmSender := fcm.Sender(&fcm.NoopSender{Logger: logger})
		if cfg, ok := fcm.ConfigFromEnv(); ok {
			httpSender, err := fcm.NewHTTPSender(cfg)
			if err != nil {
				logger.Warn("FCM credentials invalid; using noop sender", slog.Any("error", err))
			} else {
				fcmSender = httpSender
				logger.Info("FCM HTTP sender enabled", slog.String("project_id", cfg.ProjectID))
			}
		}
		if strings.EqualFold(strings.TrimSpace(os.Getenv("NOTIFICATION_RECORD_PUSHES")), "true") {
			fcmSender = &fcm.RecordSender{Inner: fcmSender}
			logger.Info("FCM push recording enabled (NOTIFICATION_RECORD_PUSHES)")
		}
		apnsSender := apns.Sender(&apns.NoopSender{Logger: logger})
		voipSender := apns.VoIPSender(&apns.VoIPNoopSender{})
		if cfg, ok := apns.ConfigFromEnv(); ok {
			httpSender, err := apns.NewHTTPSender(cfg)
			if err != nil {
				logger.Warn("APNs credentials invalid; using noop sender", slog.Any("error", err))
			} else {
				apnsSender = httpSender
				logger.Info("APNs HTTP sender enabled", slog.Bool("production", cfg.Production))
			}
			voipHTTP, err := apns.NewHTTPVoIPSender(cfg)
			if err != nil {
				logger.Warn("APNs VoIP credentials invalid; using noop sender", slog.Any("error", err))
			} else {
				voipSender = voipHTTP
				logger.Info("APNs VoIP HTTP sender enabled", slog.Bool("production", cfg.Production))
			}
		}
		pusher := &dispatch.PushDispatcher{FCM: fcmSender, APNs: apnsSender, VoIP: voipSender}

		var groupingStore grouping.Store
		if redisAddr := strings.TrimSpace(os.Getenv("NOTIFICATION_REDIS_ADDR")); redisAddr != "" {
			rdb := redis.NewClient(&redis.Options{Addr: redisAddr})
			groupingStore = grouping.NewRedisStore(rdb)
			logger.Info("push grouping redis enabled", slog.String("addr", redisAddr))
		}

		presenceChecker := presence.Checker(presence.OfflineChecker{})
		if userAddr := strings.TrimSpace(os.Getenv("USER_GRPC_ADDR")); userAddr != "" {
			if pc, err := presence.NewGRPCChecker(userAddr); err != nil {
				logger.Warn("user presence checker unavailable; offline-only routing", slog.Any("error", err))
			} else {
				presenceChecker = pc
				logger.Info("user presence checker enabled", slog.String("addr", userAddr))
			}
		}

		var chatLister chatmembers.Lister = chatmembers.NoopLister{}
		if chatAddr := strings.TrimSpace(os.Getenv("CHAT_GRPC_ADDR")); chatAddr != "" {
			if cl, err := chatmembers.NewGRPCLister(chatAddr); err != nil {
				logger.Warn("chat members lister unavailable; MessageSent push skipped", slog.Any("error", err))
			} else {
				chatLister = cl
				logger.Info("chat members lister enabled", slog.String("addr", chatAddr))
			}
		}

		var pushEnrich pushenrich.Resolver = pushenrich.NoopResolver{}
		if msgAddr := strings.TrimSpace(os.Getenv("MESSAGING_GRPC_ADDR")); msgAddr != "" {
			if userAddr := strings.TrimSpace(os.Getenv("USER_GRPC_ADDR")); userAddr != "" {
				if resolver, err := pushenrich.NewGRPCResolver(msgAddr, userAddr); err != nil {
					logger.Warn("push copy enricher unavailable; generic push body", slog.Any("error", err))
				} else {
					pushEnrich = resolver
					logger.Info("push copy enricher enabled")
				}
			}
		}

		msgPusher := &dispatch.MessagePusher{
			Tokens:   tokenStore,
			Pusher:   pusher,
			Grouping: groupingStore,
			Presence: presenceChecker,
		}
		storyPusher := &dispatch.StoryPusher{
			Tokens: tokenStore,
			Pusher: pusher,
		}

		if natsURL := strings.TrimSpace(os.Getenv("NATS_URL")); natsURL != "" {
			instanceID := strings.TrimSpace(os.Getenv("HOSTNAME"))
			go func() {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				if err := runStoryEventsConsumer(ctx, natsURL, instanceID, tokenStore, storyPusher, logger); err != nil && logger != nil {
					logger.Error("story.events consumer exited", slog.Any("error", err))
				}
			}()
			go func() {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				if err := runMessageEventsConsumer(ctx, natsURL, instanceID, tokenStore, chatLister, msgPusher, pushEnrich, logger); err != nil && logger != nil {
					logger.Error("message.events consumer exited", slog.Any("error", err))
				}
			}()
			go func() {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				if err := runMatchmakingEventsConsumer(ctx, natsURL, instanceID, tokenStore, pusher, logger); err != nil && logger != nil {
					logger.Error("matchmaking.events consumer exited", slog.Any("error", err))
				}
			}()
			go func() {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				if err := runVoiceEventsConsumer(ctx, natsURL, instanceID, tokenStore, pusher, logger); err != nil && logger != nil {
					logger.Error("voice.events consumer exited", slog.Any("error", err))
				}
			}()
		}

		lis, err := net.Listen("tcp", grpcListen)
		if err != nil {
			log.Fatalf("grpc listen: %v", err)
		}
		grpcSrv = grpc.NewServer(grpcmw.ServerOptions(logger, grpcmw.WithRegistry(metricsReg))...)
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
		Addr:    httpAddr,
		Handler: httpserver.Wrap(voiceprom.MountMetricsOnHealth(notificationHTTPHandler(serviceName), metricsReg), logger),
	}
	httpserver.ApplyHTTPServerTimeouts(server)
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
		ctx, cancel := context.WithTimeout(context.Background(), runtimeconfig.ShutdownTimeoutFromEnv())
		defer cancel()
		if grpcSrv != nil {
			grpcSrv.GracefulStop()
		}
		if err := server.Shutdown(ctx); err != nil {
			log.Fatal(err)
		}
	}
}
