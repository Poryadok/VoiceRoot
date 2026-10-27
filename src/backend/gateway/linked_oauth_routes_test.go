package main

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestLinkedOAuthTransport(t *testing.T) {
	const attempt = "3f2504e0-4f89-41d3-9a0c-0305e82c3301"
	for _, provider := range []string{"twitch", "youtube"} {
		base := "/api/v1/auth/linked-accounts/" + provider
		for _, route := range []struct {
			method, path string
			public       bool
		}{
			{http.MethodPost, base + "/link", false},
			{http.MethodPost, base + "/results/consume", false},
			{http.MethodPost, base + "/attempts/" + attempt + "/cancel", false},
			{http.MethodGet, base + "/oauth-callback", true},
		} {
			t.Run(provider+route.path, func(t *testing.T) {
				calls := []sdkAuthorizationRateCall{}
				forwarded := 0
				const query = "code=synthetic-code&state=synthetic-state"
				const body = `{"result_handle":"synthetic-handle","return_verifier":"synthetic-verifier"}`
				var logs bytes.Buffer
				h := newGatewayForContract(t, gatewayTestOptions{
					tokenClaims: map[string]tokenClaims{"voice-token": {UserID: "account-1", ProfileID: "profile-1"}},
					rateLimiter: sdkAuthorizationRecordingRateLimiter{calls: &calls},
					// A deliberately incomplete transcoder must never be consulted.
					transcoder: &transcoder{},
					slogLogger: slog.New(slog.NewJSONHandler(&logs, nil)),
					restUpstreams: map[string]http.Handler{"auth": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						forwarded++
						if r.Method != route.method || r.URL.Path != route.path || r.URL.RawQuery != query {
							t.Errorf("changed upstream request: %s %s", r.Method, r.URL)
						}
						got, err := io.ReadAll(r.Body)
						if err != nil || string(got) != body {
							t.Errorf("body=%q err=%v", got, err)
						}
						if route.public {
							if r.Header.Get("Authorization") != "" {
								t.Error("callback forwarded bearer")
							}
						} else if r.Header.Get("X-Voice-User-Id") != "account-1" || r.Header.Get("X-Voice-Profile-Id") != "profile-1" {
							t.Error("missing verified identity")
						}
						w.WriteHeader(http.StatusNoContent)
					})},
				})
				if !route.public {
					resp := performRequest(h, route.method, route.path, "{}", map[string]string{"X-Voice-User-Id": "spoofed"})
					if resp.Code != http.StatusUnauthorized || forwarded != 0 || len(calls) != 0 {
						t.Fatalf("unauthenticated request: status=%d forwards=%d rate calls=%d", resp.Code, forwarded, len(calls))
					}
				}
				r := httptestRequest(route.method, route.path+"?"+query, body, map[string]string{"Authorization": "Bearer voice-token"})
				r.RemoteAddr = "203.0.113.11:4567"
				resp := performPreparedRequest(h, r)
				if resp.Code != http.StatusNoContent || forwarded != 1 {
					t.Fatalf("response=%d body=%s forwards=%d", resp.Code, resp.Body.String(), forwarded)
				}
				wantKey := "user:account-1"
				if route.public {
					wantKey = "ip:203.0.113.11"
				}
				if len(calls) != 1 || calls[0].group != "AuthOAuth" || calls[0].key != wantKey {
					t.Fatalf("rate calls=%+v want AuthOAuth/%s", calls, wantKey)
				}
				for _, secret := range []string{"synthetic-code", "synthetic-state", "synthetic-handle", "synthetic-verifier", "voice-token"} {
					if strings.Contains(logs.String(), secret) {
						t.Errorf("access log leaked %q", secret)
					}
				}
			})
		}
	}
	if rule := defaultRateLimitRules()["AuthOAuth"]; rule.Limit != 30 || rule.Window != 15*time.Minute {
		t.Fatalf("unexpected OAuth rate rule: %+v", rule)
	}
}

