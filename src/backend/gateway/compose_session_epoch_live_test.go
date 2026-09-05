package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

const epochLiveIsolationEnv = "VOICE_A1_SESSION_EPOCH_ISOLATED"

var epochLiveComposeProject = regexp.MustCompile(`^voice-a1-multi-[a-z0-9-]+$`)

// TestComposeA1SessionEpochRealtime_live proves the final strict rollout with
// Auth-issued credentials only. It is restricted to the runner-owned A1
// Compose project because the missing-floor subcase intentionally removes one
// fixture account's Auth-owned Redis key.
func TestComposeA1SessionEpochRealtime_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against the isolated A1 Compose project")
	}
	if os.Getenv(epochLiveIsolationEnv) != "true" {
		t.Skip("session-epoch fault injection requires the isolated A1 Compose runner")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()
	const password = "VoiceQaTest1!"
	now := time.Now().UnixNano()

	first := registerComposeUser(t, client, base, formatComposeEmail("a1-epoch-a", now), password)
	second := loginComposeUser(t, client, base, formatComposeEmail("a1-epoch-a", now), password)
	require.NotEmpty(t, first.AccountID, "Auth register must return the account identifier used by the floor key")
	require.NotEmpty(t, second.AccountID, "Auth login must return the account identifier used by the floor key")
	require.Equal(t, first.AccountID, second.AccountID)
	require.NotEqual(t, first.AccessToken, second.AccessToken)

	secondConn := dialComposeRealtimeWS(t, base, second.AccessToken)
	waitComposeWSHello(t, secondConn)
	composeWSSend(t, secondConn, map[string]any{"op": "heartbeat", "d": map[string]any{}})
	waitComposeWSOp(t, secondConn, "heartbeat_ack", 15*time.Second, nil)

	deleteComposeEpochLiveAccount(t, client, base, first.AccessToken, password)
	composeWSSend(t, secondConn, map[string]any{"op": "heartbeat", "d": map[string]any{}})
	requireEpochLiveRevokedClose(t, secondConn)
	requireEpochLiveUpgradeDenied(t, base, second.AccessToken, http.StatusUnauthorized, "token_revoked")

	fixture := registerComposeUser(t, client, base, formatComposeEmail("a1-epoch-floor", now), password)
	require.NotEmpty(t, fixture.AccountID, "Auth register must return the exact account identifier for the fixture floor")
	fixtureConn := dialComposeRealtimeWS(t, base, fixture.AccessToken)
	waitComposeWSHello(t, fixtureConn)
	require.NoError(t, fixtureConn.Close())

	deleteEpochLiveFixtureFloor(t, fixture.AccountID)
	requireEpochLiveUpgradeDenied(t, base, fixture.AccessToken, http.StatusServiceUnavailable, "auth_unavailable")
}

func deleteComposeEpochLiveAccount(t *testing.T, client *http.Client, base, accessToken, password string) {
	t.Helper()
	payload, err := json.Marshal(map[string]string{"password": password})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/auth/delete-account", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, resp.StatusCode, "delete-account body=%s", string(body))
}

func requireEpochLiveRevokedClose(t *testing.T, conn *websocket.Conn) {
	t.Helper()
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(15*time.Second)))
	_, body, err := conn.ReadMessage()
	if err == nil {
		var frame composeWSFrame
		if json.Unmarshal(body, &frame) == nil && frame.Op == "heartbeat_ack" {
			t.Fatal("stale socket received heartbeat_ack instead of session_revoked close")
		}
		t.Fatalf("stale socket received frame %s instead of session_revoked close", string(body))
	}
	var closeError *websocket.CloseError
	require.ErrorAs(t, err, &closeError)
	require.Equal(t, websocket.ClosePolicyViolation, closeError.Code)
	require.Equal(t, "session_revoked", closeError.Text)
}

func requireEpochLiveUpgradeDenied(t *testing.T, base, accessToken string, wantStatus int, wantError string) {
	t.Helper()
	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+accessToken)
	conn, response, err := websocket.DefaultDialer.Dial(composeWebSocketURL(t, base), headers)
	if response != nil {
		defer response.Body.Close()
	}
	require.Error(t, err)
	require.Nil(t, conn)
	require.NotNil(t, response)
	require.Equal(t, wantStatus, response.StatusCode)
	body, readErr := io.ReadAll(response.Body)
	require.NoError(t, readErr)
	var payload map[string]string
	require.NoError(t, json.Unmarshal(body, &payload), "upgrade denial body=%s", string(body))
	require.Equal(t, wantError, payload["error"])
}

func deleteEpochLiveFixtureFloor(t *testing.T, accountID string) {
	t.Helper()
	if !liveComposeEnabled() {
		t.Fatal("refusing session-epoch floor mutation without VOICE_RUN_LIVE_COMPOSE=true")
	}
	if os.Getenv(epochLiveIsolationEnv) != "true" {
		t.Fatal("refusing session-epoch floor mutation outside the isolated A1 runner")
	}
	project := os.Getenv("COMPOSE_PROJECT_NAME")
	if !epochLiveComposeProject.MatchString(project) {
		t.Fatalf("refusing session-epoch floor mutation for compose project %q", project)
	}
	parsedAccountID, err := uuid.Parse(accountID)
	require.NoError(t, err, "fixture account_id must be a UUID")
	require.Equal(t, parsedAccountID.String(), accountID, "fixture account_id must use canonical UUID spelling")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	containers, err := exec.CommandContext(
		ctx,
		"docker", "ps", "--quiet",
		"--filter", "label=com.docker.compose.project="+project,
		"--filter", "label=com.docker.compose.service=redis",
	).Output()
	require.NoError(t, err, "locate the isolated runner Redis container")
	containerIDs := strings.Fields(string(containers))
	require.Len(t, containerIDs, 1, "the isolated runner must own exactly one running Redis container")

	key := "auth:session:min_epoch:" + accountID
	deleted, err := exec.CommandContext(ctx, "docker", "exec", containerIDs[0], "redis-cli", "DEL", key).CombinedOutput()
	require.NoError(t, err, "delete the exact fixture Auth session-epoch floor: %s", string(deleted))
	require.Equal(t, "1", strings.TrimSpace(string(deleted)), "fixture Auth session-epoch floor must exist before fault injection")
}
