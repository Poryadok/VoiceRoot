package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"voice/backend/pkg/grpcclient"
	"voice/backend/pkg/httpserver"
	voiceprom "voice/backend/pkg/promhttp"
	"voice/backend/pkg/runtimeconfig"
)

const serviceName = "realtime"

func main() {
	logger := initServiceLogger(serviceName)
	addr := ":8080"
	if v := os.Getenv("LISTEN_ADDR"); v != "" {
		addr = v
	}

	startupHandler := &realtimeHandlerSlot{}
	var config realtimeConfig
	server, err := newRealtimeServerFromEnv(addr, realtimeStartupDependencies{
		buildHandler: func(loaded realtimeConfig) (http.Handler, error) {
			config = loaded
			return startupHandler, nil
		},
		buildServer: func(handler http.Handler) *http.Server {
			return &http.Server{Addr: addr, Handler: handler}
		},
	})
	if err != nil {
		logger.Error("invalid realtime configuration", slog.Any("error", err))
		os.Exit(1)
	}

	chatLister := dialChatBootstrapLister()
	memberInboxLister := dialChatMemberInboxLister()
	subscriptionChecker := dialChatSubscriptionChecker()
	presenceUpdater := dialPresenceUpdater()
	friendLister := dialFriendLister()

	hub := newWSHub()
	hub.memberInboxLister = memberInboxLister
	hub.subscriptionChecker = subscriptionChecker
	instanceID := strings.TrimSpace(os.Getenv("REALTIME_INSTANCE_ID"))
	if instanceID == "" {
		instanceID = uuid.NewString()
	}
	logger.Info("instance ready", slog.String("instance_id", instanceID))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var rf *redisFanout
	var ready readinessDeps
	natsURL := strings.TrimSpace(os.Getenv("NATS_URL"))
	ready.NatsURL = natsURL
	if redisAddr := strings.TrimSpace(os.Getenv("REALTIME_REDIS_ADDR")); redisAddr != "" {
		rdb := redis.NewClient(&redis.Options{
			Addr:     redisAddr,
			Password: strings.TrimSpace(os.Getenv("REALTIME_REDIS_PASSWORD")),
		})
		defer func() { _ = rdb.Close() }()
		ready.Redis = rdb
		rf = newRedisFanout(redisFanoutConfig{
			Client:     rdb,
			Hub:        hub,
			InstanceID: instanceID,
		})
		go func() {
			err := rf.runSubscriber(ctx)
			if err != nil && err != context.Canceled {
				logger.Error("redis subscriber exited", slog.String("error", err.Error()))
			}
		}()
	}

	metricsReg := prometheus.NewRegistry()
	initRealtimeMetrics(metricsReg)

	if natsURL := strings.TrimSpace(os.Getenv("NATS_URL")); natsURL != "" {
		go func() {
			nc, err := nats.Connect(natsURL, natsConnectOptions("voice-realtime-nats-lag")...)
			if err != nil {
				logger.Warn("nats lag poller connect failed", slog.String("error", err.Error()))
				return
			}
			defer func() { _ = nc.Drain() }()
			js, err := nc.JetStream()
			if err != nil {
				logger.Warn("nats lag poller jetstream failed", slog.String("error", err.Error()))
				return
			}
			runNatsConsumeLagPoller(ctx, js, instanceID)
		}()
		go func() {
			err := runMessageEventsConsumer(ctx, hub, natsURL, instanceID, logger)
			if err != nil && err != context.Canceled {
				logger.Error("message.events consumer exited", slog.String("error", err.Error()))
			}
		}()
		go func() {
			err := runVoiceEventsConsumer(ctx, hub, natsURL, instanceID, logger)
			if err != nil && err != context.Canceled {
				logger.Error("voice.events consumer exited", slog.String("error", err.Error()))
			}
		}()
		go func() {
			err := runChatEventsConsumer(ctx, hub, natsURL, instanceID, logger)
			if err != nil && err != context.Canceled {
				logger.Error("chat.events consumer exited", slog.String("error", err.Error()))
			}
		}()
		go func() {
			err := runMatchmakingEventsConsumer(ctx, hub, natsURL, instanceID, logger)
			if err != nil && err != context.Canceled {
				logger.Error("matchmaking.events consumer exited", slog.String("error", err.Error()))
			}
		}()
		go func() {
			err := runRoleEventsConsumer(ctx, hub, natsURL, instanceID, logger)
			if err != nil && err != context.Canceled {
				logger.Error("role.events consumer exited", slog.String("error", err.Error()))
			}
		}()
		go func() {
			err := runUserEventsConsumer(ctx, hub, friendLister, natsURL, instanceID, logger)
			if err != nil && err != context.Canceled {
				logger.Error("user.events consumer exited", slog.String("error", err.Error()))
			}
		}()
	}

	var dap deliveryAckPublisher
	var dapNC *nats.Conn
	if natsURL != "" {
		nc, err := nats.Connect(natsURL, natsConnectOptions("voice-realtime-delivery-ack")...)
		if err != nil {
			logger.Warn("delivery ack publisher connect failed", slog.String("error", err.Error()))
		} else {
			pub, err := newJetstreamDeliveryAckPublisher(nc)
			if err != nil {
				logger.Warn("delivery ack publisher init failed", slog.String("error", err.Error()))
				_ = nc.Drain()
			} else {
				dap = pub
				dapNC = nc
			}
		}
	}
	defer func() {
		if dapNC != nil {
			_ = dapNC.Drain()
		}
	}()

	startupHandler.set(httpserver.Wrap(
		voiceprom.MountMetricsOnHealth(
			newServiceHandlerWithPresenceAndSessionEpoch(
				serviceName,
				config.tokenValidator,
				chatLister,
				hub,
				rf,
				instanceID,
				presenceUpdater,
				dap,
				ready,
				wsSessionEpochPolicy{
					Strict: config.sessionEpochStrict,
					Floor:  config.sessionEpochFloor,
				},
			),
			metricsReg,
		),
		logger,
	))
	server.ReadTimeout = runtimeconfig.DurationFromEnv("HTTP_READ_TIMEOUT", 0)
	server.WriteTimeout = runtimeconfig.DurationFromEnv("HTTP_WRITE_TIMEOUT", 0)
	errCh := make(chan error, 1)
	logger.Info("listening", slog.String("addr", addr))
	go func() {
		errCh <- server.ListenAndServe()
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	select {
	case err := <-errCh:
		cancel()
		if err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	case <-stop:
		cancel()
		shutCtx, shutCancel := context.WithTimeout(context.Background(), runtimeconfig.ShutdownTimeoutFromEnv())
		defer shutCancel()
		if err := server.Shutdown(shutCtx); err != nil {
			log.Fatal(err)
		}
	}
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return strings.TrimSpace(a)
	}
	return strings.TrimSpace(b)
}

