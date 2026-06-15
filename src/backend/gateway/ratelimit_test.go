package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimitGroups(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		method     string
		target     string
		body       string
		group      string
		authHeader string
	}{
		{name: "auth login", method: http.MethodPost, target: "/api/v1/auth/login", body: `{}`, group: "AuthLogin"},
		{name: "auth register", method: http.MethodPost, target: "/api/v1/auth/register", body: `{}`, group: "AuthRegister"},
		{name: "otp", method: http.MethodPost, target: "/api/v1/auth/otp/send", body: `{}`, group: "OTP"},
		{name: "messages send", method: http.MethodPost, target: "/api/v1/messages/send", body: `{"text":"hi"}`, group: "MessagesSend", authHeader: "Bearer valid-user-token"},
		{name: "file upload", method: http.MethodPost, target: "/api/v1/files/upload", body: `file`, group: "FileUpload", authHeader: "Bearer valid-user-token"},
		{name: "avatar presigned upload", method: http.MethodPost, target: "/api/v1/users/me/avatar/presigned-upload", body: `{"content_type":"image/png","content_length":1024}`, group: "FileUpload", authHeader: "Bearer valid-user-token"},
		{name: "space creation", method: http.MethodPost, target: "/api/v1/spaces", body: `{}`, group: "SpaceCreation", authHeader: "Bearer valid-user-token"},
		{name: "bot api", method: http.MethodPost, target: "/api/v1/bots/interactions", body: `{}`, group: "BotAPI", authHeader: "Bearer valid-user-token"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name+" limited", func(t *testing.T) {
			t.Parallel()

			h := newGatewayForContract(t, gatewayTestOptions{
				tokenClaims: map[string]tokenClaims{
					"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
				},
				rateLimitedGroups: map[string]bool{tc.group: true},
				restUpstreams: map[string]http.Handler{
					"auth":     http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }),
					"messages": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }),
					"files":    http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }),
					"users":    http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }),
					"spaces":   http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }),
					"bots":     http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }),
				},
			})
			headers := map[string]string{"X-Forwarded-For": "203.0.113.10"}
			if tc.authHeader != "" {
				headers["Authorization"] = tc.authHeader
			}

			rec := performRequest(h, tc.method, tc.target, tc.body, headers)
			if rec.Code != http.StatusTooManyRequests {
				t.Fatalf("status = %d, want %d for group %s; body=%q", rec.Code, http.StatusTooManyRequests, tc.group, rec.Body.String())
			}
		})
	}
}

func TestDefaultRateLimitRules_messagesSend(t *testing.T) {
	t.Parallel()
	rule, ok := defaultRateLimitRules()["MessagesSend"]
	if !ok {
		t.Fatal("missing MessagesSend rule")
	}
	if rule.Limit != 5 || rule.Window != 5*time.Second {
		t.Fatalf("MessagesSend rule = %#v, want limit 5 window 5s", rule)
	}
}

func TestSlidingWindowLimiter_messagesSend_fivePerFiveSeconds(t *testing.T) {
	t.Parallel()
	now := time.Unix(1700, 0)
	limiter := newSlidingWindowLimiter(map[string]rateLimitRule{
		"MessagesSend": {Limit: 5, Window: 5 * time.Second},
	})
	limiter.now = func() time.Time { return now }

	key := "user:account-1"
	for i := 1; i <= 5; i++ {
		allowed, err := limiter.Allow(context.Background(), key, "MessagesSend")
		if err != nil || !allowed {
			t.Fatalf("request %d: allowed=%v err=%v, want allowed", i, allowed, err)
		}
	}
	if allowed, err := limiter.Allow(context.Background(), key, "MessagesSend"); err != nil || allowed {
		t.Fatalf("sixth request: allowed=%v err=%v, want denied", allowed, err)
	}

	// Oldest entries fall out of the 5s window.
	now = now.Add(5*time.Second + time.Millisecond)
	if allowed, err := limiter.Allow(context.Background(), key, "MessagesSend"); err != nil || !allowed {
		t.Fatalf("after window: allowed=%v err=%v, want allowed", allowed, err)
	}
}

