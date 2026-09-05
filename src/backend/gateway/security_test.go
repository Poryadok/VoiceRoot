package main

import (
	"context"
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

func TestSessionEpochFloorAllowsCurrentAndNewerTokens(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name  string
		epoch int64
	}{
		{name: "equal floor", epoch: 7},
		{name: "newer than floor", epoch: 8},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			floor := &recordingSessionEpochFloor{minimum: 7}
			var upstreamCalls int
			h := newGatewayForContract(t, gatewayTestOptions{
				tokenValidator: fixedValidator{claims: tokenClaims{
					UserID:       "account-1",
					JTI:          "jti-1",
					SessionEpoch: tc.epoch,
				}},
				sessionEpochStrict: true,
				sessionEpochFloor:  floor,
				restUpstreams: map[string]http.Handler{
					"users": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						upstreamCalls++
						w.WriteHeader(http.StatusNoContent)
					}),
				},
			})

			rec := performRequest(h, http.MethodGet, "/api/v1/users/me", "", map[string]string{"Authorization": "Bearer any"})
			if rec.Code != http.StatusNoContent {
				t.Fatalf("status/body = %d %q", rec.Code, rec.Body.String())
			}
			if floor.calls != 1 || floor.accounts[0] != "account-1" {
				t.Fatalf("floor calls/accounts = %d/%v", floor.calls, floor.accounts)
			}
			if upstreamCalls != 1 {
				t.Fatalf("upstream calls = %d, want 1", upstreamCalls)
			}
		})
	}
}

func TestSessionEpochFloorRejectsStaleToken(t *testing.T) {
	t.Parallel()

	floor := &recordingSessionEpochFloor{minimum: 7}
	var upstreamCalls int
	h := newGatewayForContract(t, gatewayTestOptions{
		tokenValidator:     fixedValidator{claims: tokenClaims{UserID: "account-1", JTI: "jti-1", SessionEpoch: 6}},
		sessionEpochStrict: true,
		sessionEpochFloor:  floor,
		restUpstreams: map[string]http.Handler{
			"users": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				upstreamCalls++
				w.WriteHeader(http.StatusNoContent)
			}),
		},
	})

	rec := performRequest(h, http.MethodGet, "/api/v1/users/me", "", map[string]string{"Authorization": "Bearer any"})
	if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), "token_revoked") {
		t.Fatalf("status/body = %d %q", rec.Code, rec.Body.String())
	}
	if floor.calls != 1 || upstreamCalls != 0 {
		t.Fatalf("floor/upstream calls = %d/%d, want 1/0", floor.calls, upstreamCalls)
	}
}

func TestStrictStaticTokenRejectsMissingOrNonPositiveSessionEpochBeforeFloor(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		claims tokenClaims
	}{
		{name: "missing", claims: tokenClaims{UserID: "account-1", JTI: "jti-1"}},
		{name: "zero", claims: tokenClaims{UserID: "account-1", JTI: "jti-1", SessionEpoch: 0}},
		{name: "negative", claims: tokenClaims{UserID: "account-1", JTI: "jti-1", SessionEpoch: -1}},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			floor := &recordingSessionEpochFloor{minimum: 1}
			var upstreamCalls int
			h := newGatewayForContract(t, gatewayTestOptions{
				tokenClaims:        map[string]tokenClaims{"static-token": tc.claims},
				sessionEpochStrict: true,
				sessionEpochFloor:  floor,
				restUpstreams: map[string]http.Handler{
					"users": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						upstreamCalls++
						w.WriteHeader(http.StatusNoContent)
					}),
				},
			})

			rec := performRequest(h, http.MethodGet, "/api/v1/users/me", "", map[string]string{"Authorization": "Bearer static-token"})
			if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), "invalid_token") {
				t.Fatalf("status/body = %d %q", rec.Code, rec.Body.String())
			}
			if floor.calls != 0 || upstreamCalls != 0 {
				t.Fatalf("floor/upstream calls = %d/%d, want 0/0", floor.calls, upstreamCalls)
			}
		})
	}
}

func TestSessionEpochFloorFailsClosedWhenUnavailable(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name  string
		floor recordingSessionEpochFloor
	}{
		{name: "missing", floor: recordingSessionEpochFloor{}},
		{name: "redis error", floor: recordingSessionEpochFloor{err: errors.New("redis down")}},
		{name: "deadline", floor: recordingSessionEpochFloor{err: context.DeadlineExceeded}},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			floor := tc.floor
			h := newGatewayForContract(t, gatewayTestOptions{
				tokenValidator:     fixedValidator{claims: tokenClaims{UserID: "account-1", JTI: "jti-1", SessionEpoch: 7}},
				sessionEpochStrict: true,
				sessionEpochFloor:  &floor,
				restUpstreams: map[string]http.Handler{
					"users": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }),
				},
			})

			rec := performRequest(h, http.MethodGet, "/api/v1/users/me", "", map[string]string{"Authorization": "Bearer any"})
			if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "auth_unavailable") {
				t.Fatalf("status/body = %d %q", rec.Code, rec.Body.String())
			}
			if floor.calls != 1 {
				t.Fatalf("floor calls = %d, want 1", floor.calls)
			}
		})
	}
}

