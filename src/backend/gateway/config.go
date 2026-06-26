package main

import (
	"database/sql"
	"log/slog"
	"net/http"
	"os"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"

	voicecfg "voice/backend/pkg/config"
	voicejwt "voice/backend/pkg/jwt"
	voicelog "voice/backend/pkg/logging"
)

func loadGatewayConfigFromEnv() gatewayConfig {
	logger := voicelog.NewJSONLogger(voicelog.LevelFromEnv(), slog.String("service", "gateway"))
	config := gatewayConfig{
		versionConfigs: map[string]versionConfig{},
		tokenClaims:    map[string]tokenClaims{},
		restUpstreams:  map[string]http.Handler{},
		slogLogger:     logger,
	}
	loadJSONEnv(logger, "GATEWAY_VERSION_CONFIGS_JSON", &config.versionConfigs)
	loadJSONEnv(logger, "GATEWAY_FORCE_UPDATE_JSON", &config.forceUpdate)
	loadJSONEnv(logger, "GATEWAY_STATIC_TOKENS_JSON", &config.tokenClaims)
	static := staticTokenValidator(config.tokenClaims)
	if strings.EqualFold(os.Getenv("GATEWAY_AUTH_MODE"), "static") {
		config.tokenValidator = static
	} else if jwksURL := strings.TrimSpace(os.Getenv("GATEWAY_JWKS_URL")); jwksURL != "" {
		jwks := voicejwt.NewJWKSValidator(jwksURL, os.Getenv("GATEWAY_JWT_ISSUER"), os.Getenv("GATEWAY_JWT_AUDIENCE"))
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
		config.versionCacheRedis = redisAddr
		config.rateLimiter = newRedisSlidingWindowLimiter(redisAddr, password, rateLimitRulesFromEnv(logger))
		config.tokenBlacklist = newRedisTokenBlacklist(redisAddr, password, os.Getenv("GATEWAY_JWT_BLACKLIST_PREFIX"))
	} else if strings.EqualFold(os.Getenv("GATEWAY_IN_MEMORY_RATE_LIMITS"), "true") {
		config.rateLimiter = newSlidingWindowLimiter(rateLimitRulesFromEnv(logger))
	}
	config.trustedProxyCIDRs = voicecfg.SplitCSV(os.Getenv("GATEWAY_TRUSTED_PROXY_CIDRS"))
	config.cors = corsConfig{
		AllowedOrigins: voicecfg.SplitCSV(os.Getenv("GATEWAY_CORS_ALLOWED_ORIGINS")),
		AllowedHeaders: voicecfg.SplitCSV(os.Getenv("GATEWAY_CORS_ALLOWED_HEADERS")),
		AllowedMethods: voicecfg.SplitCSV(os.Getenv("GATEWAY_CORS_ALLOWED_METHODS")),
	}
	return config
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
