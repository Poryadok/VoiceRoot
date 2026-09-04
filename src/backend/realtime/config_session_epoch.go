package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"

	"voice/backend/pkg/httpserver"
	voicejwt "voice/backend/pkg/jwt"
)

type realtimeConfig struct {
	tokenValidator     tokenValidator
	sessionEpochStrict bool
	sessionEpochFloor  sessionEpochFloor
}

type realtimeStartupDependencies struct {
	buildHandler func(realtimeConfig) (http.Handler, error)
	buildServer  func(http.Handler) *http.Server
}

func loadRealtimeConfigFromEnvChecked() (realtimeConfig, error) {
	strict, err := realtimeSessionEpochStrictFromEnv()
	if err != nil {
		return realtimeConfig{}, err
	}

	jwksURL := strings.TrimSpace(os.Getenv("REALTIME_JWKS_URL"))
	issuer := firstNonEmpty(os.Getenv("REALTIME_JWT_ISSUER"), os.Getenv("GATEWAY_JWT_ISSUER"))
	audience := firstNonEmpty(os.Getenv("REALTIME_JWT_AUDIENCE"), os.Getenv("GATEWAY_JWT_AUDIENCE"))
	if !strict && jwksURL == "" {
		jwksURL = strings.TrimSpace(os.Getenv("GATEWAY_JWKS_URL"))
	}
	if strict {
		if jwksURL == "" {
			return realtimeConfig{}, errors.New("REALTIME_JWKS_URL is required when REALTIME_SESSION_EPOCH_STRICT=true")
		}
		if issuer == "" {
			return realtimeConfig{}, errors.New("REALTIME_JWT_ISSUER is required when REALTIME_SESSION_EPOCH_STRICT=true")
		}
		if audience == "" {
			return realtimeConfig{}, errors.New("REALTIME_JWT_AUDIENCE is required when REALTIME_SESSION_EPOCH_STRICT=true")
		}
	}

	config := realtimeConfig{sessionEpochStrict: strict}
	if jwksURL != "" {
		config.tokenValidator = voicejwt.NewJWKSValidator(
			jwksURL,
			issuer,
			audience,
			voicejwt.WithSessionEpochRequired(strict),
		)
	}
	if strict {
		redisAddr := strings.TrimSpace(os.Getenv("REALTIME_REDIS_ADDR"))
		if redisAddr == "" {
			return realtimeConfig{}, errors.New("REALTIME_REDIS_ADDR is required when REALTIME_SESSION_EPOCH_STRICT=true")
		}
		config.sessionEpochFloor = newRedisSessionEpochFloor(redisAddr, os.Getenv("REALTIME_REDIS_PASSWORD"))
	}
	return config, nil
}

func realtimeSessionEpochStrictFromEnv() (bool, error) {
	value, ok := os.LookupEnv("REALTIME_SESSION_EPOCH_STRICT")
	if !ok {
		return false, nil
	}
	switch value {
	case "false":
		return false, nil
	case "true":
		return true, nil
	default:
		return false, fmt.Errorf("REALTIME_SESSION_EPOCH_STRICT must be exactly \"true\" or \"false\", got %q", value)
	}
}

func newRealtimeServerFromEnv(addr string, dependencies realtimeStartupDependencies) (*http.Server, error) {
	config, err := loadRealtimeConfigFromEnvChecked()
	if err != nil {
		return nil, err
	}
	if config.sessionEpochStrict {
		return nil, errors.New("REALTIME_SESSION_EPOCH_STRICT=true requires full Realtime session epoch enforcement, which is not installed")
	}
	if dependencies.buildHandler == nil {
		return nil, errors.New("realtime handler builder is nil")
	}
	if dependencies.buildServer == nil {
		return nil, errors.New("realtime server builder is nil")
	}
	handler, err := dependencies.buildHandler(config)
	if err != nil {
		return nil, err
	}
	if handler == nil {
		return nil, errors.New("realtime handler builder returned nil")
	}
	server := dependencies.buildServer(handler)
	if server == nil {
		return nil, errors.New("realtime server builder returned nil")
	}
	if server.Addr == "" {
		server.Addr = addr
	}
	httpserver.ApplyHTTPServerTimeouts(server)
	return server, nil
}

type realtimeHandlerSlot struct {
	mu      sync.RWMutex
	handler http.Handler
}

func (s *realtimeHandlerSlot) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	handler := s.handler
	s.mu.RUnlock()
	if handler == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "realtime_initializing")
		return
	}
	handler.ServeHTTP(w, r)
}

func (s *realtimeHandlerSlot) set(handler http.Handler) {
	s.mu.Lock()
	s.handler = handler
	s.mu.Unlock()
}
