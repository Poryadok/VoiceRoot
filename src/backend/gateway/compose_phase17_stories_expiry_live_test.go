package main

import (
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestComposePhase17StoriesExpiryArchive_live verifies expired stories leave active feed
// and appear in author archive (full purge→DeleteFile needs STORY_TTL_DEV in compose).
func TestComposePhase17StoriesExpiryArchive_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	if os.Getenv("VOICE_STORY_EXPIRY_LIVE_TEST") != "true" {
		t.Skip("set VOICE_STORY_EXPIRY_LIVE_TEST=true with story STORY_TTL_DEV=30s in compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 90 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()

	sess := registerComposeUser(t, client, base, formatComposeEmail("p17-expiry", n), "VoiceQaTest1!")
	story := composeCreateStory(t, client, base, sess.AccessToken, map[string]any{
		"type":         "text",
		"text_content": "expiry live story",
		"visibility":   "everyone",
	})
	storyID := composeStoryID(t, story)
	require.NotEmpty(t, storyID)

	// Wait for expiry worker (1m tick) + STORY_TTL_DEV window.
	deadline := time.Now().Add(2 * time.Minute)
	for time.Now().Before(deadline) {
		feed := composeGetStoryFeed(t, client, base, sess.AccessToken)
		stillActive := false
		for _, id := range feed {
			if id == storyID {
				stillActive = true
				break
			}
		}
		if !stillActive {
			break
		}
		time.Sleep(10 * time.Second)
	}

	archiveIDs := composeGetStoryArchive(t, client, base, sess.AccessToken)
	require.Contains(t, archiveIDs, storyID, "expired story must appear in owner archive")
}

func composeGetStoryArchive(t *testing.T, client *http.Client, base, token string) []string {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, base+"/api/v1/stories/archive", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var parsed struct {
		StoryList struct {
			Stories []struct {
				ID string `json:"id"`
			} `json:"stories"`
		} `json:"story_list"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&parsed))
	ids := make([]string, len(parsed.StoryList.Stories))
	for i, s := range parsed.StoryList.Stories {
		ids[i] = s.ID
	}
	return ids
}
