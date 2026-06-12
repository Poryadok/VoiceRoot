package main

import (
	"bufio"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"voice/backend/pkg/correlation"
	voicelog "voice/backend/pkg/logging"
	voicemw "voice/backend/pkg/middleware"
)

type gatewayConfig struct {
	versionConfigs     map[string]versionConfig
	versionStore       versionStore
	versionCacheRedis  string
	forceUpdate        *forceUpdatePolicy
	tokenClaims        map[string]tokenClaims
	rateLimitedGroups  map[string]bool
	restUpstreams      map[string]http.Handler
	transcoder         *transcoder
	realtimeUpstream   http.Handler
	requestIDGenerator func() string
	tokenValidator     tokenValidator
	tokenBlacklist     tokenBlacklist
	rateLimiter        rateLimiter
	trustedProxyCIDRs  []string
	cors               corsConfig
	metrics            *gatewayMetrics
	slogLogger         *slog.Logger
}

type gateway struct {
	config         gatewayConfig
	versionCache   versionPolicyCache
	tokenValidator tokenValidator
	tokenBlacklist tokenBlacklist
	rateLimiter    rateLimiter
	trustedProxies []trustedProxy
	metrics        *gatewayMetrics
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
		config.requestIDGenerator = correlation.GenerateRequestID
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
	if config.slogLogger == nil {
		config.slogLogger = voicelog.NewJSONLogger(voicelog.LevelFromEnv(), slog.String("service", "gateway"))
	}
	if config.versionStore == nil {
		config.versionStore = versionStoreFromEnv(config.versionConfigs, nil)
	}
	core := &gateway{
		config:         config,
		versionCache:   versionPolicyCacheFromConfig(config),
		tokenValidator: config.tokenValidator,
		tokenBlacklist: config.tokenBlacklist,
		rateLimiter:    config.rateLimiter,
		trustedProxies: parseTrustedProxies(config.trustedProxyCIDRs),
		metrics:        config.metrics,
	}
	h := http.Handler(core)
	h = voicemw.AccessLog(config.slogLogger, correlation.RequestIDHeader, gatewayAccessLogExtras)(h)
	h = voicemw.RequestID(config.requestIDGenerator)(h)
	return h
}

func (g *gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
	if g.applyCORS(rec, r) {
		g.observeRequestMetrics(r, rec.status, start)
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
	case strings.HasPrefix(r.URL.Path, "/api/v1/admin/client-versions"):
		g.handleAdminClientVersions(rec, r)
	case strings.HasPrefix(r.URL.Path, "/api/v1/"):
		g.handleREST(rec, r)
	default:
		http.NotFound(rec, r)
	}
	g.observeRequestMetrics(r, rec.status, start)
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
