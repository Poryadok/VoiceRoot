package main

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestComposePhase15E2EEdit_live documents gateway edit-in-E2E: ciphertext update, search exclusion.
func TestComposePhase15E2EEdit_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()

	n := time.Now().UnixNano()
	sessA := registerComposeUser(t, client, base, formatComposeEmail("p15-edit-a", n), "VoiceQaTest1!")
	sessB := registerComposeUser(t, client, base, formatComposeEmail("p15-edit-b", n), "VoiceQaTest1!")
	chatID := createComposeDMBetween(t, client, base, sessA, sessB)

	uploadComposePreKeyBundle(t, client, base, sessA.AccessToken, validComposePreKeyBundleB64())
	uploadComposePreKeyBundle(t, client, base, sessB.AccessToken, validComposePeerPreKeyBundleB64())
	enableComposeChatE2E(t, client, base, sessA.AccessToken, chatID)
	enableComposeChatE2E(t, client, base, sessB.AccessToken, chatID)

	secretV1 := fmt.Sprintf("phase15-edit-v1-%d", n)
	ciphertextV1 := opaqueE2ECiphertext(secretV1)
	msgID := sendComposeE2EMessage(t, client, base, sessA.AccessToken, chatID, ciphertextV1)
	require.NotEqual(t, secretV1, getComposeMessageContent(t, client, base, sessA.AccessToken, chatID, msgID))

	secretV2 := fmt.Sprintf("phase15-edit-v2-%d", n)
	ciphertextV2 := opaqueE2ECiphertext(secretV2)
	editComposeMessage(t, client, base, sessA.AccessToken, msgID, ciphertextV2)
	require.NotEqual(t, secretV2, getComposeMessageContent(t, client, base, sessA.AccessToken, chatID, msgID))

	globalURL := fmt.Sprintf("%s/api/v1/search/global?q=%s", base, url.QueryEscape(secretV2))
	requireComposeSearchExcludesToken(t, client, sessA.AccessToken, globalURL, secretV2)

	inChatURL := fmt.Sprintf("%s/api/v1/search/in-chat?chat_id=%s&q=%s",
		base, url.QueryEscape(chatID), url.QueryEscape(secretV2))
	requireComposeSearchExcludesToken(t, client, sessA.AccessToken, inChatURL, secretV2)
}

func opaqueE2ECiphertext(secret string) string {
	return "opaque-e2e-ciphertext-" + secret
}
