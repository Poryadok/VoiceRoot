package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestComposeSearchPrivacyAudience_live documents SR-05: SearchProfiles / SearchUsers
// hides targets whose allow_friend_requests audience excludes the viewer (privacy.md).
//
// Opt-in: VOICE_RUN_LIVE_COMPOSE=true VOICE_API_BASE_URL=http://127.0.0.1:18080
func TestComposeSearchPrivacyAudience_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()

	targetEmail := formatComposeEmail("sr05-target", n)
	viewerEmail := formatComposeEmail("sr05-viewer", n)
	target := registerComposeUser(t, client, base, targetEmail, "VoiceQaTest1!")
	viewer := registerComposeUser(t, client, base, viewerEmail, "VoiceQaTest1!")

	searchToken := strings.TrimSuffix(targetEmail, "@voice-qa.test")
	searchURL := fmt.Sprintf("%s/api/v1/search/users?q=%s", base, url.QueryEscape(searchToken))

	// Wait until the target is indexed and discoverable (default gaming allow_friend_requests = everyone).
	require.Eventually(t, func() bool {
		ids := searchComposeUserProfileIDs(t, client, searchURL, viewer.AccessToken)
		for _, id := range ids {
			if id == target.ProfileID {
				return true
			}
		}
		return false
	}, 45*time.Second, 2*time.Second, "target profile must appear in SearchUsers before privacy lockdown")

	nobody := map[string]any{
		"friends": false, "friends_of_friends": false, "space_members": false, "include_guests": false,
	}
	patchComposePrivacy(t, client, base, target.AccessToken, map[string]any{
		"preset":                "personal",
		"allow_friend_requests": nobody,
	})

	require.Eventually(t, func() bool {
		ids := searchComposeUserProfileIDs(t, client, searchURL, viewer.AccessToken)
		for _, id := range ids {
			if id == target.ProfileID {
				return false
			}
		}
		return true
	}, 20*time.Second, 1*time.Second, "target with allow_friend_requests=nobody must be hidden from stranger SearchUsers")
}

func searchComposeUserProfileIDs(t *testing.T, client *http.Client, searchURL, accessToken string) []string {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, searchURL, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Logf("search users status=%d body=%s", resp.StatusCode, string(body))
		return nil
	}
	var parsed struct {
		UserSearchResults struct {
			ProfileIds []string `json:"profile_ids"`
		} `json:"user_search_results"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Logf("search users unmarshal: %v body=%s", err, string(body))
		return nil
	}
	return parsed.UserSearchResults.ProfileIds
}