func TestLinkedOAuthExactClassification(t *testing.T) {
	base := "/api/v1/auth/linked-accounts/twitch"
	for _, id := range []string{"3F2504E0-4F89-41D3-9A0C-0305E82C3301", "00000000-0000-0000-0000-000000000000"} {
		if group := rateLimitGroup(http.MethodPost, base+"/attempts/"+id+"/cancel"); group != "AuthOAuth" {
			t.Errorf("UUID cancel bypassed OAuth quota: id=%s group=%q", id, group)
		}
	}
	for _, route := range []struct{ method, path string }{
		{http.MethodPost, base + "/oauth-callback"}, {http.MethodGet, base + "/link"},
		{http.MethodGet, base + "/results/consume"}, {http.MethodGet, base + "/attempts/3f2504e0-4f89-41d3-9a0c-0305e82c3301/cancel"},
		{http.MethodPost, base + "/attempts/not-a-uuid/cancel"}, {http.MethodPost, base + "/results/consume/extra"},
		{http.MethodGet, base + "/oauth-callback/"}, {http.MethodGet, "/api/v1/auth/linked-accounts/other/oauth-callback"},
		{http.MethodPost, base + "/callback"}, {http.MethodPost, base + "/unlink"},
	} {
		if isPublicRESTRoute(route.method, route.path) {
			t.Errorf("unexpected public route: %s %s", route.method, route.path)
		}
		if group := rateLimitGroup(route.method, route.path); group != "" {
			t.Errorf("unexpected OAuth rate group %q: %s %s", group, route.method, route.path)
		}
	}
}

func TestLinkedOAuthPublicCallbackIgnoresBearer(t *testing.T) {
	for _, authorization := range []string{"", "Bearer invalid-token"} {
		forwarded := false
		h := newGatewayForContract(t, gatewayTestOptions{
			tokenValidator: fixedValidator{code: "invalid_token"},
			restUpstreams: map[string]http.Handler{"auth": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				forwarded = true
				if r.Header.Get("Authorization") != "" {
					t.Error("callback forwarded Voice credential")
				}
				w.WriteHeader(http.StatusOK)
			})},
		})
		resp := performRequest(h, http.MethodGet, "/api/v1/auth/linked-accounts/twitch/oauth-callback?code=synthetic&state=synthetic", "", map[string]string{"Authorization": authorization})
		if resp.Code != http.StatusOK || !forwarded {
			t.Fatalf("unauthenticated callback: status=%d forwarded=%v", resp.Code, forwarded)
		}
	}
}

func TestLinkedOAuthRequiresSingleBearerHeader(t *testing.T) {
	for _, headers := range [][]string{nil, {"Bearer voice-token", "Bearer voice-token"}, {"Bot voice-token"}} {
		forwarded := false
		h := newGatewayForContract(t, gatewayTestOptions{
			tokenClaims:   map[string]tokenClaims{"voice-token": {UserID: "account-1", ProfileID: "profile-1"}},
			restUpstreams: map[string]http.Handler{"auth": http.HandlerFunc(func(http.ResponseWriter, *http.Request) { forwarded = true })},
		})
		r := httptestRequest(http.MethodPost, "/api/v1/auth/linked-accounts/twitch/link?access_token=voice-token", "{}", nil)
		for _, header := range headers {
			r.Header.Add("Authorization", header)
		}
		resp := performPreparedRequest(h, r)
		if resp.Code != http.StatusUnauthorized || forwarded {
			t.Errorf("headers=%v: status=%d forwarded=%v", headers, resp.Code, forwarded)
		}
	}
}

func TestLinkedOAuthRateLimitFailsClosed(t *testing.T) {
	for _, limiter := range []rateLimiter{staticGroupLimiter{"AuthOAuth": true}, errorRateLimiter{}} {
		forwarded := false
		h := newGatewayForContract(t, gatewayTestOptions{rateLimiter: limiter, restUpstreams: map[string]http.Handler{"auth": http.HandlerFunc(func(http.ResponseWriter, *http.Request) { forwarded = true })}})
		resp := performRequest(h, http.MethodGet, "/api/v1/auth/linked-accounts/twitch/oauth-callback?state=secret", "", nil)
		want := http.StatusTooManyRequests
		if _, ok := limiter.(errorRateLimiter); ok {
			want = http.StatusServiceUnavailable
		}
		if resp.Code != want || forwarded {
			t.Errorf("status=%d want=%d forwarded=%v", resp.Code, want, forwarded)
		}
	}
}
