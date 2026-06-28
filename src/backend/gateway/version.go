package main

import (
	"context"
	"net/http"
	"strconv"
	"strings"
)

type versionConfig struct {
	MinSupportedVersion string `json:"min_supported_version"`
	LatestVersion       string `json:"latest_version"`
	UpdateURL           string `json:"update_url"`
	ReleaseNotes        string `json:"release_notes"`
	ShorebirdPatch      int    `json:"shorebird_patch"`
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
	record, ok := g.resolveClientVersion(r.Context(), platform)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown_platform"})
		return
	}
	if _, ok := parseSemver(clientVersion); !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_version"})
		return
	}
	if _, ok := parseSemver(record.MinSupportedVersion); !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "version_policy_invalid"})
		return
	}
	if _, ok := parseSemver(record.LatestVersion); !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "version_policy_invalid"})
		return
	}

	forceUpdate := compareSemver(clientVersion, record.MinSupportedVersion) < 0
	updateAvailable := compareSemver(clientVersion, record.LatestVersion) < 0
	writeJSON(w, http.StatusOK, map[string]any{
		"force_update":          forceUpdate,
		"update_available":      updateAvailable,
		"latest_version":        record.LatestVersion,
		"min_supported_version": record.MinSupportedVersion,
		"update_url":            record.UpdateURL,
		"release_notes":         record.ReleaseNotes,
		"shorebird_patch":       record.ShorebirdPatch,
	})
}

func (g *gateway) resolveClientVersion(ctx context.Context, platform string) (clientVersionRecord, bool) {
	if platform == "" {
		return clientVersionRecord{}, false
	}
	if g.versionCache != nil {
		if record, ok := g.versionCache.Get(ctx, platform); ok {
			return record, true
		}
	}

	record, err := g.loadClientVersion(ctx, platform)
	if err != nil {
		return clientVersionRecord{}, false
	}
	if g.versionCache != nil {
		g.versionCache.Set(ctx, platform, record)
	}
	return record, true
}

func (g *gateway) loadClientVersion(ctx context.Context, platform string) (clientVersionRecord, error) {
	if g.config.versionStore != nil {
		record, err := g.config.versionStore.Get(ctx, platform)
		if err == nil {
			return record, nil
		}
		if !errorsIsUnknownPlatform(err) {
			return clientVersionRecord{}, err
		}
	}
	if cfg, ok := g.config.versionConfigs[platform]; ok {
		return recordFromConfig(platform, cfg), nil
	}
	return clientVersionRecord{}, errUnknownPlatform
}

func (g *gateway) forceUpdateDecision(r *http.Request) (bool, string) {
	platform := r.Header.Get("X-Voice-Client-Platform")
	version := r.Header.Get("X-Voice-Client-Version")

	if g.config.forceUpdate != nil &&
		platform == g.config.forceUpdate.Platform &&
		version == g.config.forceUpdate.Version {
		return true, g.config.forceUpdate.UpdateURL
	}

	if platform == "" || version == "" {
		return false, ""
	}
	if _, ok := parseSemver(version); !ok {
		return false, ""
	}

	record, ok := g.resolveClientVersion(r.Context(), platform)
	if !ok {
		return false, ""
	}
	if compareSemver(version, record.MinSupportedVersion) < 0 {
		return true, record.UpdateURL
	}
	return false, ""
}

func compareSemver(left, right string) int {
	lv, _ := parseSemver(left)
	rv, _ := parseSemver(right)
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

func parseSemver(version string) ([3]int, bool) {
	var parsed [3]int
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return parsed, false
	}
	for i := 0; i < len(parts); i++ {
		value, err := strconv.Atoi(parts[i])
		if err != nil || value < 0 {
			return parsed, false
		}
		parsed[i] = value
	}
	return parsed, true
}
