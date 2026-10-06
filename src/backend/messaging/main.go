package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
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

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	grpcsvc "voice/backend/messaging/internal/grpcsvc"
	"voice/backend/messaging/internal/messageevents"
	"voice/backend/messaging/internal/principalgrpc"
	"voice/backend/messaging/internal/principalruntime"
	"voice/backend/messaging/internal/s2s"
	"voice/backend/messaging/internal/store"
	"voice/backend/pkg/grpcclient"
	"voice/backend/pkg/grpcmw"
	"voice/backend/pkg/httpserver"
	voiceprom "voice/backend/pkg/promhttp"
	"voice/backend/pkg/runtimeconfig"

	authv1 "voice.app/voice/auth/v1"
	chatv1 "voice.app/voice/chat/v1"
	filev1 "voice.app/voice/file/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	moderationv1 "voice.app/voice/moderation/v1"
	rolev1 "voice.app/voice/role/v1"
	socialv1 "voice.app/voice/social/v1"
	userv1 "voice.app/voice/user/v1"
)

const serviceName = "messaging"

func loadGameTombstoneKeys() (*s2s.GameTombstoneKeySource, error) {
	config := s2s.GameTombstoneKeyConfig{
		KeyID:          strings.TrimSpace(os.Getenv("MESSAGING_TOMBSTONE_KEY_ID")),
		PrivateKeyFile: strings.TrimSpace(os.Getenv("MESSAGING_TOMBSTONE_PRIVATE_KEY_FILE")),
		NotBefore:      strings.TrimSpace(os.Getenv("MESSAGING_TOMBSTONE_NOT_BEFORE")),
		NotAfter:       strings.TrimSpace(os.Getenv("MESSAGING_TOMBSTONE_NOT_AFTER")),
	}
	if config.KeyID == "" && config.PrivateKeyFile == "" && config.NotBefore == "" && config.NotAfter == "" {
		return nil, nil
	}
	return s2s.LoadGameTombstoneKeySource(config)
}

func loadModerationPrincipalRuntime(ctx context.Context) (*principalruntime.Runtime, error) {
	config := principalruntime.Config{
		JWKSURL:     strings.TrimSpace(os.Getenv("MODERATION_PRINCIPAL_JWKS_URL")),
		TLSCertFile: strings.TrimSpace(os.Getenv("MESSAGING_PRINCIPAL_TLS_CERT_FILE")),
		TLSKeyFile:  strings.TrimSpace(os.Getenv("MESSAGING_PRINCIPAL_TLS_KEY_FILE")),
		CAFile:      strings.TrimSpace(os.Getenv("MODERATION_PRINCIPAL_JWKS_CA_FILE")),
		RedisURL:    strings.TrimSpace(os.Getenv("MESSAGING_PRINCIPAL_REPLAY_REDIS_URL")),
	}
	if config.JWKSURL == "" && config.TLSCertFile == "" && config.TLSKeyFile == "" && config.CAFile == "" {
		return nil, nil
	}
	return principalruntime.New(ctx, config)
}

func loadGatewayPrincipalRuntime(ctx context.Context) (*principalruntime.Runtime, error) {
	config := principalruntime.Config{
		JWKSURL:     strings.TrimSpace(os.Getenv("GATEWAY_PRINCIPAL_JWKS_URL")),
		TLSCertFile: strings.TrimSpace(os.Getenv("MESSAGING_GATEWAY_PRINCIPAL_TLS_CERT_FILE")),
		TLSKeyFile:  strings.TrimSpace(os.Getenv("MESSAGING_GATEWAY_PRINCIPAL_TLS_KEY_FILE")),
		CAFile:      strings.TrimSpace(os.Getenv("GATEWAY_PRINCIPAL_JWKS_CA_FILE")),
		RedisURL:    strings.TrimSpace(os.Getenv("MESSAGING_PRINCIPAL_REPLAY_REDIS_URL")),
	}
	if config.JWKSURL == "" && config.TLSCertFile == "" && config.TLSKeyFile == "" && config.CAFile == "" {
		return nil, nil
	}
	return principalruntime.NewForIssuer(ctx, config, "gateway")
}

func loadBotPrincipalRuntime(ctx context.Context) (*principalruntime.Runtime, error) {
	config := principalruntime.Config{
		JWKSURL:     strings.TrimSpace(os.Getenv("BOT_PRINCIPAL_JWKS_URL")),
		TLSCertFile: strings.TrimSpace(os.Getenv("MESSAGING_BOT_PRINCIPAL_TLS_CERT_FILE")),
		TLSKeyFile:  strings.TrimSpace(os.Getenv("MESSAGING_BOT_PRINCIPAL_TLS_KEY_FILE")),
		CAFile:      strings.TrimSpace(os.Getenv("BOT_PRINCIPAL_JWKS_CA_FILE")),
		RedisURL:    strings.TrimSpace(os.Getenv("MESSAGING_PRINCIPAL_REPLAY_REDIS_URL")),
	}
	if config.JWKSURL == "" && config.TLSCertFile == "" && config.TLSKeyFile == "" && config.CAFile == "" {
		return nil, nil
	}
	return principalruntime.NewForIssuer(ctx, config, "bot")
}

