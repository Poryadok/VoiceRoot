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
	"google.golang.org/grpc/credentials/insecure"

	"voice/backend/pkg/grpcclient"
	"voice/backend/pkg/grpcmw"
	"voice/backend/pkg/httpserver"
	voiceprom "voice/backend/pkg/promhttp"
	"voice/backend/pkg/runtimeconfig"
	grpcsvc "voice/backend/social/internal/grpcsvc"
	socials2s "voice/backend/social/internal/s2s"
	"voice/backend/social/internal/socialevents"
	"voice/backend/social/internal/store"

	socialv1 "voice.app/voice/social/v1"
)

const serviceName = "social"

func main() {
	logger := httpserver.NewLogger(serviceName)
	metricsReg := prometheus.NewRegistry()
	principalRuntime, err := loadSocialPrincipalRuntime()
	if err != nil {
		log.Fatalf("social principal configuration: %v", err)
	}
	defer principalRuntime.Close()
	errCh := make(chan error, 2)
	if principalRuntime.JWKS != nil {
		listener, err := net.Listen("tcp", principalRuntime.JWKS.Addr)
		if err != nil {
			log.Fatalf("social principal JWKS listen: %v", err)
		}
		go func() { errCh <- principalRuntime.JWKS.ServeTLS(listener, "", "") }()
	}
	httpAddr := ":8080"
	if v := os.Getenv("LISTEN_ADDR"); v != "" {
		httpAddr = v
	}
	grpcListen := ":9090"
	if v := strings.TrimSpace(os.Getenv("SOCIAL_GRPC_LISTEN")); v != "" {
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

		var privacy grpcsvc.FriendRequestPrivacyChecker
		var phoneSearchPrivacy grpcsvc.PhoneSearchPrivacyChecker
		var phoneHashes grpcsvc.PhoneHashLookup
		var spaceCoMembership grpcsvc.SpaceCoMembershipChecker
		var accountProfiles grpcsvc.AccountProfilesResolver
		var profileAccounts grpcsvc.ProfileAccountsResolver
		if principalRuntime.User != nil {
			privacy = principalRuntime.User
			phoneSearchPrivacy = principalRuntime.User
		}
		if principalRuntime.Space != nil {
			spaceCoMembership = principalRuntime.Space
		}
		if userAddr := strings.TrimSpace(os.Getenv("USER_GRPC_ADDR")); userAddr != "" {
			uconn, err := grpc.NewClient(grpcclient.DialTarget(userAddr), grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatalf("user grpc: %v", err)
			}
			defer func() { _ = uconn.Close() }()
			accountProfiles = socials2s.NewGRPCAccountProfiles(uconn)
			profileAccounts = socials2s.NewGRPCProfileAccounts(uconn)
		}
		if authAddr := strings.TrimSpace(os.Getenv("AUTH_GRPC_ADDR")); authAddr != "" {
			aconn, err := grpc.NewClient(grpcclient.DialTarget(authAddr), grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatalf("auth grpc: %v", err)
			}
			defer func() { _ = aconn.Close() }()
			phoneHashes = socials2s.NewGRPCAuthPhoneHashLookup(aconn)
		}

		lis, err := net.Listen("tcp", grpcListen)
		if err != nil {
			log.Fatalf("grpc listen: %v", err)
		}
		grpcSrv = grpc.NewServer(grpcmw.ServerOptions(logger, grpcmw.WithRegistry(metricsReg))...)
		socialSvc := &grpcsvc.SocialGRPC{
			Friends:            &store.FriendshipStore{Pool: pool},
			Blocks:             &store.BlockStore{Pool: pool},
			Contacts:           &store.ContactStore{Pool: pool},
			Privacy:            privacy,
			PhoneSearchPrivacy: phoneSearchPrivacy,
			PhoneHashes:        phoneHashes,
			SpaceCoMembership:  spaceCoMembership,
			AccountProfiles:    accountProfiles,
			ProfileAccounts:    profileAccounts,
		}
		if natsURL := strings.TrimSpace(os.Getenv("NATS_URL")); natsURL != "" {
			if pub, err := socialevents.NewJetStreamPublisher(natsURL); err == nil {
				socialSvc.Events = pub
				defer func() { _ = pub.Close() }()
				logger.Info("social.events publisher enabled")
			} else {
				logger.Warn("social.events publisher unavailable", slog.Any("error", err))
			}
		}
		socialv1.RegisterSocialServiceServer(grpcSrv, socialSvc)
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
		if principalRuntime.JWKS != nil {
			if err := principalRuntime.JWKS.Shutdown(ctx); err != nil {
				logger.Error("social principal JWKS shutdown", slog.Any("error", err))
			}
		}
		if grpcSrv != nil {
			grpcSrv.GracefulStop()
		}
		if err := server.Shutdown(ctx); err != nil {
			log.Fatal(err)
		}
	}
}
