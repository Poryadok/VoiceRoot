package main

import (
	"context"
	"io"
	"net/http"
	"testing"
)

func TestSDKAuthorizationGatewayPrincipalPolicies(t *testing.T) {
	const (
		sdkToken    = "sdk-token"
		voiceToken  = "voice-token"
		requestID   = "3f2504e0-4f89-41d3-9a0c-0305e82c3301"
		base        = "/api/v1/auth/sdk/authorizations"
		linkedToken = "linked-bootstrap-opaque"
	)

	tests := []struct {
		name        string
		method      string
		path        string
		validToken  string
		wrongTokens []string
	}{
		{name: "create requires SDK account", method: http.MethodPost, path: base, validToken: sdkToken, wrongTokens: []string{voiceToken, "service-token"}},
		{name: "consent view requires regular account", method: http.MethodGet, path: base + "/" + requestID, validToken: voiceToken, wrongTokens: []string{sdkToken, "service-token"}},
		{name: "approval requires regular account", method: http.MethodPost, path: base + "/" + requestID + "/approve", validToken: voiceToken, wrongTokens: []string{sdkToken, "service-token"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			forwarded := 0
			h := newGatewayForContract(t, gatewayTestOptions{
				tokenClaims: map[string]tokenClaims{
					sdkToken:        {UserID: "sdk-account-1", AccountType: "sdk-account"},
					voiceToken:      {UserID: "voice-account-1", AccountType: "regular"},
					"service-token": {UserID: "service:auth", AccountType: "service"},
				},
				restUpstreams: map[string]http.Handler{"auth": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					forwarded++
					w.WriteHeader(http.StatusNoContent)
				})},
			})

			cases := []struct {
				name, token string
				wantStatus  int
			}{
				{name: "valid principal", token: tt.validToken, wantStatus: http.StatusNoContent},
				{name: "unauthenticated", wantStatus: http.StatusUnauthorized},
			}
			for _, token := range tt.wrongTokens {
				cases = append(cases, struct {
					name, token string
					wantStatus  int
				}{name: "wrong principal " + token, token: token, wantStatus: http.StatusForbidden})
			}
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					before := forwarded
					headers := map[string]string{}
					if tc.token != "" {
						headers["Authorization"] = "Bearer " + tc.token
					}
					resp := performRequest(h, tt.method, tt.path, "{}", headers)
					if resp.Code != tc.wantStatus {
						t.Fatalf("status = %d, want %d; body=%s", resp.Code, tc.wantStatus, resp.Body.String())
					}
					if wantForward := tc.wantStatus == http.StatusNoContent; (forwarded > before) != wantForward {
						t.Fatalf("forwarded = %t, want %t", forwarded > before, wantForward)
					}
				})
			}
		})
	}

	t.Run("exchange accepts unauthenticated code proof", func(t *testing.T) {
		forwarded := false
		body := `{"code":"synthetic","redirectUri":"voicegame://callback","codeVerifier":"verifier-bytes","deviceProof":"proof-bytes"}`
		h := newGatewayForContract(t, gatewayTestOptions{
			restUpstreams: map[string]http.Handler{"auth": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				forwarded = true
				if r.Header.Get("Authorization") != "" {
					t.Errorf("unexpected Authorization header forwarded: %q", r.Header.Get("Authorization"))
				}
				got, err := io.ReadAll(r.Body)
				if err != nil || string(got) != body {
					t.Errorf("exchange body = %q, err=%v; want exact body %q", got, err, body)
				}
				w.WriteHeader(http.StatusNoContent)
			})},
		})
		resp := performRequest(h, http.MethodPost, base+"/"+requestID+"/exchange", body, map[string]string{
			"Authorization": "Bearer irrelevant-voice-access-token",
		})
		if resp.Code != http.StatusNoContent || !forwarded {
			t.Fatalf("exchange response = %d, forwarded = %t; body=%s", resp.Code, forwarded, resp.Body.String())
		}
	})

	t.Run("linked session accepts opaque Bearer and requires its presence", func(t *testing.T) {
		forwardedAuthorization := ""
		h := newGatewayForContract(t, gatewayTestOptions{
			restUpstreams: map[string]http.Handler{"auth": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				forwardedAuthorization = r.Header.Get("Authorization")
				w.WriteHeader(http.StatusNoContent)
			})},
		})
		resp := performRequest(h, http.MethodPost, base+"/linked-session", `{"deviceProof":"synthetic"}`, map[string]string{
			"Authorization": "Bearer " + linkedToken,
		})
		if resp.Code != http.StatusNoContent || forwardedAuthorization != "Bearer "+linkedToken {
			t.Fatalf("linked session response = %d, forwarded Authorization = %q; body=%s", resp.Code, forwardedAuthorization, resp.Body.String())
		}
		resp = performRequest(h, http.MethodPost, base+"/linked-session", `{}`, nil)
		if resp.Code != http.StatusUnauthorized {
			t.Fatalf("missing linked bearer status = %d, want 401", resp.Code)
		}
	})

	t.Run("only OAuth-facing SDK routes use AuthOAuth limit", func(t *testing.T) {
		cases := []struct {
			method, path, want string
		}{
			{http.MethodPost, base, "AuthOAuth"},
			{http.MethodPost, base + "/" + requestID + "/exchange", "AuthOAuth"},
			{http.MethodGet, base + "/" + requestID, "AuthOAuth"},
			{http.MethodPost, base + "/" + requestID + "/approve", "AuthOAuth"},
			{http.MethodPost, base + "/linked-session", "AuthOAuth"},
			{http.MethodPost, base + "/not-a-uuid/exchange", ""},
		}
		for _, tc := range cases {
			if got := rateLimitGroup(tc.method, tc.path); got != tc.want {
				t.Errorf("rateLimitGroup(%s %s) = %q, want %q", tc.method, tc.path, got, tc.want)
			}
		}
		rule := defaultRateLimitRules()["AuthOAuth"]
		if rule.Limit != 30 || rule.Window.String() != "15m0s" {
			t.Fatalf("AuthOAuth rule = %+v, want 30 / 15m", rule)
		}
	})

	t.Run("AuthOAuth buckets use account keys and IP keys for proof credentials", func(t *testing.T) {
		calls := []sdkAuthorizationRateCall{}
		h := newGatewayForContract(t, gatewayTestOptions{
			tokenClaims: map[string]tokenClaims{
				sdkToken:   {UserID: "sdk-account-1", AccountType: "sdk-account"},
				voiceToken: {UserID: "voice-account-1", AccountType: "regular"},
			},
			rateLimiter: sdkAuthorizationRecordingRateLimiter{calls: &calls},
			restUpstreams: map[string]http.Handler{"auth": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			})},
		})
		requests := []struct {
			method, path, authorization, wantKey string
		}{
			{http.MethodPost, base, "Bearer " + sdkToken, "user:sdk-account-1"},
			{http.MethodGet, base + "/" + requestID, "Bearer " + voiceToken, "user:voice-account-1"},
			{http.MethodPost, base + "/" + requestID + "/approve", "Bearer " + voiceToken, "user:voice-account-1"},
			{http.MethodPost, base + "/" + requestID + "/exchange", "", "ip:203.0.113.11"},
			{http.MethodPost, base + "/linked-session", "Bearer linked-opaque", "ip:203.0.113.11"},
		}
		for _, request := range requests {
			req := httptestRequest(request.method, request.path, `{}`, nil)
			req.RemoteAddr = "203.0.113.11:4567"
			if request.authorization != "" {
				req.Header.Set("Authorization", request.authorization)
			}
			resp := performPreparedRequest(h, req)
			if resp.Code != http.StatusNoContent {
				t.Fatalf("%s %s returned %d: %s", request.method, request.path, resp.Code, resp.Body.String())
			}
		}
		if len(calls) != len(requests) {
			t.Fatalf("rate limiter calls = %d, want %d", len(calls), len(requests))
		}
		for i, call := range calls {
			if call.group != "AuthOAuth" || call.key != requests[i].wantKey {
				t.Errorf("rate limiter call %d = (%q, %q), want (AuthOAuth, %q)", i, call.key, call.group, requests[i].wantKey)
			}
		}
	})
}

type sdkAuthorizationRateCall struct {
	key, group string
}

type sdkAuthorizationRecordingRateLimiter struct {
	calls *[]sdkAuthorizationRateCall
}

func (l sdkAuthorizationRecordingRateLimiter) Allow(_ context.Context, key, group string) (bool, error) {
	*l.calls = append(*l.calls, sdkAuthorizationRateCall{key: key, group: group})
	return true, nil
}
