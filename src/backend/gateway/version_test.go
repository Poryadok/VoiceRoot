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

func TestVersionEndpointIOS(t *testing.T) {
	t.Parallel()

	h := newGatewayForContract(t, gatewayTestOptions{
		versionConfigs: map[string]versionConfig{
			"ios": {
				MinSupportedVersion: "2.0.0",
				LatestVersion:       "2.3.1",
				UpdateURL:           "https://apps.apple.com/app/voice/id000000000",
				ReleaseNotes:        "iOS voice fixes",
				ShorebirdPatch:      7,
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
			name:                "current ios version reports no update",
			target:              "/api/v1/version?platform=ios&version=2.3.1&shorebird_patch=7",
			wantForceUpdate:     false,
			wantUpdateAvailable: false,
		},
		{
			name:                "below ios minimum forces update",
			target:              "/api/v1/version?platform=ios&version=1.9.9&shorebird_patch=1",
			wantForceUpdate:     true,
			wantUpdateAvailable: true,
		},
		{
			name:                "below ios latest offers soft update",
			target:              "/api/v1/version?platform=ios&version=2.2.0&shorebird_patch=3",
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
			if got.LatestVersion != "2.3.1" || got.MinSupported != "2.0.0" {
				t.Fatalf("versions = latest %q min %q, want latest 2.3.1 min 2.0.0", got.LatestVersion, got.MinSupported)
			}
			if got.UpdateURL != "https://apps.apple.com/app/voice/id000000000" {
				t.Fatalf("update_url = %q", got.UpdateURL)
			}
			if got.ReleaseNotes != "iOS voice fixes" {
				t.Fatalf("release_notes = %q", got.ReleaseNotes)
			}
			if got.ShorebirdPatch != 7 {
				t.Fatalf("shorebird_patch = %d, want 7", got.ShorebirdPatch)
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

func TestVersionEndpointWindows(t *testing.T) {
	t.Parallel()

	h := newGatewayForContract(t, gatewayTestOptions{
		versionConfigs: map[string]versionConfig{
			"windows": {
				MinSupportedVersion: "1.4.0",
				LatestVersion:       "1.7.2",
				UpdateURL:           "https://updates.voice.example/windows/appcast.xml",
				ReleaseNotes:        "Windows desktop voice fixes",
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
			name:                "current windows version reports no update",
			target:              "/api/v1/version?platform=windows&version=1.7.2",
			wantForceUpdate:     false,
			wantUpdateAvailable: false,
		},
		{
			name:                "below windows minimum forces update",
			target:              "/api/v1/version?platform=windows&version=1.3.9",
			wantForceUpdate:     true,
			wantUpdateAvailable: true,
		},
		{
			name:                "below windows latest offers soft update",
			target:              "/api/v1/version?platform=windows&version=1.6.0",
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

			var got struct {
				ForceUpdate     bool   `json:"force_update"`
				UpdateAvailable bool   `json:"update_available"`
				LatestVersion   string `json:"latest_version"`
				MinSupported    string `json:"min_supported_version"`
				UpdateURL       string `json:"update_url"`
				ReleaseNotes    string `json:"release_notes"`
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
			if got.UpdateURL != "https://updates.voice.example/windows/appcast.xml" {
				t.Fatalf("update_url = %q", got.UpdateURL)
			}
			if got.ReleaseNotes != "Windows desktop voice fixes" {
				t.Fatalf("release_notes = %q", got.ReleaseNotes)
			}
		})
	}
}

func TestVersionEndpointRejectsMalformedVersion(t *testing.T) {
	t.Parallel()

	h := newGatewayForContract(t, gatewayTestOptions{
		versionConfigs: map[string]versionConfig{
			"android": {
				MinSupportedVersion: "1.4.0",
				LatestVersion:       "1.7.2",
			},
		},
	})

	for _, target := range []string{
		"/api/v1/version?platform=android",
		"/api/v1/version?platform=android&version=1.2",
		"/api/v1/version?platform=android&version=bad",
	} {
		target := target
		t.Run(target, func(t *testing.T) {
			t.Parallel()
			rec := performRequest(h, http.MethodGet, target, "", nil)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body=%q", rec.Code, http.StatusBadRequest, rec.Body.String())
			}
			var got struct {
				Error string `json:"error"`
			}
			decodeJSON(t, rec.Body, &got)
			if got.Error != "invalid_version" {
				t.Fatalf("error = %q, want invalid_version", got.Error)
			}
		})
	}
}
