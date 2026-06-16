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

// TestComposePhase17Stories_live registers two users, friends them, then exercises
// story create → feed → view → react → highlight → report flows over Gateway REST.
func TestComposePhase17Stories_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 60 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()

	sessA := registerComposeUser(t, client, base, formatComposeEmail("p17-story-a", n), "VoiceQaTest1!")
	sessB := registerComposeUser(t, client, base, formatComposeEmail("p17-story-b", n), "VoiceQaTest1!")
	sendComposeFriendInvitation(t, client, base, sessA.AccessToken, sessB.ProfileID)
	acceptComposeFriendInvitation(t, client, base, sessB.AccessToken, sessA.ProfileID)

	storyID := composeCreateTextStory(t, client, base, sessA.AccessToken, "Phase 17 live story")
	require.NotEmpty(t, storyID)

	feed := composeGetStoryFeed(t, client, base, sessB.AccessToken)
	require.Contains(t, feed, storyID, "friend must see active story in feed")

	composeMarkStoryViewed(t, client, base, sessB.AccessToken, storyID)
	composeReactToStory(t, client, base, sessB.AccessToken, storyID, "🔥")

	highlightID := composeCreateHighlight(t, client, base, sessA.AccessToken, "Live wins")
	require.NotEmpty(t, highlightID)

	highlights := composeGetHighlights(t, client, base, sessB.AccessToken, sessA.ProfileID)
	require.NotEmpty(t, highlights)

	reportID := composeReportStory(t, client, base, sessB.AccessToken, storyID, "offensive")
	require.NotEmpty(t, reportID)
}

func composeCreateTextStory(t *testing.T, client *http.Client, base, token, text string) string {
	t.Helper()
	payload, _ := json.Marshal(map[string]any{
		"type":          "text",
		"text_content":  text,
		"visibility":    "friends",
	})
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/stories", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "create story body=%s", string(body))
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(body, &parsed))
	story, _ := parsed["story"].(map[string]any)
	require.NotNil(t, story)
	id, _ := story["id"].(string)
	return id
}

func composeGetStoryFeed(t *testing.T, client *http.Client, base, token string) []string {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, base+"/api/v1/stories/feed", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "feed body=%s", string(body))
	var parsed struct {
		Stories []struct {
			ID string `json:"id"`
		} `json:"stories"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	ids := make([]string, len(parsed.Stories))
	for i, s := range parsed.Stories {
		ids[i] = s.ID
	}
	return ids
}

func composeMarkStoryViewed(t *testing.T, client *http.Client, base, token, storyID string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/stories/"+storyID+"/views", bytes.NewReader([]byte(`{}`)))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func composeReactToStory(t *testing.T, client *http.Client, base, token, storyID, emoji string) {
	t.Helper()
	payload, _ := json.Marshal(map[string]string{"emoji": emoji})
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/stories/"+storyID+"/reactions", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func composeCreateHighlight(t *testing.T, client *http.Client, base, token, name string) string {
	t.Helper()
	payload, _ := json.Marshal(map[string]string{"name": name})
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/stories/highlights", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "create highlight body=%s", string(body))
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(body, &parsed))
	hl, _ := parsed["highlight"].(map[string]any)
	require.NotNil(t, hl)
	id, _ := hl["id"].(string)
	return id
}

func composeGetHighlights(t *testing.T, client *http.Client, base, token, profileID string) []string {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, base+"/api/v1/stories/highlights?profile_id="+profileID, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "highlights body=%s", string(body))
	var parsed struct {
		HighlightList struct {
			Highlights []struct {
				ID string `json:"id"`
			} `json:"highlights"`
		} `json:"highlight_list"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	ids := make([]string, len(parsed.HighlightList.Highlights))
	for i, h := range parsed.HighlightList.Highlights {
		ids[i] = h.ID
	}
	return ids
}

func composeReportStory(t *testing.T, client *http.Client, base, token, storyID, category string) string {
	t.Helper()
	payload, _ := json.Marshal(map[string]string{
		"target_type":   "story",
		"target_id":     storyID,
		"category":      category,
		"evidence_json": `{}`,
	})
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/moderation/reports", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusAccepted, resp.StatusCode, "report story body=%s", string(body))
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(body, &parsed))
	report, _ := parsed["report"].(map[string]any)
	require.NotNil(t, report)
	id, _ := report["id"].(string)
	return id
}
