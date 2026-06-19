package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestComposePhase11PhoneSync_live documents Phase 11 SyncPhoneContacts with Auth phone-hash lookup and allow_phone_search filter.
func TestComposePhase11PhoneSync_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 90 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()

	everyone := map[string]any{
		"friends": true, "friends_of_friends": true, "space_members": true, "include_guests": true,
	}
	friendsOnly := map[string]any{"friends": true}

	phoneHash := formatComposePhoneHash("p11-phone", n)
	target := registerComposeUserWithPhone(t, client, base, formatComposeEmail("p11-phone-target", n), phoneHash, "VoiceQaTest1!")
	stranger := registerComposeUser(t, client, base, formatComposeEmail("p11-phone-stranger", n), "VoiceQaTest1!")
	friend := registerComposeUser(t, client, base, formatComposeEmail("p11-phone-friend", n), "VoiceQaTest1!")

	patchComposePrivacy(t, client, base, target.AccessToken, map[string]any{
		"preset":                   "personal",
		"allow_phone_search":       friendsOnly,
		"allow_friend_requests":    everyone,
		"allow_chat_space_invites": everyone,
	})
	patchComposePrivacy(t, client, base, friend.AccessToken, map[string]any{
		"preset":                   "personal",
		"allow_friend_requests":    everyone,
		"allow_chat_space_invites": everyone,
	})

	sendComposeFriendInvitation(t, client, base, friend.AccessToken, target.ProfileID)
	acceptComposeFriendInvitation(t, client, base, target.AccessToken, friend.ProfileID)

	strangerMatches := syncComposePhoneContacts(t, client, base, stranger.AccessToken, phoneHash)
	require.NotContains(t, strangerMatches, target.ProfileID, "stranger must not match friends-only phone search")

	friendMatches := syncComposePhoneContacts(t, client, base, friend.AccessToken, phoneHash)
	require.Contains(t, friendMatches, target.ProfileID, "friend must match when allow_phone_search permits")
}

func formatComposePhoneHash(prefix string, n int64) string {
	// auth_db.accounts.phone is VARCHAR(32); store deterministic test hash.
	hash := fmt.Sprintf("%s-%016x", prefix, uint64(n))
	if len(hash) > 32 {
		return hash[:32]
	}
	return hash
}

func registerComposeUserWithPhone(t *testing.T, client *http.Client, base, email, phone, password string) authSessionResponse {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"email":            email,
		"phone":            phone,
		"password":         password,
		"guest":            false,
		"device_info_json": `{"platform":"go-live-test"}`,
	})
	require.NoError(t, err)

	resp, err := client.Post(base+"/api/v1/auth/register", "application/json", bytes.NewReader(payload))
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode,
		"register %s: status=%d body=%s", email, resp.StatusCode, string(raw))

	var envelope authSessionEnvelope
	require.NoError(t, json.Unmarshal(raw, &envelope))
	sess := envelope.Session
	require.NotEmpty(t, sess.AccessToken)
	require.NotEmpty(t, sess.ProfileID)
	return sess
}

func syncComposePhoneContacts(t *testing.T, client *http.Client, base, accessToken, phoneHash string) []string {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"hashed_phone_numbers": []string{phoneHash},
	})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/friends/contacts/sync", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "sync phone contacts body=%s", string(raw))

	var out struct {
		MatchedProfileIds []string `json:"matched_profile_ids"`
	}
	require.NoError(t, json.Unmarshal(raw, &out))
	return out.MatchedProfileIds
}
