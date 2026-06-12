package main

import (
	"encoding/json"
	"net/http"
	"strings"
)

func (g *gateway) handleAdminClientVersions(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/client-versions")
	path = strings.Trim(path, "/")
	if path == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
		return
	}
	platform := path

	claims, code := g.authenticate(r)
	if code != "" {
		status := http.StatusUnauthorized
		if code == "auth_unavailable" {
			status = http.StatusServiceUnavailable
		}
		writeJSON(w, status, map[string]string{"error": code})
		return
	}
	if !hasRole(claims, "staff") {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}

	store := g.config.versionStore
	if store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "version_store_unavailable"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		g.handleAdminClientVersionGet(w, r, store, platform)
	case http.MethodPut:
		g.handleAdminClientVersionPut(w, r, store, platform)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
	}
}

func (g *gateway) handleAdminClientVersionGet(w http.ResponseWriter, r *http.Request, store versionStore, platform string) {
	record, err := store.Get(r.Context(), platform)
	if err != nil {
		if errorsIsUnknownPlatform(err) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown_platform"})
			return
		}
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "version_store_unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, adminClientVersionResponse(record))
}

func (g *gateway) handleAdminClientVersionPut(w http.ResponseWriter, r *http.Request, store versionStore, platform string) {
	var body struct {
		MinSupportedVersion string `json:"min_supported_version"`
		LatestVersion       string `json:"latest_version"`
		UpdateURL           string `json:"update_url"`
		ReleaseNotes        string `json:"release_notes"`
		ShorebirdPatch      int    `json:"shorebird_patch"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
		return
	}
	if body.MinSupportedVersion == "" || body.LatestVersion == "" || body.UpdateURL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_version_policy"})
		return
	}
	if _, ok := parseSemver(body.MinSupportedVersion); !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_version"})
		return
	}
	if _, ok := parseSemver(body.LatestVersion); !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_version"})
		return
	}

	record := clientVersionRecord{
		Platform:            platform,
		MinSupportedVersion: body.MinSupportedVersion,
		LatestVersion:       body.LatestVersion,
		UpdateURL:           body.UpdateURL,
		ReleaseNotes:        body.ReleaseNotes,
		ShorebirdPatch:      body.ShorebirdPatch,
	}
	if err := store.Set(r.Context(), record); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "version_store_unavailable"})
		return
	}
	if g.versionCache != nil {
		g.versionCache.Invalidate(r.Context(), platform)
	}
	writeJSON(w, http.StatusOK, adminClientVersionResponse(record))
}

func adminClientVersionResponse(record clientVersionRecord) map[string]any {
	return map[string]any{
		"platform":              record.Platform,
		"min_supported_version": record.MinSupportedVersion,
		"latest_version":        record.LatestVersion,
		"update_url":            record.UpdateURL,
		"release_notes":         record.ReleaseNotes,
		"shorebird_patch":       record.ShorebirdPatch,
	}
}

func errorsIsUnknownPlatform(err error) bool {
	return strings.Contains(err.Error(), errUnknownPlatform.Error())
}
