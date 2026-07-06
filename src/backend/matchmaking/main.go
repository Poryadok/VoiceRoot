package main

import (
	"context"
	"fmt"
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
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"

	grpcsvc "voice/backend/matchmaking/internal/grpcsvc"
	"voice/backend/matchmaking/internal/matcher"
	"voice/backend/matchmaking/internal/mmevents"
	"voice/backend/matchmaking/internal/queue"
	"voice/backend/matchmaking/internal/runtimeconfig"
	"voice/backend/matchmaking/internal/s2s"
	"voice/backend/matchmaking/internal/squad"
	"voice/backend/matchmaking/internal/store"
	"voice/backend/matchmaking/internal/timeout"
	"voice/backend/pkg/grpcclient"
	"voice/backend/pkg/grpcmw"
	"voice/backend/pkg/httpserver"
	"voice/backend/pkg/runtimeconfig"
	voiceprom "voice/backend/pkg/promhttp"

	callsv1 "voice.app/voice/calls/v1"
	chatv1 "voice.app/voice/chat/v1"
	matchmakingv1 "voice.app/voice/matchmaking/v1"
	userv1 "voice.app/voice/user/v1"
)

const serviceName = "matchmaking"

func waitForGRPCReady(ctx context.Context, conn *grpc.ClientConn) error {
	conn.Connect()
	for {
		st := conn.GetState()
		if st == connectivity.Ready {
			return nil
		}
		if st == connectivity.Shutdown {
			return fmt.Errorf("grpc connection shutdown")
		}
		if !conn.WaitForStateChange(ctx, st) {
			return fmt.Errorf("grpc dial: %w", context.Cause(ctx))
		}
	}
}

