package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
	"voice/backend/gameintegration/internal/httpapi"
	"voice/backend/gameintegration/internal/registry"
	"voice/backend/pkg/httpserver"
	voicejwt "voice/backend/pkg/jwt"
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
	applications := &registry.Store{Pool: pool}
	mux := http.NewServeMux()
	api := httpapi.NewHandler(authorizer, applications)
	api.OperatorAccounts = cfg.OperatorAccounts
	api.CredentialKey = cfg.CredentialKey
	mux.Handle("/api/v1/game-integrations/", api)
	mux.Handle("/internal/v1/authorizations/environments/", httpapi.NewInternalPolicyHandler(
		httpapi.WorkloadVerifier{Key: cfg.AuthWorkloadKey, Now: time.Now,
			Nonces: httpapi.RedisNonceStore{Client: redisClient}}, applications))
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"service": "gameintegration", "status": "ok"})
	})
	server := &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: httpserver.Wrap(voiceprom.MountMetricsOnHealth(mux, metrics), logger),
	}
	httpserver.ApplyHTTPServerTimeouts(server)
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	errorsCh := make(chan error, 1)
	go func() { errorsCh <- server.ListenAndServe() }()
	select {
	case err := <-errorsCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	case <-stop:
		shutdownCtx, cancel := context.WithTimeout(context.Background(), runtimeconfig.ShutdownTimeoutFromEnv())
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Fatal(err)
		}
	}
}
