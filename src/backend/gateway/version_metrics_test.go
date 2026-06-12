package main

import (
	"net/http"
	"strings"
	"testing"
)

func TestForceUpdateBlockIncrementsGatewayMetric(t *testing.T) {
	t.Parallel()

	store := newMemoryVersionStore(map[string]clientVersionRecord{
		"windows": {
			Platform:            "windows",
			MinSupportedVersion: "1.4.0",
			LatestVersion:       "1.7.2",
			UpdateURL:           "https://updates.voice.example/windows/appcast.xml",
		},
	})
	metrics := newGatewayMetrics()

	h := newGatewayForContract(t, gatewayTestOptions{
		versionStore: store,
		metrics:      metrics,
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		restUpstreams: map[string]http.Handler{
			"users": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			}),
		},
	})

	blocked := performRequest(h, http.MethodGet, "/api/v1/users/me", "", map[string]string{
		"Authorization":           "Bearer valid-user-token",
		"X-Voice-Client-Platform": "windows",
		"X-Voice-Client-Version":  "1.3.9",
	})
	if blocked.Code != http.StatusUpgradeRequired {
		t.Fatalf("status = %d, want %d; body=%q", blocked.Code, http.StatusUpgradeRequired, blocked.Body.String())
	}

	rec := performRequest(h, http.MethodGet, "/metrics", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("metrics status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "gateway_force_update_blocks") {
		t.Fatalf("metrics body missing gateway_force_update_blocks:\n%s", body)
	}
	if !strings.Contains(body, `platform="windows"`) {
		t.Fatalf("metrics body missing windows platform label:\n%s", body)
	}
}