func TestGateway_messageSendRateLimit_perUserBuckets(t *testing.T) {
	t.Parallel()
	// Tight limit to keep the test fast while exercising the same code path as production.
	limiter := newSlidingWindowLimiter(map[string]rateLimitRule{
		"MessagesSend": {Limit: 3, Window: time.Hour},
	})

	var upstreamHits int
	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"token-a": {UserID: "account-a", ProfileID: "p-a"},
			"token-b": {UserID: "account-b", ProfileID: "p-b"},
		},
		rateLimiter: limiter,
		restUpstreams: map[string]http.Handler{
			"messages": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				upstreamHits++
				w.WriteHeader(http.StatusNoContent)
			}),
		},
	})

	send := func(token string) int {
		t.Helper()
		rec := performRequest(h, http.MethodPost, "/api/v1/messages/send", `{"text":"hi"}`, map[string]string{
			"Authorization": "Bearer " + token,
		})
		return rec.Code
	}

	for i := 0; i < 3; i++ {
		if got := send("token-a"); got != http.StatusNoContent {
			t.Fatalf("user a request %d: status=%d body=%q", i+1, got, "")
		}
	}
	if got := send("token-a"); got != http.StatusTooManyRequests {
		t.Fatalf("user a 4th: status=%d, want 429", got)
	}

	for i := 0; i < 3; i++ {
		if got := send("token-b"); got != http.StatusNoContent {
			t.Fatalf("user b request %d: status=%d", i+1, got)
		}
	}
	if got := send("token-b"); got != http.StatusTooManyRequests {
		t.Fatalf("user b 4th: status=%d, want 429", got)
	}

	if upstreamHits != 6 {
		t.Fatalf("upstream hits = %d, want 6 (limited requests must not reach upstream)", upstreamHits)
	}
}

func TestGateway_messageSendRateLimit_returns429JSON(t *testing.T) {
	t.Parallel()
	limiter := newSlidingWindowLimiter(map[string]rateLimitRule{
		"MessagesSend": {Limit: 1, Window: time.Hour},
	})
	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"tok": {UserID: "u1", ProfileID: "p1"},
		},
		rateLimiter: limiter,
		restUpstreams: map[string]http.Handler{
			"messages": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			}),
		},
	})
	hdr := map[string]string{"Authorization": "Bearer tok"}
	if c := performRequest(h, http.MethodPost, "/api/v1/messages/send", `{}`, hdr).Code; c != http.StatusNoContent {
		t.Fatalf("first status=%d", c)
	}
	rec := performRequest(h, http.MethodPost, "/api/v1/messages/send", `{}`, hdr)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("second status=%d", rec.Code)
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["error"] != "rate_limited" {
		t.Fatalf("error=%q, want rate_limited", body["error"])
	}
}

func TestRateLimitGroup_messagesSend(t *testing.T) {
	t.Parallel()
	if g := rateLimitGroup(http.MethodPost, "/api/v1/messages/send"); g != "MessagesSend" {
		t.Fatalf("group=%q", g)
	}
	if g := rateLimitGroup(http.MethodGet, "/api/v1/messages/send"); g != "" {
		t.Fatalf("GET should not rate-limit as send, got %q", g)
	}
}

func TestRateLimitGroup_authPaths(t *testing.T) {
	t.Parallel()
	if g := rateLimitGroup(http.MethodPost, "/api/v1/auth/login"); g != "AuthLogin" {
		t.Fatalf("login group=%q, want AuthLogin", g)
	}
	if g := rateLimitGroup(http.MethodPost, "/api/v1/auth/register"); g != "AuthRegister" {
		t.Fatalf("register group=%q, want AuthRegister", g)
	}
}

func TestRateLimitRulesFromEnv_authAliasDisablesBoth(t *testing.T) {
	t.Setenv("GATEWAY_RATE_LIMIT_RULES_JSON", `{"Auth":{"limit":0,"window":"15m"}}`)
	rules := rateLimitRulesFromEnv()
	if rules["AuthLogin"].Limit != 0 || rules["AuthRegister"].Limit != 0 {
		t.Fatalf("Auth alias: AuthLogin=%#v AuthRegister=%#v, want limit 0 for both", rules["AuthLogin"], rules["AuthRegister"])
	}
}

func TestRateLimitRulesFromEnv_overrideSingleGroup(t *testing.T) {
	t.Setenv("GATEWAY_RATE_LIMIT_RULES_JSON", `{"AuthLogin":{"limit":1,"window":"1h"},"MessagesSend":{"limit":99,"window":"1s"}}`)
	rules := rateLimitRulesFromEnv()
	if rules["AuthLogin"].Limit != 1 || rules["AuthLogin"].Window != time.Hour {
		t.Fatalf("AuthLogin=%#v", rules["AuthLogin"])
	}
	if rules["AuthRegister"].Limit != 5 || rules["AuthRegister"].Window != 15*time.Minute {
		t.Fatalf("AuthRegister=%#v, want default 5/15m", rules["AuthRegister"])
	}
	if rules["MessagesSend"].Limit != 99 {
		t.Fatalf("MessagesSend=%#v", rules["MessagesSend"])
	}
}

