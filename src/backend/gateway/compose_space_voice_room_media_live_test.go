//go:build linux && live

package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestComposeSpaceVoiceRoomBidirectionalAudio_live proves that two distinct
// Space members can publish and receive actual LiveKit audio for one room.
func TestComposeSpaceVoiceRoomBidirectionalAudio_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()
	owner := registerComposeUser(t, client, base, formatComposeEmail("space-media-owner", n), "VoiceQaTest1!")
	member := registerComposeUser(t, client, base, formatComposeEmail("space-media-member", n), "VoiceQaTest1!")

	spaceID := createComposeSpace(t, client, base, owner.AccessToken, "Space media QA", "two-member bidirectional media")
	invite := createComposeSpaceInvite(t, client, base, owner.AccessToken, spaceID)
	joinComposeSpaceByInvite(t, client, base, member.AccessToken, invite.Code)
	voiceRoomID := createComposeSpaceVoiceRoom(t, client, base, owner.AccessToken, spaceID, "Media")

	joinOwner := joinComposeSpaceVoiceRoom(t, client, base, owner.AccessToken, voiceRoomID, spaceID)
	defer leaveComposeSpaceVoiceRoom(t, client, base, owner.AccessToken, voiceRoomID)
	joinMember := joinComposeSpaceVoiceRoom(t, client, base, member.AccessToken, voiceRoomID, spaceID)
	defer leaveComposeSpaceVoiceRoom(t, client, base, member.AccessToken, voiceRoomID)
	require.NotEmpty(t, joinOwner.RoomID)
	require.Equal(t, joinOwner.RoomID, joinMember.RoomID)
	require.NotEmpty(t, joinOwner.LivekitRoomName)
	require.Equal(t, joinOwner.LivekitRoomName, joinMember.LivekitRoomName)

	tokenOwner := getComposeJoinToken(t, client, base, owner.AccessToken, joinOwner.RoomID)
	tokenMember := getComposeJoinToken(t, client, base, member.AccessToken, joinMember.RoomID)
	identityOwner := livekitTokenIdentity(t, tokenOwner.JWT)
	identityMember := livekitTokenIdentity(t, tokenMember.JWT)
	require.True(t, strings.HasPrefix(identityOwner, "profile:"+owner.ProfileID+":media:"))
	require.True(t, strings.HasPrefix(identityMember, "profile:"+member.ProfileID+":media:"))
	require.NotEqual(t, identityOwner, identityMember)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	livekitURL := resolveComposeLivekitURL(tokenOwner.LivekitURL, tokenMember.LivekitURL)
	peerOwner := newLivekitCallPeer(ctx, t, livekitURL, tokenOwner.JWT, identityMember)
	defer peerOwner.close()
	peerMember := newLivekitCallPeer(ctx, t, livekitURL, tokenMember.JWT, identityOwner)
	defer peerMember.close()

	require.NoError(t, peerOwner.waitRemoteAudioRTP(45*time.Second), "Space owner did not receive member RTP audio")
	require.NoError(t, peerMember.waitRemoteAudioRTP(45*time.Second), "Space member did not receive owner RTP audio")
}

type composeSpaceVoiceRoomJoin struct {
	RoomID          string `json:"roomId"`
	LivekitRoomName string `json:"livekitRoomName"`
}

func joinComposeSpaceVoiceRoom(t *testing.T, client *http.Client, base, accessToken, voiceRoomID, spaceID string) composeSpaceVoiceRoomJoin {
	t.Helper()
	body := strings.NewReader(fmt.Sprintf(`{"space":{"id":%q}}`, spaceID))
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/voice/rooms/"+voiceRoomID+"/join", body)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	var result struct {
		VoiceSession composeSpaceVoiceRoomJoin `json:"voiceSession"`
	}
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	return result.VoiceSession
}

func leaveComposeSpaceVoiceRoom(t *testing.T, client *http.Client, base, accessToken, voiceRoomID string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/voice/rooms/"+voiceRoomID+"/leave", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func livekitTokenIdentity(t *testing.T, token string) string {
	t.Helper()
	parts := strings.Split(token, ".")
	require.Len(t, parts, 3)
	claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	require.NoError(t, err)
	var claims struct {
		Sub string `json:"sub"`
	}
	require.NoError(t, json.Unmarshal(claimsJSON, &claims))
	require.NotEmpty(t, claims.Sub)
	return claims.Sub
}
