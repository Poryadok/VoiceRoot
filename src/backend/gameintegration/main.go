package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
	"voice/backend/gameintegration/internal/botproof"
	"voice/backend/gameintegration/internal/httpapi"
	"voice/backend/gameintegration/internal/registry"
	"voice/backend/gameintegration/internal/sessionowners"
	"voice/backend/pkg/httpserver"
	voicejwt "voice/backend/pkg/jwt"
	"voice/backend/pkg/principal"
	voiceprom "voice/backend/pkg/promhttp"
	"voice/backend/pkg/runtimeconfig"
)

func main() {
	cfg, err := loadConfig(os.Getenv)
	if err != nil {
		log.Fatal(err)
	}
	connectCtx, cancel := context.WithTimeout(context.Background(), runtimeconfig.PostgresConnectTimeoutFromEnv())
	pool, err := pgxpool.New(connectCtx, cfg.DatabaseURL)
	if err == nil {
		err = pool.Ping(connectCtx)
	}
	cancel()
	if err != nil {
		log.Fatalf("game integration database: %v", err)
	}
	defer pool.Close()
	redisClient := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	defer func() { _ = redisClient.Close() }()
	pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	err = redisClient.Ping(pingCtx).Err()
	cancel()
	if err != nil {
		log.Fatalf("game integration authority redis: %v", err)
	}

	logger := httpserver.NewLogger("gameintegration")
	metrics := prometheus.NewRegistry()
	jwtValidator := voicejwt.NewJWKSValidator(cfg.JWKSURL, cfg.JWTIssuer, cfg.JWTAudience)
	authorizer := httpapi.Authorizer{
		Tokens: jwtValidator,
		State:  httpapi.RedisAuthorityState{Client: redisClient},
	}
	var botAuthority registry.BotAuthorityVerifier
	if cfg.BotAuthorityURL != "" && len(cfg.BotWorkloadKey) == 32 {
		botAuthority = &botproof.Client{BaseURL: cfg.BotAuthorityURL, Key: cfg.BotWorkloadKey,
			Now: time.Now, NewNonce: func() string { return uuid.NewString() }}
	}
	applications := &registry.Store{Pool: pool, BotAuthority: botAuthority}
	mux := http.NewServeMux()
	api := httpapi.NewHandler(authorizer, applications)
	api.OperatorAccounts = cfg.OperatorAccounts
	api.CredentialKey = cfg.CredentialKey
	api.BindingExchanges = &httpapi.BindingExchangeHandler{Tokens: authorizer, Store: applications, Auth: cfg.AuthBindingClient}
	api.BindingRevocations = &httpapi.BindingRevocationHandler{Tokens: authorizer, Store: applications, Auth: cfg.AuthBindingClient}
	workerCtx, stopWorkers := context.WithCancel(context.Background())
	defer stopWorkers()
	var sessionOrchestrator *registry.SessionOrchestrator
	if cfg.SessionOwnersEnabled {
		signingKey, err := principal.LoadRSAPrivateKeyFile(cfg.PrincipalPrivateKeyFile)
		if err != nil {
			log.Fatalf("game integration principal signing key: %v", err)
		}
		nextSigningKey, err := principal.LoadRSAPrivateKeyFile(cfg.PrincipalNextPrivateKeyFile)
		if err != nil || nextSigningKey.N.BitLen() < 2048 || signingKey.PublicKey.Equal(&nextSigningKey.PublicKey) {
			log.Fatalf("game integration next principal signing key is invalid or not distinct")
		}
		issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gameintegration", KeyID: cfg.PrincipalKeyID, PrivateKey: signingKey})
		if err != nil {
			log.Fatalf("game integration principal issuer: %v", err)
		}
		ownerClients, err := sessionowners.New(cfg.SessionOwnerConfig, issuer)
		if err != nil {
			log.Fatalf("game integration session owner clients: %v", err)
		}
		defer func() { _ = ownerClients.Close() }()
		sessionOrchestrator = registry.NewSessionOrchestrator(applications, ownerClients.Adapters())
		go registry.RunSessionWorker(workerCtx, sessionOrchestrator, 250*time.Millisecond)
	}
	if cfg.AuthBindingClient != nil {
		go httpapi.RunBindingCompletionOutbox(workerCtx, applications, cfg.AuthBindingClient)
	}
	mux.Handle("/api/v1/game-integrations/", api)
	if sessionOrchestrator != nil {
		sessionHandler := httpapi.NewSessionHandler(httpapi.RegistrySessionCredentialVerifier{Store: applications, Key: cfg.CredentialKey}, sessionOrchestrator)
		mux.Handle("/api/v1/sessions", sessionHandler)
		mux.Handle("/api/v1/sessions/", sessionHandler)
		mux.Handle("/api/v1/operations/", sessionHandler)
		mux.Handle("/api/v1/session-events/", sessionHandler)
	}
	mux.Handle("/internal/v1/authorizations/environments/", httpapi.NewInternalPolicyHandler(
		httpapi.WorkloadVerifier{Key: cfg.AuthWorkloadKey, Now: time.Now,
			Nonces: httpapi.RedisNonceStore{Client: redisClient}}, applications))
	mux.Handle("/internal/v1/bindings/", httpapi.NewInternalBindingAuthorityHandler(
		httpapi.WorkloadVerifier{Key: cfg.AuthWorkloadKey, Now: time.Now,
			Nonces: httpapi.RedisNonceStore{Client: redisClient}}, applications))
	mux.Handle("/internal/v1/bindings/challenges/", httpapi.NewInternalBindingChallengeHandler(
		httpapi.WorkloadVerifier{Key: cfg.AuthWorkloadKey, Now: time.Now,
			Nonces: httpapi.RedisNonceStore{Client: redisClient}}, applications))
	mux.Handle("/internal/v1/bindings/challenges", &httpapi.InternalBindingChallengeCreateHandler{
		Verifier: httpapi.WorkloadVerifier{Key: cfg.AuthWorkloadKey, Now: time.Now,
			Nonces: httpapi.RedisNonceStore{Client: redisClient}}, Store: applications})
	permitHandler := httpapi.NewInternalExecutionPermitHandler(httpapi.WorkloadVerifier{Key: cfg.AuthWorkloadKey,
		Now: time.Now, Nonces: httpapi.RedisNonceStore{Client: redisClient}}, applications, applications)
	mux.Handle("/internal/v1/game-integrations/bindings/", permitHandler)
	mux.Handle("/internal/v1/game-integrations/execution-permits/", permitHandler)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"service": "gameintegration", "status": "ok"})
	})
	server := &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: httpserver.Wrap(voiceprom.MountMetricsOnHealth(mux, metrics), logger),
	}
	httpserver.ApplyHTTPServerTimeouts(server)
	servers := []*http.Server{server}
	var principalJWKS *http.Server
	if cfg.PrincipalJWKSListenAddr != "" {
		signingKey, err := principal.LoadRSAPrivateKeyFile(cfg.PrincipalPrivateKeyFile)
		if err != nil {
			log.Fatalf("game integration principal signing key: %v", err)
		}
		nextSigningKey, err := principal.LoadRSAPrivateKeyFile(cfg.PrincipalNextPrivateKeyFile)
		if err != nil || nextSigningKey.N.BitLen() < 2048 || signingKey.PublicKey.Equal(&nextSigningKey.PublicKey) {
			log.Fatalf("game integration next principal signing key is invalid or not distinct")
		}
		principalMux := http.NewServeMux()
		principalMux.Handle("/internal/v1/principal/jwks.json", principal.JWKSHandlerKeys([]principal.JWKSKey{
			{KeyID: cfg.PrincipalKeyID, PublicKey: &signingKey.PublicKey},
			{KeyID: cfg.PrincipalNextKeyID, PublicKey: &nextSigningKey.PublicKey},
		}))
		principalJWKS = &http.Server{Addr: cfg.PrincipalJWKSListenAddr,
			Handler: httpserver.Wrap(principalMux, logger), ReadHeaderTimeout: 5 * time.Second}
	}
	var privateListener net.Listener
	if cfg.MessagingListenAddr != "" {
		privateMux := http.NewServeMux()
		privateMux.Handle("/internal/v1/game-integrations/resource-mappings/authorize-chat",
			httpapi.NewInternalResourceMappingAuthorizationHandler(httpapi.WorkloadVerifier{
				Key: cfg.MessagingWorkloadKey, PreviousKey: cfg.MessagingPreviousWorkloadKey,
				PreviousKeyUntil: cfg.MessagingPreviousKeyUntil, Principal: "messaging", Now: time.Now,
				Nonces: httpapi.RedisNonceStore{Client: redisClient},
			}, applications))
		privateServer, listener, err := newMessagingPrivateServer(cfg, privateMux)
		if err != nil {
			log.Fatal(err)
		}
		privateListener = listener
		servers = append(servers, privateServer)
	}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	errorsCh := make(chan error, len(servers)+1)
	for index, current := range servers {
		go func(index int, current *http.Server) {
			if index == 1 {
				errorsCh <- current.Serve(privateListener)
			} else {
				errorsCh <- current.ListenAndServe()
			}
		}(index, current)
	}
	if principalJWKS != nil {
		go func() {
			logger.Info("principal JWKS TLS listener started", "addr", principalJWKS.Addr)
			errorsCh <- principalJWKS.ListenAndServeTLS(cfg.PrincipalJWKSTLSCertFile, cfg.PrincipalJWKSTLSKeyFile)
		}()
	}
	select {
	case err := <-errorsCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	case <-stop:
		shutdownCtx, cancel := context.WithTimeout(context.Background(), runtimeconfig.ShutdownTimeoutFromEnv())
		defer cancel()
		for _, current := range servers {
			if err := current.Shutdown(shutdownCtx); err != nil {
				log.Fatal(err)
			}
		}
		if principalJWKS != nil {
			if err := principalJWKS.Shutdown(shutdownCtx); err != nil {
				log.Fatal(err)
			}
		}
	}
}