func TestSessionEpochFloorTypedNilRedisStoreFailsClosed(t *testing.T) {
	t.Parallel()

	var redisFloor *redisSessionEpochFloor
	var floor sessionEpochFloor = redisFloor
	blacklist := &recordingBlacklist{}
	var upstreamCalls int
	h := newGatewayForContract(t, gatewayTestOptions{
		tokenValidator:     fixedValidator{claims: tokenClaims{UserID: "account-1", JTI: "jti-1", SessionEpoch: 7}},
		tokenBlacklist:     blacklist,
		sessionEpochStrict: true,
		sessionEpochFloor:  floor,
		restUpstreams: map[string]http.Handler{
			"users": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				upstreamCalls++
				w.WriteHeader(http.StatusNoContent)
			}),
		},
	})

	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("typed-nil session epoch floor panicked: %v", recovered)
		}
	}()
	rec := performRequest(h, http.MethodGet, "/api/v1/users/me", "", map[string]string{"Authorization": "Bearer any"})
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "auth_unavailable") {
		t.Fatalf("status/body = %d %q", rec.Code, rec.Body.String())
	}
	if blacklist.calls != 1 {
		t.Fatalf("blacklist calls = %d, want 1 before floor", blacklist.calls)
	}
	if upstreamCalls != 0 {
		t.Fatalf("upstream calls = %d, want 0", upstreamCalls)
	}
}

func TestSessionEpochFloorRunsAfterSuccessfulJTIValidation(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name      string
		blacklist recordingBlacklist
		wantCode  int
		wantBody  string
	}{
		{name: "revoked JTI", blacklist: recordingBlacklist{revoked: true}, wantCode: http.StatusUnauthorized, wantBody: "token_revoked"},
		{name: "blacklist unavailable", blacklist: recordingBlacklist{err: errors.New("redis down")}, wantCode: http.StatusServiceUnavailable, wantBody: "auth_unavailable"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			floor := &recordingSessionEpochFloor{minimum: 7}
			blacklist := tc.blacklist
			h := newGatewayForContract(t, gatewayTestOptions{
				tokenValidator:     fixedValidator{claims: tokenClaims{UserID: "account-1", JTI: "jti-1", SessionEpoch: 7}},
				tokenBlacklist:     &blacklist,
				sessionEpochStrict: true,
				sessionEpochFloor:  floor,
				restUpstreams: map[string]http.Handler{
					"users": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }),
				},
			})

			rec := performRequest(h, http.MethodGet, "/api/v1/users/me", "", map[string]string{"Authorization": "Bearer any"})
			if rec.Code != tc.wantCode || !strings.Contains(rec.Body.String(), tc.wantBody) {
				t.Fatalf("status/body = %d %q", rec.Code, rec.Body.String())
			}
			if blacklist.calls != 1 || floor.calls != 0 {
				t.Fatalf("blacklist/floor calls = %d/%d, want 1/0", blacklist.calls, floor.calls)
			}
		})
	}
}

func TestSessionEpochFloorChecksTokenWithoutJTI(t *testing.T) {
	t.Parallel()

	floor := &recordingSessionEpochFloor{minimum: 7}
	blacklist := &recordingBlacklist{}
	h := newGatewayForContract(t, gatewayTestOptions{
		tokenValidator:     fixedValidator{claims: tokenClaims{UserID: "account-1", SessionEpoch: 7}},
		tokenBlacklist:     blacklist,
		sessionEpochStrict: true,
		sessionEpochFloor:  floor,
		restUpstreams: map[string]http.Handler{
			"users": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }),
		},
	})

	rec := performRequest(h, http.MethodGet, "/api/v1/users/me", "", map[string]string{"Authorization": "Bearer any"})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status/body = %d %q", rec.Code, rec.Body.String())
	}
	if floor.calls != 1 || floor.accounts[0] != "account-1" {
		t.Fatalf("floor calls/accounts = %d/%v", floor.calls, floor.accounts)
	}
	if blacklist.calls != 0 {
		t.Fatalf("blacklist calls = %d, want 0", blacklist.calls)
	}
}

type recordingSessionEpochFloor struct {
	minimum  int64
	err      error
	calls    int
	accounts []string
}

func (f *recordingSessionEpochFloor) Minimum(_ context.Context, accountID string) (int64, error) {
	f.calls++
	f.accounts = append(f.accounts, accountID)
	return f.minimum, f.err
}

type recordingBlacklist struct {
	revoked bool
	err     error
	calls   int
}

func (b *recordingBlacklist) IsRevoked(_ context.Context, _ string) (bool, error) {
	b.calls++
	return b.revoked, b.err
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
	if !strings.Contains(body, "gateway_http_requests_total") || !strings.Contains(body, `gateway_ratelimit_hits_total{group="AuthLogin"} 1`) {
		t.Fatalf("metrics body = %q", body)
	}
}
