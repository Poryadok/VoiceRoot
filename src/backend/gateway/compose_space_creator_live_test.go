package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// This public proof requires the real Auth floor, native Space publication and
// protected Role bootstrap. Owner-only invite success checks the resulting ACL;
// component mocks and a successful Space SQL insert cannot satisfy this proof.
func TestComposeSpaceCreatorProtectedPublic_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true")
	}
	clearLiveComposeAuthRateLimit(t)
	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()
	owner := registerComposeUser(t, client, base, formatComposeEmail("creator-owner", n), "VoiceQaTest1!")
	outsider := registerComposeUser(t, client, base, formatComposeEmail("creator-outsider", n), "VoiceQaTest1!")
	guest := registerComposeGuest(t, client, base, "VoiceQaTest1!")
	revoked := registerComposeUser(t, client, base, formatComposeEmail("creator-revoked", n), "VoiceQaTest1!")
	spaceID := createComposeSpace(t, client, base, owner.AccessToken, fmt.Sprintf("protected-creator-%d", n), "real Creator boundary")
	invite := createComposeSpaceInvite(t, client, base, owner.AccessToken, spaceID)
	require.NotEmpty(t, invite.Code, "real Role Owner grant must authorize invite creation")
	require.Equal(t, http.StatusForbidden, createComposeSpaceInviteStatus(t, client, base, outsider.AccessToken, spaceID))
	deleteComposeEpochLiveAccount(t, client, base, revoked.AccessToken, "VoiceQaTest1!")
	for _, tc := range []struct {
		name, token, body string
		status            int
	}{
		{"anonymous", "", `{"name":"anonymous-denied"}`, http.StatusUnauthorized},
		{"guest", guest.AccessToken, `{"name":"guest-denied"}`, http.StatusForbidden},
		{"revoked", revoked.AccessToken, `{"name":"revoked-denied"}`, http.StatusUnauthorized},
		{"unknown_actor", owner.AccessToken, `{"name":"forged-denied","owner_profile_id":"` + outsider.ProfileID + `"}`, http.StatusBadRequest},
		{"duplicate_field", owner.AccessToken, `{"name":"first","name":"second"}`, http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodPost, base+"/api/v1/spaces", bytes.NewBufferString(tc.body))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")
			if tc.token != "" {
				req.Header.Set("Authorization", "Bearer "+tc.token)
			}
			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.Equal(t, tc.status, resp.StatusCode, "creator denial body=%s", body)
		})
	}
}
