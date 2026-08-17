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

// TestComposeVoiceCommanderFloor_live documents VC-07:
// commander mode + raise hand + GrantFloor/RevokeFloor + broadcasting on group voice.
func TestComposeVoiceCommanderFloor_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 60 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()

	owner := registerComposeUser(t, client, base, formatComposeEmail("vc07-owner", n), "VoiceQaTest1!")
	member := registerComposeUser(t, client, base, formatComposeEmail("vc07-member", n), "VoiceQaTest1!")
	filler := registerComposeUser(t, client, base, formatComposeEmail("vc07-filler", n), "VoiceQaTest1!")

	groupID := createComposeGroup(t, client, base, owner.AccessToken, "Commander QA")
	addComposeGroupMembersForInvitees(t, client, base, owner.AccessToken, groupID, member, filler)

	roomID := startComposeGroupVoice(t, client, base, owner.AccessToken, groupID)
	joinComposeVoiceCall(t, client, base, member.AccessToken, roomID)

	composePostVoiceAction(t, client, base, member.AccessToken, roomID, "raise-hand", nil)
	composePostVoiceAction(t, client, base, owner.AccessToken, roomID, "commander", map[string]any{"enabled": true})
	composePostVoiceAction(t, client, base, owner.AccessToken, roomID, "grant-floor", map[string]any{"profile_id": member.ProfileID})

	states := composeGetCallVoiceStates(t, client, base, owner.AccessToken, roomID)
	memberState := findComposeParticipantState(states, member.ProfileID)
	require.True(t, boolFromAny(memberState["has_floor"], memberState["hasFloor"]), "member must have floor after GrantFloor")
	require.False(t, boolFromAny(memberState["hand_raised"], memberState["handRaised"]), "hand clears after GrantFloor")

	ownerState := findComposeParticipantState(states, owner.ProfileID)
	require.True(t, boolFromAny(ownerState["is_commander"], ownerState["isCommander"]))

	composePostVoiceAction(t, client, base, owner.AccessToken, roomID, "broadcast", map[string]any{"enabled": true})
	states = composeGetCallVoiceStates(t, client, base, owner.AccessToken, roomID)
	ownerState = findComposeParticipantState(states, owner.ProfileID)
	require.True(t, boolFromAny(ownerState["is_broadcasting"], ownerState["isBroadcasting"]))

	composePostVoiceAction(t, client, base, owner.AccessToken, roomID, "revoke-floor", map[string]any{"profile_id": member.ProfileID})
	states = composeGetCallVoiceStates(t, client, base, owner.AccessToken, roomID)
	memberState = findComposeParticipantState(states, member.ProfileID)
	require.False(t, boolFromAny(memberState["has_floor"], memberState["hasFloor"]))
}

func startComposeGroupVoice(t *testing.T, client *http.Client, base, accessToken, groupChatID string) string {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"room_type_enum": "VOICE_SESSION_KIND_GROUP_VOICE",
		"linked_chat":    map[string]string{"id": groupChatID, "type": "CHAT_TYPE_GROUP"},
		"media_kind":     "audio",
	})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/voice/calls", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "start group voice body=%s", string(raw))
	var parsed struct {
		CallSession struct {
			RoomID string `json:"room_id"`
		} `json:"call_session"`
	}
	require.NoError(t, json.Unmarshal(raw, &parsed))
	require.NotEmpty(t, parsed.CallSession.RoomID)
	return parsed.CallSession.RoomID
}

func joinComposeVoiceCall(t *testing.T, client *http.Client, base, accessToken, roomID string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/voice/calls/"+roomID+"/join", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "join call body=%s", string(raw))
}

func composePostVoiceAction(t *testing.T, client *http.Client, base, token, roomID, action string, body map[string]any) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/voice/calls/"+roomID+"/"+action, reader)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	require.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent,
		"voice action %s body=%s", action, string(raw))
}

func composeGetCallVoiceStates(t *testing.T, client *http.Client, base, token, roomID string) []map[string]any {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, base+"/api/v1/voice/calls/"+roomID+"/states", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "voice states body=%s", string(raw))
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(raw, &parsed))
	parts, _ := parsed["participants"].([]any)
	out := make([]map[string]any, 0, len(parts))
	for _, p := range parts {
		if m, ok := p.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func findComposeParticipantState(states []map[string]any, profileID string) map[string]any {
	for _, s := range states {
		if stringFromAny(s["profile_id"], s["profileId"]) == profileID {
			return s
		}
	}
	return map[string]any{}
}

func boolFromAny(vals ...any) bool {
	for _, v := range vals {
		switch b := v.(type) {
		case bool:
			return b
		}
	}
	return false
}
