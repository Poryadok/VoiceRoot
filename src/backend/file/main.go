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
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"

	"voice/backend/file/internal/clamav"
	"voice/backend/file/internal/fileevents"
	grpcsvc "voice/backend/file/internal/grpcsvc"
	"voice/backend/file/internal/imgproc"
	"voice/backend/file/internal/jobs"
	"voice/backend/file/internal/principalgrpc"
	"voice/backend/file/internal/principalruntime"
	"voice/backend/file/internal/r2file"
	"voice/backend/file/internal/s2s"
	"voice/backend/file/internal/store"
	"voice/backend/pkg/grpcclient"
	"voice/backend/pkg/grpcmw"
	"voice/backend/pkg/httpserver"
	voiceprom "voice/backend/pkg/promhttp"
	"voice/backend/pkg/runtimeconfig"

	chatv1 "voice.app/voice/chat/v1"
	filev1 "voice.app/voice/file/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	subscriptionv1 "voice.app/voice/subscription/v1"
)

const serviceName = "file"

func waitForGRPCReady(ctx context.Context, conn *grpc.ClientConn) error {
	conn.Connect()
	for {
		st := conn.GetState()
		if st == connectivity.Ready {
			return nil
		}
		if st == connectivity.Shutdown {
			return context.Canceled
		}
		if !conn.WaitForStateChange(ctx, st) {
			return context.Cause(ctx)
		}
	}
}

