package main

import (
	"context"
	"errors"
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

	"voice/backend/pkg/authoritysource"
	"voice/backend/pkg/grpcclient"
	"voice/backend/pkg/grpcmw"
	"voice/backend/pkg/httpserver"
	authorityv1 "voice/backend/pkg/pb/voice/authority/v1"
	"voice/backend/pkg/postgres"
	voiceprom "voice/backend/pkg/promhttp"
	"voice/backend/pkg/releasepolicy"
	"voice/backend/pkg/runtimeconfig"
	"voice/backend/pkg/socialprincipal"
	grpcsvc "voice/backend/user/internal/grpcsvc"
	"voice/backend/user/internal/profilemutation"
	"voice/backend/user/internal/r2avatar"
	"voice/backend/user/internal/searchprojection"
	"voice/backend/user/internal/sourcecustody"
	"voice/backend/user/internal/store"
	"voice/backend/user/internal/userevents"

	userv1 "voice.app/voice/user/v1"
)

const serviceName = "user"

var buildSourceSHA string

// waitForRequiredGRPCReady establishes a required S2S connection before this
// service exposes its health endpoint. Lazy gRPC connections otherwise make a
// healthy User instance fail closed on its first deleted-account lookup.
func waitForRequiredGRPCReady(ctx context.Context, conn *grpc.ClientConn) error {
	if conn == nil {
		return errors.New("required grpc connection is nil")
	}
	conn.Connect()
	return grpcclient.WaitForReady(ctx, conn)
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := runUser(ctx); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func runUser(parent context.Context) error {
	startup := newUserStartup(parent)
	defer startup.Close()
	runCtx := startup.ctx
	logger := httpserver.NewLogger(serviceName)
	releaseRuntime, err := releasepolicy.LoadOwnerRuntime(os.LookupEnv, buildSourceSHA, os.ReadFile)
	if err != nil {
		return fmt.Errorf("user release policy: %v", err)
	}
	if releaseRuntime.Policy().Profile() == releasepolicy.Alpha && (strings.TrimSpace(os.Getenv("DATABASE_URL")) == "" || strings.TrimSpace(os.Getenv("USER_REDIS_ADDR")) == "") {
		return errors.New("user alpha requires PostgreSQL and presence Redis")
	}
	captureEnabled, err := validateSpaceCaptureConfig()
	if err != nil {
		return fmt.Errorf("user Space capture principal config: %w", err)
	}
	if !captureEnabled && hasOrphanCaptureRuntimeConfig() {
		return errors.New("user Space capture principal config: capture listener required")
	}
	var captureRuntime *spaceCaptureRuntime
	var capturePrincipalConfig socialprincipal.Config
	if captureEnabled {
		cfg, configErr := loadSpaceCaptureRuntimeConfig(os.LookupEnv)
		if configErr != nil || sourcecustody.ValidateUserSourceURL(os.Getenv("DATABASE_URL")) != nil {
			return errors.New("qualified User capture source unavailable")
		}
		capturePrincipalConfig, _, err = socialprincipal.LoadFromEnvWithAudience("user", "space-account-capture", "USER_SPACE_CAPTURE_PRINCIPAL_", strings.TrimSpace(os.Getenv("USER_SPACE_CAPTURE_PRINCIPAL_GRPC_LISTEN")))
		if err != nil {
			return fmt.Errorf("user Space capture principal config: %w", err)
		}
		captureRuntime, err = startSpaceCaptureRuntime(runCtx, cfg)
		if err != nil {
			return errors.New("qualified User capture source unavailable")
		}
		defer captureRuntime.Close()
		go func() {
			select {
			case <-captureRuntime.jwks.Done():
				startup.fail(errors.New("user capture JWKS stopped"))
			case <-runCtx.Done():
			}
		}()
		if captureRuntime.waitAdmission(runCtx) != nil {
			return errors.New("qualified User capture source unavailable")
		}
	}
	sourceConfig, sourceEnabled, err := authoritysource.LoadRuntimeConfig(authorityv1.AuthorityOwner_AUTHORITY_OWNER_USER, ":9097")
	if err != nil {
		return fmt.Errorf("user authority source config: %v", err)
	}
	if sourceEnabled && strings.TrimSpace(os.Getenv("DATABASE_URL")) == "" {
		return errors.New("user authority source requires DATABASE_URL")
	}
	principalConfig, principalEnabled, err := socialprincipal.LoadFromEnv("user")
	if err != nil {
		return fmt.Errorf("user privacy principal config: %v", err)
	}
	filePrincipalConfig, filePrincipalEnabled, err := socialprincipal.LoadFromEnvWithPrefix("file", "USER_FILE_PRINCIPAL_", ":9092")
	if err != nil {
		return fmt.Errorf("user file ownership principal config: %v", err)
	}
	searchPrincipalConfig, searchPrincipalEnabled, err := socialprincipal.LoadFromEnvWithAudience("user", "search", "USER_SEARCH_PRINCIPAL_", ":9093")
	if err != nil {
		return fmt.Errorf("user search projection principal config: %v", err)
	}
	authPrincipalConfig, authPrincipalEnabled, err := socialprincipal.LoadFromEnvWithAudience("user", "auth", "USER_AUTH_PRINCIPAL_", ":9094")
	if err != nil {
		return fmt.Errorf("user SDK conversion principal config: %v", err)
	}
	admissionConfig, admissionEnabled, err := socialprincipal.LoadFromEnvWithAudience("user", "auth-admission", "USER_ADMISSION_PRINCIPAL_", ":9097")
	if err != nil {
		return fmt.Errorf("user admission principal config: %v", err)
	}
	notificationPresenceConfig, notificationPresenceEnabled, err := socialprincipal.LoadFromEnvWithAudience("user", "notification", "USER_NOTIFICATION_PRINCIPAL_", ":9095")
	if err != nil {
		return fmt.Errorf("user Notification presence principal config: %v", err)
	}
	notificationPrimaryConfig, notificationPrimaryEnabled, err := socialprincipal.LoadFromEnvWithAudience("user", "notification-primary", "USER_NOTIFICATION_PRIMARY_PRINCIPAL_", ":9098")
	if err != nil {
		return fmt.Errorf("user Notification primary principal config: %v", err)
	}
	messagingPresenceConfig, messagingPresenceEnabled, err := socialprincipal.LoadFromEnvWithAudience("user", "messaging", "USER_MESSAGING_PRINCIPAL_", ":9096")
	if err != nil {
		return fmt.Errorf("user Messaging scheduled presence principal config: %v", err)
	}
	catalogConfig, catalogEnabled, err := socialprincipal.LoadFromEnvWithAudience("user", "catalog", "USER_CATALOG_PRINCIPAL_", ":9098")
	if err != nil {
		return fmt.Errorf("user catalog actor principal config: %v", err)
	}
	var catalogRuntime *socialprincipal.Runtime
	if catalogEnabled {
		if strings.TrimSpace(os.Getenv("DATABASE_URL")) == "" {
			return errors.New("user catalog actor principal requires DATABASE_URL")
		}
		catalogRuntime, err = socialprincipal.New(runCtx, catalogConfig)
		if err != nil {
			return fmt.Errorf("user catalog actor principal: %v", err)
		}
		defer func() { _ = catalogRuntime.Close() }()
	}
	searchProjectionCursorKey, err := searchprojection.CursorKeyFromEnv(searchPrincipalEnabled, os.Getenv)
	if err != nil {
		return fmt.Errorf("user search projection cursor config: %v", err)
	}
	accountDeletionConfig, err := accountDeletionConsumerConfigFromEnv(os.Getenv)
	if err != nil {
		return fmt.Errorf("account deletion consumer config: %v", err)
	}
	var privacyRuntime *socialprincipal.Runtime
	if principalEnabled {
		if strings.TrimSpace(os.Getenv("DATABASE_URL")) == "" {
			return errors.New("user privacy principal requires DATABASE_URL")
		}
		privacyRuntime, err = socialprincipal.New(runCtx, principalConfig)
		if err != nil {
			return fmt.Errorf("user privacy principal: %v", err)
		}
		defer func() { _ = privacyRuntime.Close() }()
	}
	var fileOwnershipRuntime *socialprincipal.Runtime
	if filePrincipalEnabled {
		if strings.TrimSpace(os.Getenv("DATABASE_URL")) == "" {
			return errors.New("user file ownership principal requires DATABASE_URL")
		}
		fileOwnershipRuntime, err = socialprincipal.New(runCtx, filePrincipalConfig)
		if err != nil {
			return fmt.Errorf("user file ownership principal: %v", err)
		}
		defer func() { _ = fileOwnershipRuntime.Close() }()
	}
	var searchProjectionRuntime *socialprincipal.Runtime
	if searchPrincipalEnabled {
		if strings.TrimSpace(os.Getenv("DATABASE_URL")) == "" {
			return errors.New("user search projection principal requires DATABASE_URL")
		}
		searchProjectionRuntime, err = socialprincipal.New(runCtx, searchPrincipalConfig)
		if err != nil {
			return fmt.Errorf("user search projection principal: %v", err)
		}
		defer func() { _ = searchProjectionRuntime.Close() }()
	}
	var admissionRuntime *socialprincipal.Runtime
	if admissionEnabled {
		if strings.TrimSpace(os.Getenv("DATABASE_URL")) == "" {
			return errors.New("user admission principal requires DATABASE_URL")
		}
		admissionRuntime, err = socialprincipal.New(runCtx, admissionConfig)
		if err != nil {
			return fmt.Errorf("user admission principal: %v", err)
		}
		defer func() { _ = admissionRuntime.Close() }()
	}
	var authSdkRuntime *socialprincipal.Runtime
	if authPrincipalEnabled {
		if strings.TrimSpace(os.Getenv("DATABASE_URL")) == "" {
			return errors.New("user SDK conversion principal requires DATABASE_URL")
		}
		authSdkRuntime, err = socialprincipal.New(runCtx, authPrincipalConfig)
		if err != nil {
			return fmt.Errorf("user SDK conversion principal: %v", err)
		}
		defer func() { _ = authSdkRuntime.Close() }()
	}
	var notificationPresenceRuntime *socialprincipal.Runtime
	if notificationPresenceEnabled {
		if strings.TrimSpace(os.Getenv("DATABASE_URL")) == "" {
			return errors.New("user Notification presence principal listener requires DATABASE_URL")
		}
		notificationPresenceRuntime, err = socialprincipal.New(runCtx, notificationPresenceConfig)
		if err != nil {
			return fmt.Errorf("user Notification presence principal: %v", err)
		}
		defer func() { _ = notificationPresenceRuntime.Close() }()
	}
	var notificationPrimaryRuntime *socialprincipal.Runtime
	if notificationPrimaryEnabled {
		if strings.TrimSpace(os.Getenv("DATABASE_URL")) == "" {
			return errors.New("user Notification primary principal listener requires DATABASE_URL")
		}
		notificationPrimaryRuntime, err = socialprincipal.New(runCtx, notificationPrimaryConfig)
		if err != nil {
			return fmt.Errorf("user Notification primary principal: %v", err)
		}
		defer func() { _ = notificationPrimaryRuntime.Close() }()
	}
	var messagingPresenceRuntime *socialprincipal.Runtime
	if messagingPresenceEnabled {
		if strings.TrimSpace(os.Getenv("DATABASE_URL")) == "" {
			return errors.New("user Messaging scheduled presence principal listener requires DATABASE_URL")
		}
		messagingPresenceRuntime, err = socialprincipal.New(runCtx, messagingPresenceConfig)
		if err != nil {
			return fmt.Errorf("user Messaging scheduled presence principal: %v", err)
		}
		defer func() { _ = messagingPresenceRuntime.Close() }()
	}
	metricsReg := prometheus.NewRegistry()
	httpAddr := ":8080"
	if v := os.Getenv("LISTEN_ADDR"); v != "" {
		httpAddr = v
	}
	grpcAddr := ":9090"
	if v := os.Getenv("USER_GRPC_ADDR"); v != "" {
		grpcAddr = v
	}

	dbURL := os.Getenv("DATABASE_URL")
	var pool *pgxpool.Pool
	if dbURL != "" {
		ctx, cancel := context.WithTimeout(runCtx, runtimeconfig.PostgresConnectTimeoutFromEnv())
		var p *pgxpool.Pool
		if captureRuntime != nil {
			p, err = sourcecustody.OpenUserSourcePool(ctx, dbURL)
		} else {
			p, err = postgres.NewPool(ctx, dbURL)
		}
		cancel()
		if err != nil {
			return fmt.Errorf("postgres: %v", err)
		}
		pool = p
		defer pool.Close()
	}

	if pool != nil {
		// Every User endpoint and consumer shares the same profile writer.
		// Capture startup can then install its guarded writer consistently.
		profiles := store.NewProfileStore(pool)
		var captureAuthority grpcsvc.SpaceAccountCaptureAuthority
		if captureRuntime != nil {
			guarded, capture, bindErr := captureRuntime.bind(runCtx, pool)
			if bindErr != nil {
				return errors.New("qualified User capture source unavailable")
			}
			profiles = guarded
			captureAuthority, err = grpcsvc.NewCustodiedSpaceAccountCaptureAuthority(capture.CaptureProfiles)
			if err != nil {
				return errors.New("qualified User capture source unavailable")
			}
		}
		var blocks grpcsvc.AccountBlockChecker
		var socialGraph grpcsvc.SocialGraphChecker
		var spaceCoMembership grpcsvc.SpaceCoMembershipChecker
		var deletedAccounts grpcsvc.DeletedAccountsChecker
		if socialAddr := strings.TrimSpace(os.Getenv("SOCIAL_GRPC_ADDR")); socialAddr != "" {
			sconn, err := grpc.NewClient(grpcclient.DialTarget(socialAddr),
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			)
			if err != nil {
				return fmt.Errorf("social grpc: %v", err)
			}
			sconn.Connect()
			waitCtx, waitCancel := context.WithTimeout(runCtx, grpcclient.DialTimeoutFromEnv())
			for {
				st := sconn.GetState()
				if st == connectivity.Ready {
					break
				}
				if st == connectivity.Shutdown {
					waitCancel()
					_ = sconn.Close()
					return fmt.Errorf("social grpc: unexpected shutdown")
				}
				if !sconn.WaitForStateChange(waitCtx, st) {
					waitCancel()
					_ = sconn.Close()
					return fmt.Errorf("social grpc dial: %v", context.Cause(waitCtx))
				}
			}
			waitCancel()
			defer func() { _ = sconn.Close() }()
			blocks = grpcsvc.NewSocialGRPCBlocks(sconn)
			socialGraph = grpcsvc.NewSocialGRPCGraph(sconn)
		}
		var spaceConn *grpc.ClientConn
		if spaceAddr := strings.TrimSpace(os.Getenv("SPACE_GRPC_ADDR")); spaceAddr != "" {
			spconn, err := grpc.NewClient(grpcclient.DialTarget(spaceAddr), grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				return fmt.Errorf("space grpc: %v", err)
			}
			defer func() { _ = spconn.Close() }()
			spaceConn = spconn
			spaceCoMembership = grpcsvc.NewSpaceGRPCCoMembership(spconn)
		}
		authAddr := strings.TrimSpace(os.Getenv("AUTH_GRPC_ADDR"))
		if authAddr == "" {
			return errors.New("AUTH_GRPC_ADDR is required for deleted-account visibility")
		}
		authConn, err := grpc.NewClient(grpcclient.DialTarget(authAddr),
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			return fmt.Errorf("auth grpc: %v", err)
		}
		waitCtx, waitCancel := context.WithTimeout(runCtx, grpcclient.DialTimeoutFromEnv())
		if err := waitForRequiredGRPCReady(waitCtx, authConn); err != nil {
			waitCancel()
			_ = authConn.Close()
			return fmt.Errorf("auth grpc dial: %v", err)
		}
		waitCancel()
		defer func() { _ = authConn.Close() }()
		deletedAccounts = grpcsvc.NewAuthGRPCDeletedAccounts(authConn)

		var presence *store.PresenceStore
		var profileMutations grpcsvc.ProfileMutationLimiter = profilemutation.New(nil)
		if redisAddr := strings.TrimSpace(os.Getenv("USER_REDIS_ADDR")); redisAddr != "" {
			rdb := redis.NewClient(&redis.Options{Addr: redisAddr})
			defer func() { _ = rdb.Close() }()
			pctx, cancel := context.WithTimeout(runCtx, 5*time.Second)
			err := rdb.Ping(pctx).Err()
			cancel()
			if err != nil {
				return fmt.Errorf("redis ping: %v", err)
			}
			presence = store.NewDurablePresenceStore(rdb, store.NewLastSeenStore(pool))
			profileMutations = profilemutation.New(rdb)
		}

		var avatarPresigner grpcsvc.AvatarPresigner
		avatarPublicBase := ""
		r2cfg := r2avatar.EnvConfigFromOSEnv()
		if r2cfg.Endpoint != "" {
			p, err := r2avatar.NewS3R2PutPresigner(r2cfg)
			if err != nil {
				logger.Warn("USER_R2_* set but presigner init failed (avatar upload disabled)", slog.String("error", err.Error()))
			} else {
				avatarPresigner = p
				avatarPublicBase = strings.TrimSpace(r2cfg.PublicBaseURL)
			}
		}

		var events grpcsvc.UserEventsPublisher
		natsURL := strings.TrimSpace(os.Getenv("NATS_URL"))
		if natsURL != "" {
			pub, err := userevents.NewJetStreamPublisher(natsURL)
			if err != nil {
				return fmt.Errorf("nats jetstream publisher: %v", err)
			}
			pub.Logger = logger
			defer func() { _ = pub.Close() }()
			events = pub
		}

		dnsResolver := grpcsvc.DNSResolverFromEnv()
		if dnsResolver != nil {
			logger.Info("USER_DNS_STUB_URL set; org verification uses HTTP TXT fixture")
		}

		sharedOptions := grpcmw.ServerOptions(logger, grpcmw.WithRegistry(metricsReg))
		sharedOptions = append(sharedOptions, startup.options()...)

		ordinaryOptions := append([]grpc.ServerOption{}, sharedOptions...)
		ordinaryOptions = append(ordinaryOptions, grpc.ChainUnaryInterceptor(socialprincipal.OrdinaryUnaryInterceptor("user")))
		srv := grpc.NewServer(ordinaryOptions...)
		userSvc := &grpcsvc.UserGRPC{
			ReleasePolicy:       releaseRuntime.Policy(),
			Profiles:            profiles,
			ProfileMutations:    profileMutations,
			Privacy:             store.NewPrivacyStore(pool),
			Presence:            presence,
			Blocks:              blocks,
			SocialGraph:         socialGraph,
			SpaceCoMembership:   spaceCoMembership,
			AvatarPresigner:     avatarPresigner,
			AvatarPublicBaseURL: avatarPublicBase,
			Events:              events,
			DeletedAccounts:     deletedAccounts,
			DNSResolver:         dnsResolver,
		}
		owner, err := os.Hostname()
		if err != nil {
			return fmt.Errorf("user release instance: %v", err)
		}
		routes := []string{}
		for _, method := range userv1.UserService_ServiceDesc.Methods {
			routes = append(routes, "grpc:/"+userv1.UserService_ServiceDesc.ServiceName+"/"+method.MethodName)
		}
		if err := releaseRuntime.WriteSnapshot("user", owner, userSvc.ReleasePolicy, []string{"user.update_profile_custom_status.v1", "user.update_presence_custom_status.v1"}, routes, map[string]string{"profiles": "postgres", "presence": "redis"}); err != nil {
			return fmt.Errorf("user release snapshot: %v", err)
		}
		lis, err := net.Listen("tcp", grpcAddr)
		if err != nil {
			return fmt.Errorf("grpc listen: %v", err)
		}
		defer func() { _ = lis.Close() }()

		if sourceEnabled {
			sourceServer, sourceRuntime, err := newUserAuthorityServer(runCtx, sharedOptions, sourceConfig, profiles)
			if err != nil {
				return fmt.Errorf("user authority source startup: %v", err)
			}
			defer func() { _ = sourceRuntime.Close() }()
			sourceListener, err := net.Listen("tcp", sourceConfig.ListenAddr)
			if err != nil {
				return fmt.Errorf("user authority source listen: %v", err)
			}
			defer sourceServer.Stop()
			if err := startup.Serve(sourceServer, sourceListener); err != nil {
				return fmt.Errorf("user authority source serve: %v", err)
			}
		}
		if accountDeletionConfig.Enabled {
			accountDeletionConsumer, err := userevents.NewAccountDeletionConsumer(accountDeletionConfig.NATSURL, accountDeletionConfig.CredentialsFile, profiles)
			if err != nil {
				return fmt.Errorf("account deletion consumer: %v", err)
			}
			if err := accountDeletionConsumer.Start(); err != nil {
				_ = accountDeletionConsumer.Close()
				return fmt.Errorf("account deletion consumer startup: %v", err)
			}
			defer func() { _ = accountDeletionConsumer.Close() }()
			accountDeletionContext, stopAccountDeletionConsumer := context.WithCancel(runCtx)
			defer stopAccountDeletionConsumer()
			go func() {
				if err := accountDeletionConsumer.Run(accountDeletionContext); err != nil && accountDeletionContext.Err() == nil {
					log.Printf("account deletion consumer stopped: %v", err)
				}
			}()
		}

		if natsURL != "" {
			owner, hostErr := os.Hostname()
			if hostErr != nil || strings.TrimSpace(owner) == "" {
				return fmt.Errorf("search projection outbox owner: %v", hostErr)
			}
			dispatcher, projectionNATS, dispatchErr := searchprojection.NewOutboxDispatcher(profiles, natsURL, owner)
			if dispatchErr != nil {
				return fmt.Errorf("search projection outbox dispatcher: %v", dispatchErr)
			}
			defer func() { _ = projectionNATS.Drain() }()
			dispatchCtx, stopDispatcher := context.WithCancel(runCtx)
			defer stopDispatcher()
			go dispatcher.Run(dispatchCtx)
		}
		userv1.RegisterUserServiceServer(srv, userSvc)
		defer srv.Stop()
		if captureAuthority != nil {
			capturePrincipal, principalErr := socialprincipal.New(runCtx, capturePrincipalConfig)
			if principalErr != nil {
				return fmt.Errorf("user Space capture principal: %v", principalErr)
			}
			defer func() { _ = capturePrincipal.Close() }()
			listener, listenErr := net.Listen("tcp", capturePrincipalConfig.ListenAddr)
			if listenErr != nil {
				return fmt.Errorf("user Space capture listen: %v", listenErr)
			}
			options := append([]grpc.ServerOption{}, sharedOptions...)
			options = append(options, capturePrincipal.ServerOptions()...)
			server := grpc.NewServer(options...)
			grpcsvc.RegisterSpaceAccountCaptureServer(server, captureAuthority)
			defer server.Stop()
			if serveErr := startup.Serve(server, listener); serveErr != nil {
				return fmt.Errorf("user Space capture serve: %v", serveErr)
			}
		}
		if privacyRuntime != nil {
			privacyListener, err := net.Listen("tcp", principalConfig.ListenAddr)
			if err != nil {
				return fmt.Errorf("user privacy principal listen: %v", err)
			}
			privacyOptions := append([]grpc.ServerOption{}, sharedOptions...)
			privacyOptions = append(privacyOptions, privacyRuntime.ServerOptions()...)
			privacyServer := grpc.NewServer(privacyOptions...)
			grpcsvc.RegisterSocialPrivacyServer(privacyServer, userSvc)
			defer privacyServer.Stop()
			if err := startup.Serve(privacyServer, privacyListener); err != nil {
				return fmt.Errorf("user privacy principal serve: %v", err)
			}
		}
		if fileOwnershipRuntime != nil {
			fileOwnershipListener, err := net.Listen("tcp", filePrincipalConfig.ListenAddr)
			if err != nil {
				return fmt.Errorf("user file ownership principal listen: %v", err)
			}
			fileOwnershipOptions := append([]grpc.ServerOption{}, sharedOptions...)
			fileOwnershipOptions = append(fileOwnershipOptions, fileOwnershipRuntime.ServerOptions()...)
			fileOwnershipServer := grpc.NewServer(fileOwnershipOptions...)
			grpcsvc.RegisterFileOwnershipServer(fileOwnershipServer, userSvc)
			defer fileOwnershipServer.Stop()
			if err := startup.Serve(fileOwnershipServer, fileOwnershipListener); err != nil {
				return fmt.Errorf("user file ownership principal serve: %v", err)
			}
		}
		if searchProjectionRuntime != nil {
			searchProjectionListener, err := net.Listen("tcp", searchPrincipalConfig.ListenAddr)
			if err != nil {
				return fmt.Errorf("user search projection principal listen: %v", err)
			}
			searchProjectionOptions := append([]grpc.ServerOption{}, sharedOptions...)
			searchProjectionOptions = append(searchProjectionOptions, searchProjectionRuntime.ServerOptions()...)
			searchProjectionServer := grpc.NewServer(searchProjectionOptions...)
			grpcsvc.RegisterSearchProjectionServer(searchProjectionServer, userSvc, searchProjectionCursorKey)
			defer searchProjectionServer.Stop()
			if err := startup.Serve(searchProjectionServer, searchProjectionListener); err != nil {
				return fmt.Errorf("user search projection principal serve: %v", err)
			}
		}
		if admissionRuntime != nil {
			listener, err := net.Listen("tcp", admissionConfig.ListenAddr)
			if err != nil {
				return fmt.Errorf("user admission principal listen: %v", err)
			}
			options := append([]grpc.ServerOption{}, sharedOptions...)
			options = append(options, admissionRuntime.ServerOptions()...)
			server := grpc.NewServer(options...)
			grpcsvc.RegisterAdmissionEligibilityServer(server, userSvc.Profiles)
			defer server.Stop()
			if err := startup.Serve(server, listener); err != nil {
				return fmt.Errorf("user admission principal serve: %v", err)
			}
		}
		if authSdkRuntime != nil {
			authSDKListener, err := net.Listen("tcp", authPrincipalConfig.ListenAddr)
			if err != nil {
				return fmt.Errorf("user SDK conversion principal listen: %v", err)
			}
			authSDKOptions := append([]grpc.ServerOption{}, sharedOptions...)
			authSDKOptions = append(authSDKOptions, authSdkRuntime.ServerOptions()...)
			authSDKServer := grpc.NewServer(authSDKOptions...)
			grpcsvc.RegisterSdkConversionServer(authSDKServer, userSvc.Profiles)
			defer authSDKServer.Stop()
			if err := startup.Serve(authSDKServer, authSDKListener); err != nil {
				return fmt.Errorf("user SDK conversion principal serve: %v", err)
			}
		}
		if notificationPresenceRuntime != nil {
			notificationPresenceListener, err := net.Listen("tcp", notificationPresenceConfig.ListenAddr)
			if err != nil {
				return fmt.Errorf("user Notification presence principal listen: %v", err)
			}
			notificationPresenceOptions := append([]grpc.ServerOption{}, sharedOptions...)
			notificationPresenceOptions = append(notificationPresenceOptions, notificationPresenceRuntime.ServerOptions()...)
			notificationPresenceServer := grpc.NewServer(notificationPresenceOptions...)
			grpcsvc.RegisterNotificationPresenceServer(notificationPresenceServer, userSvc)
			defer notificationPresenceServer.Stop()
			if err := startup.Serve(notificationPresenceServer, notificationPresenceListener); err != nil {
				return fmt.Errorf("user Notification presence principal serve: %v", err)
			}
		}
		if catalogRuntime != nil {
			listener, err := net.Listen("tcp", catalogConfig.ListenAddr)
			if err != nil {
				return fmt.Errorf("user catalog actor principal listen: %v", err)
			}
			options := append([]grpc.ServerOption{}, sharedOptions...)
			options = append(options, catalogRuntime.ServerOptions()...)
			server := grpc.NewServer(options...)
			grpcsvc.RegisterCatalogActorServer(server, userSvc.Profiles)
			defer server.Stop()
			if err := startup.Serve(server, listener); err != nil {
				return fmt.Errorf("user catalog actor principal serve: %v", err)
			}
		}
		if notificationPrimaryRuntime != nil {
			notificationPrimaryListener, err := net.Listen("tcp", notificationPrimaryConfig.ListenAddr)
			if err != nil {
				return fmt.Errorf("user Notification primary principal listen: %v", err)
			}
			notificationPrimaryOptions := append([]grpc.ServerOption{}, sharedOptions...)
			notificationPrimaryOptions = append(notificationPrimaryOptions, notificationPrimaryRuntime.ServerOptions()...)
			notificationPrimaryServer := grpc.NewServer(notificationPrimaryOptions...)
			grpcsvc.RegisterNotificationPrimaryServer(notificationPrimaryServer, userSvc)
			defer notificationPrimaryServer.Stop()
			if err := startup.Serve(notificationPrimaryServer, notificationPrimaryListener); err != nil {
				return fmt.Errorf("user Notification primary principal serve: %v", err)
			}
		}
		if messagingPresenceRuntime != nil {
			messagingPresenceListener, err := net.Listen("tcp", messagingPresenceConfig.ListenAddr)
			if err != nil {
				return fmt.Errorf("user Messaging scheduled presence principal listen: %v", err)
			}
			messagingPresenceOptions := append([]grpc.ServerOption{}, sharedOptions...)
			messagingPresenceOptions = append(messagingPresenceOptions, messagingPresenceRuntime.ServerOptions()...)
			messagingPresenceServer := grpc.NewServer(messagingPresenceOptions...)
			grpcsvc.RegisterMessagingScheduledPresenceServer(messagingPresenceServer, userSvc)
			defer messagingPresenceServer.Stop()
			if err := startup.Serve(messagingPresenceServer, messagingPresenceListener); err != nil {
				return fmt.Errorf("user Messaging scheduled presence principal serve: %v", err)
			}
		}
		if err := startup.Serve(srv, lis); err != nil {
			return fmt.Errorf("grpc serve: %v", err)
		}
		logger.Info("gRPC transport listening; application admission held", slog.String("addr", grpcAddr))
		if spaceConn != nil {
			waitCtx, waitCancel := context.WithTimeout(runCtx, grpcclient.DialTimeoutFromEnv())
			err := waitForRequiredGRPCReady(waitCtx, spaceConn)
			waitCancel()
			if err != nil {
				return fmt.Errorf("space grpc dial: %v", err)
			}
		}
	} else {
		logger.Warn("DATABASE_URL not set; gRPC disabled (health only)")
	}

	server := &http.Server{
		Addr:    httpAddr,
		Handler: startup.http(httpserver.Wrap(voiceprom.MountMetricsOnHealth(healthHandler(serviceName), metricsReg), logger)),
	}
	httpserver.ApplyHTTPServerTimeouts(server)
	defer func() { _ = server.Close() }()
	httpListener, err := net.Listen("tcp", httpAddr)
	if err != nil {
		return fmt.Errorf("http listen: %v", err)
	}
	defer func() { _ = httpListener.Close() }()
	started := &startupListener{Listener: httpListener, entered: make(chan struct{})}
	errCh := make(chan error, 1)
	logger.Info("listening", slog.String("addr", httpAddr))
	go func() {
		err := server.Serve(started)
		if err != nil && err != http.ErrServerClosed {
			startup.fail(err)
		}
		errCh <- err
	}()
	select {
	case <-started.entered:
	case <-runCtx.Done():
		return context.Cause(runCtx)
	}
	if err := startup.Release(); err != nil {
		return err
	}

	select {
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			startup.fail(err)
			return err
		}
	case <-runCtx.Done():
		ctx, cancel := context.WithTimeout(context.Background(), runtimeconfig.ShutdownTimeoutFromEnv())
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			return err
		}
		if parent.Err() == nil {
			return context.Cause(runCtx)
		}
	}
	return nil
}
