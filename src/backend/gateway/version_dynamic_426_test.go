package main

import (
	"net/http"
	"testing"
)

func TestDynamicForceUpdateBlocksWindowsClientBelowMinSupported(t *testing.T) {
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
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		restUpstreams: map[string]http.Handler{
			"messages": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			}),
		},
	})

	blocked := performRequest(h, http.MethodPost, "/api/v1/messages/send", `{"text":"hi"}`, map[string]string{
		"Authorization":           "Bearer valid-user-token",
		"X-Voice-Client-Platform": "windows",
		"X-Voice-Client-Version":  "1.3.9",
	})
	if blocked.Code != http.StatusUpgradeRequired {
		t.Fatalf("status = %d, want %d; body=%q", blocked.Code, http.StatusUpgradeRequired, blocked.Body.String())
	}
	var got struct {
		Error     string `json:"error"`
		UpdateURL string `json:"update_url"`
	}
	decodeJSON(t, blocked.Body, &got)
	if got.Error != "client_outdated" || got.UpdateURL != "https://updates.voice.example/windows/appcast.xml" {
		t.Fatalf("error body = %+v", got)
	}
}

func TestDynamicForceUpdateExemptsVersionEndpoint(t *testing.T) {
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
	})

	version := performRequest(h, http.MethodGet, "/api/v1/version?platform=windows&version=1.3.9", "", map[string]string{
		"X-Voice-Client-Platform": "windows",
		"X-Voice-Client-Version":  "1.3.9",
	})
	if version.Code == http.StatusUpgradeRequired {
		t.Fatalf("/api/v1/version must not be blocked by dynamic force-update policy")
	}
	if version.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%q", version.Code, http.StatusOK, version.Body.String())
	}
}

func TestDynamicForceUpdateAllowsSupportedWindowsClient(t *testing.T) {
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
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		restUpstreams: map[string]http.Handler{
			"users": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			}),
		},
	})

	allowed := performRequest(h, http.MethodGet, "/api/v1/users/me", "", map[string]string{
		"Authorization":           "Bearer valid-user-token",
		"X-Voice-Client-Platform": "windows",
		"X-Voice-Client-Version":  "1.7.2",
	})
	if allowed.Code == http.StatusUpgradeRequired {
		t.Fatalf("supported windows client must not be blocked; body=%q", allowed.Body.String())
	}
	if allowed.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body=%q", allowed.Code, http.StatusNoContent, allowed.Body.String())
	}
}
