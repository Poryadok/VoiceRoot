package main

import (
	"net/http"
	"strings"
	"testing"
)

func TestVersionEndpoint(t *testing.T) {
	t.Parallel()

	h := newGatewayForContract(t, gatewayTestOptions{
		versionConfigs: map[string]versionConfig{
			"android": {
				MinSupportedVersion: "1.4.0",
				LatestVersion:       "1.7.2",
				UpdateURL:           "https://updates.voice.example/android",
				ReleaseNotes:        "Android voice fixes",
				ShorebirdPatch:      42,
			},
		},
	})

	tests := []struct {
		name                string
		target              string
		wantForceUpdate     bool
		wantUpdateAvailable bool
	}{
		{
			name:                "current supported version reports no update",
			target:              "/api/v1/version?platform=android&version=1.7.2&shorebird_patch=42",
			wantForceUpdate:     false,
			wantUpdateAvailable: false,
		},
		{
			name:                "below minimum version forces update",
			target:              "/api/v1/version?platform=android&version=1.3.9&shorebird_patch=7",
			wantForceUpdate:     true,
			wantUpdateAvailable: true,
		},
		{
			name:                "below latest but still supported offers soft update",
			target:              "/api/v1/version?platform=android&version=1.6.0&shorebird_patch=12",
			wantForceUpdate:     false,
			wantUpdateAvailable: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rec := performRequest(h, http.MethodGet, tc.target, "", nil)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d; body=%q", rec.Code, http.StatusOK, rec.Body.String())
			}
			if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
				t.Fatalf("Content-Type = %q, want application/json", got)
			}

			var got struct {
				ForceUpdate     bool   `json:"force_update"`
				UpdateAvailable bool   `json:"update_available"`
				LatestVersion   string `json:"latest_version"`
				MinSupported    string `json:"min_supported_version"`
				UpdateURL       string `json:"update_url"`
				ReleaseNotes    string `json:"release_notes"`
				ShorebirdPatch  int    `json:"shorebird_patch"`
			}
			decodeJSON(t, rec.Body, &got)

			if got.ForceUpdate != tc.wantForceUpdate {
				t.Fatalf("force_update = %v, want %v", got.ForceUpdate, tc.wantForceUpdate)
			}
			if got.UpdateAvailable != tc.wantUpdateAvailable {
				t.Fatalf("update_available = %v, want %v", got.UpdateAvailable, tc.wantUpdateAvailable)
			}
			if got.LatestVersion != "1.7.2" || got.MinSupported != "1.4.0" {
				t.Fatalf("versions = latest %q min %q, want latest 1.7.2 min 1.4.0", got.LatestVersion, got.MinSupported)
			}
			if got.UpdateURL != "https://updates.voice.example/android" {
				t.Fatalf("update_url = %q", got.UpdateURL)
			}
			if got.ReleaseNotes != "Android voice fixes" {
				t.Fatalf("release_notes = %q", got.ReleaseNotes)
			}
			if got.ShorebirdPatch != 42 {
				t.Fatalf("shorebird_patch = %d, want 42", got.ShorebirdPatch)
			}
		})
	}
}

func TestForceUpdateBlocksEveryAPIRouteExceptVersion(t *testing.T) {
	t.Parallel()

	h := newGatewayForContract(t, gatewayTestOptions{
		forceUpdate: &forceUpdatePolicy{
			Platform:  "android",
			Version:   "1.3.9",
			UpdateURL: "https://updates.voice.example/android",
		},
		restUpstreams: map[string]http.Handler{
			"messages": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			}),
		},
	})

	blocked := performRequest(h, http.MethodPost, "/api/v1/messages/send", `{"text":"hi"}`, map[string]string{
		"Authorization":           "Bearer valid-user-token",
		"X-Voice-Client-Platform": "android",
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
	if got.Error != "client_outdated" || got.UpdateURL != "https://updates.voice.example/android" {
		t.Fatalf("error body = %+v", got)
	}

	version := performRequest(h, http.MethodGet, "/api/v1/version?platform=android&version=1.3.9", "", nil)
	if version.Code == http.StatusUpgradeRequired {
		t.Fatalf("/api/v1/version must not be blocked by force-update policy")
	}
}
