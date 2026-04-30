package main

import (
	"net/http"
	"strconv"
	"strings"
)

type versionConfig struct {
	MinSupportedVersion string
	LatestVersion       string
	UpdateURL           string
	ReleaseNotes        string
	ShorebirdPatch      int
}

type forceUpdatePolicy struct {
	Platform  string
	Version   string
	UpdateURL string
}

func (g *gateway) handleVersion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
		return
	}

	platform := r.URL.Query().Get("platform")
	clientVersion := r.URL.Query().Get("version")
	cfg, ok := g.config.versionConfigs[platform]
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown_platform"})
		return
	}

	forceUpdate := compareSemver(clientVersion, cfg.MinSupportedVersion) < 0
	updateAvailable := compareSemver(clientVersion, cfg.LatestVersion) < 0
	writeJSON(w, http.StatusOK, map[string]any{
		"force_update":          forceUpdate,
		"update_available":      updateAvailable,
		"latest_version":        cfg.LatestVersion,
		"min_supported_version": cfg.MinSupportedVersion,
		"update_url":            cfg.UpdateURL,
		"release_notes":         cfg.ReleaseNotes,
		"shorebird_patch":       cfg.ShorebirdPatch,
	})
}

func (g *gateway) isForceUpdateBlocked(r *http.Request) bool {
	if g.config.forceUpdate == nil {
		return false
	}
	return r.Header.Get("X-Voice-Client-Platform") == g.config.forceUpdate.Platform &&
		r.Header.Get("X-Voice-Client-Version") == g.config.forceUpdate.Version
}

func compareSemver(left, right string) int {
	lv := parseSemver(left)
	rv := parseSemver(right)
	for i := 0; i < 3; i++ {
		switch {
		case lv[i] < rv[i]:
			return -1
		case lv[i] > rv[i]:
			return 1
		}
	}
	return 0
}

func parseSemver(version string) [3]int {
	var parsed [3]int
	parts := strings.Split(version, ".")
	for i := 0; i < len(parts) && i < 3; i++ {
		value, err := strconv.Atoi(parts[i])
		if err == nil {
			parsed[i] = value
		}
	}
	return parsed
}
