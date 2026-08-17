package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func liveVerificationStubURL() string {
	if v := strings.TrimSpace(os.Getenv("VOICE_VERIFICATION_STUB_URL")); v != "" {
		return strings.TrimRight(v, "/")
	}
	return "http://127.0.0.1:14180"
}

// TestComposeVerificationOAuthStart_live is VR-02: OAuth start + callback through
// compose Helix/YPP stubs grants personal verification badges (verification.md).
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

	twitchCB := composeLinkedAccountOAuthCallback(t, client, base, sess.AccessToken, "twitch")
	require.Equal(t, "personal", stringFromAny(twitchCB["verification_type"], twitchCB["verificationType"]))
	require.Equal(t, "twitch", stringFromAny(twitchCB["badge"]))

	status := composeGetVerificationStatus(t, client, base, sess.AccessToken)
	require.Equal(t, "personal", stringFromAny(status["verification_type"], status["verificationType"]))
	require.Equal(t, "twitch", stringFromAny(status["badge"]))

	youtubeURL := composeLinkedAccountOAuthStart(t, client, base, sess.AccessToken, "youtube")
	require.Contains(t, youtubeURL, "google", "YouTube authorize URL must be returned")

	ytCB := composeLinkedAccountOAuthCallback(t, client, base, sess.AccessToken, "youtube")
	require.Equal(t, "personal", stringFromAny(ytCB["verification_type"], ytCB["verificationType"]))
	require.Equal(t, "youtube", stringFromAny(ytCB["badge"]))

	linked := composeListLinkedAccounts(t, client, base, sess.AccessToken)
	platforms := make([]string, 0, len(linked))
	for _, row := range linked {
		platforms = append(platforms, stringFromAny(row["platform"]))
	}
	require.Contains(t, platforms, "twitch")
	require.Contains(t, platforms, "youtube")
}

// TestComposeOrganizationDNSVerification_live is VR-03: StartOrganizationVerification
// TXT + HTTP DNS fixture grant organization/dns badge.
func TestComposeOrganizationDNSVerification_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()
	sess := registerComposeUser(t, client, base, formatComposeEmail("vr03-dns", n), "VoiceQaTest1!")
	domain := fmt.Sprintf("vr03-%d.example.test", n)

	payload, err := json.Marshal(map[string]string{
		"profile_id": sess.ProfileID,
		"domain":     domain,
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
	require.Equal(t, domain, started.Domain)
	require.Contains(t, started.TxtRecord, "voice-verify=")

	unchecked := composeCheckOrganizationVerification(t, client, base, sess.AccessToken, sess.ProfileID)
	require.NotEqual(t, "organization", stringFromAny(unchecked["verification_type"], unchecked["verificationType"]),
		"DNS TXT not published yet — badge must remain ungranted")

	composePublishDNSTxt(t, client, domain, started.TxtRecord)

	granted := composeCheckOrganizationVerification(t, client, base, sess.AccessToken, sess.ProfileID)
	require.Equal(t, "organization", stringFromAny(granted["verification_type"], granted["verificationType"]))
	require.Equal(t, "dns", stringFromAny(granted["badge"]))

	status := composeGetVerificationStatus(t, client, base, sess.AccessToken)
	require.Equal(t, "organization", stringFromAny(status["verification_type"], status["verificationType"]))
	require.Equal(t, "dns", stringFromAny(status["badge"]))
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

func composeLinkedAccountOAuthCallback(t *testing.T, client *http.Client, base, token, provider string) map[string]any {
	t.Helper()
	payload, err := json.Marshal(map[string]string{
		"code":         "compose-code",
		"redirect_uri": "https://app.voice.test/oauth/" + provider,
	})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/auth/linked-accounts/"+provider+"/callback", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "oauth callback %s body=%s", provider, string(raw))
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(raw, &parsed))
	return parsed
}

func composeListLinkedAccounts(t *testing.T, client *http.Client, base, token string) []map[string]any {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, base+"/api/v1/auth/linked-accounts", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "list linked accounts body=%s", string(raw))
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(raw, &parsed))
	rows, _ := parsed["linked_accounts"].([]any)
	if rows == nil {
		rows, _ = parsed["linkedAccounts"].([]any)
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		if m, ok := row.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func composeGetVerificationStatus(t *testing.T, client *http.Client, base, token string) map[string]any {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, base+"/api/v1/users/me/verification", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "get verification body=%s", string(raw))
	return composeVerificationStatusMap(t, raw)
}

func composeCheckOrganizationVerification(t *testing.T, client *http.Client, base, token, profileID string) map[string]any {
	t.Helper()
	checkReq, err := http.NewRequest(http.MethodPost, base+"/api/v1/users/me/verification/organization/check",
		bytes.NewReader([]byte(`{"profile_id":"`+profileID+`"}`)))
	require.NoError(t, err)
	checkReq.Header.Set("Authorization", "Bearer "+token)
	checkReq.Header.Set("Content-Type", "application/json")
	checkResp, err := client.Do(checkReq)
	require.NoError(t, err)
	defer checkResp.Body.Close()
	checkRaw, _ := io.ReadAll(checkResp.Body)
	require.Equal(t, http.StatusOK, checkResp.StatusCode, "check org verification body=%s", string(checkRaw))
	return composeVerificationStatusMap(t, checkRaw)
}

func composeVerificationStatusMap(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var checked map[string]any
	require.NoError(t, json.Unmarshal(raw, &checked))
	status, _ := checked["verification_status"].(map[string]any)
	if status == nil {
		status, _ = checked["verificationStatus"].(map[string]any)
	}
	if status == nil {
		return checked
	}
	return status
}

func composePublishDNSTxt(t *testing.T, client *http.Client, domain, txt string) {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"domain": domain,
		"txt":    txt,
	})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPut, liveVerificationStubURL()+"/dns-txt", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "publish DNS TXT fixture body=%s", string(raw))
}
