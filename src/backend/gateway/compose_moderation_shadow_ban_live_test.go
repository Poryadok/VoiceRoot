package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestComposeModerationShadowBan_live documents shadow ban hides DM delivery from peers (docs/features/reports.md).
func TestComposeModerationShadowBan_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 90 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()

	target := registerComposeUser(t, client, base, formatComposeEmail("shadow-target", n), "VoiceQaTest1!")
	peer := registerComposeUser(t, client, base, formatComposeEmail("shadow-peer", n), "VoiceQaTest1!")
	chatID := createComposeDMBetween(t, client, base, target, peer)

	staffToken := composeStaffToken(t, client, base)
	if staffToken == "" {
		t.Skip("no staff token available in compose; set GATEWAY_STATIC_TOKENS_JSON staff entry")
	}

	sanctionBody, err := json.Marshal(map[string]any{
		"target_account_id": target.AccountID,
		"type":              "shadow_ban",
		"reason":            "compose shadow ban",
	})
	require.NoError(t, err)
	sanctionReq, err := http.NewRequest(http.MethodPost, base+"/api/v1/admin/moderation/sanctions", bytes.NewReader(sanctionBody))
	require.NoError(t, err)
	sanctionReq.Header.Set("Authorization", "Bearer "+staffToken)
	sanctionReq.Header.Set("Content-Type", "application/json")
	sanctionResp, err := client.Do(sanctionReq)
	require.NoError(t, err)
	defer sanctionResp.Body.Close()
	require.Equal(t, http.StatusOK, sanctionResp.StatusCode)

	marker := "shadow-ban-" + formatComposeEmail("marker", n)
	sendComposeMessage(t, client, base, target.AccessToken, chatID, marker)

	contents := composeMessageContentsInChat(t, client, base, peer.AccessToken, chatID)
	require.NotContains(t, contents, marker, "peer must not see shadow-banned sender content")
}
