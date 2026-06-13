package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestComposePhase11Trust_live documents PLAN Phase 11 acceptance: reports 202, 2FA gate, friends-only DM block.
func TestComposePhase11Trust_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 60 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()

	stranger := registerComposeUser(t, client, base, formatComposeEmail("p11-stranger", n), "VoiceQaTest1!")
	target := registerComposeUser(t, client, base, formatComposeEmail("p11-target", n), "VoiceQaTest1!")

	// Privacy: friends-only DM on target profile.
	privacyBody, err := json.Marshal(map[string]any{
		"settings": map[string]any{
			"preset":                "personal",
			"allow_dm":              "friends",
			"show_online":           "friends",
			"show_game_status":      "friends",
			"show_mm_rating":        "friends",
			"show_phone":            "nobody",
			"show_stories":          "friends",
			"allow_friend_requests": "everyone",
			"allow_guest_dm":        false,
		},
	})
	require.NoError(t, err)
	privReq, err := http.NewRequest(http.MethodPatch, base+"/api/v1/users/me/privacy", bytes.NewReader(privacyBody))
	require.NoError(t, err)
	privReq.Header.Set("Authorization", "Bearer "+target.AccessToken)
	privReq.Header.Set("Content-Type", "application/json")
	privResp, err := client.Do(privReq)
	require.NoError(t, err)
	defer privResp.Body.Close()
	require.Equal(t, http.StatusOK, privResp.StatusCode)

	// Stranger cannot open DM when target allows friends only.
	dmPayload, err := json.Marshal(map[string]string{"other_profile_id": target.ProfileID})
	require.NoError(t, err)
	dmReq, err := http.NewRequest(http.MethodPost, base+"/api/v1/chats/dm", bytes.NewReader(dmPayload))
	require.NoError(t, err)
	dmReq.Header.Set("Authorization", "Bearer "+stranger.AccessToken)
	dmReq.Header.Set("Content-Type", "application/json")
	dmResp, err := client.Do(dmReq)
	require.NoError(t, err)
	defer dmResp.Body.Close()
	require.Equal(t, http.StatusForbidden, dmResp.StatusCode)

	// Report user → 202 Accepted (no status updates to reporter).
	reportBody, err := json.Marshal(map[string]any{
		"target_type":   "user",
		"target_id":     target.ProfileID,
		"category":      "mm_toxic",
		"evidence_json": "{}",
	})
	require.NoError(t, err)
	reportReq, err := http.NewRequest(http.MethodPost, base+"/api/v1/moderation/reports", bytes.NewReader(reportBody))
	require.NoError(t, err)
	reportReq.Header.Set("Authorization", "Bearer "+stranger.AccessToken)
	reportReq.Header.Set("Content-Type", "application/json")
	reportResp, err := client.Do(reportReq)
	require.NoError(t, err)
	defer reportResp.Body.Close()
	require.Equal(t, http.StatusAccepted, reportResp.StatusCode)

	// 2FA: enroll returns backup codes; login requires TOTP when enabled.
	enrollReq, err := http.NewRequest(http.MethodPost, base+"/api/v1/auth/2fa/enable", bytes.NewReader([]byte(`{"password":"VoiceQaTest1!"}`)))
	require.NoError(t, err)
	enrollReq.Header.Set("Authorization", "Bearer "+target.AccessToken)
	enrollReq.Header.Set("Content-Type", "application/json")
	enrollResp, err := client.Do(enrollReq)
	require.NoError(t, err)
	defer enrollResp.Body.Close()
	require.Equal(t, http.StatusOK, enrollResp.StatusCode)

	var enrollParsed struct {
		TotpURI     string   `json:"totp_uri"`
		BackupCodes []string `json:"backup_codes"`
	}
	require.NoError(t, json.NewDecoder(enrollResp.Body).Decode(&enrollParsed))
	require.NotEmpty(t, enrollParsed.TotpURI)
	require.NotEmpty(t, enrollParsed.BackupCodes)

	verifyReq, err := http.NewRequest(http.MethodPost, base+"/api/v1/auth/2fa/verify", bytes.NewReader([]byte(`{"totp_code":"000000"}`)))
	require.NoError(t, err)
	verifyReq.Header.Set("Authorization", "Bearer "+target.AccessToken)
	verifyReq.Header.Set("Content-Type", "application/json")
	verifyResp, err := client.Do(verifyReq)
	require.NoError(t, err)
	defer verifyResp.Body.Close()
	require.Equal(t, http.StatusOK, verifyResp.StatusCode)

	loginPayload, err := json.Marshal(map[string]string{
		"email":            formatComposeEmail("p11-target", n),
		"password":         "VoiceQaTest1!",
		"device_info_json": `{"platform":"go-live-test"}`,
	})
	require.NoError(t, err)
	loginNoTotp, err := http.NewRequest(http.MethodPost, base+"/api/v1/auth/login", bytes.NewReader(loginPayload))
	require.NoError(t, err)
	loginNoTotp.Header.Set("Content-Type", "application/json")
	loginResp, err := client.Do(loginNoTotp)
	require.NoError(t, err)
	defer loginResp.Body.Close()
	require.Equal(t, http.StatusUnauthorized, loginResp.StatusCode)
}