func main() {
	logger := httpserver.NewLogger(serviceName)
	filePrincipal, err := loadFilePrincipalRuntime()
	if err != nil {
		log.Fatalf("file-to-user principal runtime: %v", err)
	}
	if filePrincipal != nil {
		defer filePrincipal.Close()
		jwksListener, listenErr := net.Listen("tcp", filePrincipal.JWKS.Addr)
		if listenErr != nil {
			log.Fatalf("file principal JWKS listen: %v", listenErr)
		}
		go func() {
			if serveErr := filePrincipal.JWKS.ServeTLS(jwksListener, "", ""); serveErr != nil && serveErr != http.ErrServerClosed {
				log.Fatalf("file principal JWKS serve: %v", serveErr)
			}
		}()
		if os.Getenv("FILE_PRINCIPAL_PROBE") == "1" {
			if probeErr := runFilePrincipalProbe(context.Background(), filePrincipal); probeErr != nil {
				log.Fatalf("file principal probe: %v", probeErr)
			}
			return
		}
	}
	principalConfig, principalEnabled, err := principalruntime.LoadFromEnv()
	if err != nil {
		log.Fatalf("file principal config: %v", err)
	}
	var protectedRuntime *principalruntime.Runtime
	if principalEnabled {
		protectedRuntime, err = principalruntime.New(context.Background(), principalConfig)
		if err != nil {
			log.Fatalf("file principal runtime: %v", err)
		}
		defer func() { _ = protectedRuntime.Close() }()
	}
	metricsReg := prometheus.NewRegistry()
	addr := ":8080"
	if v := os.Getenv("LISTEN_ADDR"); v != "" {
		addr = v
	}
	grpcListen := ":9090"
	if v := strings.TrimSpace(os.Getenv("FILE_GRPC_LISTEN")); v != "" {
		grpcListen = v
	}

	var grpcSrv *grpc.Server
	var protectedSrv *grpc.Server
	var revocableDownloadHTTP http.Handler
	dbURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if dbURL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), runtimeconfig.PostgresConnectTimeoutFromEnv())
		pool, err := pgxpool.New(ctx, dbURL)
		cancel()
		if err != nil {
			log.Fatalf("postgres: %v", err)
		}
		defer pool.Close()

		presigner, err := r2file.NewS3R2Presigner(r2file.EnvConfigFromOSEnv())
		if err != nil {
			log.Printf("%s: R2 config incomplete; uploads disabled: %v", serviceName, err)
		}

		var chatGuard grpcsvc.ChatGuard
		var chatClient chatv1.ChatServiceClient
		if chatAddr := strings.TrimSpace(os.Getenv("CHAT_GRPC_ADDR")); chatAddr != "" {
			cconn, err := grpc.NewClient(grpcclient.DialTarget(chatAddr), grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatalf("chat grpc: %v", err)
			}
			defer func() { _ = cconn.Close() }()
			waitCtx, waitCancel := context.WithTimeout(context.Background(), grpcclient.DialTimeoutFromEnv())
			if err := waitForGRPCReady(waitCtx, cconn); err != nil {
				waitCancel()
				log.Fatalf("chat grpc dial: %v", err)
			}
			waitCancel()
			chatClient = chatv1.NewChatServiceClient(cconn)
			chatGuard = s2s.NewGRPCChatGuard(chatClient)
		}
		if messagingAddr := strings.TrimSpace(os.Getenv("MESSAGING_GRPC_ADDR")); messagingAddr != "" {
			mconn, err := grpc.NewClient(grpcclient.DialTarget(messagingAddr), grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatalf("messaging grpc: %v", err)
			}
			defer func() { _ = mconn.Close() }()
			waitCtx, waitCancel := context.WithTimeout(context.Background(), grpcclient.DialTimeoutFromEnv())
			if err := waitForGRPCReady(waitCtx, mconn); err != nil {
				waitCancel()
				log.Fatalf("messaging grpc dial: %v", err)
			}
			waitCancel()
			if chatClient != nil {
				chatGuard = s2s.NewGRPCChatGuard(chatClient, messagingv1.NewMessagingServiceClient(mconn))
			}
		}
		var scanner grpcsvc.Scanner
		if clamAddr := strings.TrimSpace(os.Getenv("CLAMAV_ADDR")); clamAddr != "" {
			scanner = clamav.Scanner{Addr: clamAddr}
		}
		var reader grpcsvc.ObjectReader
		var entitlements grpcsvc.EntitlementResolver
		if subscriptionAddr := strings.TrimSpace(os.Getenv("SUBSCRIPTION_GRPC_ADDR")); subscriptionAddr != "" {
			conn, err := grpc.NewClient(grpcclient.DialTarget(subscriptionAddr), grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatalf("subscription grpc: %v", err)
			}
			defer func() { _ = conn.Close() }()
			waitCtx, waitCancel := context.WithTimeout(context.Background(), grpcclient.DialTimeoutFromEnv())
			if err := waitForGRPCReady(waitCtx, conn); err != nil {
				waitCancel()
				log.Fatalf("subscription grpc dial: %v", err)
			}
			waitCancel()
			entitlements = s2s.NewSubscriptionEntitlements(subscriptionv1.NewSubscriptionServiceClient(conn))
			logger.Info("subscription entitlement resolver enabled", slog.String("addr", subscriptionAddr))
		}
		var processor grpcsvc.ImageProcessor
		var deleter r2file.ObjectDeleter
		if presigner != nil {
			reader = presigner
			processor = imgproc.Processor{Reader: presigner, Writer: presigner, Deleter: presigner}
			deleter = presigner
		}
		var eventPub fileevents.Publisher = fileevents.NoopPublisher{}
		if natsURL := strings.TrimSpace(os.Getenv("NATS_URL")); natsURL != "" {
			pub, err := fileevents.NewJetStreamPublisher(natsURL)
			if err != nil {
				log.Fatalf("nats: %v", err)
			}
			defer func() { _ = pub.Close() }()
			eventPub = pub
		}
		filesStore := store.NewFilesStore(pool)
		jobs.StartExpiryWorker(context.Background(), filesStore, deleter, eventPub, logger)
		jobs.StartReferenceGCWorker(context.Background(), filesStore, deleter, fileReferenceGCIntervalFromEnv(), logger)
		lis, err := net.Listen("tcp", grpcListen)
		if err != nil {
			log.Fatalf("grpc listen: %v", err)
		}
		ordinaryObservation, protectedObservation := fileObservability(logger, metricsReg)
		options := append(ordinaryObservation, grpc.ChainUnaryInterceptor(principalgrpc.OrdinaryUnaryInterceptor()))
		grpcSrv = grpc.NewServer(options...)
		service := grpcsvc.New(grpcsvc.Deps{
			Files:                    filesStore,
			Presigner:                presigner,
			RevocableDownloadKey:     []byte(strings.TrimSpace(os.Getenv("FILE_DOWNLOAD_SIGNING_KEY"))),
			RevocableDownloadBaseURL: strings.TrimRight(strings.TrimSpace(os.Getenv("FILE_DOWNLOAD_BASE_URL")), "/"),
			Deleter:                  deleter,
			ChatGuard:                chatGuard,
			Reader:                   reader,
			Processor:                processor,
			Scanner:                  scanner,
			Entitlements:             entitlements,
			Events:                   eventPub,
		})
		revocableDownloadHTTP = grpcsvc.NewRevocableDownloadHTTPHandler(
			[]byte(strings.TrimSpace(os.Getenv("FILE_DOWNLOAD_SIGNING_KEY"))), nil, service)
		filev1.RegisterFileServiceServer(grpcSrv, service)
		if protectedRuntime != nil {
			protectedListener, err := net.Listen("tcp", principalConfig.ListenAddr)
			if err != nil {
				log.Fatalf("file principal listen: %v", err)
			}
			protectedOptions := append(protectedObservation, protectedRuntime.ServerOptions()...)
			protectedSrv = grpc.NewServer(protectedOptions...)
			filev1.RegisterFileServiceServer(protectedSrv, service)
			go func() {
				if err := protectedSrv.Serve(protectedListener); err != nil {
					log.Fatalf("file principal serve: %v", err)
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
		if principalEnabled {
			log.Fatal("DATABASE_URL required for protected File listener")
		}
		logger.Warn("DATABASE_URL not set; gRPC disabled (health only)")
	}

	httpMux := http.NewServeMux()
	if revocableDownloadHTTP != nil {
		httpMux.Handle("/api/v1/files/download/", revocableDownloadHTTP)
	}
	httpMux.Handle("/", voiceprom.MountMetricsOnHealth(healthHandler(serviceName), metricsReg))
	server := &http.Server{Addr: addr, Handler: httpserver.Wrap(httpMux, logger)}
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
		if err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	case <-stop:
		ctx, cancel := context.WithTimeout(context.Background(), runtimeconfig.ShutdownTimeoutFromEnv())
		defer cancel()
		if grpcSrv != nil {
			grpcSrv.GracefulStop()
		}
		if protectedSrv != nil {
			protectedSrv.GracefulStop()
		}
		if err := server.Shutdown(ctx); err != nil {
			log.Fatal(err)
		}
	}
}

func fileReferenceGCIntervalFromEnv() time.Duration {
	raw := strings.TrimSpace(os.Getenv("FILE_REFERENCE_GC_INTERVAL"))
	if raw == "" {
		return time.Minute
	}
	interval, err := time.ParseDuration(raw)
	if err != nil || interval <= 0 {
		log.Fatalf("FILE_REFERENCE_GC_INTERVAL must be a positive Go duration")
	}
	return interval
}

func fileObservability(logger *slog.Logger, registry *prometheus.Registry) ([]grpc.ServerOption, []grpc.ServerOption) {
	metrics := grpcmw.UnaryMetricsForRegistry(registry)
	ordinary := []grpc.ServerOption{grpc.ChainUnaryInterceptor(grpcmw.UnaryRecovery(logger), metrics, grpcmw.UnaryAccessLog(logger))}
	// Unverified request IDs are attacker-controlled. Protected denials export
	// bounded gRPC outcome metrics without logging headers or panic payloads.
	protected := []grpc.ServerOption{grpc.ChainUnaryInterceptor(grpcmw.UnaryRecovery(nil), metrics)}
	return ordinary, protected
}
