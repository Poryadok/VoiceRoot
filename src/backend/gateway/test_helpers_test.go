package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type gatewayTestOptions struct {
	versionConfigs     map[string]versionConfig
	forceUpdate        *forceUpdatePolicy
	tokenClaims        map[string]tokenClaims
	rateLimitedGroups  map[string]bool
	restUpstreams      map[string]http.Handler
	realtimeUpstream   http.Handler
	requestIDGenerator func() string
}

func newGatewayForContract(_ *testing.T, options gatewayTestOptions) http.Handler {
	return newGateway(gatewayConfig{
		versionConfigs:     options.versionConfigs,
		forceUpdate:        options.forceUpdate,
		tokenClaims:        options.tokenClaims,
		rateLimitedGroups:  options.rateLimitedGroups,
		restUpstreams:      options.restUpstreams,
		realtimeUpstream:   options.realtimeUpstream,
		requestIDGenerator: options.requestIDGenerator,
	})
}

func performRequest(h http.Handler, method, target string, body string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
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
