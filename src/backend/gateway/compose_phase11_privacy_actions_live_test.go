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

// TestComposePhase11PrivacyActions_live documents PLAN Phase 11 action privacy enforcement via Gateway.
func TestComposePhase11PrivacyActions_live(t *testing.T) {
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

	target := registerComposeUser(t, client, base, formatComposeEmail("p11act-target", n), "VoiceQaTest1!")
	stranger := registerComposeUser(t, client, base, formatComposeEmail("p11act-stranger", n), "VoiceQaTest1!")
	groupOwner := registerComposeUser(t, client, base, formatComposeEmail("p11act-gowner", n), "VoiceQaTest1!")
	groupMember := registerComposeUser(t, client, base, formatComposeEmail("p11act-gmember", n), "VoiceQaTest1!")
	groupFiller := registerComposeUser(t, client, base, formatComposeEmail("p11act-gfill", n), "VoiceQaTest1!")
	spaceOwner := registerComposeUser(t, client, base, formatComposeEmail("p11act-sowner", n), "VoiceQaTest1!")

	patchComposePrivacy(t, client, base, target.AccessToken, map[string]any{
		"preset":                   "personal",
		"allow_dm":                 everyone,
		"show_online":              friendsOnly,
		"show_game_status":         friendsOnly,
		"show_mm_rating":           friendsOnly,
		"show_phone":               map[string]any{"friends": false, "friends_of_friends": false, "space_members": false, "include_guests": false},
		"show_stories":             friendsOnly,
		"allow_friend_requests":    everyone,
		"allow_guest_dm":           false,
		"allow_phone_search":       friendsOnly,
		"allow_calls":              friendsOnly,
		"allow_chat_space_invites": friendsOnly,
		"allow_files":              friendsOnly,
		"allow_voice_messages":     friendsOnly,
	})

	// Setup users need permissive invite policy so the group can be formed before testing denial.
	for _, tok := range []string{groupOwner.AccessToken, groupMember.AccessToken, groupFiller.AccessToken, spaceOwner.AccessToken} {
		patchComposePrivacy(t, client, base, tok, map[string]any{
			"preset":                   "personal",
			"allow_chat_space_invites": everyone,
		})
	}

	chatID := createComposeDM(t, client, base, stranger.AccessToken, target.ProfileID)
	require.Equal(t, http.StatusForbidden, composeStartCallStatus(t, client, base, stranger.AccessToken, chatID, target.ProfileID))

	groupID := createComposeGroup(t, client, base, groupOwner.AccessToken, "Privacy actions QA")
	addComposeGroupMembers(t, client, base, groupOwner.AccessToken, groupID, groupMember.ProfileID, groupFiller.ProfileID)
	require.Equal(t, http.StatusForbidden, addComposeGroupMembersStatus(t, client, base, groupMember.AccessToken, groupID, target.ProfileID))

	spaceID := createComposeSpace(t, client, base, spaceOwner.AccessToken, "Privacy actions QA", "phase 11 live")
	invite := createComposeSpaceInvite(t, client, base, spaceOwner.AccessToken, spaceID)
	require.Equal(t, http.StatusForbidden, joinComposeSpaceByInviteStatus(t, client, base, target.AccessToken, invite.Code))

	if composeFileUploadAvailable(t, client, base, stranger.AccessToken) {
		fileID, fileType := composeUploadSmallTextFile(t, client, base, stranger.AccessToken, chatID)
		attachments, err := json.Marshal([]map[string]string{
			{"file_id": fileID, "type": fileType},
		})
		require.NoError(t, err)
		require.Equal(t, http.StatusForbidden, sendComposeMessageWithAttachmentsStatus(t, client, base, stranger.AccessToken, chatID, string(attachments)))

		voiceFileID, _ := composeUploadSmallAudioFile(t, client, base, stranger.AccessToken, chatID)
		voiceAttachments, err := json.Marshal([]map[string]string{
			{"file_id": voiceFileID, "type": "voice_message"},
		})
		require.NoError(t, err)
		require.Equal(t, http.StatusForbidden, sendComposeMessageWithAttachmentsStatus(t, client, base, stranger.AccessToken, chatID, string(voiceAttachments)))

		patchComposePrivacy(t, client, base, groupMember.AccessToken, map[string]any{
			"preset":               "personal",
			"allow_files":          friendsOnly,
			"allow_voice_messages": friendsOnly,
		})
		if composeFileUploadAvailable(t, client, base, groupFiller.AccessToken) {
			groupFileID, groupFileType := composeUploadSmallTextFile(t, client, base, groupFiller.AccessToken, groupID)
			groupAttachments, err := json.Marshal([]map[string]string{
				{"file_id": groupFileID, "type": groupFileType},
			})
			require.NoError(t, err)
			require.Equal(t, http.StatusForbidden, sendComposeMessageWithAttachmentsStatus(t, client, base, groupFiller.AccessToken, groupID, string(groupAttachments)))
		}
	}
}

func patchComposePrivacy(t *testing.T, client *http.Client, base, accessToken string, settings map[string]any) {
	t.Helper()
	body, err := json.Marshal(map[string]any{"settings": settings})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPatch, base+"/api/v1/users/me/privacy", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "PATCH privacy body=%s", string(respBody))
}

func addComposeGroupMembersStatus(t *testing.T, client *http.Client, base, accessToken, chatID string, profileIDs ...string) int {
	t.Helper()
	payload, err := json.Marshal(map[string]any{"profile_ids": profileIDs})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/chats/"+chatID+"/members", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	return resp.StatusCode
}

func sendComposeMessageWithAttachmentsStatus(t *testing.T, client *http.Client, base, accessToken, chatID, attachmentsJSON string) int {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"chat":              map[string]string{"id": chatID},
		"content":           "",
		"attachments_json":  attachmentsJSON,
		"client_message_id": composeClientMessageID(),
	})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/messages/send", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	return resp.StatusCode
}
