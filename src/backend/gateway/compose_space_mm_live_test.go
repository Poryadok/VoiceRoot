package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestComposeSpaceMatchmakingQueue_live documents MM-08:
// space-scoped StartSpaceQueue sets space_id; non-members denied; space queue
// matches stay isolated from a concurrent global searcher.
func TestComposeSpaceMatchmakingQueue_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 60 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()

	owner := registerComposeUser(t, client, base, formatComposeEmail("mm08-owner", n), "VoiceQaTest1!")
	member := registerComposeUser(t, client, base, formatComposeEmail("mm08-member", n), "VoiceQaTest1!")
	outsider := registerComposeUser(t, client, base, formatComposeEmail("mm08-out", n), "VoiceQaTest1!")

	spaceID := createComposeSpace(t, client, base, owner.AccessToken, "Space MM QA", "mm-08")
	invite := createComposeSpaceInvite(t, client, base, owner.AccessToken, spaceID)
	joinComposeSpaceByInvite(t, client, base, member.AccessToken, invite.Code)

	patchComposeSpaceMmConfig(t, client, base, owner.AccessToken, spaceID, `{"enabled":true}`)

	gameID := findComposeMatchmakingGameID(t, client, base, owner.AccessToken, "MM Duo Live")
	criteria := `{"region":"eu"}`

	sessionOwner := startComposeSpaceMatchmakingQueue(t, client, base, owner.AccessToken, spaceID, gameID, "Duo", criteria)
	require.Equal(t, "searching", sessionOwner["status"])
	require.Equal(t, spaceID, stringFromAny(sessionOwner["spaceId"], sessionOwner["space_id"]))

	sessionMember := startComposeSpaceMatchmakingQueue(t, client, base, member.AccessToken, spaceID, gameID, "Duo", criteria)
	require.Equal(t, spaceID, stringFromAny(sessionMember["spaceId"], sessionMember["space_id"]))

	status, raw := startComposeSpaceMatchmakingQueueStatus(t, client, base, outsider.AccessToken, spaceID, gameID, "Duo", criteria)
	require.Equal(t, http.StatusForbidden, status, "non-member must be denied; body=%s", string(raw))

	globalSession := startComposeMatchmakingSearch(t, client, base, outsider.AccessToken, gameID, "Duo", criteria)
	require.NotEmpty(t, globalSession)

	deadline := time.Now().Add(35 * time.Second)
	var matchID string
	ownerSessionID := stringFromAny(sessionOwner["id"])
	for time.Now().Before(deadline) {
		st := getComposeMatchmakingSearch(t, client, base, owner.AccessToken, ownerSessionID)
		if id := stringFromAny(st["matchId"], st["match_id"]); id != "" {
			matchID = id
			break
		}
		time.Sleep(2 * time.Second)
	}
	require.NotEmpty(t, matchID, "space queue members must match each other")

	match := getComposeMatch(t, client, base, owner.AccessToken, matchID)
	parties, _ := match["parties"].([]any)
	if parties == nil {
		parties, _ = match["Parties"].([]any)
	}
	require.NotEmpty(t, parties)
	for _, p := range parties {
		pm, _ := p.(map[string]any)
		require.NotNil(t, pm)
		members, _ := pm["memberProfileIds"].([]any)
		if members == nil {
			members, _ = pm["member_profile_ids"].([]any)
		}
		for _, m := range members {
			require.NotEqual(t, outsider.ProfileID, m, "global outsider must not join space-scoped match")
		}
	}

	globalStatus := getComposeMatchmakingSearch(t, client, base, outsider.AccessToken, globalSession)
	require.Equal(t, "searching", globalStatus["status"], "global searcher remains searching outside space queue")
}

func patchComposeSpaceMmConfig(t *testing.T, client *http.Client, base, accessToken, spaceID, mmConfigJSON string) {
	t.Helper()
	payload, err := json.Marshal(map[string]string{"mm_config_json": mmConfigJSON})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPatch, base+"/api/v1/spaces/"+spaceID+"/matchmaking/config", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "PATCH space mm config body=%s", string(raw))
}

func startComposeSpaceMatchmakingQueue(
	t *testing.T, client *http.Client, base, token, spaceID, gameID, mode, criteriaJSON string,
) map[string]any {
	t.Helper()
	status, raw := startComposeSpaceMatchmakingQueueStatus(t, client, base, token, spaceID, gameID, mode, criteriaJSON)
	require.Equal(t, http.StatusOK, status, "body=%s", string(raw))
	var payload map[string]any
	require.NoError(t, json.Unmarshal(raw, &payload))
	session, ok := payload["searchSession"].(map[string]any)
	if !ok {
		session, ok = payload["search_session"].(map[string]any)
	}
	require.True(t, ok, "expected searchSession: %s", string(raw))
	return session
}

func startComposeSpaceMatchmakingQueueStatus(
	t *testing.T, client *http.Client, base, token, spaceID, gameID, mode, criteriaJSON string,
) (int, []byte) {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"gameId":       gameID,
		"mode":         mode,
		"criteriaJson": criteriaJSON,
	})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/spaces/"+spaceID+"/matchmaking/queue", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, raw
}

func stringFromAny(vals ...any) string {
	for _, v := range vals {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return ""
}
