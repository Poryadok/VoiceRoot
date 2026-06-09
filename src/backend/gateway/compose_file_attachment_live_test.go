package main

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestComposeFileAttachment_live: upload file, send attachment message, peer receives via WS.
func TestComposeFileAttachment_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 60 * time.Second}
	base := liveGatewayBaseURL()

	n := time.Now().UnixNano()
	sessA := registerComposeUser(t, client, base, formatComposeEmail("file-a", n), "VoiceQaTest1!")
	sessB := registerComposeUser(t, client, base, formatComposeEmail("file-b", n), "VoiceQaTest1!")
	if !composeFileUploadAvailable(t, client, base, sessA.AccessToken) {
		t.Skip("object storage not configured (MinIO/R2); set FILE_R2_* in .env for compose app profile")
	}

	chatID := createComposeDM(t, client, base, sessA.AccessToken, sessB.ProfileID)
	fileID, fileType := composeUploadSmallTextFile(t, client, base, sessA.AccessToken, chatID)

	wsB := connectComposeWSSubscribed(t, base, sessB.AccessToken, chatID)
	attachments, err := json.Marshal([]map[string]string{
		{"file_id": fileID, "type": fileType},
	})
	require.NoError(t, err)

	msgCh := make(chan composeWSFrame, 1)
	go func() {
		msgCh <- waitComposeWSOp(t, wsB, "message_create", 25*time.Second, func(d map[string]any) bool {
			return d["chat_id"] == chatID
		})
	}()
	msgID := sendComposeMessageWithAttachmentsJSON(t, client, base, sessA.AccessToken, chatID, string(attachments))

	select {
	case frame := <-msgCh:
		var d map[string]any
		require.NoError(t, json.Unmarshal(frame.D, &d))
		require.Equal(t, msgID, d["message_id"])
	case <-time.After(30 * time.Second):
		t.Fatal("timeout waiting for attachment message_create")
	}
}
