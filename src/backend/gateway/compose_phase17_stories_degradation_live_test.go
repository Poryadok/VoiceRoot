package main

import (
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestComposePhase17StoriesWhenSocialDown_live ensures story create/feed degrade without Social.
func TestComposePhase17StoriesWhenSocialDown_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	if os.Getenv("VOICE_STORY_DEGRADATION_TEST") != "true" {
		t.Skip("set VOICE_STORY_DEGRADATION_TEST=true with social container stopped")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 30 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()

	sess := registerComposeUser(t, client, base, formatComposeEmail("p17-deg-story", n), "VoiceQaTest1!")
	story := composeCreateStory(t, client, base, sess.AccessToken, map[string]any{
		"type":         "text",
		"text_content": "degradation story",
		"visibility":   "everyone",
	})
	storyID := composeStoryID(t, story)
	require.NotEmpty(t, storyID)

	feed := composeGetStoryFeed(t, client, base, sess.AccessToken)
	require.NotNil(t, feed, "feed must respond when Social is down")
}
