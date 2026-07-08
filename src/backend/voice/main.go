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

	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	grpcsvc "voice/backend/voice/internal/grpcsvc"
	"voice/backend/pkg/grpcclient"
	"voice/backend/pkg/grpcmw"
	"voice/backend/pkg/httpserver"
	"voice/backend/pkg/runtimeconfig"
	voiceprom "voice/backend/pkg/promhttp"
	"voice/backend/voice/internal/livekit"
	"voice/backend/voice/internal/s2s"
	voicestore "voice/backend/voice/internal/store"
	"voice/backend/voice/internal/voiceevents"

	callsv1 "voice.app/voice/calls/v1"
	chatv1 "voice.app/voice/chat/v1"
	userv1 "voice.app/voice/user/v1"
)

const serviceName = "voice"

func main() {
	logger := httpserver.NewLogger(serviceName)
	metricsReg := prometheus.NewRegistry()
	addr := ":8080"
	if v := os.Getenv("LISTEN_ADDR"); v != "" {
		addr = v
	}
	grpcListen := ":9090"
	if v := strings.TrimSpace(os.Getenv("VOICE_GRPC_LISTEN")); v != "" {
		grpcListen = v
	}

	var grpcSrv *grpc.Server
	runCtx, runCancel := context.WithCancel(context.Background())
	defer runCancel()

	var callStore voicestore.CallStore
	if redisAddr := strings.TrimSpace(os.Getenv("VOICE_REDIS_ADDR")); redisAddr != "" {
		rdb := redis.NewClient(&redis.Options{
			Addr:     redisAddr,
			Password: strings.TrimSpace(os.Getenv("VOICE_REDIS_PASSWORD")),
		})
		defer func() { _ = rdb.Close() }()
		callStore = voicestore.NewRedisCallStore(rdb, strings.TrimSpace(os.Getenv("VOICE_REDIS_PREFIX")))
	} else {
		callStore = voicestore.NewMemoryCallStore()
		logger.Warn("VOICE_REDIS_ADDR not set; using in-memory call store")
	}

	var events voiceevents.Publisher
	if natsURL := strings.TrimSpace(os.Getenv("NATS_URL")); natsURL != "" {
		jsPub, err := voiceevents.NewJetStreamPublisher(natsURL)
		if err != nil {
			log.Fatalf("nats jetstream publisher: %v", err)
		}
		defer func() { _ = jsPub.Close() }()
		if err := jsPub.EnsureStream(); err != nil {
			log.Fatalf("nats ensure voice_events stream: %v", err)
		}
		jsPub.Logger = logger
		events = jsPub
	}

	var chatMembers grpcsvc.ChatMembership
	if chatAddr := strings.TrimSpace(os.Getenv("CHAT_GRPC_ADDR")); chatAddr != "" {
		cconn, err := grpc.NewClient(grpcclient.DialTarget(chatAddr), grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Fatalf("chat grpc: %v", err)
		}
		defer func() { _ = cconn.Close() }()
		chatMembers = s2s.NewGRPCChatMembership(chatv1.NewChatServiceClient(cconn))
	}

	var callPrivacy grpcsvc.CallPrivacyChecker
	var callFriends grpcsvc.CallProfileFriendChecker
	var callSpaceCoMembership grpcsvc.CallSpaceCoMembershipChecker
	if userAddr := strings.TrimSpace(os.Getenv("USER_GRPC_ADDR")); userAddr != "" {
		uconn, err := grpc.NewClient(grpcclient.DialTarget(userAddr), grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Fatalf("user grpc: %v", err)
		}
		defer func() { _ = uconn.Close() }()
		callPrivacy = &s2s.GRPCUserPrivacy{Client: userv1.NewUserServiceClient(uconn)}
	}
	if socialAddr := strings.TrimSpace(os.Getenv("SOCIAL_GRPC_ADDR")); socialAddr != "" {
		sconn, err := grpc.NewClient(grpcclient.DialTarget(socialAddr), grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Fatalf("social grpc: %v", err)
		}
		defer func() { _ = sconn.Close() }()
		callFriends = s2s.NewGRPCSocialFriends(sconn)
	}
	if spaceAddr := strings.TrimSpace(os.Getenv("SPACE_GRPC_ADDR")); spaceAddr != "" {
		spconn, err := grpc.NewClient(grpcclient.DialTarget(spaceAddr), grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Fatalf("space grpc: %v", err)
		}
		defer func() { _ = spconn.Close() }()
		callSpaceCoMembership = s2s.NewGRPCSpaceCoMembership(spconn)
	}

	tokenTTL := time.Hour
	voiceSvc := &grpcsvc.VoiceGRPC{
		Calls:             callStore,
		ChatMembers:       chatMembers,
		Privacy:           callPrivacy,
		Friends:           callFriends,
		SpaceCoMembership: callSpaceCoMembership,
		Tokens: livekit.NewHS256TokenIssuer(
			strings.TrimSpace(os.Getenv("LIVEKIT_API_KEY")),
			strings.TrimSpace(os.Getenv("LIVEKIT_API_SECRET")),
			strings.TrimSpace(os.Getenv("LIVEKIT_URL")),
			tokenTTL,
		),
		Events:      events,
		RingTimeout: 30 * time.Second,
		Logger:      logger,
	}
	lis, err := net.Listen("tcp", grpcListen)
	if err != nil {
		log.Fatalf("grpc listen: %v", err)
	}
	grpcSrv = grpc.NewServer(grpcmw.ServerOptions(logger, grpcmw.WithRegistry(metricsReg))...)
	callsv1.RegisterVoiceServiceServer(grpcSrv, voiceSvc)
	go func() {
		logger.Info("gRPC listening", slog.String("addr", grpcListen))
		if err := grpcSrv.Serve(lis); err != nil {
			log.Fatalf("grpc serve: %v", err)
		}
	}()
	go runMissedCallSweeper(runCtx, voiceSvc, logger)

	server := &http.Server{
		Addr:    addr,
		Handler: httpserver.Wrap(voiceprom.MountMetricsOnHealth(healthHandler(serviceName), metricsReg), logger),
	}
	httpserver.ApplyHTTPServerTimeouts(server)
	errCh := make(chan error, 1)
	logger.Info("listening", slog.String("addr", addr))
	go func() {
		errCh <- server.ListenAndServe()
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	select {
	case err := <-errCh:
		runCancel()
		if err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	case <-stop:
		runCancel()
		ctx, cancel := context.WithTimeout(context.Background(), runtimeconfig.ShutdownTimeoutFromEnv())
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			log.Fatal(err)
		}
		if grpcSrv != nil {
			grpcSrv.GracefulStop()
		}
	}
}

func runMissedCallSweeper(ctx context.Context, svc *grpcsvc.VoiceGRPC, logger *slog.Logger) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := svc.MarkExpiredCallsMissed(ctx); err != nil {
				logger.Error("voice missed-call sweeper", slog.String("error", err.Error()))
			}
		}
	}
}
