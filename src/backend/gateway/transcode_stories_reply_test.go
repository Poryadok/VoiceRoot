package main

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestTranscodeStories_Reply documents POST /api/v1/stories/{id}/reply → private DM reply per stories.md.
func TestTranscodeStories_Reply(t *testing.T) {
	t.Parallel()
	rec := &recordingStoryGRPC{}
	h := newStoriesContractGateway(t, rec)

	body := `{"text":"private reply"}`
	resp := performRequest(h, http.MethodPost, "/api/v1/stories/story-42/reply", body, map[string]string{
		"Authorization": "Bearer valid-user-token",
		"Content-Type":  "application/json",
	})
	require.Equal(t, http.StatusOK, resp.Code,
		"gateway must expose POST /api/v1/stories/{id}/reply for private story replies")
}
