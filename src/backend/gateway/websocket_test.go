package main

import (
	"net/http"
	"testing"
)

func TestWebSocketBoundary(t *testing.T) {
	t.Parallel()

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		realtimeUpstream: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("X-Upstream-Namespace", "realtime")
			w.WriteHeader(http.StatusSwitchingProtocols)
		}),
	})

	plain := performRequest(h, http.MethodGet, "/ws", "", nil)
	if plain.Code != http.StatusBadRequest {
		t.Fatalf("plain /ws status = %d, want %d", plain.Code, http.StatusBadRequest)
	}

	upgradeWithoutJWT := performRequest(h, http.MethodGet, "/ws", "", map[string]string{
		"Connection":            "Upgrade",
		"Upgrade":               "websocket",
		"Sec-WebSocket-Key":     "dGhlIHNhbXBsZSBub25jZQ==",
		"Sec-WebSocket-Version": "13",
	})
	if upgradeWithoutJWT.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated /ws upgrade status = %d, want %d", upgradeWithoutJWT.Code, http.StatusUnauthorized)
	}

	upgrade := performRequest(h, http.MethodGet, "/ws", "", map[string]string{
		"Authorization":         "Bearer valid-user-token",
		"Connection":            "Upgrade",
		"Upgrade":               "websocket",
		"Sec-WebSocket-Key":     "dGhlIHNhbXBsZSBub25jZQ==",
		"Sec-WebSocket-Version": "13",
	})
	if upgrade.Code != http.StatusSwitchingProtocols {
		t.Fatalf("authenticated /ws upgrade status = %d, want %d; body=%q", upgrade.Code, http.StatusSwitchingProtocols, upgrade.Body.String())
	}
	if got := upgrade.Header().Get("X-Upstream-Namespace"); got != "realtime" {
		t.Fatalf("/ws upstream = %q, want realtime", got)
	}
}

func TestWebSocketPropagatesRequestContext(t *testing.T) {
	t.Parallel()

	var downstream http.Header
	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {
				UserID:           "account-1",
				ProfileID:        "profile-1",
				Roles:            []string{"member"},
				SubscriptionTier: "free",
			},
		},
		realtimeUpstream: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			downstream = r.Header.Clone()
			w.WriteHeader(http.StatusSwitchingProtocols)
		}),
	})

	rec := performRequest(h, http.MethodGet, "/ws", "", map[string]string{
		"Authorization":         "Bearer valid-user-token",
		"Connection":            "keep-alive, Upgrade",
		"Upgrade":               "websocket",
		"Sec-WebSocket-Key":     "dGhlIHNhbXBsZSBub25jZQ==",
		"Sec-WebSocket-Version": "13",
		"X-Request-Id":          "client-request-id",
	})

	if rec.Code != http.StatusSwitchingProtocols {
		t.Fatalf("status = %d, want %d; body=%q", rec.Code, http.StatusSwitchingProtocols, rec.Body.String())
	}
	for header, want := range map[string]string{
		"X-Request-Id":              "client-request-id",
		"X-Voice-User-Id":           "account-1",
		"X-Voice-Profile-Id":        "profile-1",
		"X-Voice-Roles":             "member",
		"X-Voice-Subscription-Tier": "free",
		"Sec-WebSocket-Version":     "13",
		"Sec-WebSocket-Key":         "dGhlIHNhbXBsZSBub25jZQ==",
		"Upgrade":                   "websocket",
		"Connection":                "keep-alive, Upgrade",
	} {
		if got := downstream.Get(header); got != want {
			t.Fatalf("%s = %q, want %q", header, got, want)
		}
	}
}
