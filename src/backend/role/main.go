package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"

	"voice/backend/pkg/grpcmw"
	"voice/backend/pkg/httpserver"
	"voice/backend/pkg/postgres"
	voiceprom "voice/backend/pkg/promhttp"
	"voice/backend/pkg/runtimeconfig"
	grpcsvc "voice/backend/role/internal/grpcsvc"
	"voice/backend/role/internal/outboxdelivery"
	"voice/backend/role/internal/principalruntime"
	"voice/backend/role/internal/roleevents"
	"voice/backend/role/internal/store"
)

const serviceName = "role"

func main() {
	logger := httpserver.NewLogger(serviceName)
	roleIssuer, roleJWKS, issuerErr := loadRolePrincipalIssuerFromEnv()
	if issuerErr != nil {
		log.Fatalf("Role principal issuer: %v", issuerErr)
	}
	memberAuthority, closeMemberAuthority, memberAuthorityErr := loadMemberAuthorityClient(roleIssuer)
	if memberAuthorityErr != nil {
		log.Fatalf("Role membership authority: %v", memberAuthorityErr)
	}
	defer closeMemberAuthority()
	sessionFloor, closeSessionFloor, sessionFloorErr := loadRoleSessionFloor(roleIssuer)
	if sessionFloorErr != nil {
		log.Fatalf("Role Auth session floor: %v", sessionFloorErr)
	}
	defer closeSessionFloor()
	targetClients, closeTargets, targetErr := loadOverrideTargetClients(roleIssuer)
	if targetErr != nil {
		log.Fatalf("Role target clients: %v", targetErr)
	}
	defer closeTargets()
	metricsReg := prometheus.NewRegistry()
	runCtx, runCancel := context.WithCancel(context.Background())
	defer runCancel()
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
	if (memberAuthority != nil || sessionFloor != nil) && !principalEnabled {
		log.Fatal("Role authority clients require protected principal listener")
	}
	var grpcSrv, principalSrv *grpc.Server
	if dbURL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), runtimeconfig.PostgresConnectTimeoutFromEnv())
		pool, err := postgres.NewPool(ctx, dbURL)
		cancel()
		if err != nil {
			log.Fatalf("postgres: %v", err)
		}
		defer pool.Close()

		roleStore := &store.RoleStore{Pool: pool}
		stopRetention, err := startRoleDeletionRetention(roleStore, logger)
		if err != nil {
			log.Fatalf("Role lifecycle retention startup: %v", err)
		}
		defer stopRetention()
		var events roleevents.Publisher = roleevents.NoopPublisher{}
		if natsURL := strings.TrimSpace(os.Getenv("NATS_URL")); natsURL != "" {
			jsPub, err := roleevents.NewJetStreamPublisher(natsURL)
			if err != nil {
				log.Fatalf("nats jetstream publisher: %v", err)
			}
			defer func() { _ = jsPub.Close() }()
			jsPub.Logger = logger
			events = jsPub
			voicePolicy := outboxdelivery.NewVoicePolicyDispatcher(roleStore, jsPub)
			voicePolicy.OnError = func(err error) {
				logger.Warn("Role Voice policy invalidation delivery failed", slog.String("error", err.Error()))
			}
			go func() {
				if err := voicePolicy.Run(runCtx); err != nil && runCtx.Err() == nil {
					logger.Error("Role Voice policy invalidation delivery stopped", slog.String("error", err.Error()))
				}
			}()
		}

		lis, err := net.Listen("tcp", grpcListen)
		if err != nil {
			log.Fatalf("grpc listen: %v", err)
		}
		var principal *principalruntime.Runtime
		if principalEnabled {
			principal, err = principalruntime.NewWithSessionEpochChecker(context.Background(), principalConfig, sessionFloor)
			if err != nil {
				log.Fatalf("principal runtime: %v", err)
			}
			defer func() { _ = principal.Close() }()
			if principalConfig.AuthoritySourceEnabled {
				if err := principal.ActivateAuthoritySource(context.Background(), roleStore); err != nil {
					log.Fatalf("authority source activation: %v", err)
				}
			}
		}
		// Construct shared metrics once; both listeners use the same collectors.
		grpcSrv, principalSrv = newRoleGRPCServers(grpcmw.ServerOptions(logger, grpcmw.WithRegistry(metricsReg)), &grpcsvc.RoleGRPC{Store: roleStore, Events: events, OverrideTargets: targetClients, MemberAuthority: memberAuthority}, principal)
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
	var jwksServer *http.Server
	if len(roleJWKS.Keys) > 0 {
		certificate, err := tls.LoadX509KeyPair(strings.TrimSpace(os.Getenv("ROLE_PRINCIPAL_JWKS_TLS_CERT_FILE")), strings.TrimSpace(os.Getenv("ROLE_PRINCIPAL_JWKS_TLS_KEY_FILE")))
		if err != nil {
			log.Fatalf("Role principal JWKS TLS: %v", err)
		}
		addr := strings.TrimSpace(os.Getenv("ROLE_PRINCIPAL_JWKS_HTTPS_LISTEN"))
		if addr == "" {
			addr = ":9092"
		}
		jwksServer = &http.Server{Addr: addr, TLSConfig: &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{certificate}}, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/.well-known/jwks.json" {
				http.NotFound(w, r)
				return
			}
			if r.Method != http.MethodGet {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			w.Header().Set("Content-Type", "application/jwk-set+json")
			w.Header().Set("Cache-Control", "public, max-age=30")
			_ = json.NewEncoder(w).Encode(roleJWKS)
		})}
		httpserver.ApplyHTTPServerTimeouts(jwksServer)
		defer func() { _ = jwksServer.Close() }()
		go func() { errCh <- jwksServer.ListenAndServeTLS("", "") }()
	}
	logger.Info("HTTP listening", slog.String("addr", httpAddr))
	go func() { errCh <- server.ListenAndServe() }()

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
		shutdownRoleServers(ctx, grpcSrv, principalSrv)
		if err := server.Shutdown(ctx); err != nil {
			log.Fatal(err)
		}
	}
}
