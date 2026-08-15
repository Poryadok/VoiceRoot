package main

import (
	"encoding/json"
	"net/http"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestComposeAnalyticsExport_live documents staff CSV export and audit path (analytics.md DoD §3).
func TestComposeAnalyticsExport_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()
	staffToken := composeStaffToken(t, client, base)
	if staffToken == "" {
		t.Skip("no staff token; set GATEWAY_STATIC_TOKENS_JSON")
	}

	markerRoute := "/api/v1/analytics/export"
	req, err := http.NewRequest(http.MethodGet, base+markerRoute+"?format=csv", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+staffToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	require.True(t, composeAnalyticsAuditContainsRoute(t, markerRoute),
		"expected gateway analytics audit store entry for %s", markerRoute)
}

func composeAnalyticsAuditContainsRoute(t *testing.T, route string) bool {
	t.Helper()
	root := repoRootFromTest(t)
	cmd := exec.Command("docker", "compose", "exec", "-T", "redis", "redis-cli", "LRANGE", analyticsAuditRedisKey, "0", "19")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Logf("redis analytics audit unavailable: %v", err)
		return false
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var entry analyticsAuditEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		if entry.Route == route {
			return true
		}
	}
	return false
}
