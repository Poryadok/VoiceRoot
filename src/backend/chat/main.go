package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
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
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"

	"voice/backend/chat/internal/chatevents"
	"voice/backend/chat/internal/gisprincipal"
	grpcsvc "voice/backend/chat/internal/grpcsvc"
	"voice/backend/chat/internal/store"
	"voice/backend/pkg/grpcclient"
	"voice/backend/pkg/grpcmw"
	"voice/backend/pkg/httpserver"
	voiceprom "voice/backend/pkg/promhttp"
	"voice/backend/pkg/runtimeconfig"

	authv1 "voice.app/voice/auth/v1"
	chatv1 "voice.app/voice/chat/v1"
	filev1 "voice.app/voice/file/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	rolev1 "voice.app/voice/role/v1"
	userv1 "voice.app/voice/user/v1"
)

const serviceName = "chat"

// waitForRequiredGRPCReady establishes a required S2S connection before this
// service exposes its health endpoint. Lazy gRPC connections otherwise make a
// healthy Chat instance fail closed on its first deleted-account lookup.
func waitForRequiredGRPCReady(ctx context.Context, conn *grpc.ClientConn) error {
	if conn == nil {
		return errors.New("required grpc connection is nil")
	}
	conn.Connect()
	return grpcclient.WaitForReady(ctx, conn)
}