func loadGameIntegrationPrincipalRuntime(ctx context.Context) (*principalruntime.Runtime, error) {
	config := principalruntime.Config{
		JWKSURL:     strings.TrimSpace(os.Getenv("GAME_INTEGRATION_PRINCIPAL_JWKS_URL")),
		TLSCertFile: strings.TrimSpace(os.Getenv("MESSAGING_GAME_INTEGRATION_JWKS_TLS_CERT_FILE")),
		TLSKeyFile:  strings.TrimSpace(os.Getenv("MESSAGING_GAME_INTEGRATION_JWKS_TLS_KEY_FILE")),
		CAFile:      strings.TrimSpace(os.Getenv("GAME_INTEGRATION_PRINCIPAL_JWKS_CA_FILE")),
		RedisURL:    strings.TrimSpace(os.Getenv("MESSAGING_PRINCIPAL_REPLAY_REDIS_URL")),
	}
	if config.JWKSURL == "" && config.TLSCertFile == "" && config.TLSKeyFile == "" && config.CAFile == "" {
		return nil, nil
	}
	return principalruntime.NewForIssuer(ctx, config, "gameintegration")
}

func loadChatLifecyclePrincipalRuntime(ctx context.Context) (*principalruntime.Runtime, error) {
	config := principalruntime.Config{
		JWKSURL:     strings.TrimSpace(os.Getenv("CHAT_PRINCIPAL_JWKS_URL")),
		TLSCertFile: strings.TrimSpace(os.Getenv("MESSAGING_CHAT_PRINCIPAL_JWKS_TLS_CERT_FILE")),
		TLSKeyFile:  strings.TrimSpace(os.Getenv("MESSAGING_CHAT_PRINCIPAL_JWKS_TLS_KEY_FILE")),
		CAFile:      strings.TrimSpace(os.Getenv("CHAT_PRINCIPAL_JWKS_CA_FILE")),
		RedisURL:    strings.TrimSpace(os.Getenv("MESSAGING_PRINCIPAL_REPLAY_REDIS_URL")),
	}
	if config.JWKSURL == "" && config.TLSCertFile == "" && config.TLSKeyFile == "" && config.CAFile == "" {
		return nil, nil
	}
	return principalruntime.NewForIssuer(ctx, config, "chat")
}

func loadSpaceLifecyclePrincipalRuntime(ctx context.Context) (*principalruntime.Runtime, error) {
	config := principalruntime.Config{
		JWKSURL:     strings.TrimSpace(os.Getenv("SPACE_PRINCIPAL_JWKS_URL")),
		TLSCertFile: strings.TrimSpace(os.Getenv("MESSAGING_SPACE_PRINCIPAL_JWKS_TLS_CERT_FILE")),
		TLSKeyFile:  strings.TrimSpace(os.Getenv("MESSAGING_SPACE_PRINCIPAL_JWKS_TLS_KEY_FILE")),
		CAFile:      strings.TrimSpace(os.Getenv("SPACE_PRINCIPAL_JWKS_CA_FILE")),
		RedisURL:    strings.TrimSpace(os.Getenv("MESSAGING_PRINCIPAL_REPLAY_REDIS_URL")),
	}
	if config.JWKSURL == "" && config.TLSCertFile == "" && config.TLSKeyFile == "" && config.CAFile == "" {
		return nil, nil
	}
	return principalruntime.NewForIssuer(ctx, config, "space")
}

func loadGameAuthKeys() (*s2s.GameAuthKeys, error) {
	config := s2s.GameAuthKeysConfig{JWKSURL: strings.TrimSpace(os.Getenv("AUTH_PRINCIPAL_JWKS_URL")), TLSCertFile: strings.TrimSpace(os.Getenv("MESSAGING_AUTH_PRINCIPAL_TLS_CERT_FILE")), TLSKeyFile: strings.TrimSpace(os.Getenv("MESSAGING_AUTH_PRINCIPAL_TLS_KEY_FILE")), CAFile: strings.TrimSpace(os.Getenv("AUTH_PRINCIPAL_JWKS_CA_FILE"))}
	if config.JWKSURL == "" && config.TLSCertFile == "" && config.TLSKeyFile == "" && config.CAFile == "" {
		return nil, nil
	}
	return s2s.NewGameAuthKeys(config)
}

func loadGameMessageExecutionPermitClient() (*s2s.GameMessageExecutionPermitClient, error) {
	config := s2s.GameMessageExecutionPermitConfig{Endpoint: strings.TrimSpace(os.Getenv("AUTH_GAME_MESSAGE_EXECUTION_PERMIT_URL")), TLSCertFile: strings.TrimSpace(os.Getenv("MESSAGING_AUTH_EXECUTION_PERMIT_TLS_CERT_FILE")), TLSKeyFile: strings.TrimSpace(os.Getenv("MESSAGING_AUTH_EXECUTION_PERMIT_TLS_KEY_FILE")), CAFile: strings.TrimSpace(os.Getenv("AUTH_GAME_MESSAGE_EXECUTION_PERMIT_CA_FILE"))}
	if config.Endpoint == "" && config.TLSCertFile == "" && config.TLSKeyFile == "" && config.CAFile == "" {
		return nil, nil
	}
	return s2s.NewGameMessageExecutionPermitClient(config)
}

