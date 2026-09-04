package main

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"

	voicecfg "voice/backend/pkg/config"
	"voice/backend/pkg/httpserver"
	voicejwt "voice/backend/pkg/jwt"
	voicelog "voice/backend/pkg/logging"
)

func loadGatewayConfigFromEnv() gatewayConfig {
	config, err := loadGatewayConfigFromEnvChecked()
	if err == nil {
		return config
	}
	logger := voicelog.NewJSONLogger(voicelog.LevelFromEnv(), slog.String("service", "gateway"))
	logger.Warn("invalid session epoch configuration; using compatibility mode", slog.Any("error", err))
	return loadGatewayConfigFromEnvMode(false)
}

func loadGatewayConfigFromEnvChecked() (gatewayConfig, error) {
	strict, err := sessionEpochStrictFromEnv()
	if err != nil {
		return gatewayConfig{}, err
	}
	if strict {
		if strings.TrimSpace(os.Getenv("GATEWAY_REDIS_ADDR")) == "" {
			return gatewayConfig{}, errors.New("GATEWAY_REDIS_ADDR is required when GATEWAY_SESSION_EPOCH_STRICT=true")
		}
	}
	return loadGatewayConfigFromEnvMode(strict), nil
}

func sessionEpochStrictFromEnv() (bool, error) {
	value, ok := os.LookupEnv("GATEWAY_SESSION_EPOCH_STRICT")
	if !ok {
		return false, nil
	}
	switch value {
	case "false":
		return false, nil
	case "true":
		return true, nil
	default:
		return false, fmt.Errorf("GATEWAY_SESSION_EPOCH_STRICT must be exactly \"true\" or \"false\", got %q", value)
	}
}

func loadGatewayConfigFromEnvMode(strict bool) gatewayConfig {
	logger := voicelog.NewJSONLogger(voicelog.LevelFromEnv(), slog.String("service", "gateway"))
	config := gatewayConfig{
		versionConfigs:     map[string]versionConfig{},
		tokenClaims:        map[string]tokenClaims{},
		restUpstreams:      map[string]http.Handler{},
		slogLogger:         logger,
		sessionEpochStrict: strict,
	}
	loadJSONEnv(logger, "GATEWAY_VERSION_CONFIGS_JSON", &config.versionConfigs)
	loadJSONEnv(logger, "GATEWAY_FORCE_UPDATE_JSON", &config.forceUpdate)
	loadJSONEnv(logger, "GATEWAY_STATIC_TOKENS_JSON", &config.tokenClaims)
	static := staticTokenValidator(config.tokenClaims)
	if strings.EqualFold(os.Getenv("GATEWAY_AUTH_MODE"), "static") {
		config.tokenValidator = static
	} else if jwksURL := strings.TrimSpace(os.Getenv("GATEWAY_JWKS_URL")); jwksURL != "" {
		jwks := voicejwt.NewJWKSValidator(jwksURL, os.Getenv("GATEWAY_JWT_ISSUER"), os.Getenv("GATEWAY_JWT_AUDIENCE"), voicejwt.WithSessionEpochRequired(strict))
		if len(static) > 0 {
			config.tokenValidator = chainedTokenValidator{static: static, next: jwks}
		} else {
			config.tokenValidator = jwks
		}
	}
	config.restUpstreams = restUpstreamsFromEnv(logger)
	config.transcoder = newTranscoder(grpcClientsFromEnv(logger))
	config.realtimeUpstream = proxyFromEnv("GATEWAY_REALTIME_UPSTREAM_URL", logger)
	if dbURL := strings.TrimSpace(os.Getenv("GATEWAY_DATABASE_URL")); dbURL != "" {
		if db, err := sql.Open("pgx", dbURL); err == nil {
			config.versionStore = versionStoreFromEnv(config.versionConfigs, db)
		} else {
			logger.Warn("invalid GATEWAY_DATABASE_URL", slog.Any("error", err))
		}
	}
	if redisAddr := strings.TrimSpace(os.Getenv("GATEWAY_REDIS_ADDR")); redisAddr != "" {
		password := os.Getenv("GATEWAY_REDIS_PASSWORD")
		if strict {
			config.sessionEpochFloor = newRedisSessionEpochFloor(redisAddr, password)
		}
		config.versionCacheRedis = redisAddr
		config.rateLimiter = newRedisSlidingWindowLimiter(redisAddr, password, rateLimitRulesFromEnv(logger))
		config.tokenBlacklist = newRedisTokenBlacklist(redisAddr, password, os.Getenv("GATEWAY_JWT_BLACKLIST_PREFIX"))
		ticketPrefix := strings.TrimSpace(os.Getenv("GATEWAY_WS_TICKET_PREFIX"))
		config.wsTicketStore = newRedisWsTicketStore(redisAddr, password, ticketPrefix)
		config.analyticsAudit = newRedisAnalyticsAuditStore(redisAddr, password)
	} else if strings.EqualFold(os.Getenv("GATEWAY_IN_MEMORY_RATE_LIMITS"), "true") {
		config.rateLimiter = newSlidingWindowLimiter(rateLimitRulesFromEnv(logger))
	}
	if config.wsTicketStore == nil {
		config.wsTicketStore = newMemoryWsTicketStore()
	}
	config.trustedProxyCIDRs = voicecfg.SplitCSV(os.Getenv("GATEWAY_TRUSTED_PROXY_CIDRS"))
	config.cors = corsConfig{
		AllowedOrigins: voicecfg.SplitCSV(os.Getenv("GATEWAY_CORS_ALLOWED_ORIGINS")),
		AllowedHeaders: voicecfg.SplitCSV(os.Getenv("GATEWAY_CORS_ALLOWED_HEADERS")),
		AllowedMethods: voicecfg.SplitCSV(os.Getenv("GATEWAY_CORS_ALLOWED_METHODS")),
	}
	config.analyticsTelemetry = gatewayAnalyticsFromEnv()
	return config
}

func newGatewayServerFromEnv(addr string, factory func(http.Handler) *http.Server) (*http.Server, error) {
	config, err := loadGatewayConfigFromEnvChecked()
	if err != nil {
		return nil, err
	}
	if factory == nil {
		return nil, errors.New("gateway server factory is nil")
	}
	server := factory(newGateway(config))
	if server == nil {
		return nil, errors.New("gateway server factory returned nil")
	}
	if server.Addr == "" {
		server.Addr = addr
	}
	httpserver.ApplyHTTPServerTimeouts(server)
	return server, nil
}

func loadJSONEnv(logger *slog.Logger, name string, dst any) {
	voicecfg.LoadJSONEnv(name, dst, func(name string, err error) {
		if logger != nil {
			logger.Warn("invalid JSON env", slog.String("env", name), slog.Any("error", err))
		}
	})
}

func restUpstreamsFromEnv(logger *slog.Logger) map[string]http.Handler {
	upstreams := map[string]http.Handler{}
	var urls map[string]string
	loadJSONEnv(logger, "GATEWAY_REST_UPSTREAMS_JSON", &urls)
	for namespace, rawURL := range urls {
		if !isPublicRESTNamespace(namespace) || namespace == "analytics" && rawURL == "" {
			continue
		}
		if proxy := reverseProxy(rawURL, logger); proxy != nil {
			upstreams[namespace] = proxy
		}
	}
	for _, namespace := range publicRESTNamespaces() {
		envName := "GATEWAY_" + strings.ToUpper(namespace) + "_UPSTREAM_URL"
		if proxy := proxyFromEnv(envName, logger); proxy != nil {
			upstreams[namespace] = proxy
		}
	}
	return upstreams
}