func main() {
	logger := httpserver.NewLogger(serviceName)
	chatPrincipalIssuer, chatPrincipalJWKS, principalErr := loadChatPrincipalIssuerFromEnv()
	if principalErr != nil {
		log.Fatalf("Chat principal issuer: %v", principalErr)
	}
	metricsReg := prometheus.NewRegistry()
	httpAddr := ":8080"
	if v := os.Getenv("LISTEN_ADDR"); v != "" {
		httpAddr = v
	}
	grpcListen := ":9090"
	if v := strings.TrimSpace(os.Getenv("CHAT_GRPC_LISTEN")); v != "" {
		grpcListen = v
	}
	gisListen := strings.TrimSpace(os.Getenv("CHAT_GIS_GRPC_LISTEN"))
	gisPrincipalConfig, gisPrincipalConfigured, gisConfigErr := gisprincipal.ConfigFromEnv()
	if gisConfigErr != nil {
		log.Fatalf("GIS principal configuration: %v", gisConfigErr)
	}
	if (gisListen != "") != gisPrincipalConfigured {
		log.Fatal("CHAT_GIS_GRPC_LISTEN and complete GIS principal configuration must be set together")
	}
	spaceLifecycleConfig, spaceLifecycleListen, spaceLifecycleConfigured, spaceLifecycleConfigErr := gisprincipal.SpaceLifecycleConfigFromEnv()
	if spaceLifecycleConfigErr != nil {
		log.Fatalf("Space lifecycle principal configuration: %v", spaceLifecycleConfigErr)
	}
	spacePurgeOwners := spacePurgeOwnerConfigFromEnv(os.Getenv)
	searchManifestConfig, searchManifestListen, searchManifestConfigured, searchManifestConfigErr := gisprincipal.SearchManifestConfigFromEnv()
	if searchManifestConfigErr != nil {
		log.Fatalf("Search manifest principal configuration: %v", searchManifestConfigErr)
	}
	if spacePurgeOwners.configured() && !spaceLifecycleConfigured {
		log.Fatal("Chat Space purge owner clients require the Space lifecycle principal listener")
	}
	if spaceLifecycleConfigured {
		if chatPrincipalIssuer == nil {
			log.Fatal("Chat Space purge requires the Chat service-principal signing keys")
		}
		if err := spacePurgeOwners.validate(); err != nil {
			log.Fatal(err)
		}
	}

	dbURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	var grpcSrv *grpc.Server
	var gisGRPCSrv *grpc.Server
	var gisPrincipalRuntime *gisprincipal.Runtime
	var spaceLifecycleGRPCSrv *grpc.Server
	var spaceLifecyclePrincipalRuntime *gisprincipal.Runtime
	var searchManifestGRPCSrv *grpc.Server
	var accountDeletedConsumerDone <-chan error
	runCtx, runCancel := context.WithCancel(context.Background())
	defer runCancel()
	if dbURL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), runtimeconfig.PostgresConnectTimeoutFromEnv())
		pool, err := pgxpool.New(ctx, dbURL)
		cancel()
		if err != nil {
			log.Fatalf("postgres: %v", err)
		}
		defer pool.Close()
		if gisPrincipalConfigured {
			gisPrincipalRuntime, err = gisprincipal.New(context.Background(), gisPrincipalConfig)
			if err != nil {
				log.Fatalf("GIS principal runtime: %v", err)
			}
			defer func() { _ = gisPrincipalRuntime.Close() }()
			gisLis, listenErr := net.Listen("tcp", gisListen)
			if listenErr != nil {
				log.Fatalf("GIS gRPC listen: %v", listenErr)
			}
			gisGRPCSrv = grpc.NewServer(gisPrincipalRuntime.ServerOptions()...)
			chatv1.RegisterGameIntegrationChatServiceServer(gisGRPCSrv, &grpcsvc.GameIntegrationChatGRPC{Store: &store.DMStore{Pool: pool}})
			go func() {
				logger.Info("GIS mTLS gRPC listening", slog.String("addr", gisListen))
				if err := gisGRPCSrv.Serve(gisLis); err != nil {
					log.Fatalf("GIS gRPC serve: %v", err)
				}
			}()
		}

		var blocks grpcsvc.AccountBlockChecker
		var friends grpcsvc.ProfileFriendChecker
		var contacts grpcsvc.ProfileContactChecker
		if socialAddr := strings.TrimSpace(os.Getenv("SOCIAL_GRPC_ADDR")); socialAddr != "" {
			sconn, err := grpc.NewClient(grpcclient.DialTarget(socialAddr), grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatalf("social grpc: %v", err)
			}
			sconn.Connect()
			waitCtx, waitCancel := context.WithTimeout(context.Background(), grpcclient.DialTimeoutFromEnv())
			for {
				st := sconn.GetState()
				if st == connectivity.Ready {
					break
				}
				if st == connectivity.Shutdown {
					waitCancel()
					_ = sconn.Close()
					log.Fatalf("social grpc: unexpected shutdown")
				}
				if !sconn.WaitForStateChange(waitCtx, st) {
					waitCancel()
					_ = sconn.Close()
					log.Fatalf("social grpc dial: %v", context.Cause(waitCtx))
				}
			}
			waitCancel()
			defer func() { _ = sconn.Close() }()
			blocks = grpcsvc.NewSocialGRPCBlocks(sconn)
			friends = grpcsvc.NewSocialGRPCFriends(sconn)
			contacts = grpcsvc.NewSocialGRPCContacts(sconn)
		}

		var profiles grpcsvc.UserProfileLookup
		var dmPeerDisplayNames grpcsvc.DMPeerDisplayNameLookup
		var lifecycleOwners grpcsvc.LifecycleOwnerLookup
		var privacy grpcsvc.PrivacyChecker
		var spaceCoMembership grpcsvc.SpaceCoMembershipChecker
		var accountProfiles accountDeletedProfileLister
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
			profiles = &grpcsvc.UserGRPCProfiles{Client: userClient}
			dmPeerDisplayNames = grpcsvc.NewUserGRPCDMPeerDisplayNames(userClient)
			lifecycleOwners = &grpcsvc.UserGRPCLifecycleOwners{Client: userClient}
			privacy = &grpcsvc.UserGRPCPrivacy{Client: userClient}
			accountProfiles = grpcsvc.NewUserGRPCAccountProfiles(userClient)
		}
		if spaceAddr := strings.TrimSpace(os.Getenv("SPACE_GRPC_ADDR")); spaceAddr != "" {
			spconn, err := grpc.NewClient(grpcclient.DialTarget(spaceAddr), grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatalf("space grpc: %v", err)
			}
			defer func() { _ = spconn.Close() }()
			spaceCoMembership = grpcsvc.NewSpaceGRPCCoMembership(spconn)
		}

		var roleClient rolev1.RoleServiceClient
		if roleAddr := strings.TrimSpace(os.Getenv("ROLE_GRPC_ADDR")); roleAddr != "" {
			rconn, err := grpc.NewClient(grpcclient.DialTarget(roleAddr), grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatalf("role grpc: %v", err)
			}
			defer func() { _ = rconn.Close() }()
			roleClient = rolev1.NewRoleServiceClient(rconn)
		}

		var listEnrich grpcsvc.ListChatsEnrichment
		var e2ePreKeyGate grpcsvc.E2EPreKeyGate
		var deletedAccounts grpcsvc.AccountDeletedChecker
		if authAddr := strings.TrimSpace(os.Getenv("AUTH_GRPC_ADDR")); authAddr != "" {
			aconn, err := grpc.NewClient(grpcclient.DialTarget(authAddr), grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatalf("auth grpc: %v", err)
			}
			waitCtx, waitCancel := context.WithTimeout(context.Background(), grpcclient.DialTimeoutFromEnv())
			if err := waitForRequiredGRPCReady(waitCtx, aconn); err != nil {
				waitCancel()
				_ = aconn.Close()
				log.Fatalf("auth grpc dial: %v", err)
			}
			waitCancel()
			defer func() { _ = aconn.Close() }()
			deletedAccounts = grpcsvc.NewAuthGRPCDeletedAccounts(authv1.NewAuthServiceClient(aconn))
		}
		if msgAddr := strings.TrimSpace(os.Getenv("MESSAGING_GRPC_ADDR")); msgAddr != "" {
			mconn, err := grpc.NewClient(grpcclient.DialTarget(msgAddr), grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatalf("messaging grpc: %v", err)
			}
			defer func() { _ = mconn.Close() }()
			msgClient := messagingv1.NewMessagingServiceClient(mconn)
			listEnrich = grpcsvc.NewMessagingListEnricher(msgClient)
			e2ePreKeyGate = grpcsvc.NewMessagingE2EPreKeyGate(msgClient)
		}

		dmStore := &store.DMStore{Pool: pool}
		var spaceMembers *store.SpaceMembersStore
		if spaceDB := strings.TrimSpace(os.Getenv("SPACE_DATABASE_URL")); spaceDB != "" {
			sctx, scancel := context.WithTimeout(context.Background(), runtimeconfig.PostgresConnectTimeoutFromEnv())
			spacePool, serr := pgxpool.New(sctx, spaceDB)
			scancel()
			if serr != nil {
				log.Fatalf("space postgres: %v", serr)
			}
			defer spacePool.Close()
			spaceMembers = &store.SpaceMembersStore{Pool: spacePool}
		}
		natsURL := strings.TrimSpace(os.Getenv("NATS_URL"))
		var chatEvents chatevents.Publisher
		var jsPub *chatevents.JetStreamPublisher
		if natsURL != "" {
			jsPub, err = chatevents.NewJetStreamPublisher(natsURL)
			if err != nil {
				log.Fatalf("nats jetstream publisher: %v", err)
			}
			defer func() { _ = jsPub.Close() }()
			jsPub.Logger = logger
			chatEvents = jsPub
			go func() {
				if err := runMessageActivityConsumer(runCtx, natsURL, dmStore, logger); err != nil && !errors.Is(err, context.Canceled) {
					logger.Error("message activity consumer stopped", slog.String("error", err.Error()))
				}
			}()
			if friends == nil {
				logger.Warn("friend accepted consumer disabled: SOCIAL_GRPC_ADDR not set")
			} else {
				go func() {
					if err := runFriendAcceptedConsumer(runCtx, natsURL, dmStore, friends, chatEvents, logger); err != nil && !errors.Is(err, context.Canceled) {
						logger.Error("friend accepted consumer stopped", slog.String("error", err.Error()))
					}
				}()
			}
			if accountProfiles == nil {
				logger.Warn("user.account_deleted consumer disabled: USER_GRPC_ADDR not set")
			} else {
				consumerDone := make(chan error, 1)
				accountDeletedConsumerDone = consumerDone
				go func() {
					err := runAccountDeletedConsumer(runCtx, natsURL, os.Getenv("HOSTNAME"), accountProfiles, dmStore, jsPub, logger)
					if err != nil && !errors.Is(err, context.Canceled) {
						logger.Error("user.account_deleted consumer stopped", slog.String("error", err.Error()))
					}
					consumerDone <- err
				}()
			}
		}
		if spaceLifecycleConfigured {
			spaceLifecyclePrincipalRuntime, err = gisprincipal.New(context.Background(), spaceLifecycleConfig)
			if err != nil {
				log.Fatalf("Space lifecycle principal runtime: %v", err)
			}
			defer func() { _ = spaceLifecyclePrincipalRuntime.Close() }()
			lifecycleLis, listenErr := net.Listen("tcp", spaceLifecycleListen)
			if listenErr != nil {
				log.Fatalf("Space lifecycle gRPC listen: %v", listenErr)
			}
			messagingConn, dialErr := dialSpacePurgeOwner(spacePurgeOwners.MessagingAddr, spacePurgeOwners.MessagingCA, spacePurgeOwners.MessagingServerName, spacePurgeOwners.ClientCert, spacePurgeOwners.ClientKey)
			if dialErr != nil {
				log.Fatalf("Chat purge Messaging mTLS client: %v", dialErr)
			}
			defer func() { _ = messagingConn.Close() }()
			fileConn, dialErr := dialSpacePurgeOwner(spacePurgeOwners.FileAddr, spacePurgeOwners.FileCA, spacePurgeOwners.FileServerName, spacePurgeOwners.ClientCert, spacePurgeOwners.ClientKey)
			if dialErr != nil {
				log.Fatalf("Chat purge File mTLS client: %v", dialErr)
			}
			defer func() { _ = fileConn.Close() }()
			spaceLifecycleGRPCSrv = grpc.NewServer(spaceLifecyclePrincipalRuntime.ServerOptions()...)
			chatv1.RegisterChatServiceServer(spaceLifecycleGRPCSrv, &grpcsvc.SpaceLifecycleGRPC{
				Store: &store.SpaceLifecycleStore{Pool: pool}, Issuer: chatPrincipalIssuer,
				Messaging: messagingv1.NewMessagingServiceClient(messagingConn), File: filev1.NewFileServiceClient(fileConn),
			})
			go func() {
				logger.Info("Space lifecycle mTLS gRPC listening", slog.String("addr", spaceLifecycleListen))
				if err := spaceLifecycleGRPCSrv.Serve(lifecycleLis); err != nil {
					log.Fatalf("Space lifecycle gRPC serve: %v", err)
				}
			}()
		}

		if searchManifestConfigured {
			searchRuntime, runtimeErr := gisprincipal.New(runCtx, searchManifestConfig)
			if runtimeErr != nil {
				log.Fatalf("Search manifest principal runtime: %v", runtimeErr)
			}
			defer func() { _ = searchRuntime.Close() }()
			listener, listenErr := net.Listen("tcp", searchManifestListen)
			if listenErr != nil {
				log.Fatalf("Search manifest gRPC listen: %v", listenErr)
			}
			searchManifestGRPCSrv = grpc.NewServer(searchRuntime.ServerOptions()...)
			chatv1.RegisterChatServiceServer(searchManifestGRPCSrv, &grpcsvc.SpaceLifecycleGRPC{Store: &store.SpaceLifecycleStore{Pool: pool}})
			go func() {
				logger.Info("Search manifest mTLS gRPC listening", slog.String("addr", searchManifestListen))
				if err := searchManifestGRPCSrv.Serve(listener); err != nil {
					log.Fatalf("Search manifest gRPC serve: %v", err)
				}
			}()
		}
		lis, err := net.Listen("tcp", grpcListen)
		if err != nil {
			log.Fatalf("grpc listen: %v", err)
		}
		grpcSrv = grpc.NewServer(grpcmw.ServerOptions(logger, grpcmw.WithRegistry(metricsReg))...)
		listEnrichmentFailures := grpcsvc.NewListEnrichmentFailuresCounter(metricsReg)
		chatv1.RegisterChatServiceServer(grpcSrv, &grpcsvc.ChatGRPC{
			DM:                     dmStore,
			StickerPacks:           dmStore,
			Profiles:               profiles,
			DMPeerDisplayNames:     dmPeerDisplayNames,
			LifecycleOwners:        lifecycleOwners,
			Blocks:                 blocks,
			Privacy:                privacy,
			Friends:                friends,
			Contacts:               contacts,
			SpaceCoMembership:      spaceCoMembership,
			ListEnrich:             listEnrich,
			ListEnrichmentFailures: listEnrichmentFailures,
			DeletedAccounts:        deletedAccounts,
			E2EPreKeyGate:          e2ePreKeyGate,
			ChatEvents:             chatEvents,
			Roles:                  roleClient,
			SpaceMembers:           spaceMembers,
			Logger:                 logger,
		})
		go func() {
			logger.Info("gRPC listening", slog.String("addr", grpcListen))
			if err := grpcSrv.Serve(lis); err != nil {
				log.Fatalf("grpc serve: %v", err)
			}
		}()
	} else {
		if gisPrincipalConfigured || spaceLifecycleConfigured || searchManifestConfigured {
			log.Fatal("protected principal listeners require DATABASE_URL")
		}
		logger.Warn("DATABASE_URL not set; gRPC disabled (health only)")
	}

	server := &http.Server{
		Addr:    httpAddr,
		Handler: httpserver.Wrap(voiceprom.MountMetricsOnHealth(healthHandler(serviceName), metricsReg), logger),
	}
	httpserver.ApplyHTTPServerTimeouts(server)
	errCh := make(chan error, 2)
	var principalJWKSHTTPServer *http.Server
	if len(chatPrincipalJWKS.Keys) > 0 {
		certFile := strings.TrimSpace(os.Getenv("CHAT_PRINCIPAL_JWKS_TLS_CERT_FILE"))
		keyFile := strings.TrimSpace(os.Getenv("CHAT_PRINCIPAL_JWKS_TLS_KEY_FILE"))
		clientCAFile := strings.TrimSpace(os.Getenv("CHAT_PRINCIPAL_JWKS_CLIENT_CA_FILE"))
		if certFile == "" || keyFile == "" || clientCAFile == "" {
			log.Fatal("Chat principal JWKS TLS server certificate, key, and client CA are required")
		}
		certificate, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			log.Fatalf("Chat principal JWKS TLS identity: %v", err)
		}
		caPEM, err := os.ReadFile(clientCAFile)
		if err != nil {
			log.Fatalf("Chat principal JWKS client CA: %v", err)
		}
		clientCAs := x509.NewCertPool()
		if !clientCAs.AppendCertsFromPEM(caPEM) {
			log.Fatal("Chat principal JWKS client CA contains no certificates")
		}
		addr := strings.TrimSpace(os.Getenv("CHAT_PRINCIPAL_JWKS_HTTPS_LISTEN"))
		if addr == "" {
			addr = ":8443"
		}
		principalJWKSHTTPServer = &http.Server{
			Addr: addr, Handler: chatPrincipalJWKSHandler(chatPrincipalJWKS),
			TLSConfig: &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{certificate}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: clientCAs},
		}
		httpserver.ApplyHTTPServerTimeouts(principalJWKSHTTPServer)
		go func() { errCh <- principalJWKSHTTPServer.ListenAndServeTLS("", "") }()
		logger.Info("Chat principal JWKS mTLS endpoint listening", slog.String("addr", addr))
	}
	logger.Info("listening", slog.String("addr", httpAddr))
	go func() {
		errCh <- server.ListenAndServe()
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	shutdownServer := false
	select {
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	case <-stop:
		shutdownServer = true
	}
	runCancel()
	ctx, cancel := context.WithTimeout(context.Background(), runtimeconfig.ShutdownTimeoutFromEnv())
	defer cancel()
	waitForAccountDeletedConsumerShutdown(ctx, accountDeletedConsumerDone, logger)
	if shutdownServer {
		if searchManifestGRPCSrv != nil {
			searchManifestGRPCSrv.GracefulStop()
		}
		if spaceLifecycleGRPCSrv != nil {
			spaceLifecycleGRPCSrv.GracefulStop()
		}
		if gisGRPCSrv != nil {
			gisGRPCSrv.GracefulStop()
		}
		if grpcSrv != nil {
			grpcSrv.GracefulStop()
		}
		if err := server.Shutdown(ctx); err != nil {
			log.Fatal(err)
		}
		if principalJWKSHTTPServer != nil {
			if err := principalJWKSHTTPServer.Shutdown(ctx); err != nil {
				log.Fatal(err)
			}
		}
	}
}