func loadGameResourceMappingAuthorizationClient() (*s2s.GameResourceMappingAuthorizationClient, error) {
	config := s2s.GameResourceMappingAuthorizationConfig{
		Endpoint:          strings.TrimSpace(os.Getenv("GAME_INTEGRATION_MESSAGING_RESOURCE_MAPPING_URL")),
		TLSCertFile:       strings.TrimSpace(os.Getenv("MESSAGING_GAME_INTEGRATION_TLS_CERT_FILE")),
		TLSKeyFile:        strings.TrimSpace(os.Getenv("MESSAGING_GAME_INTEGRATION_TLS_KEY_FILE")),
		CAFile:            strings.TrimSpace(os.Getenv("GAME_INTEGRATION_MESSAGING_CA_FILE")),
		WorkloadKeyBase64: strings.TrimSpace(os.Getenv("GAME_INTEGRATION_MESSAGING_WORKLOAD_KEY_B64")),
	}
	if config.Endpoint == "" && config.TLSCertFile == "" && config.TLSKeyFile == "" && config.CAFile == "" && config.WorkloadKeyBase64 == "" {
		return nil, nil
	}
	return s2s.NewGameResourceMappingAuthorizationClient(config)
}

func startGameTombstoneJWKS(source *s2s.GameTombstoneKeySource) (*http.Server, error) {
	address := strings.TrimSpace(os.Getenv("MESSAGING_TOMBSTONE_JWKS_LISTEN"))
	certFile := strings.TrimSpace(os.Getenv("MESSAGING_TOMBSTONE_JWKS_TLS_CERT_FILE"))
	keyFile := strings.TrimSpace(os.Getenv("MESSAGING_TOMBSTONE_JWKS_TLS_KEY_FILE"))
	caFile := strings.TrimSpace(os.Getenv("MESSAGING_TOMBSTONE_JWKS_CLIENT_CA_FILE"))
	if source == nil && address == "" && certFile == "" && keyFile == "" && caFile == "" {
		return nil, nil
	}
	if source == nil || address == "" || certFile == "" || keyFile == "" || caFile == "" {
		return nil, fmt.Errorf("tombstone JWKS requires a signing key, dedicated listener, server certificate, and client CA")
	}
	certificate, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("tombstone JWKS server certificate: %w", err)
	}
	caPEM, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("tombstone JWKS client CA: %w", err)
	}
	clientCAs := x509.NewCertPool()
	if !clientCAs.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("tombstone JWKS client CA contains no certificates")
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("tombstone JWKS listen: %w", err)
	}
	server := &http.Server{Handler: source.JWKSHandler(), TLSConfig: &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{certificate}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: clientCAs}}
	go func() {
		if err := server.ServeTLS(listener, "", ""); err != nil && err != http.ErrServerClosed {
			log.Printf("tombstone JWKS server exited: %v", err)
		}
	}()
	return server, nil
}

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
	runCtx, runCancel := context.WithCancel(context.Background())
	defer runCancel()
	metricsReg := prometheus.NewRegistry()
	httpAddr := ":8080"
	if v := os.Getenv("LISTEN_ADDR"); v != "" {
		httpAddr = v
	}
	grpcListen := ":9090"
	if v := strings.TrimSpace(os.Getenv("MESSAGING_GRPC_LISTEN")); v != "" {
		grpcListen = v
	}

	dbURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	var grpcSrv *grpc.Server
	var gameIntegrationGRPCSrv *grpc.Server
	var chatLifecycleGRPCSrv *grpc.Server
	var gameIntegrationPrincipal *principalruntime.Runtime
	var chatLifecyclePrincipal *principalruntime.Runtime
	var spaceLifecyclePrincipal *principalruntime.Runtime
	if dbURL != "" {
		tombstoneKeys, err := loadGameTombstoneKeys()
		if err != nil {
			log.Fatalf("messaging tombstone key configuration: %v", err)
		}
		jwksServer, err := startGameTombstoneJWKS(tombstoneKeys)
		if err != nil {
			log.Fatalf("messaging tombstone JWKS configuration: %v", err)
		}
		if jwksServer != nil {
			defer func() { _ = jwksServer.Close() }()
		}
		moderationPrincipal, err := loadModerationPrincipalRuntime(context.Background())
		if err != nil {
			log.Fatalf("moderation principal runtime: %v", err)
		}
		if moderationPrincipal != nil {
			defer func() { _ = moderationPrincipal.Close() }()
		}
		gatewayPrincipal, err := loadGatewayPrincipalRuntime(context.Background())
		if err != nil {
			log.Fatalf("Gateway principal runtime: %v", err)
		}
		if gatewayPrincipal != nil {
			defer func() { _ = gatewayPrincipal.Close() }()
		} else {
			logger.Warn("ApplyGameMessage ingress is fail-closed because Gateway principal verification is not configured")
		}
		botPrincipal, err := loadBotPrincipalRuntime(context.Background())
		if err != nil {
			log.Fatalf("Bot principal runtime: %v", err)
		}
		if botPrincipal != nil {
			defer func() { _ = botPrincipal.Close() }()
		} else {
			logger.Warn("SendGameEventMessage ingress is fail-closed because Bot principal verification is not configured")
		}
		gameIntegrationPrincipal, err = loadGameIntegrationPrincipalRuntime(context.Background())
		if err != nil {
			log.Fatalf("game integration principal runtime: %v", err)
		}
		if gameIntegrationPrincipal != nil {
			defer func() { _ = gameIntegrationPrincipal.Close() }()
		}
		chatLifecyclePrincipal, err = loadChatLifecyclePrincipalRuntime(context.Background())
		if err != nil {
			log.Fatalf("Chat lifecycle principal runtime: %v", err)
		}
		if chatLifecyclePrincipal != nil {
			defer func() { _ = chatLifecyclePrincipal.Close() }()
		}
		spaceLifecyclePrincipal, err = loadSpaceLifecyclePrincipalRuntime(context.Background())
		if err != nil {
			log.Fatalf("Space lifecycle principal runtime: %v", err)
		}
		if spaceLifecyclePrincipal != nil {
			defer func() { _ = spaceLifecyclePrincipal.Close() }()
		}
		gameAuthKeys, err := loadGameAuthKeys()
		if err != nil {
			log.Fatalf("game Auth key client: %v", err)
		}
		if gameAuthKeys != nil {
			defer gameAuthKeys.Close()
		}
		gameExecutionPermits, err := loadGameMessageExecutionPermitClient()
		if err != nil {
			log.Fatalf("game Auth execution permit client: %v", err)
		}
		if gameExecutionPermits != nil {
			defer gameExecutionPermits.Close()
		}
		gameResourceMappings, err := loadGameResourceMappingAuthorizationClient()
		if err != nil {
			log.Fatalf("game integration resource mapping client: %v", err)
		}
		if gameResourceMappings != nil {
			defer gameResourceMappings.Close()
		}
		ctx, cancel := context.WithTimeout(context.Background(), runtimeconfig.PostgresConnectTimeoutFromEnv())
		pool, err := pgxpool.New(ctx, dbURL)
		cancel()
		if err != nil {
			log.Fatalf("postgres: %v", err)
		}
		defer pool.Close()
		if spaceLifecyclePrincipal != nil || chatLifecyclePrincipal != nil {
			check, stop := context.WithTimeout(context.Background(), 5*time.Second)
			err := store.RequireAttachmentIntentSchema(check, pool)
			stop()
			if err != nil {
				log.Fatal(err)
			}
		}
		managedChatPurge, err := newManagedChatPurgeRuntime(context.Background(), pool)
		if err != nil {
			log.Fatalf("managed chat purge runtime: %v", err)
		}
		if managedChatPurge != nil {
			defer managedChatPurge.Close()
		}
		var tombstoneProcessor grpcsvc.GameTombstoneProcessor
		if tombstoneKeys != nil && moderationPrincipal != nil {
			tombstoneProcessor = &grpcsvc.VerifiedGameTombstoneProcessor{Store: &store.MessagesStore{Pool: pool}, Keys: tombstoneKeys}
		} else {
			logger.Warn("game tombstone RPC is fail-closed because its signing key or moderation principal runtime is not configured")
		}

		var chatMetaPool *pgxpool.Pool
		if chatDB := strings.TrimSpace(os.Getenv("CHAT_DATABASE_URL")); chatDB != "" {
			cctx, ccancel := context.WithTimeout(context.Background(), runtimeconfig.PostgresConnectTimeoutFromEnv())
			cp, err := pgxpool.New(cctx, chatDB)
			ccancel()
			if err != nil {
				log.Fatalf("chat postgres: %v", err)
			}
			chatMetaPool = cp
			defer chatMetaPool.Close()
		}

		var spaceMetaPool *pgxpool.Pool
		if spaceDB := strings.TrimSpace(os.Getenv("SPACE_DATABASE_URL")); spaceDB != "" {
			sctx, scancel := context.WithTimeout(context.Background(), runtimeconfig.PostgresConnectTimeoutFromEnv())
			sp, err := pgxpool.New(sctx, spaceDB)
			scancel()
			if err != nil {
				log.Fatalf("space postgres: %v", err)
			}
			spaceMetaPool = sp
			defer spaceMetaPool.Close()
		}

		var chatGuard grpcsvc.ChatGuard
		var chatTypeResolver grpcsvc.AuthoritativeChatTypeResolver
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
			chatClient := chatv1.NewChatServiceClient(cconn)
			chatGuard = s2s.NewGRPCChatGuard(chatClient)
			chatTypeResolver = s2s.NewGRPCChatTypeResolver(chatClient)
		} else if chatMetaPool != nil {
			chatGuard = &store.SQLChatGuard{Pool: chatMetaPool, SpacePool: spaceMetaPool}
			chatTypeResolver = &store.SQLChatTypeResolver{Pool: chatMetaPool}
		} else {
			chatGuard = &store.SQLChatGuard{Pool: pool, SpacePool: spaceMetaPool}
			chatTypeResolver = &store.SQLChatTypeResolver{Pool: pool}
		}

		var blocks grpcsvc.AccountPairBlockChecker
		var accountBlocks grpcsvc.AccountBlockChecker
		var profilePairBlocks grpcsvc.ProfilePairBlockChecker
		var friends grpcsvc.ProfileFriendChecker
		var spaceCoMembership grpcsvc.SpaceCoMembershipChecker
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
			socialBlocks := s2s.NewSocialGRPCBlocks(sconn)
			blocks = socialBlocks
			accountBlocks = socialBlocks
			profilePairBlocks = s2s.NewSocialGRPCProfileBlocks(socialv1.NewSocialServiceClient(sconn))
			friends = s2s.NewSocialGRPCFriends(sconn)
		}

		var profiles grpcsvc.ProfileAccountLookup
		var privacy grpcsvc.PrivacyChecker
		var userPresence *s2s.GRPCUserPresence
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
			userCli := userv1.NewUserServiceClient(uconn)
			profiles = &s2s.UserGRPCProfiles{Client: userCli}
			privacy = &s2s.GRPCUserPrivacy{Client: userCli}
			userPresence = &s2s.GRPCUserPresence{Client: userCli}
		}

		var deletedAccounts grpcsvc.AccountDeletedChecker
		if authAddr := strings.TrimSpace(os.Getenv("AUTH_GRPC_ADDR")); authAddr != "" {
			aconn, err := grpc.NewClient(grpcclient.DialTarget(authAddr), grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatalf("auth grpc: %v", err)
			}
			defer func() { _ = aconn.Close() }()
			waitCtx, waitCancel := context.WithTimeout(context.Background(), grpcclient.DialTimeoutFromEnv())
			if err := waitForGRPCReady(waitCtx, aconn); err != nil {
				waitCancel()
				log.Fatalf("auth grpc dial: %v", err)
			}
			waitCancel()
			deletedAccounts = s2s.NewAuthGRPCDeletedAccounts(authv1.NewAuthServiceClient(aconn))
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
			spaceCoMembership = s2s.NewGRPCSpaceCoMembership(spconn)
		}

		var rolePerms *s2s.GRPCRolePermissions
		if roleAddr := strings.TrimSpace(os.Getenv("ROLE_GRPC_ADDR")); roleAddr != "" {
			rconn, err := grpc.NewClient(grpcclient.DialTarget(roleAddr), grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatalf("role grpc: %v", err)
			}
			defer func() { _ = rconn.Close() }()
			waitCtx, waitCancel := context.WithTimeout(context.Background(), grpcclient.DialTimeoutFromEnv())
			if err := waitForGRPCReady(waitCtx, rconn); err != nil {
				waitCancel()
				log.Fatalf("role grpc dial: %v", err)
			}
			waitCancel()
			rolePerms = &s2s.GRPCRolePermissions{Client: rolev1.NewRoleServiceClient(rconn)}
		}

		var files grpcsvc.FileMetadataLookup
		var gameFiles grpcsvc.GameAttachmentManifestVerifier
		if fileAddr := strings.TrimSpace(os.Getenv("FILE_GRPC_ADDR")); fileAddr != "" {
			fconn, err := grpc.NewClient(grpcclient.DialTarget(fileAddr), grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatalf("file grpc: %v", err)
			}
			defer func() { _ = fconn.Close() }()
			waitCtx, waitCancel := context.WithTimeout(context.Background(), grpcclient.DialTimeoutFromEnv())
			if err := waitForGRPCReady(waitCtx, fconn); err != nil {
				waitCancel()
				log.Fatalf("file grpc dial: %v", err)
			}
			waitCancel()
			fileClient := filev1.NewFileServiceClient(fconn)
			files = s2s.NewFileGRPCMetadata(fileClient)
			gameFiles = &s2s.GameAttachmentManifestVerifier{Client: fileClient}
		}

		var msgEvents messageevents.MessageEventsPublisher
		if natsURL := strings.TrimSpace(os.Getenv("NATS_URL")); natsURL != "" {
			jsPub, err := messageevents.NewJetStreamPublisher(natsURL)
			if err != nil {
				log.Fatalf("nats jetstream publisher: %v", err)
			}
			defer func() { _ = jsPub.Close() }()
			if err := jsPub.EnsureStream(); err != nil {
				log.Fatalf("nats ensure message_events stream: %v", err)
			}
			jsPub.Logger = logger
			msgEvents = jsPub
			outboxDone := make(chan struct{})
			defer func() { runCancel(); <-outboxDone }()
			go func() {
				defer close(outboxDone)
				if err := (&messageevents.OutboxDispatcher{Store: &store.MessagesStore{Pool: pool}, Publisher: jsPub, Logger: logger}).Run(runCtx); err != nil && runCtx.Err() == nil {
					logger.Error("message event outbox dispatcher exited", slog.String("error", err.Error()))
				}
			}()
			go func() {
				if err := runDeliveryAckConsumer(runCtx, natsURL, &store.MessagesStore{Pool: pool}, &store.SQLChatThreadPolicy{Pool: chatMetaPool}, logger); err != nil && runCtx.Err() == nil {
					logger.Error("delivery ack consumer exited", slog.String("error", err.Error()))
				}
			}()
			targets, ok := chatGuard.(dmReceiptVisibilityResolver)
			if !ok {
				log.Fatalf("chat receipt visibility targets not configured")
			}
			go func() {
				if err := runReceiptPrivacyConsumer(runCtx, natsURL, &store.MessagesStore{Pool: pool}, targets, logger); err != nil && runCtx.Err() == nil {
					logger.Error("receipt privacy consumer exited", slog.String("error", err.Error()))
				}
			}()
		}

		var platformMod grpcsvc.PlatformModerationChecker
		if modAddr := strings.TrimSpace(os.Getenv("MODERATION_GRPC_ADDR")); modAddr != "" {
			mconn, err := grpc.NewClient(grpcclient.DialTarget(modAddr), grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatalf("moderation grpc: %v", err)
			}
			defer func() { _ = mconn.Close() }()
			waitCtx, waitCancel := context.WithTimeout(context.Background(), grpcclient.DialTimeoutFromEnv())
			if err := waitForGRPCReady(waitCtx, mconn); err != nil {
				waitCancel()
				log.Fatalf("moderation grpc dial: %v", err)
			}
			waitCancel()
			platformMod = &s2s.GRPCPlatformModeration{Client: moderationv1.NewModerationServiceClient(mconn)}
		}

		lis, err := net.Listen("tcp", grpcListen)
		if err != nil {
			log.Fatalf("grpc listen: %v", err)
		}
		messagingGRPCOptions := grpcmw.ServerOptions(logger, grpcmw.WithRegistry(metricsReg))
		grpcSrv = grpc.NewServer(append(messagingGRPCOptions, grpc.ChainUnaryInterceptor(
			principalgrpc.TombstoneUnaryInterceptor(moderationPrincipal),
			principalgrpc.ApplyGameMessageUnaryInterceptor(gatewayPrincipal),
			principalgrpc.SendGameEventMessageUnaryInterceptor(botPrincipal),
			principalgrpc.ChatLifecycleOrdinaryUnaryInterceptor(),
			principalgrpc.SpaceLifecycleOrdinaryUnaryInterceptor(),
			principalgrpc.GameIntegrationOrdinaryUnaryInterceptor(),
		))...)
		var chatThreadPolicy *store.SQLChatThreadPolicy
		if chatMetaPool != nil {
			chatThreadPolicy = &store.SQLChatThreadPolicy{Pool: chatMetaPool}
		} else {
			logger.Warn("CHAT_DATABASE_URL not set; thread policy checks disabled")
		}
		var gameMessages grpcsvc.GameMessageProcessor
		var gameMessagePermitProcessor *grpcsvc.VerifiedGameMessageProcessor
		if gameAuthKeys != nil && gameExecutionPermits != nil && chatGuard != nil {
			gameMessagePermitProcessor = &grpcsvc.VerifiedGameMessageProcessor{Store: &store.MessagesStore{Pool: pool}, AuthKeys: gameAuthKeys, Permits: gameExecutionPermits, Bindings: &grpcsvc.AuthBackedGameBindingAuthority{Chats: chatGuard, ResourceMappings: gameResourceMappings}, Files: gameFiles}
			gameMessages = gameMessagePermitProcessor
			go gameMessagePermitProcessor.RunGameMessagePermitCompletionDispatcher(context.Background(), logger)
			if gameResourceMappings == nil {
				logger.Warn("ApplyGameMessage new writes remain fail-closed until the GIS app/environment/binding-to-chat resource mapping client is configured; exact receipts remain readable")
			}
		} else {
			logger.Warn("ApplyGameMessage is fail-closed because Auth status keys, T16 execution-permit authority, or Chat membership authority is unavailable; exact receipts remain readable when storage is available")
		}
		messagingService := &grpcsvc.MessagingGRPC{
			Messages:              &store.MessagesStore{Pool: pool},
			SpacePurgeReceipts:    &store.MessagesStore{Pool: pool},
			SpaceManifestImporter: &store.MessagesStore{Pool: pool},
			SpaceLifecycleFences:  &store.MessagesStore{Pool: pool},
			SpaceLifecyclePurger:  &store.MessagesStore{Pool: pool},
			GameMessages:          gameMessages,
			GameTombstones:        tombstoneProcessor,
			Reactions:             &store.ReactionsStore{Pool: pool},
			Pins:                  &store.PinsStore{Pool: pool},
			SharedMedia:           &store.SharedMediaStore{Pool: pool},
			ChatGuard:             chatGuard,
			ChatTypeResolver:      chatTypeResolver,
			Blocks:                blocks,
			AccountBlocks:         accountBlocks,
			ProfilePairBlocks:     profilePairBlocks,
			UserProfiles:          profiles,
			DeletedAccounts:       deletedAccounts,
			Privacy:               privacy,
			Friends:               friends,
			SpaceCoMembership:     spaceCoMembership,
			Files:                 files,
			MessageEvents:         msgEvents,
			Moderation: &store.SQLModerationGuard{
				Pool:      pool,
				ChatPool:  chatMetaPool,
				MsgPool:   pool,
				SpacePool: spaceMetaPool,
			},
			ChatMentionsMeta: func() *store.SQLChatMentionsMeta {
				metaPool := pool
				if chatMetaPool != nil {
					metaPool = chatMetaPool
				}
				return &store.SQLChatMentionsMeta{Pool: metaPool, SpacePool: spaceMetaPool}
			}(),
			RolePermissions:     rolePerms,
			ChatRolePermissions: rolePerms,
			ChatThreadPolicy:    chatThreadPolicy,
			PreKeyBundles:       &store.E2EPreKeyStore{Pool: pool},
			UserPresence:        userPresence,
			PlatformMod:         platformMod,
			Logger:              logger,
			ManagedChatPurger: func() grpcsvc.ManagedChatPurgeProcessor {
				if managedChatPurge == nil {
					return nil
				}
				return managedChatPurge.Coordinator
			}(),
		}
		messagingv1.RegisterMessagingServiceServer(grpcSrv, messagingService)
		if managedChatPurge != nil {
			messagingService.SpaceFileProducer = &grpcsvc.SpaceFileProducerCoordinator{Store: &store.MessagesStore{Pool: pool}, Files: filev1.NewFileServiceClient(managedChatPurge.Connections[0]), Issuer: managedChatPurge.Coordinator.Issuer}
			messagingService.AttachmentReferences = &grpcsvc.AttachmentReferenceCoordinator{Files: filev1.NewFileServiceClient(managedChatPurge.Connections[0]), Issuer: managedChatPurge.Coordinator.Issuer}
			retentionCtx, retentionCancel := context.WithCancel(context.Background())
			retentionDone := make(chan struct{})
			defer func() { retentionCancel(); <-retentionDone }()
			go func() {
				defer close(retentionDone)
				ticker := time.NewTicker(time.Minute)
				defer ticker.Stop()
				for {
					if err := (&store.MessagesStore{Pool: pool}).ReconcileAttachmentIntents(retentionCtx, messagingService.AttachmentReferences.Release); err != nil && retentionCtx.Err() == nil {
						logger.Error("Messaging abandoned attachment recovery pass failed")
					}
					if err := (&store.MessagesStore{Pool: pool}).CleanupSpaceLifecycleEvidence(retentionCtx); err != nil && retentionCtx.Err() == nil {
						logger.Error("Messaging Space lifecycle retention pass failed")
					}
					select {
					case <-retentionCtx.Done():
						return
					case <-ticker.C:
					}
				}
			}()
		}
		if gameIntegrationPrincipal != nil {
			serverCertPath := strings.TrimSpace(os.Getenv("MESSAGING_GAME_INTEGRATION_SERVER_TLS_CERT_FILE"))
			serverKeyPath := strings.TrimSpace(os.Getenv("MESSAGING_GAME_INTEGRATION_SERVER_TLS_KEY_FILE"))
			clientCAPath := strings.TrimSpace(os.Getenv("MESSAGING_GAME_INTEGRATION_CLIENT_CA_FILE"))
			if serverCertPath == "" || serverKeyPath == "" || clientCAPath == "" {
				log.Fatal("GIS principal verifier requires the dedicated Messaging mTLS listener certificate and client CA")
			}
			certificate, err := tls.LoadX509KeyPair(serverCertPath, serverKeyPath)
			if err != nil {
				log.Fatalf("Messaging GIS listener certificate: %v", err)
			}
			clientCAPEM, err := os.ReadFile(clientCAPath)
			if err != nil {
				log.Fatalf("Messaging GIS listener client CA: %v", err)
			}
			clientCAs := x509.NewCertPool()
			if !clientCAs.AppendCertsFromPEM(clientCAPEM) {
				log.Fatal("Messaging GIS listener client CA is invalid")
			}
			protectedListen := strings.TrimSpace(os.Getenv("MESSAGING_GAME_INTEGRATION_GRPC_LISTEN"))
			if protectedListen == "" {
				protectedListen = ":9091"
			}
			protectedListener, err := net.Listen("tcp", protectedListen)
			if err != nil {
				log.Fatalf("Messaging GIS listener: %v", err)
			}
			// Reuse the shared middleware options so both listeners record into the
			// same collectors without registering duplicate Prometheus metrics.
			protectedOptions := append([]grpc.ServerOption(nil), messagingGRPCOptions...)
			protectedOptions = append(protectedOptions,
				grpc.Creds(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{certificate}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: clientCAs})),
				grpc.ChainUnaryInterceptor(principalgrpc.GameIntegrationStrictUnaryInterceptor(gameIntegrationPrincipal)),
			)
			gameIntegrationGRPCSrv = grpc.NewServer(protectedOptions...)
			messagingv1.RegisterMessagingServiceServer(gameIntegrationGRPCSrv, messagingService)
			go func() {
				logger.Info("Messaging GIS mTLS gRPC listening", slog.String("addr", protectedListen))
				if err := gameIntegrationGRPCSrv.Serve(protectedListener); err != nil {
					log.Fatalf("Messaging GIS gRPC serve: %v", err)
				}
			}()
		}
		spaceLifecycleListen := strings.TrimSpace(os.Getenv("MESSAGING_SPACE_LIFECYCLE_GRPC_LISTEN"))
		spaceLifecycleServerCert := strings.TrimSpace(os.Getenv("MESSAGING_SPACE_LIFECYCLE_SERVER_TLS_CERT_FILE"))
		spaceLifecycleServerKey := strings.TrimSpace(os.Getenv("MESSAGING_SPACE_LIFECYCLE_SERVER_TLS_KEY_FILE"))
		spaceLifecycleClientCA := strings.TrimSpace(os.Getenv("MESSAGING_SPACE_LIFECYCLE_CLIENT_CA_FILE"))
		if spaceLifecyclePrincipal != nil || spaceLifecycleListen != "" || spaceLifecycleServerCert != "" || spaceLifecycleServerKey != "" || spaceLifecycleClientCA != "" {
			if spaceLifecyclePrincipal == nil || spaceLifecycleServerCert == "" || spaceLifecycleServerKey == "" || spaceLifecycleClientCA == "" {
				log.Fatal("Space lifecycle listener requires Space principal verification and complete mTLS configuration")
			}
			certificate, err := tls.LoadX509KeyPair(spaceLifecycleServerCert, spaceLifecycleServerKey)
			if err != nil {
				log.Fatalf("Messaging Space lifecycle listener certificate: %v", err)
			}
			clientCAPEM, err := os.ReadFile(spaceLifecycleClientCA)
			if err != nil {
				log.Fatalf("Messaging Space lifecycle listener client CA: %v", err)
			}
			clientCAs := x509.NewCertPool()
			if !clientCAs.AppendCertsFromPEM(clientCAPEM) {
				log.Fatal("Messaging Space lifecycle listener client CA is invalid")
			}
			if spaceLifecycleListen == "" {
				spaceLifecycleListen = ":9093"
			}
			protectedListener, err := net.Listen("tcp", spaceLifecycleListen)
			if err != nil {
				log.Fatalf("Messaging Space lifecycle listener: %v", err)
			}
			protectedOptions := append([]grpc.ServerOption(nil), messagingGRPCOptions...)
			protectedOptions = append(protectedOptions,
				grpc.Creds(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{certificate}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: clientCAs})),
				grpc.ChainUnaryInterceptor(principalgrpc.SpaceLifecycleStrictUnaryInterceptor(spaceLifecyclePrincipal)),
			)
			spaceLifecycleGRPCSrv := grpc.NewServer(protectedOptions...)
			messagingv1.RegisterMessagingServiceServer(spaceLifecycleGRPCSrv, messagingService)
			defer spaceLifecycleGRPCSrv.Stop()
			go func() {
				logger.Info("Messaging Space lifecycle mTLS gRPC listening", slog.String("addr", spaceLifecycleListen))
				if err := spaceLifecycleGRPCSrv.Serve(protectedListener); err != nil {
					log.Fatalf("Messaging Space lifecycle gRPC serve: %v", err)
				}
			}()
		}
		chatLifecycleListen := strings.TrimSpace(os.Getenv("MESSAGING_CHAT_LIFECYCLE_GRPC_LISTEN"))
		chatLifecycleServerCert := strings.TrimSpace(os.Getenv("MESSAGING_CHAT_LIFECYCLE_SERVER_TLS_CERT_FILE"))
		chatLifecycleServerKey := strings.TrimSpace(os.Getenv("MESSAGING_CHAT_LIFECYCLE_SERVER_TLS_KEY_FILE"))
		chatLifecycleClientCA := strings.TrimSpace(os.Getenv("MESSAGING_CHAT_LIFECYCLE_CLIENT_CA_FILE"))
		if chatLifecyclePrincipal != nil || chatLifecycleListen != "" || chatLifecycleServerCert != "" || chatLifecycleServerKey != "" || chatLifecycleClientCA != "" {
			if chatLifecyclePrincipal == nil || chatLifecycleServerCert == "" || chatLifecycleServerKey == "" || chatLifecycleClientCA == "" {
				log.Fatal("Chat lifecycle listener requires Chat principal verification and complete mTLS configuration")
			}
			certificate, err := tls.LoadX509KeyPair(chatLifecycleServerCert, chatLifecycleServerKey)
			if err != nil {
				log.Fatalf("Messaging Chat lifecycle listener certificate: %v", err)
			}
			clientCAPEM, err := os.ReadFile(chatLifecycleClientCA)
			if err != nil {
				log.Fatalf("Messaging Chat lifecycle listener client CA: %v", err)
			}
			clientCAs := x509.NewCertPool()
			if !clientCAs.AppendCertsFromPEM(clientCAPEM) {
				log.Fatal("Messaging Chat lifecycle listener client CA is invalid")
			}
			if chatLifecycleListen == "" {
				chatLifecycleListen = ":9092"
			}
			protectedListener, err := net.Listen("tcp", chatLifecycleListen)
			if err != nil {
				log.Fatalf("Messaging Chat lifecycle listener: %v", err)
			}
			protectedOptions := append([]grpc.ServerOption(nil), messagingGRPCOptions...)
			protectedOptions = append(protectedOptions,
				grpc.Creds(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{certificate}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: clientCAs})),
				grpc.ChainUnaryInterceptor(principalgrpc.ChatLifecycleStrictUnaryInterceptor(chatLifecyclePrincipal)),
			)
			chatLifecycleGRPCSrv = grpc.NewServer(protectedOptions...)
			messagingv1.RegisterMessagingServiceServer(chatLifecycleGRPCSrv, messagingService)
			defer chatLifecycleGRPCSrv.Stop()
			go func() {
				logger.Info("Messaging Chat lifecycle mTLS gRPC listening", slog.String("addr", chatLifecycleListen))
				if err := chatLifecycleGRPCSrv.Serve(protectedListener); err != nil {
					log.Fatalf("Messaging Chat lifecycle gRPC serve: %v", err)
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
		if gameIntegrationGRPCSrv != nil {
			gameIntegrationGRPCSrv.GracefulStop()
		}
		if err := server.Shutdown(ctx); err != nil {
			log.Fatal(err)
		}
	}
}
