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
	"google.golang.org/grpc"

	"voice/backend/pkg/grpcmw"
	"voice/backend/pkg/httpserver"
	voiceprom "voice/backend/pkg/promhttp"
	"voice/backend/pkg/runtimeconfig"
	grpcsvc "voice/backend/role/internal/grpcsvc"
	"voice/backend/role/internal/principalruntime"
	"voice/backend/role/internal/roleevents"
	"voice/backend/role/internal/store"
)

const serviceName = "role"

func main() {
	logger := httpserver.NewLogger(serviceName)
	metricsReg := prometheus.NewRegistry()
	httpAddr := ":8080"
	if v := os.Getenv("LISTEN_ADDR"); v != "" {
		httpAddr = v
	}
	grpcListen := ":9090"
	if v := strings.TrimSpace(os.Getenv("ROLE_GRPC_LISTEN")); v != "" {
		grpcListen = v
	}

	dbURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	principalConfig, principalEnabled, err := principalruntime.LoadFromEnv()
	if err != nil {
		log.Fatalf("principal config: %v", err)
	}
	if principalEnabled && dbURL == "" {
		log.Fatal("principal listener requires DATABASE_URL")
	}
	var grpcSrv, principalSrv *grpc.Server
	if dbURL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), runtimeconfig.PostgresConnectTimeoutFromEnv())
		pool, err := pgxpool.New(ctx, dbURL)
		cancel()
		if err != nil {
			log.Fatalf("postgres: %v", err)
		}
		defer pool.Close()

		roleStore := &store.RoleStore{Pool: pool}
		var events roleevents.Publisher = roleevents.NoopPublisher{}
		if natsURL := strings.TrimSpace(os.Getenv("NATS_URL")); natsURL != "" {
			jsPub, err := roleevents.NewJetStreamPublisher(natsURL)
			if err != nil {
				log.Fatalf("nats jetstream publisher: %v", err)
			}
			defer func() { _ = jsPub.Close() }()
			jsPub.Logger = logger
			events = jsPub
		}

		lis, err := net.Listen("tcp", grpcListen)
		if err != nil {
			log.Fatalf("grpc listen: %v", err)
		}
		var principal *principalruntime.Runtime
		if principalEnabled {
			principal, err = principalruntime.New(context.Background(), principalConfig)
			if err != nil {
				log.Fatalf("principal runtime: %v", err)
			}
			defer func() { _ = principal.Close() }()
		}
		// Construct shared metrics once; both listeners use the same collectors.
		grpcSrv, principalSrv = newRoleGRPCServers(grpcmw.ServerOptions(logger, grpcmw.WithRegistry(metricsReg)), &grpcsvc.RoleGRPC{Store: roleStore, Events: events}, principal)
		if principalSrv != nil {
			protectedListener, err := net.Listen("tcp", principalConfig.ListenAddr)
			if err != nil {
				log.Fatalf("principal grpc listen: %v", err)
			}
			go func() {
				logger.Info("ownership gRPC TLS listening", slog.String("addr", principalConfig.ListenAddr))
				if err := principalSrv.Serve(protectedListener); err != nil {
					log.Fatalf("principal grpc serve: %v", err)
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

	server := &http.Server{
		Addr:    httpAddr,
		Handler: httpserver.Wrap(voiceprom.MountMetricsOnHealth(healthHandler(serviceName), metricsReg), logger),
	}
	httpserver.ApplyHTTPServerTimeouts(server)
	errCh := make(chan error, 1)
	logger.Info("HTTP listening", slog.String("addr", httpAddr))
	go func() { errCh <- server.ListenAndServe() }()

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
		shutdownRoleServers(ctx, grpcSrv, principalSrv)
		if err := server.Shutdown(ctx); err != nil {
			log.Fatal(err)
		}
	}
}
