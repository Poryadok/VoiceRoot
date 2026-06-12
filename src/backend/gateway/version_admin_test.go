package main

import (
	"net/http"
	"testing"
)

func TestAdminClientVersionsStaffCanReadAndWriteWindowsPolicy(t *testing.T) {
	t.Parallel()

	store := newMemoryVersionStore(map[string]clientVersionRecord{
		"windows": {
			Platform:            "windows",
			MinSupportedVersion: "1.4.0",
			LatestVersion:       "1.7.2",
			UpdateURL:           "https://updates.voice.example/windows/appcast.xml",
			ReleaseNotes:        "initial",
		},
	})

	h := newGatewayForContract(t, gatewayTestOptions{
		versionStore: store,
		tokenClaims: map[string]tokenClaims{
			"staff-token": {UserID: "staff-account", Roles: []string{"staff"}},
		},
	})

	get := performRequest(h, http.MethodGet, "/api/v1/admin/client-versions/windows", "", map[string]string{
		"Authorization": "Bearer staff-token",
	})
	if get.Code != http.StatusOK {
		t.Fatalf("staff get status = %d, want %d; body=%q", get.Code, http.StatusOK, get.Body.String())
	}
	var current struct {
		Platform            string `json:"platform"`
		MinSupportedVersion string `json:"min_supported_version"`
		LatestVersion       string `json:"latest_version"`
		UpdateURL           string `json:"update_url"`
		ReleaseNotes        string `json:"release_notes"`
	}
	decodeJSON(t, get.Body, &current)
	if current.Platform != "windows" || current.LatestVersion != "1.7.2" {
		t.Fatalf("staff get body = %+v", current)
	}

	put := performRequest(
		h,
		http.MethodPut,
		"/api/v1/admin/client-versions/windows",
		`{"min_supported_version":"1.5.0","latest_version":"1.8.0","update_url":"https://updates.voice.example/windows/v1.8.xml","release_notes":"staff updated"}`,
		map[string]string{
			"Authorization": "Bearer staff-token",
			"Content-Type":  "application/json",
		},
	)
	if put.Code != http.StatusOK {
		t.Fatalf("staff put status = %d, want %d; body=%q", put.Code, http.StatusOK, put.Body.String())
	}

	afterGet := performRequest(h, http.MethodGet, "/api/v1/admin/client-versions/windows", "", map[string]string{
		"Authorization": "Bearer staff-token",
	})
	if afterGet.Code != http.StatusOK {
		t.Fatalf("staff after put get status = %d, want %d; body=%q", afterGet.Code, http.StatusOK, afterGet.Body.String())
	}
	var updated struct {
		MinSupportedVersion string `json:"min_supported_version"`
		LatestVersion       string `json:"latest_version"`
		ReleaseNotes        string `json:"release_notes"`
	}
	decodeJSON(t, afterGet.Body, &updated)
	if updated.MinSupportedVersion != "1.5.0" || updated.LatestVersion != "1.8.0" || updated.ReleaseNotes != "staff updated" {
		t.Fatalf("updated policy = %+v", updated)
	}
}

func TestAdminClientVersionsNonStaffForbidden(t *testing.T) {
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
			"member-token": {UserID: "account-1", Roles: []string{"member"}},
		},
	})

	get := performRequest(h, http.MethodGet, "/api/v1/admin/client-versions/windows", "", map[string]string{
		"Authorization": "Bearer member-token",
	})
	if get.Code != http.StatusForbidden {
		t.Fatalf("non-staff get status = %d, want %d; body=%q", get.Code, http.StatusForbidden, get.Body.String())
	}

	put := performRequest(
		h,
		http.MethodPut,
		"/api/v1/admin/client-versions/windows",
		`{"min_supported_version":"1.5.0","latest_version":"1.8.0","update_url":"https://updates.voice.example/windows/v1.8.xml"}`,
		map[string]string{
			"Authorization": "Bearer member-token",
			"Content-Type":  "application/json",
		},
	)
	if put.Code != http.StatusForbidden {
		t.Fatalf("non-staff put status = %d, want %d; body=%q", put.Code, http.StatusForbidden, put.Body.String())
	}
}
