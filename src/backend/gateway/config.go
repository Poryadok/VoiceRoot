package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
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
		config.tokenValidator = newJWTValidator(jwksURL, os.Getenv("GATEWAY_JWT_ISSUER"), os.Getenv("GATEWAY_JWT_AUDIENCE"))
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
	config.trustedProxyCIDRs = splitCSV(os.Getenv("GATEWAY_TRUSTED_PROXY_CIDRS"))
	config.cors = corsConfig{
		AllowedOrigins: splitCSV(os.Getenv("GATEWAY_CORS_ALLOWED_ORIGINS")),
		AllowedHeaders: splitCSV(os.Getenv("GATEWAY_CORS_ALLOWED_HEADERS")),
		AllowedMethods: splitCSV(os.Getenv("GATEWAY_CORS_ALLOWED_METHODS")),
	}
	return config
}

func loadJSONEnv(name string, dst any) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return
	}
	if err := json.Unmarshal([]byte(raw), dst); err != nil {
		log.Printf("invalid %s: %v", name, err)
	}
}

func splitCSV(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			values = append(values, part)
		}
	}
	return values
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
