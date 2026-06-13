package main

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestComposeSharedMedia_live: upload file, send attachment, list shared media.
func TestComposeSharedMedia_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 60 * time.Second}
	base := liveGatewayBaseURL()

	n := time.Now().UnixNano()
	sessA := registerComposeUser(t, client, base, formatComposeEmail("sm-a", n), "VoiceQaTest1!")
	sessB := registerComposeUser(t, client, base, formatComposeEmail("sm-b", n), "VoiceQaTest1!")
	if !composeFileUploadAvailable(t, client, base, sessA.AccessToken) {
		t.Skip("object storage not configured (MinIO/R2); set FILE_R2_* in .env for compose app profile")
	}

	chatID := createComposeDM(t, client, base, sessA.AccessToken, sessB.ProfileID)
	fileID, fileType := composeUploadSmallTextFile(t, client, base, sessA.AccessToken, chatID)

	attachments, err := json.Marshal([]map[string]string{
		{"file_id": fileID, "type": fileType},
	})
	require.NoError(t, err)
	sendComposeMessageWithAttachmentsJSON(t, client, base, sessA.AccessToken, chatID, string(attachments))

	listReq, err := http.NewRequest(http.MethodGet, base+"/api/v1/chats/"+chatID+"/shared-media?kind=files", nil)
	require.NoError(t, err)
	listReq.Header.Set("Authorization", "Bearer "+sessA.AccessToken)
	listResp, err := client.Do(listReq)
	require.NoError(t, err)
	defer listResp.Body.Close()
	require.Equal(t, http.StatusOK, listResp.StatusCode)

	var body map[string]any
	require.NoError(t, json.NewDecoder(listResp.Body).Decode(&body))
	list := body["shared_media_list"].(map[string]any)
	items := list["items"].([]any)
	require.NotEmpty(t, items)

	outsider := registerComposeUser(t, client, base, formatComposeEmail("sm-out", n), "VoiceQaTest1!")
	denyReq, err := http.NewRequest(http.MethodGet, base+"/api/v1/chats/"+chatID+"/shared-media?kind=files", nil)
	require.NoError(t, err)
	denyReq.Header.Set("Authorization", "Bearer "+outsider.AccessToken)
	denyResp, err := client.Do(denyReq)
	require.NoError(t, err)
	defer denyResp.Body.Close()
	require.Equal(t, http.StatusForbidden, denyResp.StatusCode)
}
