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

// TestComposeVerificationOAuthStart_live documents VR-02 (partial):
// Twitch/YouTube OAuth link start returns authorization_url. Full Partner/YPP
// badge grant needs real provider OAuth fixtures — not available in compose.
func TestComposeVerificationOAuthStart_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()
	sess := registerComposeUser(t, client, base, formatComposeEmail("vr02-oauth", n), "VoiceQaTest1!")

	twitchURL := composeLinkedAccountOAuthStart(t, client, base, sess.AccessToken, "twitch")
	require.Contains(t, twitchURL, "twitch", "Twitch authorize URL must be returned")

	youtubeURL := composeLinkedAccountOAuthStart(t, client, base, sess.AccessToken, "youtube")
	require.Contains(t, youtubeURL, "google", "YouTube authorize URL must be returned")
}

// TestComposeOrganizationDNSVerification_live documents VR-03 (partial):
// StartOrganizationVerification returns TXT; Check without DNS stub stays unverified.
func TestComposeOrganizationDNSVerification_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()
	sess := registerComposeUser(t, client, base, formatComposeEmail("vr03-dns", n), "VoiceQaTest1!")

	payload, err := json.Marshal(map[string]string{
		"profile_id": sess.ProfileID,
		"domain":     "example.com",
	})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/users/me/verification/organization", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+sess.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "start org verification body=%s", string(raw))

	var started struct {
		Domain    string `json:"domain"`
		TxtRecord string `json:"txt_record"`
	}
	require.NoError(t, json.Unmarshal(raw, &started))
	require.Equal(t, "example.com", started.Domain)
	require.Contains(t, started.TxtRecord, "voice-verify=")

	checkReq, err := http.NewRequest(http.MethodPost, base+"/api/v1/users/me/verification/organization/check",
		bytes.NewReader([]byte(`{"profile_id":"`+sess.ProfileID+`"}`)))
	require.NoError(t, err)
	checkReq.Header.Set("Authorization", "Bearer "+sess.AccessToken)
	checkReq.Header.Set("Content-Type", "application/json")
	checkResp, err := client.Do(checkReq)
	require.NoError(t, err)
	defer checkResp.Body.Close()
	checkRaw, _ := io.ReadAll(checkResp.Body)
	// Without a DNS stub publishing the TXT, badge grant cannot complete in compose.
	require.True(t, checkResp.StatusCode == http.StatusOK || checkResp.StatusCode == http.StatusServiceUnavailable,
		"check org verification body=%s", string(checkRaw))
	if checkResp.StatusCode == http.StatusOK {
		var checked map[string]any
		require.NoError(t, json.Unmarshal(checkRaw, &checked))
		status, _ := checked["verification_status"].(map[string]any)
		if status == nil {
			status, _ = checked["verificationStatus"].(map[string]any)
		}
		if status != nil {
			vtype := stringFromAny(status["verification_type"], status["verificationType"])
			require.NotEqual(t, "organization", vtype,
				"DNS TXT not published in compose — badge must remain ungranted (partial VR-03)")
		}
	}
}

func composeLinkedAccountOAuthStart(t *testing.T, client *http.Client, base, token, provider string) string {
	t.Helper()
	payload, err := json.Marshal(map[string]string{
		"redirect_uri": "https://app.voice.test/oauth/" + provider,
	})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/auth/linked-accounts/"+provider+"/link", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "oauth start %s body=%s", provider, string(raw))
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(raw, &parsed))
	url := stringFromAny(parsed["authorization_url"], parsed["authorizationUrl"])
	require.NotEmpty(t, url)
	return url
}
