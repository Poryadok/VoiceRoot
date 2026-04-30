package main

import (
	"bufio"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
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
	tokenBlacklist     tokenBlacklist
	rateLimiter        rateLimiter
	trustedProxyCIDRs  []string
	cors               corsConfig
	metrics            *gatewayMetrics
	logger             *log.Logger
}

type gateway struct {
	config         gatewayConfig
	tokenValidator tokenValidator
	tokenBlacklist tokenBlacklist
	rateLimiter    rateLimiter
	trustedProxies []trustedProxy
	metrics        *gatewayMetrics
	logger         *log.Logger
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
	if config.tokenBlacklist == nil {
		config.tokenBlacklist = noTokenBlacklist{}
	}
	if config.rateLimiter == nil {
		config.rateLimiter = staticGroupLimiter(config.rateLimitedGroups)
	}
	if config.metrics == nil {
		config.metrics = newGatewayMetrics()
	}
	if config.logger == nil {
		config.logger = log.Default()
	}
	return &gateway{
		config:         config,
		tokenValidator: config.tokenValidator,
		tokenBlacklist: config.tokenBlacklist,
		rateLimiter:    config.rateLimiter,
		trustedProxies: parseTrustedProxies(config.trustedProxyCIDRs),
		metrics:        config.metrics,
		logger:         config.logger,
	}
}

func (g *gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	requestID := r.Header.Get("X-Request-Id")
	if requestID == "" {
		requestID = g.config.requestIDGenerator()
	}
	w.Header().Set("X-Request-Id", requestID)
	r.Header.Set("X-Request-Id", requestID)

	rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
	if g.applyCORS(rec, r) {
		g.recordRequest(r, rec.status, start)
		return
	}

	switch {
	case r.URL.Path == "/health":
		g.handleHealth(rec, r)
	case r.URL.Path == "/metrics":
		g.handleMetrics(rec, r)
	case r.URL.Path == "/ws":
		g.handleWebSocket(rec, r)
	case r.URL.Path == "/api/v1/version":
		g.handleVersion(rec, r)
	case strings.HasPrefix(r.URL.Path, "/api/v1/"):
		g.handleREST(rec, r)
	default:
		http.NotFound(rec, r)
	}
	g.recordRequest(r, rec.status, start)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}
	return h.Hijack()
}

func (r *statusRecorder) Push(target string, opts *http.PushOptions) error {
	p, ok := r.ResponseWriter.(http.Pusher)
	if !ok {
		return http.ErrNotSupported
	}
	return p.Push(target, opts)
}
