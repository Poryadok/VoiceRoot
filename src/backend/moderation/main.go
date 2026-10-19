package main

import (
	"context"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/google/uuid"
	"google.golang.org/grpc"

	"voice/backend/moderation/internal/authclient"
	"voice/backend/moderation/internal/automod"
	grpcsvc "voice/backend/moderation/internal/grpcsvc"
	"voice/backend/moderation/internal/intakeprincipal"
	"voice/backend/moderation/internal/mmclient"
	"voice/backend/moderation/internal/moderationevents"
	"voice/backend/moderation/internal/store"
	"voice/backend/moderation/internal/userclient"
	"voice/backend/pkg/analyticsevents"
	"voice/backend/pkg/grpcmw"
	"voice/backend/pkg/httpserver"
	"voice/backend/pkg/postgres"
	"voice/backend/pkg/principal"
	"voice/backend/pkg/runtimeconfig"

	moderationv1 "voice.app/voice/moderation/v1"
)

const serviceName = "moderation"

func audienceSizeFromEnv() int64 {
	if v := strings.TrimSpace(os.Getenv("MODERATION_PLATFORM_AUDIENCE_SIZE")); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			return n
		}
	}
	return 1000
}

func main() {
	logger := httpserver.NewLogger(serviceName)
	serviceContext, stopService := context.WithCancel(context.Background())
	defer stopService()
	authorityConfig, err := loadAutoModAuthorityConfig()
	if err != nil {
		log.Fatalf("automod authority configuration: %v", err)
	}
	var localIssuer *principal.Issuer
	var localJWKS *moderationJWKSRuntime
	_, keysConfigured := os.LookupEnv("MODERATION_PRINCIPAL_SIGNING_KEYS_DIR")
	_, activeKeyConfigured := os.LookupEnv("MODERATION_PRINCIPAL_ACTIVE_KID")
	if authorityConfig != nil || keysConfigured || activeKeyConfigured {
		issuer, publicKeys, err := loadModerationIssuerFromEnv()
		if err != nil {
			log.Fatalf("moderation issuer: %v", err)
		}
		localIssuer = issuer
		localJWKS, err = startModerationJWKSFromEnv(serviceContext, publicKeys)
		if err != nil {
			log.Fatalf("moderation private JWKS: %v", err)
		}
		defer localJWKS.Close()
	}
	var bootGate *automod.BootGate
	var authorityRuntime *autoModAuthorityRuntime
	if authorityConfig != nil {
		bootGate = automod.NewBootGate()
		defer bootGate.Close()
		// P0 discovery has no dependency on the domain DB connection. The store
		// is bound exactly once below, before any P1/P2 receipt is possible.
		authorityRuntime, err = newAutoModAuthorityRuntime(serviceContext, *authorityConfig, automod.NewStore(nil), localIssuer, bootGate)
		if err != nil {
			log.Fatalf("automod private authority: %v", err)
		}
		defer authorityRuntime.Close()
		bootID, generation := bootGate.Identity()
		if err = bootGate.Advance(automod.BootReceipt{BootID: bootID, Generation: generation, Phase: automod.BootLocalIssuer, ProofID: uuid.New()}); err != nil {
			log.Fatalf("automod local bootstrap receipt: %v", err)
		}
		go func() {
			select {
			case <-localJWKS.done:
				bootGate.Close()
			case <-serviceContext.Done():
			}
		}()
	}
	addr := ":8080"
	if v := os.Getenv("LISTEN_ADDR"); v != "" {
		addr = v
	}
	grpcAddr := ":9090"
	if v := strings.TrimSpace(os.Getenv("MODERATION_GRPC_LISTEN")); v != "" {
		grpcAddr = v
	}

	var grpcSrv *grpc.Server
	dbURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if dbURL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), runtimeconfig.PostgresConnectTimeoutFromEnv())
		pool, err := postgres.NewPool(ctx, dbURL)
		cancel()
		if err != nil {
			log.Fatalf("postgres: %v", err)
		}
		defer pool.Close()
		if authorityRuntime != nil {
			authorityRuntime.Store.Pool = pool
		}

		lis, err := net.Listen("tcp", grpcAddr)
		if err != nil {
			log.Fatalf("grpc listen: %v", err)
		}
		publicOptions := append(grpcmw.ServerOptions(logger), grpc.ChainUnaryInterceptor(intakeprincipal.OrdinaryUnary()))
		if bootGate != nil {
			publicOptions = append(publicOptions, grpc.ChainUnaryInterceptor(autoModPublicGate(bootGate)))
		}
		grpcSrv = grpc.NewServer(publicOptions...)
		svc := &grpcsvc.ModerationGRPC{
			Reports:              &store.ReportStore{Pool: pool},
			Sanctions:            &store.SanctionStore{Pool: pool},
			Appeals:              &store.AppealStore{Pool: pool},
			AuditLog:             &store.AuditLogStore{Pool: pool},
			AutoMod:              &store.AutoModStore{Pool: pool},
			PlatformAudienceSize: audienceSizeFromEnv(),
		}
		validationAuthority := reportValidationAuthority{Issuer: localIssuer}
		if authorityRuntime != nil {
			svc.Reports.Subjects = authorityRuntime.Store
			privacy := store.ReportPrivacyAcquire(func(ctx context.Context) (store.ReportPrivacyCheck, error) {
				check, err := bootGate.AcquireEffects(ctx)
				return store.ReportPrivacyCheck(check), err
			})
			svc.Reports.Privacy = privacy
			svc.ReportPrivacy = privacy
			svc.ReportAuthority = authorityRuntime.Auth
			validationAuthority.Subjects = authorityRuntime.Auth
			validationAuthority.Privacy = privacy
		}
		if authAddr := strings.TrimSpace(os.Getenv("AUTH_GRPC_ADDR")); authAddr != "" {
			if authClient, err := authclient.Dial(authAddr); err != nil {
				log.Fatalf("auth grpc: %v", err)
			} else {
				svc.Auth = authClient
			}
		}
		if userAddr := strings.TrimSpace(os.Getenv("USER_GRPC_ADDR")); userAddr != "" {
			if userClient, err := userclient.Dial(userAddr); err != nil {
				log.Fatalf("user grpc: %v", err)
			} else {
				svc.Users = userClient
			}
		}
		if mmAddr := strings.TrimSpace(os.Getenv("MATCHMAKING_GRPC_ADDR")); mmAddr != "" {
			if mmClient, err := mmclient.Dial(mmAddr); err != nil {
				log.Fatalf("matchmaking grpc: %v", err)
			} else {
				svc.Matchmaking = mmClient
			}
		}
		if natsURL := strings.TrimSpace(os.Getenv("NATS_URL")); natsURL != "" {
			if pub, err := analyticsevents.NewJetStreamPublisher(natsURL, "moderation"); err == nil {
				svc.Analytics = pub
				defer pub.Close()
			} else {
				logger.Warn("analytics publisher disabled", slog.Any("error", err))
			}
			if domainPub, err := moderationevents.NewJetStreamPublisher(natsURL); err == nil {
				svc.DomainEvents = domainPub
				defer func() { _ = domainPub.Close() }()
				logger.Info("moderation.events publisher enabled")
			} else {
				logger.Warn("moderation.events publisher unavailable", slog.Any("error", err))
			}
		}
		stopIntake, err := startReportIntake(context.Background(), svc, logger)
		if err != nil {
			log.Fatalf("report intake transport: %v", err)
		}
		defer stopIntake()
		stopValidation, err := startReportValidationWithAuthority(serviceContext, svc.Reports, logger, nil, validationAuthority)
		if err != nil {
			log.Fatalf("report validation runtime: %v", err)
		}
		defer stopValidation()
		moderationv1.RegisterModerationServiceServer(grpcSrv, svc)
		if bootGate == nil {
			go grpcsvc.RunTempBanExpirySweeper(serviceContext, svc, logger)
		}
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
		Addr:    addr,
		Handler: httpserver.Wrap(healthHandlerWithGate(serviceName, bootGate), logger),
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
