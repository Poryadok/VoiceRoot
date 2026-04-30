package main

import (
	"net/http"
	"strings"
)

type gatewayConfig struct {
	versionConfigs     map[string]versionConfig
	forceUpdate        *forceUpdatePolicy
	tokenClaims        map[string]tokenClaims
	rateLimitedGroups  map[string]bool
	restUpstreams      map[string]http.Handler
	realtimeUpstream   http.Handler
	requestIDGenerator func() string
	tokenValidator     tokenValidator
	rateLimiter        rateLimiter
}

type gateway struct {
	config         gatewayConfig
	tokenValidator tokenValidator
	rateLimiter    rateLimiter
}

func handler() http.Handler {
	return newGateway(loadGatewayConfigFromEnv())
}

func newGateway(config gatewayConfig) http.Handler {
	if config.versionConfigs == nil {
		config.versionConfigs = map[string]versionConfig{}
	}
	if config.tokenClaims == nil {
		config.tokenClaims = map[string]tokenClaims{}
	}
	if config.rateLimitedGroups == nil {
		config.rateLimitedGroups = map[string]bool{}
	}
	if config.restUpstreams == nil {
		config.restUpstreams = map[string]http.Handler{}
	}
	if config.requestIDGenerator == nil {
		config.requestIDGenerator = generateRequestID
	}
	if config.tokenValidator == nil {
		config.tokenValidator = staticTokenValidator(config.tokenClaims)
	}
	if config.rateLimiter == nil {
		config.rateLimiter = staticGroupLimiter(config.rateLimitedGroups)
	}
	return &gateway{
		config:         config,
		tokenValidator: config.tokenValidator,
		rateLimiter:    config.rateLimiter,
	}
}

func (g *gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-Id")
	if requestID == "" {
		requestID = g.config.requestIDGenerator()
	}
	w.Header().Set("X-Request-Id", requestID)
	r.Header.Set("X-Request-Id", requestID)

	switch {
	case r.URL.Path == "/health":
		g.handleHealth(w, r)
	case r.URL.Path == "/ws":
		g.handleWebSocket(w, r)
	case r.URL.Path == "/api/v1/version":
		g.handleVersion(w, r)
	case strings.HasPrefix(r.URL.Path, "/api/v1/"):
		g.handleREST(w, r)
	default:
		http.NotFound(w, r)
	}
}
