package main

import (
	"net/http"
	"testing"

	"github.com/alicebob/miniredis/v2"
)

func TestVersionEndpointCachesPolicyInRedis(t *testing.T) {
	t.Parallel()

	mr := miniredis.RunT(t)
	store := newMemoryVersionStore(map[string]clientVersionRecord{
		"windows": {
			Platform:            "windows",
			MinSupportedVersion: "1.4.0",
			LatestVersion:       "1.7.2",
			UpdateURL:           "https://updates.voice.example/windows/appcast.xml",
			ReleaseNotes:        "cached policy",
		},
	})

	h := newGatewayForContract(t, gatewayTestOptions{
		versionStore:      store,
		versionCacheRedis: mr.Addr(),
		versionConfigs: map[string]versionConfig{
			"windows": {
				MinSupportedVersion: "9.9.9",
				LatestVersion:       "9.9.9",
			},
		},
	})

	target := "/api/v1/version?platform=windows&version=1.7.2"
	first := performRequest(h, http.MethodGet, target, "", nil)
	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d, want %d; body=%q", first.Code, http.StatusOK, first.Body.String())
	}

	store.data["windows"] = clientVersionRecord{
		Platform:            "windows",
		MinSupportedVersion: "2.0.0",
		LatestVersion:       "2.1.0",
		UpdateURL:           "https://updates.voice.example/windows/v2.xml",
		ReleaseNotes:        "uncached policy",
	}

	second := performRequest(h, http.MethodGet, target, "", nil)
	if second.Code != http.StatusOK {
		t.Fatalf("second status = %d, want %d; body=%q", second.Code, http.StatusOK, second.Body.String())
	}

	var cached struct {
		LatestVersion string `json:"latest_version"`
		ReleaseNotes  string `json:"release_notes"`
	}
	decodeJSON(t, second.Body, &cached)
	if cached.LatestVersion != "1.7.2" || cached.ReleaseNotes != "cached policy" {
		t.Fatalf("second response = %+v, want cached policy from first call", cached)
	}
	if store.getCount() != 1 {
		t.Fatalf("version store gets = %d, want 1 (second call should hit redis cache)", store.getCount())
	}
}

func TestAdminClientVersionPutInvalidatesVersionCache(t *testing.T) {
	t.Parallel()

	mr := miniredis.RunT(t)
	store := newMemoryVersionStore(map[string]clientVersionRecord{
		"windows": {
			Platform:            "windows",
			MinSupportedVersion: "1.4.0",
			LatestVersion:       "1.7.2",
			UpdateURL:           "https://updates.voice.example/windows/appcast.xml",
			ReleaseNotes:        "before admin update",
		},
	})

	h := newGatewayForContract(t, gatewayTestOptions{
		versionStore:      store,
		versionCacheRedis: mr.Addr(),
		tokenClaims: map[string]tokenClaims{
			"staff-token": {UserID: "staff-account", Roles: []string{"staff"}},
		},
	})

	warm := performRequest(h, http.MethodGet, "/api/v1/version?platform=windows&version=1.7.2", "", nil)
	if warm.Code != http.StatusOK {
		t.Fatalf("warm status = %d, want %d; body=%q", warm.Code, http.StatusOK, warm.Body.String())
	}

	put := performRequest(
		h,
		http.MethodPut,
		"/api/v1/admin/client-versions/windows",
		`{"min_supported_version":"1.4.0","latest_version":"1.8.0","update_url":"https://updates.voice.example/windows/v1.8.xml","release_notes":"after admin update"}`,
		map[string]string{
			"Authorization": "Bearer staff-token",
			"Content-Type":  "application/json",
		},
	)
	if put.Code != http.StatusOK {
		t.Fatalf("admin put status = %d, want %d; body=%q", put.Code, http.StatusOK, put.Body.String())
	}

	after := performRequest(h, http.MethodGet, "/api/v1/version?platform=windows&version=1.7.2", "", nil)
	if after.Code != http.StatusOK {
		t.Fatalf("after status = %d, want %d; body=%q", after.Code, http.StatusOK, after.Body.String())
	}

	var got struct {
		LatestVersion string `json:"latest_version"`
		ReleaseNotes  string `json:"release_notes"`
		UpdateURL     string `json:"update_url"`
	}
	decodeJSON(t, after.Body, &got)
	if got.LatestVersion != "1.8.0" || got.ReleaseNotes != "after admin update" {
		t.Fatalf("version response after cache invalidation = %+v", got)
	}
	if got.UpdateURL != "https://updates.voice.example/windows/v1.8.xml" {
		t.Fatalf("update_url = %q", got.UpdateURL)
	}
}
