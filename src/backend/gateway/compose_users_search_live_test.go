package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func liveComposeEnabled() bool {
	v := strings.TrimSpace(os.Getenv("VOICE_RUN_LIVE_COMPOSE"))
	return v == "true" || v == "1"
}

func liveGatewayBaseURL() string {
	if u := strings.TrimSpace(os.Getenv("VOICE_API_BASE_URL")); u != "" {
		return strings.TrimRight(u, "/")
	}
	return "http://127.0.0.1:18080"
}

// clearLiveComposeAuthRateLimit removes gateway auth rate-limit keys in compose Redis
// so parallel live tests can register without 429 (dev stack only).
func clearLiveComposeAuthRateLimit(t *testing.T) {
	t.Helper()
	if !liveComposeEnabled() {
		return
	}
	root := repoRootFromTest(t)
	for _, pattern := range []string{"ratelimit:AuthLogin:*", "ratelimit:AuthRegister:*", "ratelimit:Auth:*", "ratelimit:FileUpload:*"} {
		cmd := exec.Command("docker", "compose", "exec", "-T", "redis", "redis-cli", "--scan", "--pattern", pattern)
		cmd.Dir = root
		out, err := cmd.Output()
		if err != nil {
			t.Logf("skip auth rate-limit clear pattern %s (redis unavailable): %v", pattern, err)
			continue
		}
		for _, key := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			key = strings.TrimSpace(key)
			if key == "" {
				continue
			}
			del := exec.Command("docker", "compose", "exec", "-T", "redis", "redis-cli", "DEL", key)
			del.Dir = root
			_ = del.Run()
		}
	}
}

type authSessionResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ProfileID    string `json:"profile_id"`
	AccountID    string `json:"account_id"`
}

type authSessionEnvelope struct {
	Session authSessionResponse `json:"session"`
}

// TestComposeUsersSearch_live exercises GET /api/v1/users/search through Gateway
// with User + Social gRPC upstreams (docker compose --profile app).
//
// Opt-in: VOICE_RUN_LIVE_COMPOSE=true VOICE_API_BASE_URL=http://127.0.0.1:18080
func TestComposeUsersSearch_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 30 * time.Second}
	base := liveGatewayBaseURL()

	healthResp, err := client.Get(base + "/health")
	require.NoError(t, err)
	healthResp.Body.Close()
	require.Equal(t, http.StatusOK, healthResp.StatusCode, "gateway health at %s", base)

	n := time.Now().UnixNano()
	emailA := fmt.Sprintf("search-a-%d@voice-qa.test", n)
	emailB := fmt.Sprintf("search-b-%d@voice-qa.test", n)
	const password = "VoiceQaTest1!"

	sessA := registerComposeUser(t, client, base, emailA, password)
	registerComposeUser(t, client, base, emailB, password)

	searchToken := strings.TrimSuffix(emailB, "@voice-qa.test")
	searchURL := base + "/api/v1/users/search?q=" + searchToken
	req, err := http.NewRequest(http.MethodGet, searchURL, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+sessA.AccessToken)

	searchResp, err := client.Do(req)
	require.NoError(t, err)
	defer searchResp.Body.Close()
	body, _ := io.ReadAll(searchResp.Body)
	require.Equal(t, http.StatusOK, searchResp.StatusCode,
		"GET /api/v1/users/search must not 404 once users upstream is wired; body=%s", string(body))

	var parsed struct {
		ProfileList struct {
			Profiles []struct {
				ID          string `json:"id"`
				AccountID   string `json:"account_id"`
				DisplayName string `json:"display_name"`
			} `json:"profiles"`
		} `json:"profile_list"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	require.NotEmpty(t, parsed.ProfileList.Profiles, "expected at least one profile for q=%q", searchToken)

	found := false
	for _, p := range parsed.ProfileList.Profiles {
		if strings.Contains(strings.ToLower(p.DisplayName), strings.ToLower(searchToken)) {
			found = true
			break
		}
	}
	require.True(t, found, "search results should include profile B by display_name/email hint; body=%s", string(body))
}

func registerComposeUser(t *testing.T, client *http.Client, base, email, password string) authSessionResponse {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"email":            email,
		"password":         password,
		"guest":            false,
		"device_info_json": `{"platform":"go-live-test"}`,
	})
	require.NoError(t, err)

	resp, err := client.Post(base+"/api/v1/auth/register", "application/json", bytes.NewReader(payload))
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode,
		"register %s: status=%d body=%s", email, resp.StatusCode, string(raw))

	var envelope authSessionEnvelope
	require.NoError(t, json.Unmarshal(raw, &envelope))
	sess := envelope.Session
	require.NotEmpty(t, sess.AccessToken, "register %s: body=%s", email, string(raw))
	require.NotEmpty(t, sess.ProfileID, "register %s: body=%s", email, string(raw))
	return sess
}
