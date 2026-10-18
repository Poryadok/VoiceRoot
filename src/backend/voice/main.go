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
	"sync/atomic"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"voice/backend/pkg/grpcclient"
	"voice/backend/pkg/grpcmw"
	"voice/backend/pkg/httpserver"
	voicepostgres "voice/backend/pkg/postgres"
	"voice/backend/pkg/principal"
	voiceprom "voice/backend/pkg/promhttp"
	"voice/backend/pkg/runtimeconfig"
	"voice/backend/pkg/spacecommunication"
	"voice/backend/voice/internal/federationmedia"
	"voice/backend/voice/internal/gameprincipal"
	"voice/backend/voice/internal/gameprovision"
	grpcsvc "voice/backend/voice/internal/grpcsvc"
	"voice/backend/voice/internal/livekit"
	"voice/backend/voice/internal/matchsquad"
	"voice/backend/voice/internal/matchsquadmemberprincipal"
	"voice/backend/voice/internal/matchsquadprincipal"
	"voice/backend/voice/internal/ordinarycall"
	"voice/backend/voice/internal/principalgrpc"
	"voice/backend/voice/internal/principaljwks"
	"voice/backend/voice/internal/rolegrant"
	"voice/backend/voice/internal/s2s"
	"voice/backend/voice/internal/sessionfloor"
	"voice/backend/voice/internal/spacelifecycle"
	"voice/backend/voice/internal/spacemedia"
	"voice/backend/voice/internal/spaceprincipalruntime"
	voicestore "voice/backend/voice/internal/store"
	"voice/backend/voice/internal/voiceevents"
	"voice/backend/voice/internal/voiceuserprincipalruntime"

	callsv1 "voice.app/voice/calls/v1"
	chatv1 "voice.app/voice/chat/v1"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	subscriptionv1 "voice.app/voice/subscription/v1"
	userv1 "voice.app/voice/user/v1"
)

const serviceName = "voice"

