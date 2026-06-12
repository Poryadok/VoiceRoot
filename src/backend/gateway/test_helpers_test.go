package main

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type gatewayTestOptions struct {
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

func newGatewayForContract(_ *testing.T, options gatewayTestOptions) http.Handler {
	log := options.slogLogger
	if log == nil {
		log = slog.New(slog.NewJSONHandler(io.Discard, &slog.HandlerOptions{}))
	}
	return newGateway(gatewayConfig{
		versionConfigs:     options.versionConfigs,
		versionStore:       options.versionStore,
		versionCacheRedis:  options.versionCacheRedis,
		forceUpdate:        options.forceUpdate,
		tokenClaims:        options.tokenClaims,
		rateLimitedGroups:  options.rateLimitedGroups,
		restUpstreams:      options.restUpstreams,
		transcoder:         options.transcoder,
		realtimeUpstream:   options.realtimeUpstream,
		requestIDGenerator: options.requestIDGenerator,
		tokenValidator:     options.tokenValidator,
		tokenBlacklist:     options.tokenBlacklist,
		rateLimiter:        options.rateLimiter,
		trustedProxyCIDRs:  options.trustedProxyCIDRs,
		cors:               options.cors,
		metrics:            options.metrics,
		slogLogger:         log,
	})
}

func performRequest(h http.Handler, method, target string, body string, headers map[string]string) *httptest.ResponseRecorder {
	return performPreparedRequest(h, httptestRequest(method, target, body, headers))
}

func httptestRequest(method, target string, body string, headers map[string]string) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return req
}

func performPreparedRequest(h http.Handler, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decodeJSON(t *testing.T, body io.Reader, dst any) {
	t.Helper()
	if err := json.NewDecoder(body).Decode(dst); err != nil {
		t.Fatalf("decode json: %v", err)
	}
}

type fakeBlacklist struct {
	revoked bool
	err     error
}

func (b fakeBlacklist) IsRevoked(_ context.Context, _ string) (bool, error) {
	return b.revoked, b.err
}

type fixedValidator struct {
	claims tokenClaims
	code   string
}

func (v fixedValidator) Validate(_ *http.Request) (tokenClaims, string) {
	return v.claims, v.code
}

type captureLimiter struct {
	keys *[]string
}

func (l captureLimiter) Allow(_ context.Context, key, _ string) (bool, error) {
	*l.keys = append(*l.keys, key)
	return true, nil
}
