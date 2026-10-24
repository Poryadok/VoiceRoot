//go:build linux && live

package main

import (
	"context"
	"io"
	"net/http"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestStagingVoiceCallBidirectionalAudio_live uses only dedicated existing
// sessions and a DM supplied by an authorized operator. It never registers
// users, accesses a datastore or changes privacy settings.
func TestStagingVoiceCallBidirectionalAudio_live(t *testing.T) {
	if os.Getenv("VOICE_RUN_STAGING_MEDIA") != "true" {
		t.Skip("set VOICE_RUN_STAGING_MEDIA=true for the authorized media check")
	}
	fixtureFile, err := os.Open(os.Getenv("VOICE_STAGING_MEDIA_FIXTURE"))
	require.NoError(t, err, "open private media fixture")
	defer fixtureFile.Close()
	require.NoError(t, syscall.Flock(int(fixtureFile.Fd()), syscall.LOCK_EX|syscall.LOCK_NB),
		"media fixture is already in use")
	info, err := fixtureFile.Stat()
	require.NoError(t, err)
	require.True(t, info.Mode().IsRegular() && info.Mode().Perm()&0077 == 0,
		"media fixture must be an owner-only regular file")
	raw, err := io.ReadAll(io.LimitReader(fixtureFile, 65537))
	require.NoError(t, err)
	fixture, err := parseStagingMediaFixture(raw, os.Getenv("VOICE_STAGING_URL"))
	require.NoError(t, err)
	client := &http.Client{
		Timeout:       15 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	for _, peer := range []stagingMediaPeer{fixture.Caller, fixture.Callee} {
		require.NoError(t, checkStagingMediaProfile(client, fixture.APIOrigin, peer))
		require.Nil(t, getComposeActiveCall(t, client, fixture.APIOrigin, peer.AccessToken),
			"dedicated media participant already has a call; refusing to replace it")
	}
	call := startComposeCall(t, client, fixture.APIOrigin, fixture.Caller.AccessToken,
		fixture.ChatID, fixture.Callee.ProfileID)
	// Register this immediately: accept, token, ICE and RTP failures still end
	// only the call created above, never an unrelated active call.
	t.Cleanup(func() {
		endComposeCall(t, client, fixture.APIOrigin, fixture.Caller.AccessToken, call.RoomID)
		require.Nil(t, getComposeActiveCall(t, client, fixture.APIOrigin, fixture.Caller.AccessToken))
		require.Nil(t, getComposeActiveCall(t, client, fixture.APIOrigin, fixture.Callee.AccessToken))
	})
	_ = acceptComposeCall(t, client, fixture.APIOrigin, fixture.Callee.AccessToken, call.RoomID)
	tokenA := getComposeJoinToken(t, client, fixture.APIOrigin, fixture.Caller.AccessToken, call.RoomID)
	tokenB := getComposeJoinToken(t, client, fixture.APIOrigin, fixture.Callee.AccessToken, call.RoomID)
	require.True(t, tokenA.LivekitURL == fixture.LivekitOrigin && tokenB.LivekitURL == fixture.LivekitOrigin,
		"Voice returned an unexpected media origin; no fallback or TLS bypass permitted")
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	peerA := newLivekitCallPeer(ctx, t, fixture.LivekitOrigin, tokenA.JWT, fixture.Callee.ProfileID)
	defer peerA.close()
	peerB := newLivekitCallPeer(ctx, t, fixture.LivekitOrigin, tokenB.JWT, fixture.Caller.ProfileID)
	defer peerB.close()
	require.NoError(t, peerA.waitRemoteAudioRTP(45*time.Second), "caller did not receive callee audio RTP")
	require.NoError(t, peerB.waitRemoteAudioRTP(45*time.Second), "callee did not receive caller audio RTP")
	t.Log("both peers received ten nonempty advancing audio RTP packets")
}
