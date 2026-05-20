package main

import (
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestBlacklistBoundary(t *testing.T) {
	t.Parallel()

	upstreams := map[string]http.Handler{
		"users": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }),
	}

	revoked := newGatewayForContract(t, gatewayTestOptions{
		tokenValidator: fixedValidator{claims: tokenClaims{UserID: "account-1", JTI: "jti-1"}},
		tokenBlacklist: fakeBlacklist{revoked: true},
		restUpstreams:  upstreams,
	})
	rec := performRequest(revoked, http.MethodGet, "/api/v1/users/me", "", map[string]string{"Authorization": "Bearer any"})
	if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), "token_revoked") {
		t.Fatalf("revoked status/body = %d %q", rec.Code, rec.Body.String())
	}

	unavailable := newGatewayForContract(t, gatewayTestOptions{
		tokenValidator: fixedValidator{claims: tokenClaims{UserID: "account-1", JTI: "jti-1"}},
		tokenBlacklist: fakeBlacklist{err: errors.New("redis down")},
		restUpstreams:  upstreams,
	})
	rec = performRequest(unavailable, http.MethodGet, "/api/v1/users/me", "", map[string]string{"Authorization": "Bearer any"})
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "auth_unavailable") {
		t.Fatalf("blacklist failure status/body = %d %q", rec.Code, rec.Body.String())
	}
}

func TestTrustedProxyControlsForwardedFor(t *testing.T) {
	t.Parallel()

	var keys []string
	limiter := captureLimiter{keys: &keys}
	h := newGatewayForContract(t, gatewayTestOptions{
		rateLimiter:       limiter,
		trustedProxyCIDRs: []string{"192.0.2.0/24"},
		restUpstreams: map[string]http.Handler{
			"auth": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }),
		},
	})

	req := httptestRequest(http.MethodPost, "/api/v1/auth/login", "{}", map[string]string{"X-Forwarded-For": "198.51.100.5"})
	req.RemoteAddr = "203.0.113.10:1234"
	rec := performPreparedRequest(h, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("untrusted proxy status = %d", rec.Code)
	}
	if keys[len(keys)-1] != "ip:203.0.113.10" {
		t.Fatalf("untrusted key = %q", keys[len(keys)-1])
	}

	req = httptestRequest(http.MethodPost, "/api/v1/auth/login", "{}", map[string]string{"X-Forwarded-For": "198.51.100.5, 192.0.2.1"})
	req.RemoteAddr = "192.0.2.9:1234"
	rec = performPreparedRequest(h, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("trusted proxy status = %d", rec.Code)
	}
	if keys[len(keys)-1] != "ip:198.51.100.5" {
		t.Fatalf("trusted key = %q", keys[len(keys)-1])
	}
}

func TestCORSPreflight(t *testing.T) {
	t.Parallel()

	h := newGatewayForContract(t, gatewayTestOptions{
		cors: corsConfig{AllowedOrigins: []string{"https://app.voice.example"}},
	})
	allowed := performRequest(h, http.MethodOptions, "/api/v1/users/me", "", map[string]string{
		"Origin": "https://app.voice.example",
	})
	if allowed.Code != http.StatusNoContent {
		t.Fatalf("allowed preflight status = %d", allowed.Code)
	}
	if got := allowed.Header().Get("Access-Control-Allow-Origin"); got != "https://app.voice.example" {
		t.Fatalf("allow origin = %q", got)
	}

	denied := performRequest(h, http.MethodOptions, "/api/v1/users/me", "", map[string]string{
		"Origin": "https://evil.example",
	})
	if denied.Code != http.StatusForbidden || !strings.Contains(denied.Body.String(), "cors_origin_denied") {
		t.Fatalf("denied preflight status/body = %d %q", denied.Code, denied.Body.String())
	}
}

func TestMetricsEndpointAndRateLimitHit(t *testing.T) {
	t.Parallel()

	metrics := newGatewayMetrics()
	h := newGatewayForContract(t, gatewayTestOptions{
		metrics:           metrics,
		rateLimitedGroups: map[string]bool{"AuthLogin": true},
		restUpstreams: map[string]http.Handler{
			"auth": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }),
		},
	})
	limited := performRequest(h, http.MethodPost, "/api/v1/auth/login", "{}", nil)
	if limited.Code != http.StatusTooManyRequests {
		t.Fatalf("limited status = %d", limited.Code)
	}
	rec := performRequest(h, http.MethodGet, "/metrics", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("metrics status = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "gateway_request_count") || !strings.Contains(body, "gateway_ratelimit_hit{group=\"AuthLogin\"} 1") {
		t.Fatalf("metrics body = %q", body)
	}
}
