package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestComposePremiumCosmetics_live is SUB-06: after premium webhook (JWT may still be free),
// Gateway live entitlements unlock GIF avatar, banner, and a 3rd profile.
func TestComposePremiumCosmetics_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 90 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()

	sess := registerComposeUser(t, client, base, formatComposeEmail("sub06-cosm", n), "VoiceQaTest1!")
	require.NotEmpty(t, sess.AccountID)
	require.NotEmpty(t, sess.ProfileID)

	// Free tier: GIF rejected before webhook.
	freeGif := composeAvatarPresignStatus(t, client, base, sess.AccessToken, "image/gif", 4096)
	require.Equal(t, http.StatusPreconditionFailed, freeGif, "free GIF must be denied")

	composeActivatePremiumWebhook(t, client, base, sess.AccountID)
	require.Equal(t, "premium", composeGetSubscriptionPlan(t, client, base, sess.AccessToken))

	// Same access token (stale JWT free) — live Subscription resolve must unlock cosmetics.
	gifStatus := composeAvatarPresignStatus(t, client, base, sess.AccessToken, "image/gif", 4096)
	switch gifStatus {
	case http.StatusOK:
		// premium GIF unlocked
	case http.StatusPreconditionFailed:
		pngStatus := composeAvatarPresignStatus(t, client, base, sess.AccessToken, "image/png", 1024)
		if pngStatus == http.StatusPreconditionFailed {
			t.Log("avatar object storage not configured; skipping GIF assert")
		} else {
			t.Fatalf("premium GIF still denied (status=%d) while PNG works (status=%d)", gifStatus, pngStatus)
		}
	default:
		t.Fatalf("unexpected GIF presign status=%d", gifStatus)
	}

	banner := fmt.Sprintf("https://cdn-test.example/banners/%s.png", sess.ProfileID)
	bannerStatus := composePatchProfileBannerStatus(t, client, base, sess.AccessToken, banner)
	require.Equal(t, http.StatusOK, bannerStatus, "premium profile banner")

	_, _ = composeCreateAltProfile(t, client, base, sess.AccessToken, "Cosmetics Alt One", "personal")
	thirdResp := composePostJSON(t, client, base+"/api/v1/users/profiles", sess.AccessToken,
		`{"display_name":"Cosmetics Alt Two"}`)
	require.Equal(t, http.StatusOK, thirdResp.StatusCode, composeReadBody(t, thirdResp))
}

func composeAvatarPresignStatus(t *testing.T, client *http.Client, base, accessToken, contentType string, contentLength int64) int {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"content_type":   contentType,
		"content_length": contentLength,
	})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/users/me/avatar/presigned-upload", strings.NewReader(string(payload)))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	_, _ = io.ReadAll(resp.Body)
	return resp.StatusCode
}

func composePatchProfileBannerStatus(t *testing.T, client *http.Client, base, accessToken, bannerURL string) int {
	t.Helper()
	payload, err := json.Marshal(map[string]any{"banner_url": bannerURL})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPatch, base+"/api/v1/users/me", strings.NewReader(string(payload)))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Logf("patch banner body=%s", string(body))
	}
	return resp.StatusCode
}
