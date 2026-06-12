package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDynamicForceUpdateBlocksWebSocketUpgrade(t *testing.T) {
	t.Parallel()

	store := newMemoryVersionStore(map[string]clientVersionRecord{
		"windows": {
			Platform:            "windows",
			MinSupportedVersion: "1.4.0",
			LatestVersion:       "1.7.2",
			UpdateURL:           "https://updates.voice.example/windows/appcast.xml",
		},
	})

	h := newGatewayForContract(t, gatewayTestOptions{
		versionStore: store,
		realtimeUpstream: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusSwitchingProtocols)
		}),
	})

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	req.Header.Set("X-Voice-Client-Platform", "windows")
	req.Header.Set("X-Voice-Client-Version", "1.3.9")

	rec := performPreparedRequest(h, req)
	if rec.Code != http.StatusUpgradeRequired {
		t.Fatalf("status = %d, want %d; body=%q", rec.Code, http.StatusUpgradeRequired, rec.Body.String())
	}
}