func main() {
	logger := newServiceLogger()
	metricsReg := prometheus.NewRegistry()
	addr := ":8080"
	if v := os.Getenv("LISTEN_ADDR"); v != "" {
		addr = v
	}
	grpcListen := ":9090"
	if v := strings.TrimSpace(os.Getenv("VOICE_GRPC_LISTEN")); v != "" {
		grpcListen = v
	}

	var grpcSrv *grpc.Server
	runCtx, runCancel := context.WithCancel(context.Background())
	defer runCancel()

	lifecycleStore, lifecyclePool, closeLifecycleDatabase, lifecycleEnabled, err := openLifecycleDatabase(runCtx)
	if err != nil {
		log.Fatalf("voice lifecycle database: %v", err)
	}
	defer closeLifecycleDatabase()
	var lifecycleReadiness func(context.Context) error
	if lifecycleEnabled {
		lifecycleReadiness = lifecycleStore.CheckSchema
	}

	gamePrincipalConfig, gamePrincipalEnabled, err := gameprincipal.LoadFromEnv()
	if err != nil {
		log.Fatalf("voice GIS principal configuration: %v", err)
	}
	principalJWKSConfig, principalJWKSEnabled, err := principaljwks.LoadFromEnv(os.Getenv)
	if err != nil {
		log.Fatalf("voice principal JWKS configuration: %v", err)
	}
	roleGrantConfig, roleGrantEnabled, err := rolegrant.LoadFromEnv(os.Getenv)
	if err != nil {
		log.Fatalf("voice Role grant checker configuration: %v", err)
	}
	communicationConfig, communicationEnabled, err := spacecommunication.LoadClientConfig(os.LookupEnv, "VOICE_SPACE_COMMUNICATION_")
	if err != nil {
		log.Fatalf("Voice Space communication client configuration: %v", err)
	}
	spacePrincipalConfig, spacePrincipalEnabled, err := spaceprincipalruntime.LoadFromEnv()
	if err != nil {
		log.Fatalf("voice Space lifecycle principal configuration: %v", err)
	}
	userPrincipalConfig, userPrincipalEnabled, err := voiceuserprincipalruntime.LoadFromEnv()
	if err != nil {
		log.Fatalf("voice user-principal configuration: %v", err)
	}
	authFloorConfig, authFloorEnabled, err := sessionfloor.ConfigFromEnv()
	if err != nil {
		log.Fatalf("voice Auth session-floor configuration: %v", err)
	}
	if userPrincipalEnabled != authFloorEnabled {
		log.Fatal("Voice user-principal listener requires the private Auth session-floor client")
	}
	var spaceLifecycleStore *spacelifecycle.PostgresStore
	var spaceLifecycleRuntime *spaceprincipalruntime.Runtime
	var spaceLifecycleController grpcsvc.SpaceLifecycleController
	if spacePrincipalEnabled {
		if !lifecycleEnabled || lifecyclePool == nil {
			log.Fatal("Voice Space lifecycle principal requires VOICE_DATABASE_URL")
		}
		spaceLifecycleStore = spacelifecycle.NewPostgresStore(lifecyclePool, spacemedia.BeginPartyRoomClose)
		if err := spaceLifecycleStore.CheckSchema(runCtx); err != nil {
			log.Fatalf("voice Space lifecycle database schema: %v", err)
		}
		media := livekit.NewSDKRoomLifecycle(
			strings.TrimSpace(os.Getenv("LIVEKIT_URL")),
			strings.TrimSpace(os.Getenv("LIVEKIT_API_KEY")),
			strings.TrimSpace(os.Getenv("LIVEKIT_API_SECRET")),
		)
		spaceLifecycleController = &spacelifecycle.Service{
			Store:   spaceLifecycleStore,
			Effects: spacelifecycle.NewEffects(lifecyclePool, media),
		}
		spaceLifecycleRuntime, err = spaceprincipalruntime.New(runCtx, spacePrincipalConfig)
		if err != nil {
			log.Fatalf("voice Space lifecycle principal runtime: %v", err)
		}
		defer func() { _ = spaceLifecycleRuntime.Close() }()
		lifecycleReadiness = func(ctx context.Context) error {
			if err := lifecycleStore.CheckSchema(ctx); err != nil {
				return err
			}
			return spaceLifecycleStore.CheckSchema(ctx)
		}
	}
	if gamePrincipalEnabled && !roleGrantEnabled {
		log.Fatal("Voice managed game-session admission requires both GIS provisioning and Role grant checker configuration")
	}
	if (roleGrantEnabled || userPrincipalEnabled || communicationEnabled) && !principalJWKSEnabled {
		log.Fatal("Voice Role grants and user-principal listener require the Voice service principal signer")
	}
	var voiceIssuer *principal.Issuer
	if roleGrantEnabled || userPrincipalEnabled || communicationEnabled {
		voiceIssuer, err = principal.NewIssuer(principal.IssuerConfig{
			Issuer: "voice", KeyID: principalJWKSConfig.KeyID, PrivateKey: principalJWKSConfig.SigningKey,
		})
		if err != nil {
			log.Fatalf("voice principal issuer: %v", err)
		}
	}
	var userPrincipalRuntime *voiceuserprincipalruntime.Runtime
	var sessionEpochChecker *sessionfloor.Client
	if userPrincipalEnabled {
		epochClient, epochConn, clientErr := authFloorConfig.Dial(voiceIssuer)
		if clientErr != nil {
			log.Fatalf("voice Auth session-floor client: %v", clientErr)
		}
		defer func() { _ = epochConn.Close() }()
		sessionEpochChecker = epochClient
		userPrincipalRuntime, err = voiceuserprincipalruntime.New(runCtx, userPrincipalConfig, epochClient)
		if err != nil {
			log.Fatalf("voice user-principal runtime: %v", err)
		}
		defer func() { _ = userPrincipalRuntime.Close() }()
	}
	var callStore voicestore.CallStore
	var callProjectionRedis *redis.Client
	callStoreDurable := false
	if redisAddr := strings.TrimSpace(os.Getenv("VOICE_REDIS_ADDR")); redisAddr != "" {
		rdb := redis.NewClient(&redis.Options{
			Addr:     redisAddr,
			Password: strings.TrimSpace(os.Getenv("VOICE_REDIS_PASSWORD")),
		})
		callProjectionRedis = rdb
		defer func() { _ = rdb.Close() }()
		callStore = voicestore.NewRedisCallStore(rdb, strings.TrimSpace(os.Getenv("VOICE_REDIS_PREFIX")))
		callStoreDurable = true
	} else {
		callStore = voicestore.NewMemoryCallStore()
		logger.Warn("VOICE_REDIS_ADDR not set; using in-memory call store")
	}
	ordinaryCalls, err := openOrdinaryRuntime(runCtx, callStore)
	if err != nil {
		log.Fatalf("ordinary Voice startup: %v", err)
	}
	if ordinaryCalls != nil {
		defer ordinaryCalls.pool.Close()
	}
	if ordinaryCalls != nil {
		priorReadiness := lifecycleReadiness
		lifecycleReadiness = func(ctx context.Context) error {
			if priorReadiness != nil {
				if err := priorReadiness(ctx); err != nil {
					return err
				}
			}
			return ordinaryCalls.Store.CheckSchema(ctx)
		}
	}
	matchSquadConfig, matchSquadEnabled, err := matchsquadprincipal.LoadFromEnv(os.LookupEnv)
	if err != nil {
		log.Fatalf("voice MatchSquad principal configuration: %v", err)
	}
	matchSquadMemberConfig, matchSquadMemberEnabled, err := matchsquadmemberprincipal.LoadFromEnv(os.LookupEnv)
	if err != nil {
		log.Fatalf("voice MatchSquad member principal configuration: %v", err)
	}
	if matchSquadEnabled != matchSquadMemberEnabled {
		log.Fatal("Voice MatchSquad provisioning and delegated member listeners must be enabled together")
	}
	var matchSquadRuntime *matchsquadprincipal.Runtime
	var matchSquadMemberRuntime *matchsquadmemberprincipal.Runtime
	var matchSquadController *matchsquad.Service
	var matchSquadServer *grpc.Server
	var matchSquadListener net.Listener
	var matchSquadMemberServer *grpc.Server
	var matchSquadMemberListener net.Listener
	if matchSquadEnabled {
		if !lifecycleEnabled || lifecyclePool == nil || callProjectionRedis == nil {
			log.Fatal("Voice MatchSquad requires the Voice lifecycle database and durable Redis projection")
		}
		matchSquadRuntime, err = matchsquadprincipal.New(runCtx, matchSquadConfig)
		if err != nil {
			log.Fatalf("voice MatchSquad principal runtime: %v", err)
		}
		defer func() { _ = matchSquadRuntime.Close() }()
		matchSquadController = &matchsquad.Service{
			Pool: lifecyclePool, Calls: callStore,
			Effects: livekit.NewSDKRoomLifecycle(strings.TrimSpace(os.Getenv("LIVEKIT_URL")), strings.TrimSpace(os.Getenv("LIVEKIT_API_KEY")), strings.TrimSpace(os.Getenv("LIVEKIT_API_SECRET"))),
			Now:     time.Now,
		}
		if err := matchSquadController.CheckSchema(runCtx); err != nil {
			log.Fatalf("voice MatchSquad database schema: %v", err)
		}
		matchSquadListener, err = net.Listen("tcp", matchSquadConfig.ListenerAddr)
		if err != nil {
			log.Fatalf("voice MatchSquad listener: %v", err)
		}
		matchSquadServer = grpc.NewServer(matchSquadRuntime.ServerOptions()...)
		callsv1.RegisterMatchSquadVoiceServiceServer(matchSquadServer, &grpcsvc.MatchSquadVoiceGRPC{Controller: matchSquadController})
		go func() {
			logger.Info("MatchSquad protected listener started", slog.String("addr", matchSquadConfig.ListenerAddr))
			if serveErr := matchSquadServer.Serve(matchSquadListener); serveErr != nil {
				logger.Error("MatchSquad protected listener stopped", slog.String("error", serveErr.Error()))
			}
		}()
	}
	var gamePrincipalRuntime *gameprincipal.Runtime
	var gameProvisionServer *grpc.Server
	var gameProvisionListener net.Listener
	var managedGameSessionRooms grpcsvc.ManagedGameSessionRoomLookup
	var managedGameSessionGrants grpcsvc.ManagedGameSessionGrantChecker
	var voiceRoomGrantResolver s2s.VoiceRoomGrantResolver
	var conversionAdmission grpcsvc.SdkConversionAdmissionGuard
	var accountVoiceFences gameprovision.AccountVoiceFenceStore = gameprovision.UnavailableAccountVoiceFenceStore{}
	var accountVoiceProfiles grpcsvc.AccountVoiceProfileResolver
	if roleGrantEnabled {
		checker, roleConn, clientErr := rolegrant.New(roleGrantConfig, voiceIssuer)
		if clientErr != nil {
			log.Fatalf("voice Role grant checker: %v", clientErr)
		}
		defer func() { _ = roleConn.Close() }()
		voiceRoomGrantResolver = checker
		if gamePrincipalEnabled {
			managedGameSessionGrants = checker
		}
	}
	if gamePrincipalEnabled {
		gamePrincipalRuntime, err = gameprincipal.New(runCtx, gamePrincipalConfig)
		if err != nil {
			log.Fatalf("voice GIS principal runtime: %v", err)
		}
		defer func() { _ = gamePrincipalRuntime.Close() }()
		dsn := strings.TrimSpace(os.Getenv("VOICE_DATABASE_URL"))
		if dsn == "" {
			log.Fatal("VOICE_DATABASE_URL is required when GIS room provisioning is enabled")
		}
		pool, poolErr := voicepostgres.NewPool(runCtx, dsn)
		if poolErr != nil {
			log.Fatalf("voice GIS database: %v", poolErr)
		}
		defer pool.Close()
		gameStore := gameprovision.NewPostgresStore(pool)
		if ordinaryCalls != nil {
			gameStore.EnableOrdinaryConversions()
		}
		if err := gameStore.CheckSchema(runCtx); err != nil {
			log.Fatalf("voice GIS database schema: %v", err)
		}
		accountVoiceFences = gameprovision.NewPostgresAccountVoiceFenceStore(pool)
		if err := gameprovision.ApplyAccountVoiceFenceSchema(runCtx, pool); err != nil {
			log.Fatalf("voice account fence schema: %v", err)
		}
		managedGameSessionRooms = gameStore
		conversionAdmission = gameStore
		roomCloser := livekit.NewSDKRoomLifecycle(strings.TrimSpace(os.Getenv("LIVEKIT_URL")),
			strings.TrimSpace(os.Getenv("LIVEKIT_API_KEY")), strings.TrimSpace(os.Getenv("LIVEKIT_API_SECRET")))
		mediaFencer := gameprovision.NewLiveKitRoomFencer(gameStore, roomCloser)
		gameProvisionListener, err = net.Listen("tcp", gamePrincipalConfig.ListenAddr)
		if err != nil {
			log.Fatalf("voice GIS listener: %v", err)
		}
		gameProvisionServer = grpc.NewServer(gamePrincipalRuntime.ServerOptions()...)
		provisionService := &grpcsvc.GameSessionProvisioningGRPC{
			Store: gameStore, Roster: gameStore, Closer: gameStore, Fencer: mediaFencer,
			Conversion: gameStore, Calls: callStore, CallCloser: roomCloser, Now: time.Now,
		}
		if ordinaryCalls != nil {
			provisionService.OrdinaryCalls = ordinaryCalls.Coordinator
		}
		callsv1.RegisterGameSessionProvisioningServiceServer(gameProvisionServer, provisionService)
		go func() {
			logger.Info("GIS room provisioning listener started", slog.String("addr", gamePrincipalConfig.ListenAddr))
			if serveErr := gameProvisionServer.Serve(gameProvisionListener); serveErr != nil {
				logger.Error("GIS room provisioning listener stopped", slog.String("error", serveErr.Error()))
			}
		}()
		go func() {
			ticker := time.NewTicker(200 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-runCtx.Done():
					return
				case <-ticker.C:
					fenceCtx, cancel := context.WithTimeout(runCtx, 4*time.Second)
					if err := gameStore.FenceExpiredGameSessionLeases(fenceCtx, mediaFencer); err != nil {
						logger.Warn("expired managed game session media fence failed", slog.String("error", err.Error()))
					}
					cancel()
				}
			}
		}()
	} else if dsn := strings.TrimSpace(os.Getenv("VOICE_DATABASE_URL")); dsn != "" {
		pool, poolErr := voicepostgres.NewPool(runCtx, dsn)
		if poolErr != nil {
			logger.Warn("managed game session lookup unavailable", slog.String("error", poolErr.Error()))
		} else {
			gameStore := gameprovision.NewPostgresStore(pool)
			if ordinaryCalls != nil {
				gameStore.EnableOrdinaryConversions()
			}
			if schemaErr := gameStore.CheckSchema(runCtx); schemaErr != nil {
				pool.Close()
				logger.Warn("managed game session lookup schema unavailable", slog.String("error", schemaErr.Error()))
			} else {
				defer pool.Close()
				managedGameSessionRooms = gameStore
				conversionAdmission = gameStore
				if schemaErr := gameprovision.ApplyAccountVoiceFenceSchema(runCtx, pool); schemaErr != nil {
					logger.Warn("account-wide Voice admission unavailable", slog.String("error", schemaErr.Error()))
				} else {
					accountVoiceFences = gameprovision.NewPostgresAccountVoiceFenceStore(pool)
				}
			}
		}
	}

	var events voiceevents.Publisher
	if natsURL := strings.TrimSpace(os.Getenv("NATS_URL")); natsURL != "" {
		jsPub, err := voiceevents.NewJetStreamPublisher(natsURL)
		if err != nil {
			log.Fatalf("nats jetstream publisher: %v", err)
		}
		defer func() { _ = jsPub.Close() }()
		jsPub.Logger = logger
		events = jsPub
	}

	var chatMembers grpcsvc.ChatMembership
	if chatAddr := strings.TrimSpace(os.Getenv("CHAT_GRPC_ADDR")); chatAddr != "" {
		cconn, err := grpc.NewClient(grpcclient.DialTarget(chatAddr), grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Fatalf("chat grpc: %v", err)
		}
		defer func() { _ = cconn.Close() }()
		chatMembers = s2s.NewGRPCChatMembership(chatv1.NewChatServiceClient(cconn))
	}

	var callPrivacy grpcsvc.CallPrivacyChecker
	var callFriends grpcsvc.CallProfileFriendChecker
	var callSpaceCoMembership grpcsvc.CallSpaceCoMembershipChecker
	var spaceMembers grpcsvc.SpaceMembership
	var voiceRoomAccessResolver grpcsvc.AuthoritativeVoiceRoomAccessResolver
	var spacePro grpcsvc.SpaceProLookup
	var rolePerms grpcsvc.RolePermissionChecker
	if userAddr := strings.TrimSpace(os.Getenv("USER_GRPC_ADDR")); userAddr != "" {
		uconn, err := grpc.NewClient(grpcclient.DialTarget(userAddr), grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Fatalf("user grpc: %v", err)
		}
		defer func() { _ = uconn.Close() }()
		userClient := userv1.NewUserServiceClient(uconn)
		callPrivacy = &s2s.GRPCUserPrivacy{Client: userClient}
		accountVoiceProfiles = &s2s.GRPCProfileAccountResolver{Client: userClient}
	}
	if socialAddr := strings.TrimSpace(os.Getenv("SOCIAL_GRPC_ADDR")); socialAddr != "" {
		sconn, err := grpc.NewClient(grpcclient.DialTarget(socialAddr), grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Fatalf("social grpc: %v", err)
		}
		defer func() { _ = sconn.Close() }()
		callFriends = s2s.NewGRPCSocialFriends(sconn)
	}
	if spaceAddr := strings.TrimSpace(os.Getenv("SPACE_GRPC_ADDR")); spaceAddr != "" {
		spconn, err := grpc.NewClient(grpcclient.DialTarget(spaceAddr), grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Fatalf("space grpc: %v", err)
		}
		defer func() { _ = spconn.Close() }()
		spaceCli := spacev1.NewSpaceServiceClient(spconn)
		callSpaceCoMembership = s2s.NewGRPCSpaceCoMembership(spconn)
		spaceMembers = s2s.NewGRPCSpaceMembership(spaceCli)
	}
	if communicationEnabled {
		conn, err := communicationConfig.Dial()
		if err != nil {
			log.Fatalf("Voice Space communication mTLS: %v", err)
		}
		defer func() { _ = conn.Close() }()
		voiceRoomAccessResolver = s2s.NewGRPCVoiceRoomAccessResolver(spacev1.NewSpaceServiceClient(conn), voiceIssuer)
	}
	if subAddr := strings.TrimSpace(os.Getenv("SUBSCRIPTION_GRPC_ADDR")); subAddr != "" {
		subconn, err := grpc.NewClient(grpcclient.DialTarget(subAddr), grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Fatalf("subscription grpc: %v", err)
		}
		defer func() { _ = subconn.Close() }()
		spacePro = s2s.NewGRPCSpacePro(subscriptionv1.NewSubscriptionServiceClient(subconn))
	}
	if roleAddr := strings.TrimSpace(os.Getenv("ROLE_GRPC_ADDR")); roleAddr != "" {
		rconn, err := grpc.NewClient(grpcclient.DialTarget(roleAddr), grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Fatalf("role grpc: %v", err)
		}
		defer func() { _ = rconn.Close() }()
		roleClient := rolev1.NewRoleServiceClient(rconn)
		rolePerms = s2s.NewGRPCRolePermissions(roleClient)
	}
	if matchSquadMemberEnabled {
		if matchSquadController == nil || lifecyclePool == nil || accountVoiceProfiles == nil {
			log.Fatal("Voice MatchSquad member admission requires the MatchSquad provider, lifecycle database, and User profile resolver")
		}
		if err := gameprovision.ApplyAccountVoiceFenceSchema(runCtx, lifecyclePool); err != nil {
			log.Fatalf("voice MatchSquad account fence schema: %v", err)
		}
		accountVoiceFences = gameprovision.NewPostgresAccountVoiceFenceStore(lifecyclePool)
		matchSquadMemberRuntime, err = matchsquadmemberprincipal.New(runCtx, matchSquadMemberConfig)
		if err != nil {
			log.Fatalf("voice MatchSquad member principal runtime: %v", err)
		}
		defer func() { _ = matchSquadMemberRuntime.Close() }()
		memberMedia := livekit.NewSDKRoomLifecycle(strings.TrimSpace(os.Getenv("LIVEKIT_URL")), strings.TrimSpace(os.Getenv("LIVEKIT_API_KEY")), strings.TrimSpace(os.Getenv("LIVEKIT_API_SECRET")))
		memberService := &matchsquad.MatchSquadMemberService{
			Pool: lifecyclePool, Calls: callStore, Fences: accountVoiceFences, Profiles: accountVoiceProfiles,
			Principal: matchSquadMemberRuntime, Tokens: livekit.NewHS256TokenIssuer(
				strings.TrimSpace(os.Getenv("LIVEKIT_API_KEY")), strings.TrimSpace(os.Getenv("LIVEKIT_API_SECRET")),
				strings.TrimSpace(os.Getenv("LIVEKIT_URL")), 60*time.Second,
			),
			RoomEffect: memberMedia, Now: time.Now,
		}
		if err := memberService.CheckSchema(runCtx); err != nil {
			log.Fatalf("voice MatchSquad member database schema: %v", err)
		}
		matchSquadController.Members = memberService
		matchSquadMemberListener, err = net.Listen("tcp", matchSquadMemberConfig.ListenerAddr)
		if err != nil {
			log.Fatalf("voice MatchSquad member listener: %v", err)
		}
		matchSquadMemberServer = grpc.NewServer(matchSquadMemberRuntime.ServerOptions()...)
		callsv1.RegisterMatchSquadMemberServiceServer(matchSquadMemberServer, &grpcsvc.MatchSquadMemberGRPC{Controller: memberService})
		go func() {
			logger.Info("MatchSquad delegated member listener started", slog.String("addr", matchSquadMemberConfig.ListenerAddr))
			if serveErr := matchSquadMemberServer.Serve(matchSquadMemberListener); serveErr != nil {
				logger.Error("MatchSquad member listener stopped", slog.String("error", serveErr.Error()))
			}
		}()
		go func() {
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-runCtx.Done():
					return
				case <-ticker.C:
					repairCtx, cancel := context.WithTimeout(runCtx, 4*time.Second)
					if err := memberService.RepairExpiredLeaves(repairCtx, 64); err != nil {
						logger.Warn("expired MatchSquad leave repair failed", slog.String("error", err.Error()))
					}
					cancel()
				}
			}
		}()
	}

	if service, ok := spaceLifecycleController.(*spacelifecycle.Service); ok {
		fence, ok := callStore.(interface {
			SetSpaceLifecycle(context.Context, string, int64, bool) error
		})
		if !ok || !callStoreDurable {
			log.Fatal("P3 requires durable lifecycle-fenced Redis")
		}
		service.Effects.ProjectionFence = fence
		service.Effects.NativeDrain = &spacemedia.Coordinator{Calls: callStore, Store: callStore, Admissions: spacemedia.NewPostgresAdmissionStore(lifecyclePool)}
	}
	tokenTTL := time.Hour
	spaceMediaAdmissions := spacemedia.NewPostgresAdmissionStore(lifecyclePool)
	federatedMedia, err := federationmedia.LoadFromEnv(os.Getenv)
	if err != nil {
		log.Fatal("federated media configuration invalid")
	}
	if federatedMedia != nil {
		defer federatedMedia.Close()
	}
	voiceSvc := &grpcsvc.VoiceGRPC{
		Calls:                    callStore,
		ManagedGameSessionRooms:  managedGameSessionRooms,
		ManagedGameSessionGrants: managedGameSessionGrants,
		SdkConversionAdmission:   conversionAdmission,
		AccountVoiceFences:       accountVoiceFences,
		AccountVoiceProfiles:     accountVoiceProfiles,
		ChatMembers:              chatMembers,
		SpaceMembers:             spaceMembers,
		VoiceRoomAccessResolver:  voiceRoomAccessResolver,
		SpaceVoiceRoomGrants:     voiceRoomGrantResolver,
		SpaceMediaAdmissions:     spaceMediaAdmissions,
		SpaceTokens: livekit.NewSpaceTokenIssuer(
			strings.TrimSpace(os.Getenv("LIVEKIT_API_KEY")),
			strings.TrimSpace(os.Getenv("LIVEKIT_API_SECRET")),
			strings.TrimSpace(os.Getenv("LIVEKIT_URL")),
			60*time.Second,
		),
		SpaceLifecycle:    spaceLifecycleController,
		SpacePro:          spacePro,
		Roles:             rolePerms,
		Privacy:           callPrivacy,
		Friends:           callFriends,
		SpaceCoMembership: callSpaceCoMembership,
		Tokens: livekit.NewHS256TokenIssuer(
			strings.TrimSpace(os.Getenv("LIVEKIT_API_KEY")),
			strings.TrimSpace(os.Getenv("LIVEKIT_API_SECRET")),
			strings.TrimSpace(os.Getenv("LIVEKIT_URL")),
			tokenTTL,
		),
		Events:              events,
		RingTimeout:         30 * time.Second,
		Logger:              logger,
		SessionEpochChecker: sessionEpochChecker,
	}
	if ordinaryCalls != nil {
		if conversionAdmission == nil || accountVoiceProfiles == nil || chatMembers == nil {
			log.Fatal("ordinary Voice authority dependencies unavailable")
		}
		voiceSvc.OrdinaryCalls = ordinaryCalls.Coordinator
		publisher, ok := events.(ordinarycall.OutboxPublisher)
		if !ok {
			log.Fatal("ordinary Voice durable publisher unavailable")
		}
		relay := &ordinarycall.OutboxRelay{Store: ordinaryCalls.Store, Publisher: publisher, OnError: func(err error) {
			logger.Warn("ordinary Voice outbox delivery failed", slog.String("error", err.Error()))
		}}
		go func() {
			if err := relay.Run(runCtx); err != nil && runCtx.Err() == nil {
				logger.Error("ordinary Voice outbox stopped", slog.String("error", err.Error()))
			}
		}()
		go func() {
			if err := ordinaryCalls.Coordinator.RunRecovery(runCtx, ordinaryCalls.Store, func(err error) { logger.Warn("ordinary Voice recovery failed", slog.String("error", err.Error())) }); err != nil && runCtx.Err() == nil {
				logger.Error("ordinary Voice recovery stopped", slog.String("error", err.Error()))
			}
		}()
	}
	if federatedMedia != nil {
		voiceSvc.FederatedMedia = federatedMedia
	}
	if natsURL := strings.TrimSpace(os.Getenv("NATS_URL")); natsURL != "" {
		if sessionEpochChecker == nil {
			log.Fatal("Space media admission requires the current Auth session-floor checker")
		}
		if !callStoreDurable {
			log.Fatal("Space media invalidation requires the durable Voice Redis call store")
		}
		if err := voiceSvc.SpaceMediaAdmissions.CheckSchema(runCtx); err != nil {
			log.Fatalf("Space media PostgreSQL admission schema: %v", err)
		}
		if voiceRoomAccessResolver == nil || voiceRoomGrantResolver == nil {
			log.Fatal("Space media invalidation requires Space and Role authority resolvers")
		}
		mediaLifecycle := livekit.NewSDKRoomLifecycle(
			strings.TrimSpace(os.Getenv("LIVEKIT_URL")),
			strings.TrimSpace(os.Getenv("LIVEKIT_API_KEY")),
			strings.TrimSpace(os.Getenv("LIVEKIT_API_SECRET")),
		)
		projectionGates, ok := callStore.(voicestore.SpaceProjectionGates)
		if !ok {
			log.Fatal("Space requires an incarnation-fenced projection store")
		}
		voiceSvc.SpaceProjectionGates = projectionGates
		spaceMediaServing := &atomic.Bool{}
		voiceSvc.SpaceMediaServing = spaceMediaServing
		spaceMediaCoordinator := &spacemedia.Coordinator{
			ProjectionGates: projectionGates, Store: callStore, Calls: callStore, Admissions: spaceMediaAdmissions,
			SessionEpochChecker: sessionEpochChecker,
			Access:              voiceRoomAccessResolver, Grants: voiceRoomGrantResolver, Media: mediaLifecycle,
			Every: spacemedia.ReconcileInterval,
			OnError: func(err error) {
				logger.Warn("Space media authority reconciliation failed", slog.String("error", err.Error()))
			},
			OnReadiness: spaceMediaServing.Store,
		}
		if service, ok := spaceLifecycleController.(*spacelifecycle.Service); ok {
			service.Effects.NativeDrain = spaceMediaCoordinator
			fence, ok := callStore.(interface {
				SetSpaceLifecycle(context.Context, string, int64, bool) error
			})
			if !ok {
				log.Fatal("P3 requires lifecycle-fenced Redis projection")
			}
			service.Effects.ProjectionFence = fence
			// Protected P3 has one independently wired exact-drain recovery loop.
		}
		voiceSvc.SpaceMediaRevoker = spaceMediaCoordinator
		invalidationConsumer, err := spacemedia.StartInvalidationConsumer(runCtx, natsURL, spaceMediaCoordinator, logger)
		if err != nil {
			log.Fatalf("Space media invalidation consumer startup: %v", err)
		}
		defer func() { _ = invalidationConsumer.Close() }()
		if err := spaceMediaCoordinator.ReconcileAll(runCtx); err != nil {
			log.Fatalf("Space media initial reconciliation: %v", err)
		}
		admissionPublisher, ok := events.(spacemedia.AdmissionOutboxPublisher)
		if !ok {
			log.Fatal("Space media durable event publisher is not configured")
		}
		admissionRelay := &spacemedia.AdmissionOutboxRelay{

			Store: spaceMediaAdmissions, Publisher: admissionPublisher,
			OnError: func(err error) {
				logger.Warn("Space media admission outbox delivery failed", slog.String("error", err.Error()))
			},
		}
		go runRosterOutbox(runCtx, lifecyclePool, admissionPublisher, logger)
		go func() {
			if err := admissionRelay.Run(runCtx); err != nil && runCtx.Err() == nil {
				logger.Error("Space media admission outbox relay stopped", slog.String("error", err.Error()))
			}
		}()
		go func() {
			if err := spaceMediaCoordinator.Run(runCtx); err != nil && runCtx.Err() == nil {
				logger.Error("Space media authority reconciler stopped", slog.String("error", err.Error()))
			}
		}()
	}
	// Publish P3 recovery only after every optional consumer has final bindings.
	if configured, ok := spaceLifecycleController.(*spacelifecycle.Service); ok {
		service := configured.RecoverySnapshot()
		go func() {
			ticker := time.NewTicker(spacemedia.ReconcileInterval)
			defer ticker.Stop()
			for {
				select {
				case <-runCtx.Done():
					return
				case <-ticker.C:
					ctx, cancel := context.WithTimeout(runCtx, 4*time.Second)
					if err := service.ResumePending(ctx); err != nil {
						logger.Warn("P3 exact drain pending", slog.String("error", err.Error()))
					}
					cancel()
				}
			}
		}()
	}
	lis, err := net.Listen("tcp", grpcListen)
	if err != nil {
		log.Fatalf("grpc listen: %v", err)
	}
	ordinaryOptions := grpcmw.ServerOptions(logger, grpcmw.WithRegistry(metricsReg))
	ordinaryOptions = append(ordinaryOptions, grpc.ChainUnaryInterceptor(principalgrpc.OrdinaryUnaryInterceptor()))
	grpcSrv = grpc.NewServer(ordinaryOptions...)
	callsv1.RegisterVoiceServiceServer(grpcSrv, voiceSvc)
	go func() {
		logger.Info("gRPC listening", slog.String("addr", grpcListen))
		if err := grpcSrv.Serve(lis); err != nil {
			log.Fatalf("grpc serve: %v", err)
		}
	}()
	var spaceLifecycleServer *grpc.Server
	var spaceLifecycleListener net.Listener
	if spaceLifecycleRuntime != nil {
		spaceLifecycleListener, err = net.Listen("tcp", spacePrincipalConfig.ListenAddr)
		if err != nil {
			log.Fatalf("voice Space lifecycle principal listen: %v", err)
		}
		protectedOptions := grpcmw.ServerOptions(logger, grpcmw.WithRegistry(metricsReg))
		protectedOptions = append(protectedOptions, spaceLifecycleRuntime.ServerOptions()...)
		spaceLifecycleServer = grpc.NewServer(protectedOptions...)
		callsv1.RegisterVoiceServiceServer(spaceLifecycleServer, voiceSvc)
		go func() {
			logger.Info("Space lifecycle principal listener started", slog.String("addr", spacePrincipalConfig.ListenAddr))
			if serveErr := spaceLifecycleServer.Serve(spaceLifecycleListener); serveErr != nil {
				logger.Error("Space lifecycle principal listener stopped", slog.String("error", serveErr.Error()))
			}
		}()
		go runSpaceLifecycleReceiptSweeper(runCtx, spaceLifecycleStore, logger)
	}
	var userPrincipalServer *grpc.Server
	var userPrincipalListener net.Listener
	if userPrincipalRuntime != nil {
		userPrincipalListener, err = net.Listen("tcp", userPrincipalConfig.ListenAddr)
		if err != nil {
			log.Fatalf("voice user-principal listener: %v", err)
		}
		userPrincipalServer = grpc.NewServer(append(grpcmw.ServerOptions(logger, grpcmw.WithRegistry(metricsReg)), userPrincipalRuntime.ServerOptions()...)...)
		callsv1.RegisterVoiceServiceServer(userPrincipalServer, voiceSvc)
		go func() {
			logger.Info("Voice user-principal listener started", slog.String("addr", userPrincipalConfig.ListenAddr))
			if serveErr := userPrincipalServer.Serve(userPrincipalListener); serveErr != nil {
				logger.Error("Voice user-principal listener stopped", slog.String("error", serveErr.Error()))
			}
		}()
	}
	go runMissedCallSweeper(runCtx, voiceSvc, logger)

	server := &http.Server{
		Addr:    addr,
		Handler: httpserver.Wrap(voiceprom.MountMetricsOnHealth(healthHandler(serviceName, lifecycleReadiness), metricsReg), logger),
	}
	httpserver.ApplyHTTPServerTimeouts(server)
	var principalJWKSServer *http.Server
	if principalJWKSEnabled {
		mux := http.NewServeMux()
		mux.Handle("/internal/v1/principal/jwks.json", principal.JWKSHandlerKeys([]principal.JWKSKey{
			{KeyID: principalJWKSConfig.KeyID, PublicKey: &principalJWKSConfig.SigningKey.PublicKey},
			{KeyID: principalJWKSConfig.NextKeyID, PublicKey: &principalJWKSConfig.NextSigningKey.PublicKey},
		}))
		principalJWKSServer = &http.Server{Addr: principalJWKSConfig.ListenAddr, Handler: httpserver.Wrap(mux, logger), ReadHeaderTimeout: 5 * time.Second}
	}
	errCh := make(chan error, 2)
	logger.Info("listening", slog.String("addr", addr))
	go func() {
		errCh <- server.ListenAndServe()
	}()
	if principalJWKSServer != nil {
		go func() {
			logger.Info("principal JWKS TLS listener started", slog.String("addr", principalJWKSServer.Addr))
			errCh <- principalJWKSServer.ListenAndServeTLS(principalJWKSConfig.TLSCert, principalJWKSConfig.TLSKey)
		}()
	}

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
		if err := server.Shutdown(ctx); err != nil {
			log.Fatal(err)
		}
		if principalJWKSServer != nil {
			if err := principalJWKSServer.Shutdown(ctx); err != nil {
				log.Fatal(err)
			}
		}
		if grpcSrv != nil {
			grpcSrv.GracefulStop()
		}
		if spaceLifecycleServer != nil {
			spaceLifecycleServer.GracefulStop()
		}
		if userPrincipalServer != nil {
			userPrincipalServer.GracefulStop()
		}
		if spaceLifecycleListener != nil {
			_ = spaceLifecycleListener.Close()
		}
		if userPrincipalListener != nil {
			_ = userPrincipalListener.Close()
		}
		if gameProvisionServer != nil {
			gameProvisionServer.GracefulStop()
		}
		if gameProvisionListener != nil {
			_ = gameProvisionListener.Close()
		}
		if matchSquadServer != nil {
			matchSquadServer.GracefulStop()
		}
		if matchSquadListener != nil {
			_ = matchSquadListener.Close()
		}
		if matchSquadMemberServer != nil {
			matchSquadMemberServer.GracefulStop()
		}
		if matchSquadMemberListener != nil {
			_ = matchSquadMemberListener.Close()
		}
	}
}

func runMissedCallSweeper(ctx context.Context, svc *grpcsvc.VoiceGRPC, logger *slog.Logger) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := svc.MarkExpiredCallsMissed(ctx); err != nil {
				logger.Error("voice missed-call sweeper", slog.String("error", err.Error()))
			}
		}
	}
}

func runSpaceLifecycleReceiptSweeper(ctx context.Context, store *spacelifecycle.PostgresStore, logger *slog.Logger) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sweepCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			if err := store.ExpireReceipts(sweepCtx, time.Now().UTC().Add(-30*24*time.Hour)); err != nil {
				logger.Error("Voice Space lifecycle receipt retention", slog.String("error", err.Error()))
			}
			cancel()
		}
	}
}
