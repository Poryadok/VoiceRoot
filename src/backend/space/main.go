package main

import (
	"context"
	"crypto/tls"
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
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	"voice/backend/pkg/authoritysource"
	"voice/backend/pkg/grpcclient"
	"voice/backend/pkg/grpcmw"
	"voice/backend/pkg/httpserver"
	authorityv1 "voice/backend/pkg/pb/voice/authority/v1"
	voiceprom "voice/backend/pkg/promhttp"
	"voice/backend/pkg/runtimeconfig"
	"voice/backend/pkg/socialprincipal"
	"voice/backend/space/internal/authctx"
	grpcsvc "voice/backend/space/internal/grpcsvc"
	"voice/backend/space/internal/lifecycleprincipal"
	"voice/backend/space/internal/s2s"
	"voice/backend/space/internal/spaceevents"
	"voice/backend/space/internal/store"
	"voice/backend/space/internal/subscriptionconsume"

	authv1 "voice.app/voice/auth/v1"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	userv1 "voice.app/voice/user/v1"
)

const (
	serviceName                         = "space"
	spaceMutationLockPoolMaxConns int32 = 8
)

func main() {
	logger := httpserver.NewLogger(serviceName)
	sourceConfig, sourceEnabled, err := authoritysource.LoadRuntimeConfig(authorityv1.AuthorityOwner_AUTHORITY_OWNER_SPACE, ":9097")
	if err != nil {
		log.Fatalf("Space authority source config: %v", err)
	}
	if sourceEnabled && strings.TrimSpace(os.Getenv("DATABASE_URL")) == "" {
		log.Fatal("Space authority source requires DATABASE_URL")
	}
	lifecycleConfig, lifecycleEnabled, err := loadLifecycleRuntimeConfig(os.Getenv)
	if err != nil {
		log.Fatalf("Space lifecycle config: %v", err)
	}
	if lifecycleEnabled && strings.TrimSpace(os.Getenv("DATABASE_URL")) == "" {
		log.Fatal("Space lifecycle requires DATABASE_URL")
	}
	publicConfig, publicEnabled, err := lifecycleprincipal.FromEnv()
	if err != nil {
		log.Fatalf("Space Gateway lifecycle config: %v", err)
	}
	if lifecycleEnabled && !publicEnabled {
		log.Fatal("Space lifecycle requires the protected Gateway listener")
	}
	if publicEnabled && strings.TrimSpace(os.Getenv("DATABASE_URL")) == "" {
		log.Fatal("Space Gateway lifecycle listener requires DATABASE_URL")
	}
	principalConfig, principalEnabled, err := socialprincipal.LoadFromEnv("space")
	if err != nil {
		log.Fatalf("space privacy principal config: %v", err)
	}
	var privacyRuntime *socialprincipal.Runtime
	if principalEnabled {
		if strings.TrimSpace(os.Getenv("DATABASE_URL")) == "" {
			log.Fatal("space privacy principal requires DATABASE_URL")
		}
		privacyRuntime, err = socialprincipal.New(context.Background(), principalConfig)
		if err != nil {
			log.Fatalf("space privacy principal: %v", err)
		}
		defer func() { _ = privacyRuntime.Close() }()
	}
	principalIssuer, publicJWKS, err := loadSpacePrincipalIssuerFromEnv()
	if err != nil {
		log.Fatalf("space principal issuer: %v", err)
	}
	metricsReg := prometheus.NewRegistry()
	ownershipOutboxAlertGauge := newOwnershipOutboxDeliveryAlertGauge(metricsReg)
	runCtx, runCancel := context.WithCancel(context.Background())
	defer runCancel()
	httpAddr := ":8080"
	if v := os.Getenv("LISTEN_ADDR"); v != "" {
		httpAddr = v
	}
	grpcListen := ":9090"
	if v := strings.TrimSpace(os.Getenv("SPACE_GRPC_LISTEN")); v != "" {
		grpcListen = v
	}

	dbURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	var grpcSrv *grpc.Server
	var publicServer *grpc.Server
	var sourceServer *grpc.Server
	var outboxRuntime *ownershipOutboxRuntime
	var recoveryRuntime *ownershipRecoveryRuntime
	var deletionRuntime *lifecycleRuntime
	if dbURL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), runtimeconfig.PostgresConnectTimeoutFromEnv())
		pool, err := pgxpool.New(ctx, dbURL)
		if err != nil {
			cancel()
			log.Fatalf("postgres: %v", err)
		}
		defer pool.Close()

		lockPoolConfig, err := pgxpool.ParseConfig(dbURL)
		if err != nil {
			cancel()
			log.Fatalf("postgres mutation lock config: %v", err)
		}
		// Advisory leases pin sessions, so they use an independently bounded
		// pool and cannot exhaust connections needed by SpaceStore queries. Its
		// DSN must use direct PostgreSQL or session pooling, not transaction mode.
		lockPoolConfig.MinConns = 0
		lockPoolConfig.MaxConns = spaceMutationLockPoolMaxConns
		mutationLockPool, err := pgxpool.NewWithConfig(ctx, lockPoolConfig)
		cancel()
		if err != nil {
			log.Fatalf("postgres mutation lock pool: %v", err)
		}
		defer mutationLockPool.Close()

		spaceStore := &store.SpaceStore{Pool: pool}
		go store.RunCommunityRosterExpiryWorker(runCtx, spaceStore, time.Second)
		natsURL := strings.TrimSpace(os.Getenv("NATS_URL"))
		var spaceEvents spaceevents.Publisher
		var jsPub *spaceevents.JetStreamPublisher
		if natsURL != "" {
			jsPub, err = spaceevents.NewJetStreamPublisher(natsURL)
			if err != nil {
				log.Fatalf("nats jetstream publisher: %v", err)
			}
			if err := jsPub.Validate(); err != nil {
				log.Fatalf("nats jetstream bootstrap: %v", err)
			}
			defer func() { _ = jsPub.Close() }()
			jsPub.Logger = logger
			spaceEvents = jsPub
		}

		lis, err := net.Listen("tcp", grpcListen)
		if err != nil {
			log.Fatalf("grpc listen: %v", err)
		}
		var roleClient rolev1.RoleServiceClient
		if roleAddr := strings.TrimSpace(os.Getenv("ROLE_GRPC_ADDR")); roleAddr != "" {
			rconn, err := grpc.NewClient(grpcclient.DialTarget(roleAddr), grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatalf("role grpc dial: %v", err)
			}
			defer func() { _ = rconn.Close() }()
			roleClient = rolev1.NewRoleServiceClient(rconn)
		}

		var ownershipRoleClient rolev1.RoleServiceClient
		if addr := strings.TrimSpace(os.Getenv("ROLE_PRINCIPAL_GRPC_ADDR")); addr != "" {
			if principalIssuer == nil {
				log.Fatal("Space principal issuer is required for Role ownership transport")
			}
			tlsConfig, err := ownershipRoleTLSFromEnv()
			if err != nil {
				log.Fatalf("role ownership TLS: %v", err)
			}
			conn, err := grpc.NewClient(grpcclient.DialTarget(addr), grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
			if err != nil {
				log.Fatalf("role ownership grpc: %v", err)
			}
			defer func() { _ = conn.Close() }()
			ownershipRoleClient = rolev1.NewRoleServiceClient(conn)
		} else if strings.TrimSpace(os.Getenv("ROLE_PRINCIPAL_TLS_CA_FILE")) != "" || strings.TrimSpace(os.Getenv("ROLE_PRINCIPAL_TLS_SERVER_NAME")) != "" || strings.TrimSpace(os.Getenv("SPACE_ROLE_CLIENT_CERT_FILE")) != "" || strings.TrimSpace(os.Getenv("SPACE_ROLE_CLIENT_KEY_FILE")) != "" {
			log.Fatal("ROLE_PRINCIPAL_GRPC_ADDR is required with Role ownership TLS settings")
		}

		var ownershipAuthClient authv1.AuthServiceClient
		if addr := strings.TrimSpace(os.Getenv("AUTH_PRINCIPAL_GRPC_ADDR")); addr != "" {
			if principalIssuer == nil {
				log.Fatal("Space principal issuer is required for Auth ownership transport")
			}
			tlsConfig, err := ownershipAuthTLSFromEnv()
			if err != nil {
				log.Fatalf("auth ownership TLS: %v", err)
			}
			conn, err := grpc.NewClient(grpcclient.DialTarget(addr), grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
			if err != nil {
				log.Fatalf("auth ownership grpc: %v", err)
			}
			defer func() { _ = conn.Close() }()
			ownershipAuthClient = authv1.NewAuthServiceClient(conn)
		} else if strings.TrimSpace(os.Getenv("AUTH_PRINCIPAL_TLS_CA_FILE")) != "" || strings.TrimSpace(os.Getenv("AUTH_PRINCIPAL_TLS_SERVER_NAME")) != "" {
			log.Fatal("AUTH_PRINCIPAL_GRPC_ADDR is required with Auth ownership TLS settings")
		}

		sharedOptions := grpcmw.ServerOptions(logger, grpcmw.WithRegistry(metricsReg))
		if sourceEnabled {
			var sourceRuntime *authoritysource.Runtime
			sourceServer, sourceRuntime, err = newSpaceAuthorityServer(runCtx, sharedOptions, sourceConfig, spaceStore)
			if err != nil {
				log.Fatalf("Space authority source activation: %v", err)
			}
			defer func() { _ = sourceRuntime.Close() }()
			sourceListener, err := net.Listen("tcp", sourceConfig.ListenAddr)
			if err != nil {
				log.Fatalf("Space authority source listen: %v", err)
			}
			defer sourceServer.Stop()
			go func() {
				if err := sourceServer.Serve(sourceListener); err != nil {
					log.Fatalf("Space authority source serve: %v", err)
				}
			}()
		}
		grpcOptions := append([]grpc.ServerOption{}, sharedOptions...)
		grpcOptions = append(grpcOptions, grpc.ChainUnaryInterceptor(
			lifecycleprincipal.OrdinaryUnary(),
			socialprincipal.OrdinaryUnaryInterceptor("space"),
			authctx.VerifiedServiceIdentityUnaryInterceptor(os.Getenv("SPACE_VOICE_S2S_TOKEN")),
			authctx.VerifiedGameIntegrationUnaryInterceptor(os.Getenv("SPACE_GAME_INTEGRATION_S2S_TOKEN")),
		))
		grpcSrv = grpc.NewServer(grpcOptions...)
		spaceSvc := &grpcsvc.SpaceGRPC{
			Store:             spaceStore,
			SpaceEvents:       spaceEvents,
			Roles:             roleClient,
			OwnershipRoles:    ownershipRoleClient,
			OwnershipAuth:     ownershipAuthClient,
			PrincipalIssuer:   principalIssuer,
			MutationLocker:    store.NewSpaceMutationLocker(mutationLockPool),
			SpaceCoMembership: &grpcsvc.StoreCoMembership{Store: spaceStore},
			Logger:            logger,
		}
		if userAddr := strings.TrimSpace(os.Getenv("USER_GRPC_ADDR")); userAddr != "" {
			uconn, err := grpc.NewClient(grpcclient.DialTarget(userAddr), grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatalf("user grpc: %v", err)
			}
			uconn.Connect()
			waitCtx, waitCancel := context.WithTimeout(context.Background(), grpcclient.DialTimeoutFromEnv())
			for {
				st := uconn.GetState()
				if st == connectivity.Ready {
					break
				}
				if st == connectivity.Shutdown {
					waitCancel()
					_ = uconn.Close()
					log.Fatalf("user grpc: unexpected shutdown")
				}
				if !uconn.WaitForStateChange(waitCtx, st) {
					waitCancel()
					_ = uconn.Close()
					log.Fatalf("user grpc dial: %v", context.Cause(waitCtx))
				}
			}
			waitCancel()
			defer func() { _ = uconn.Close() }()
			userClient := userv1.NewUserServiceClient(uconn)
			spaceSvc.Privacy = &s2s.GRPCUserPrivacy{Client: userClient}
			spaceSvc.ProfileAccounts = s2s.NewGRPCUserProfiles(uconn)
		}
		if socialAddr := strings.TrimSpace(os.Getenv("SOCIAL_GRPC_ADDR")); socialAddr != "" {
			sconn, err := grpc.NewClient(grpcclient.DialTarget(socialAddr), grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatalf("social grpc: %v", err)
			}
			defer func() { _ = sconn.Close() }()
			spaceSvc.Friends = s2s.NewGRPCSocialFriends(sconn)
			spaceSvc.Blocks = s2s.NewGRPCSocialBlocks(sconn)
		}
		if chatAddr := strings.TrimSpace(os.Getenv("CHAT_GRPC_ADDR")); chatAddr != "" {
			cconn, err := grpc.NewClient(grpcclient.DialTarget(chatAddr), grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatalf("chat grpc: %v", err)
			}
			defer func() { _ = cconn.Close() }()
			spaceSvc.Chats = grpcsvc.NewGRPCChatLookup(cconn)
		}
		if natsURL != "" {
			entitlements := &subscriptionconsume.SpaceStoreEntitlement{Store: spaceStore}
			entitlementConsumer, err := subscriptionconsume.Start(runCtx, natsURL, "space_subscription_entitlement", entitlements)
			if err != nil {
				log.Fatalf("space subscription consumer startup: %v", err)
			}
			defer entitlementConsumer.Close()
			go func() {
				if err := entitlementConsumer.Run(runCtx); err != nil && runCtx.Err() == nil {
					log.Fatalf("space subscription consumer stopped: %v", err)
				}
			}()
			logger.Info("space subscription entitlement consumer enabled")
		}
		outboxRuntime = startOwnershipOutboxRuntime(runCtx, ownershipOutboxRuntimeConfigFromEnv(), ownershipOutboxRuntimeDependencies{
			store: spaceStore, transport: jsPub, alertGauge: ownershipOutboxAlertGauge, logger: logger,
		})
		if outboxRuntime != nil {
			defer outboxRuntime.Stop()
		}
		recoveryRuntime = startOwnershipRecoveryRuntime(runCtx, spaceStore, spaceSvc, logger)
		if recoveryRuntime != nil {
			defer recoveryRuntime.Stop()
		}
		if lifecycleEnabled {
			deletionRuntime, err = newLifecycleRuntime(runCtx, lifecycleConfig, spaceStore, spaceSvc, jsPub, logger)
			if err != nil {
				log.Fatalf("Space lifecycle runtime: %v", err)
			}
			defer deletionRuntime.Stop()
		}
		spacev1.RegisterSpaceServiceServer(grpcSrv, spaceSvc)
		if publicEnabled {
			publicRuntime, err := lifecycleprincipal.New(runCtx, publicConfig)
			if err != nil {
				log.Fatalf("Space Gateway lifecycle runtime: %v", err)
			}
			defer func() { _ = publicRuntime.Close() }()
			publicListener, err := net.Listen("tcp", publicConfig.Listen)
			if err != nil {
				log.Fatalf("Space Gateway lifecycle listen: %v", err)
			}
			options := append([]grpc.ServerOption{}, sharedOptions...)
			options = append(options, publicRuntime.ServerOptions()...)
			publicServer = grpc.NewServer(options...)
			spacev1.RegisterSpaceServiceServer(publicServer, &grpcsvc.PublicLifecycleSpace{SpaceGRPC: spaceSvc})
			go func() {
				logger.Info("Gateway lifecycle principal listener started", slog.String("addr", publicConfig.Listen))
				if err := publicServer.Serve(publicListener); err != nil {
					logger.Error("Gateway lifecycle principal listener stopped", slog.String("error", err.Error()))
				}
			}()
		}
		if privacyRuntime != nil {
			privacyListener, err := net.Listen("tcp", principalConfig.ListenAddr)
			if err != nil {
				log.Fatalf("space privacy principal listen: %v", err)
			}
			privacyOptions := append([]grpc.ServerOption{}, sharedOptions...)
			privacyOptions = append(privacyOptions, privacyRuntime.ServerOptions()...)
			privacyServer := grpc.NewServer(privacyOptions...)
			grpcsvc.RegisterSocialPrivacyServer(privacyServer, spaceSvc)
			defer privacyServer.Stop()
			go func() {
				if err := privacyServer.Serve(privacyListener); err != nil {
					log.Fatalf("space privacy principal serve: %v", err)
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
		Handler: httpserver.Wrap(voiceprom.MountMetricsOnHealth(spaceHTTPHandler(serviceName, principalJWKS{}), metricsReg), logger),
	}
	httpserver.ApplyHTTPServerTimeouts(server)
	var jwksServer *http.Server
	errCh := make(chan error, 2)
	if len(publicJWKS.Keys) > 0 {
		certificate, err := tls.LoadX509KeyPair(strings.TrimSpace(os.Getenv("SPACE_PRINCIPAL_JWKS_TLS_CERT_FILE")), strings.TrimSpace(os.Getenv("SPACE_PRINCIPAL_JWKS_TLS_KEY_FILE")))
		if err != nil {
			log.Fatalf("space principal JWKS TLS: %v", err)
		}
		jwksAddr := strings.TrimSpace(os.Getenv("SPACE_PRINCIPAL_JWKS_HTTPS_LISTEN"))
		if jwksAddr == "" {
			jwksAddr = ":8443"
		}
		jwksServer = &http.Server{Addr: jwksAddr, Handler: spaceJWKSHandler(publicJWKS), TLSConfig: &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{certificate}}}
		httpserver.ApplyHTTPServerTimeouts(jwksServer)
		go func() { errCh <- jwksServer.ListenAndServeTLS("", "") }()
		logger.Info("space principal JWKS listening", slog.String("addr", jwksAddr))
	}
	logger.Info("listening", slog.String("addr", httpAddr))
	go func() {
		errCh <- server.ListenAndServe()
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	select {
	case err := <-errCh:
		runCancel()
		outboxRuntime.Stop()
		recoveryRuntime.Stop()
		deletionRuntime.Stop()
		if err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	case <-stop:
		runCancel()
		outboxRuntime.Stop()
		recoveryRuntime.Stop()
		deletionRuntime.Stop()
		ctx, cancel := context.WithTimeout(context.Background(), runtimeconfig.ShutdownTimeoutFromEnv())
		defer cancel()
		if grpcSrv != nil {
			grpcSrv.GracefulStop()
		}
		if publicServer != nil {
			publicServer.GracefulStop()
		}
		if sourceServer != nil {
			sourceServer.Stop()
		}
		if err := server.Shutdown(ctx); err != nil {
			log.Fatal(err)
		}
		if jwksServer != nil {
			if err := jwksServer.Shutdown(ctx); err != nil {
				log.Fatal(err)
			}
		}
	}
}