func TestGateway_authLoginManyAttempts_no429WhenAuthDisabled(t *testing.T) {
	t.Setenv("GATEWAY_RATE_LIMIT_RULES_JSON", `{"Auth":{"limit":0,"window":"15m"}}`)
	limiter := newSlidingWindowLimiter(rateLimitRulesFromEnv())
	h := newGatewayForContract(t, gatewayTestOptions{
		rateLimiter: limiter,
		restUpstreams: map[string]http.Handler{
			"auth": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }),
		},
	})
	headers := map[string]string{"X-Forwarded-For": "203.0.113.42"}
	for i := 1; i <= 6; i++ {
		rec := performRequest(h, http.MethodPost, "/api/v1/auth/login", `{}`, headers)
		if rec.Code == http.StatusTooManyRequests {
			t.Fatalf("login attempt %d: status=429, want no rate limit when Auth disabled", i)
		}
		if rec.Code != http.StatusNoContent {
			t.Fatalf("login attempt %d: status=%d body=%q", i, rec.Code, rec.Body.String())
		}
	}
}

func TestGateway_authLoginAndRegister_separateBuckets(t *testing.T) {
	t.Parallel()
	limiter := newSlidingWindowLimiter(map[string]rateLimitRule{
		"AuthLogin":    {Limit: 1, Window: time.Hour},
		"AuthRegister": {Limit: 1, Window: time.Hour},
	})
	h := newGatewayForContract(t, gatewayTestOptions{
		rateLimiter: limiter,
		restUpstreams: map[string]http.Handler{
			"auth": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }),
		},
	})
	headers := map[string]string{"X-Forwarded-For": "203.0.113.43"}
	if c := performRequest(h, http.MethodPost, "/api/v1/auth/login", `{}`, headers).Code; c != http.StatusNoContent {
		t.Fatalf("first login status=%d", c)
	}
	if c := performRequest(h, http.MethodPost, "/api/v1/auth/login", `{}`, headers).Code; c != http.StatusTooManyRequests {
		t.Fatalf("second login status=%d, want 429", c)
	}
	if c := performRequest(h, http.MethodPost, "/api/v1/auth/register", `{}`, headers).Code; c != http.StatusNoContent {
		t.Fatalf("register after login limited: status=%d, want separate bucket", c)
	}
}

func TestRateLimitKey_messagesSend_usesUserID(t *testing.T) {
	t.Parallel()
	g := &gateway{config: gatewayConfig{trustedProxyCIDRs: nil}}
	r := httptest.NewRequest(http.MethodPost, "/api/v1/messages/send", nil)
	claims := tokenClaims{UserID: "account-99", ProfileID: "prof-1"}
	if got := g.rateLimitKey(r, claims, false); got != "user:account-99" {
		t.Fatalf("rateLimitKey = %q, want user:account-99", got)
	}
}

func TestGateway_botAPIRateLimit_returns429WithRetryAfter(t *testing.T) {
	t.Parallel()
	limiter := newSlidingWindowLimiter(map[string]rateLimitRule{
		"BotAPI": {Limit: 1, Window: time.Minute},
	})
	tc := newTranscoder(&grpcClients{bot: &fakeBotClient{}})
	h := newGatewayForContract(t, gatewayTestOptions{
		rateLimiter: limiter,
		transcoder:  tc,
	})
	headers := map[string]string{"Authorization": "Bot vb_ratelimit_test"}
	rec := performRequest(h, http.MethodGet, "/api/v1/bots/me/interactions/poll", "", headers)
	if rec.Code != http.StatusOK && rec.Code != http.StatusNoContent {
		t.Fatalf("first poll status=%d body=%s", rec.Code, rec.Body.String())
	}
	rec2 := performRequest(h, http.MethodGet, "/api/v1/bots/me/interactions/poll", "", headers)
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("second poll status=%d, want 429", rec2.Code)
	}
	if got := rec2.Header().Get("Retry-After"); got == "" {
		t.Fatalf("missing Retry-After header on 429")
	}
}

func TestRateLimitKey_botRoute_usesBotToken(t *testing.T) {
	t.Parallel()
	g := &gateway{config: gatewayConfig{trustedProxyCIDRs: nil}}
	r := httptest.NewRequest(http.MethodGet, "/api/v1/bots/me/interactions/poll", nil)
	r.Header.Set("Authorization", "Bot vb_token_key")
	if got := g.rateLimitKey(r, tokenClaims{}, false); got != "bot:vb_token_key" {
		t.Fatalf("rateLimitKey = %q, want bot:vb_token_key", got)
	}
}
