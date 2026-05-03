package main

import (
	"log"
	"net/http"
	"os"
	"strings"

	voicecfg "voice/backend/pkg/config"
	voicejwt "voice/backend/pkg/jwt"
)

func loadGatewayConfigFromEnv() gatewayConfig {
	config := gatewayConfig{
		versionConfigs: map[string]versionConfig{},
		tokenClaims:    map[string]tokenClaims{},
		restUpstreams:  map[string]http.Handler{},
	}
	loadJSONEnv("GATEWAY_VERSION_CONFIGS_JSON", &config.versionConfigs)
	loadJSONEnv("GATEWAY_FORCE_UPDATE_JSON", &config.forceUpdate)
	if strings.EqualFold(os.Getenv("GATEWAY_AUTH_MODE"), "static") {
		loadJSONEnv("GATEWAY_STATIC_TOKENS_JSON", &config.tokenClaims)
		config.tokenValidator = staticTokenValidator(config.tokenClaims)
	} else if jwksURL := strings.TrimSpace(os.Getenv("GATEWAY_JWKS_URL")); jwksURL != "" {
		config.tokenValidator = voicejwt.NewJWKSValidator(jwksURL, os.Getenv("GATEWAY_JWT_ISSUER"), os.Getenv("GATEWAY_JWT_AUDIENCE"))
	}
	config.restUpstreams = restUpstreamsFromEnv()
	config.realtimeUpstream = proxyFromEnv("GATEWAY_REALTIME_UPSTREAM_URL")
	if redisAddr := strings.TrimSpace(os.Getenv("GATEWAY_REDIS_ADDR")); redisAddr != "" {
		password := os.Getenv("GATEWAY_REDIS_PASSWORD")
		config.rateLimiter = newRedisSlidingWindowLimiter(redisAddr, password, defaultRateLimitRules())
		config.tokenBlacklist = newRedisTokenBlacklist(redisAddr, password, os.Getenv("GATEWAY_JWT_BLACKLIST_PREFIX"))
	} else if strings.EqualFold(os.Getenv("GATEWAY_IN_MEMORY_RATE_LIMITS"), "true") {
		config.rateLimiter = newSlidingWindowLimiter(defaultRateLimitRules())
	}
	config.trustedProxyCIDRs = voicecfg.SplitCSV(os.Getenv("GATEWAY_TRUSTED_PROXY_CIDRS"))
	config.cors = corsConfig{
		AllowedOrigins: voicecfg.SplitCSV(os.Getenv("GATEWAY_CORS_ALLOWED_ORIGINS")),
		AllowedHeaders: voicecfg.SplitCSV(os.Getenv("GATEWAY_CORS_ALLOWED_HEADERS")),
		AllowedMethods: voicecfg.SplitCSV(os.Getenv("GATEWAY_CORS_ALLOWED_METHODS")),
	}
	return config
}

func loadJSONEnv(name string, dst any) {
	voicecfg.LoadJSONEnv(name, dst, func(name string, err error) {
		log.Printf("invalid %s: %v", name, err)
	})
}

func restUpstreamsFromEnv() map[string]http.Handler {
	upstreams := map[string]http.Handler{}
	var urls map[string]string
	loadJSONEnv("GATEWAY_REST_UPSTREAMS_JSON", &urls)
	for namespace, rawURL := range urls {
		if !isPublicRESTNamespace(namespace) || namespace == "analytics" && rawURL == "" {
			continue
		}
		if proxy := reverseProxy(rawURL); proxy != nil {
			upstreams[namespace] = proxy
		}
	}
	for _, namespace := range publicRESTNamespaces() {
		envName := "GATEWAY_" + strings.ToUpper(namespace) + "_UPSTREAM_URL"
		if proxy := proxyFromEnv(envName); proxy != nil {
			upstreams[namespace] = proxy
		}
	}
	return upstreams
}
