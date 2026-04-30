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