func main() {
	logger := httpserver.NewLogger(serviceName)
	metricsReg := prometheus.NewRegistry()
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
		ctx, cancel := context.WithTimeout(context.Background(), runtimeconfig.PostgresConnectTimeoutFromEnv())
		pool, err := pgxpool.New(ctx, dbURL)
		cancel()
		if err != nil {
			log.Fatalf("postgres: %v", err)
		}
		defer pool.Close()

		gameStore := &store.GameStore{Pool: pool}
		profileStore := &store.ProfileGamesStore{Pool: pool}
		sessionStore := &store.SessionStore{Pool: pool}
		matchStore := &store.MatchStore{Pool: pool}
		ratingStore := &store.RatingStore{Pool: pool}
		banStore := &store.BanStore{Pool: pool}

		var events mmevents.Publisher = mmevents.NoopPublisher{}
		if natsURL := strings.TrimSpace(os.Getenv("NATS_URL")); natsURL != "" {
			pub, err := mmevents.NewJetStreamPublisher(natsURL)
			if err != nil {
				logger.Warn("NATS publisher disabled", slog.Any("error", err))
			} else {
				pub.Logger = logger
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
		var squadProvisioner grpcsvc.SquadProvisioner
		chatAddr := strings.TrimSpace(os.Getenv("CHAT_GRPC_ADDR"))
		voiceAddr := strings.TrimSpace(os.Getenv("VOICE_GRPC_ADDR"))
		if chatAddr != "" && voiceAddr != "" {
			cconn, err := grpc.NewClient(grpcclient.DialTarget(chatAddr), grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatalf("chat grpc: %v", err)
			}
			defer func() { _ = cconn.Close() }()
			vconn, err := grpc.NewClient(grpcclient.DialTarget(voiceAddr), grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatalf("voice grpc: %v", err)
			}
			defer func() { _ = vconn.Close() }()
			waitCtx, waitCancel := context.WithTimeout(context.Background(), grpcclient.DialTimeoutFromEnv())
			if err := waitForGRPCReady(waitCtx, cconn); err != nil {
				waitCancel()
				log.Fatalf("chat grpc dial: %v", err)
			}
			if err := waitForGRPCReady(waitCtx, vconn); err != nil {
				waitCancel()
				log.Fatalf("voice grpc dial: %v", err)
			}
			waitCancel()
			squadProvisioner = &squad.Provisioner{
				Chat:  &squad.GRPCChatClient{Client: chatv1.NewChatServiceClient(cconn)},
				Voice: &squad.GRPCVoiceClient{Client: callsv1.NewVoiceServiceClient(vconn)},
			}
		} else if chatAddr != "" || voiceAddr != "" {
			logger.Warn("squad provisioning disabled: set both CHAT_GRPC_ADDR and VOICE_GRPC_ADDR")
		}

		var ratingPrivacy grpcsvc.MmRatingPrivacyChecker
		var ratingFriends grpcsvc.MmRatingProfileFriendChecker
		var ratingSpaceCoMembership grpcsvc.MmRatingSpaceCoMembershipChecker
		if userAddr := strings.TrimSpace(os.Getenv("USER_GRPC_ADDR")); userAddr != "" {
			uconn, err := grpc.NewClient(grpcclient.DialTarget(userAddr), grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatalf("user grpc: %v", err)
			}
			defer func() { _ = uconn.Close() }()
			waitCtx, waitCancel := context.WithTimeout(context.Background(), grpcclient.DialTimeoutFromEnv())
			if err := waitForGRPCReady(waitCtx, uconn); err != nil {
				waitCancel()
				log.Fatalf("user grpc dial: %v", err)
			}
			waitCancel()
			ratingPrivacy = &s2s.GRPCUserPrivacy{Client: userv1.NewUserServiceClient(uconn)}
		}
		if socialAddr := strings.TrimSpace(os.Getenv("SOCIAL_GRPC_ADDR")); socialAddr != "" {
			sconn, err := grpc.NewClient(grpcclient.DialTarget(socialAddr), grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatalf("social grpc: %v", err)
			}
			defer func() { _ = sconn.Close() }()
			waitCtx, waitCancel := context.WithTimeout(context.Background(), grpcclient.DialTimeoutFromEnv())
			if err := waitForGRPCReady(waitCtx, sconn); err != nil {
				waitCancel()
				log.Fatalf("social grpc dial: %v", err)
			}
			waitCancel()
			ratingFriends = s2s.NewGRPCSocialFriends(sconn)
		}
		if spaceAddr := strings.TrimSpace(os.Getenv("SPACE_GRPC_ADDR")); spaceAddr != "" {
			spconn, err := grpc.NewClient(grpcclient.DialTarget(spaceAddr), grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatalf("space grpc: %v", err)
			}
			defer func() { _ = spconn.Close() }()
			waitCtx, waitCancel := context.WithTimeout(context.Background(), grpcclient.DialTimeoutFromEnv())
			if err := waitForGRPCReady(waitCtx, spconn); err != nil {
				waitCancel()
				log.Fatalf("space grpc dial: %v", err)
			}
			waitCancel()
			ratingSpaceCoMembership = s2s.NewGRPCSpaceCoMembership(spconn)
		}

		grpcSrv = grpc.NewServer(grpcmw.ServerOptions(logger, grpcmw.WithRegistry(metricsReg))...)
		mmSvc := &grpcsvc.MatchmakingGRPC{
			Games:                   gameStore,
			ProfileGames:            profileStore,
			Sessions:                sessionStore,
			Matches:                 matchStore,
			Ratings:                 ratingStore,
			Bans:                    banStore,
			Queue:                   redisQueue,
			Events:                  events,
			Squad:                   squadProvisioner,
			Logger:                  logger,
			RatingPrivacy:           ratingPrivacy,
			RatingFriends:           ratingFriends,
			RatingSpaceCoMembership: ratingSpaceCoMembership,
		}
		matchmakingv1.RegisterMatchmakingServiceServer(grpcSrv, mmSvc)

		if redisQueue != nil {
			worker := &matcher.Worker{
				Queue:    redisQueue,
				Sessions: sessionStore,
				Matches:  matchStore,
				Games:    gameStore,
				Bans:     banStore,
				Events:   matcher.MMEventsAdapter{Pub: events},
				Logger:   logger,
			}
			go func() {
				ticker := time.NewTicker(2 * time.Second)
				defer ticker.Stop()
				for range ticker.C {
					if err := worker.RunOnce(context.Background()); err != nil && logger != nil {
						logger.Warn("matcher tick failed", slog.Any("error", err))
					}
				}
			}()

			sweeper := &timeout.Sweeper{
				Sessions: sessionStore,
				Queue:    redisQueue,
				Events:   events,
				Timing:   runtimeconfig.LoadSearchTiming(),
				Logger:   logger,
			}
			go func() {
				ticker := time.NewTicker(30 * time.Second)
				defer ticker.Stop()
				for range ticker.C {
					if err := sweeper.RunOnce(context.Background()); err != nil && logger != nil {
						logger.Warn("search timeout sweeper failed", slog.Any("error", err))
					}
				}
			}()
		}
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
		Addr:    httpAddr,
		Handler: httpserver.Wrap(voiceprom.MountMetricsOnHealth(health, metricsReg), logger),
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