func dialChatMemberInboxLister() chatMemberInboxLister {
	addr := strings.TrimSpace(os.Getenv("REALTIME_CHAT_GRPC_ADDR"))
	if addr == "" {
		return nil
	}
	conn, err := grpc.NewClient(grpcclient.DialTarget(addr), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		svcLogger.Warn("chat member inbox grpc client unavailable", slog.String("addr", addr), slog.String("error", err.Error()))
		return nil
	}
	return newGRPCChatMemberInboxLister(conn)
}

func dialChatSubscriptionChecker() chatSubscriptionChecker {
	addr := strings.TrimSpace(os.Getenv("REALTIME_CHAT_GRPC_ADDR"))
	if addr == "" {
		return nil
	}
	conn, err := grpc.NewClient(grpcclient.DialTarget(addr), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		svcLogger.Warn("chat subscription grpc client unavailable", slog.String("addr", addr), slog.String("error", err.Error()))
		return nil
	}
	return newGRPCChatSubscriptionChecker(conn)
}

func dialChatBootstrapLister() chatBootstrapLister {
	addr := strings.TrimSpace(os.Getenv("REALTIME_CHAT_GRPC_ADDR"))
	if addr == "" {
		return nil
	}
	conn, err := grpc.NewClient(grpcclient.DialTarget(addr), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		svcLogger.Warn("chat grpc client unavailable", slog.String("addr", addr), slog.String("error", err.Error()))
		return nil
	}
	return newGRPCDMChatLister(conn)
}

func dialPresenceUpdater() presenceUpdater {
	addr := strings.TrimSpace(os.Getenv("REALTIME_USER_GRPC_ADDR"))
	if addr == "" {
		return nil
	}
	conn, err := grpc.NewClient(grpcclient.DialTarget(addr), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		svcLogger.Warn("user grpc client unavailable", slog.String("addr", addr), slog.String("error", err.Error()))
		return nil
	}
	return newGRPCPresenceUpdater(conn)
}

func dialFriendLister() friendLister {
	addr := strings.TrimSpace(os.Getenv("REALTIME_SOCIAL_GRPC_ADDR"))
	if addr == "" {
		return nil
	}
	conn, err := grpc.NewClient(grpcclient.DialTarget(addr), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		svcLogger.Warn("social grpc client unavailable", slog.String("addr", addr), slog.String("error", err.Error()))
		return nil
	}
	return newGRPCFriendLister(conn)
}
