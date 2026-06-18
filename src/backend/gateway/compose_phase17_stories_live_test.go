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
// story create (with @mention) → feed → view → react → reply→DM → highlight privacy → report flows over Gateway REST.
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
	sessC := registerComposeUser(t, client, base, formatComposeEmail("p17-story-c", n), "VoiceQaTest1!")
	sendComposeFriendInvitation(t, client, base, sessA.AccessToken, sessB.ProfileID)
	acceptComposeFriendInvitation(t, client, base, sessB.AccessToken, sessA.ProfileID)

	story := composeCreateStory(t, client, base, sessA.AccessToken, map[string]any{
		"type":                 "text",
		"text_content":         "Phase 17 live story @friend",
		"visibility":           "friends",
		"mention_profile_ids":  []string{sessB.ProfileID},
	})
	storyID := composeStoryID(t, story)
	require.NotEmpty(t, storyID)
	require.Contains(t, composeStoryMentionIDs(t, story), sessB.ProfileID,
		"create story must persist mention_profile_ids for @username notifications")

	feed := composeGetStoryFeed(t, client, base, sessB.AccessToken)
	require.Contains(t, feed, storyID, "friend must see active story in feed")

	composeMarkStoryViewed(t, client, base, sessB.AccessToken, storyID)
	composeReactToStory(t, client, base, sessB.AccessToken, storyID, "🔥")

	reply := composeReplyToStory(t, client, base, sessB.AccessToken, storyID, "private story reply")
	require.NotEmpty(t, reply.ChatID)
	require.NotEmpty(t, reply.MessageID)
	getComposeMessagesContains(t, client, base, sessA.AccessToken, reply.ChatID, reply.MessageID, "private story reply")

	highlightID := composeCreateHighlight(t, client, base, sessA.AccessToken, "Live wins", "friends")
	require.NotEmpty(t, highlightID)
	composeAddToHighlight(t, client, base, sessA.AccessToken, highlightID, storyID)

	friendHighlights := composeGetHighlights(t, client, base, sessB.AccessToken, sessA.ProfileID)
	require.Contains(t, friendHighlights, highlightID, "friend must see friends-only highlight")

	strangerHighlights := composeGetHighlights(t, client, base, sessC.AccessToken, sessA.ProfileID)
	require.NotContains(t, strangerHighlights, highlightID,
		"stranger must not see friends-only highlight (independent highlight privacy)")

	reportID := composeReportStory(t, client, base, sessB.AccessToken, storyID, "offensive")
	require.NotEmpty(t, reportID)
}

type composeStoryReply struct {
	ChatID    string
	MessageID string
}

func composeCreateStory(t *testing.T, client *http.Client, base, token string, payload map[string]any) map[string]any {
	t.Helper()
	body, err := json.Marshal(payload)
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/stories", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "create story body=%s", string(raw))
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(raw, &parsed))
	story, _ := parsed["story"].(map[string]any)
	require.NotNil(t, story)
	return story
}

func composeStoryID(t *testing.T, story map[string]any) string {
	t.Helper()
	id, _ := story["id"].(string)
	return id
}

func composeStoryMentionIDs(t *testing.T, story map[string]any) []string {
	t.Helper()
	if raw, ok := story["mention_profile_ids"].([]any); ok {
		out := make([]string, 0, len(raw))
		for _, v := range raw {
			if s, ok := v.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	for _, key := range []string{"mention_profile_ids_json", "mentionProfileIdsJson"} {
		if raw, ok := story[key].(string); ok && raw != "" {
			var ids []string
			require.NoError(t, json.Unmarshal([]byte(raw), &ids))
			return ids
		}
	}
	return nil
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

func composeReplyToStory(t *testing.T, client *http.Client, base, token, storyID, text string) composeStoryReply {
	t.Helper()
	payload, _ := json.Marshal(map[string]string{"text": text})
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/stories/"+storyID+"/reply", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "reply story body=%s", string(body))
	var parsed struct {
		ChatID    string `json:"chat_id"`
		MessageID string `json:"message_id"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	if parsed.ChatID == "" || parsed.MessageID == "" {
		var alt map[string]any
		require.NoError(t, json.Unmarshal(body, &alt))
		if parsed.ChatID == "" {
			if v, ok := alt["chatId"].(string); ok {
				parsed.ChatID = v
			}
		}
		if parsed.MessageID == "" {
			if v, ok := alt["messageId"].(string); ok {
				parsed.MessageID = v
			}
		}
	}
	require.NotEmpty(t, parsed.ChatID)
	require.NotEmpty(t, parsed.MessageID)
	return composeStoryReply{ChatID: parsed.ChatID, MessageID: parsed.MessageID}
}

func composeCreateHighlight(t *testing.T, client *http.Client, base, token, name, visibility string) string {
	t.Helper()
	payload, _ := json.Marshal(map[string]string{"name": name, "visibility": visibility})
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

func composeAddToHighlight(t *testing.T, client *http.Client, base, token, highlightID, storyID string) {
	t.Helper()
	payload, _ := json.Marshal(map[string]string{"story_id": storyID})
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/stories/highlights/"+highlightID+"/stories", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
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
