package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	voicejwt "voice/backend/pkg/jwt"
)

func TestGatewayConfigFromEnvBuildsRESTProxy(t *testing.T) {
	var gotPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.RequestURI()
		w.Header().Set("X-Upstream", "auth")
		w.WriteHeader(http.StatusAccepted)
	}))
	t.Cleanup(upstream.Close)

	t.Setenv("GATEWAY_REST_UPSTREAMS_JSON", `{"auth": "`+upstream.URL+`", "federation": "http://federation.invalid"}`)

	h := newGateway(loadGatewayConfigFromEnv())
	rec := performRequest(h, http.MethodPost, "/api/v1/auth/login?device=web", `{}`, nil)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body=%q", rec.Code, http.StatusAccepted, rec.Body.String())
	}
	if got := rec.Header().Get("X-Upstream"); got != "auth" {
		t.Fatalf("X-Upstream = %q, want auth", got)
	}
	if gotPath != "/api/v1/auth/login?device=web" {
		t.Fatalf("proxied path = %q", gotPath)
	}
}

func TestGatewayConfigFromEnvSelectsRedisLimiter(t *testing.T) {
	t.Setenv("GATEWAY_REDIS_ADDR", "127.0.0.1:6379")
	t.Setenv("GATEWAY_IN_MEMORY_RATE_LIMITS", "true")

	config := loadGatewayConfigFromEnv()
	if _, ok := config.rateLimiter.(*redisSlidingWindowLimiter); !ok {
		t.Fatalf("rateLimiter = %T, want *redisSlidingWindowLimiter", config.rateLimiter)
	}
	if _, ok := config.tokenBlacklist.(*redisTokenBlacklist); !ok {
		t.Fatalf("tokenBlacklist = %T, want *redisTokenBlacklist", config.tokenBlacklist)
	}
}

func TestGatewayConfigFromEnvSelectsAuthMode(t *testing.T) {
	t.Setenv("GATEWAY_AUTH_MODE", "static")
	t.Setenv("GATEWAY_STATIC_TOKENS_JSON", `{"dev-token":{"UserID":"account-1"}}`)
	staticConfig := loadGatewayConfigFromEnv()
	if _, ok := staticConfig.tokenValidator.(staticTokenValidator); !ok {
		t.Fatalf("static tokenValidator = %T, want staticTokenValidator", staticConfig.tokenValidator)
	}

	t.Setenv("GATEWAY_AUTH_MODE", "")
	t.Setenv("GATEWAY_JWKS_URL", "https://auth.voice.example/.well-known/jwks.json")
	jwksConfig := loadGatewayConfigFromEnv()
	if _, ok := jwksConfig.tokenValidator.(*voicejwt.Validator); !ok {
		t.Fatalf("jwks tokenValidator = %T, want *voicejwt.Validator", jwksConfig.tokenValidator)
	}
}

func TestSlidingWindowLimiter(t *testing.T) {
	now := time.Unix(100, 0)
	limiter := newSlidingWindowLimiter(map[string]rateLimitRule{
		"Auth": {Limit: 2, Window: 10 * time.Second},
	})
	limiter.now = func() time.Time { return now }

	if allowed, err := limiter.Allow(context.Background(), "ip:203.0.113.10", "Auth"); err != nil || !allowed {
		t.Fatalf("first allow = %v err=%v, want allowed", allowed, err)
	}
	if allowed, err := limiter.Allow(context.Background(), "ip:203.0.113.10", "Auth"); err != nil || !allowed {
		t.Fatalf("second allow = %v err=%v, want allowed", allowed, err)
	}
	if allowed, err := limiter.Allow(context.Background(), "ip:203.0.113.10", "Auth"); err != nil || allowed {
		t.Fatalf("third allow = %v err=%v, want limited", allowed, err)
	}

	now = now.Add(11 * time.Second)
	if allowed, err := limiter.Allow(context.Background(), "ip:203.0.113.10", "Auth"); err != nil || !allowed {
		t.Fatalf("after window allow = %v err=%v, want allowed", allowed, err)
	}
}
