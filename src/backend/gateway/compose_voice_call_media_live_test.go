//go:build linux && live

package main

import (
	"context"
	"math"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/livekit/protocol/livekit"
	lksdk "github.com/livekit/server-sdk-go/v2"
	lkmedia "github.com/livekit/server-sdk-go/v2/pkg/media"
	"github.com/pion/webrtc/v4"
	"github.com/stretchr/testify/require"
)

// TestComposeVoiceCallBidirectionalAudio_live joins both call parties to LiveKit using
// production JWTs from GET /api/v1/voice/calls/{room}/token, publishes synthetic PCM audio,
// and asserts each side subscribes to the other's audio track.
//
// Opt-in (same as signaling live tests; requires LiveKit published on the host):
//
//	VOICE_RUN_LIVE_COMPOSE=true VOICE_API_BASE_URL=http://127.0.0.1:18080 go test -run TestComposeVoiceCallBidirectionalAudio_live -count=1 -timeout 3m ./...
func TestComposeVoiceCallBidirectionalAudio_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()

	n := time.Now().UnixNano()
	sessA := registerComposeUser(t, client, base, formatComposeEmail("call-media-a", n), "VoiceQaTest1!")
	sessB := registerComposeUser(t, client, base, formatComposeEmail("call-media-b", n), "VoiceQaTest1!")
	chatID := createComposeDMBetween(t, client, base, sessA, sessB)

	call := startComposeCall(t, client, base, sessA.AccessToken, chatID, sessB.ProfileID)
	_ = acceptComposeCall(t, client, base, sessB.AccessToken, call.RoomID)

	tokenA := getComposeJoinToken(t, client, base, sessA.AccessToken, call.RoomID)
	tokenB := getComposeJoinToken(t, client, base, sessB.AccessToken, call.RoomID)

	livekitURL := resolveComposeLivekitURL(tokenA.LivekitURL, tokenB.LivekitURL)
	t.Logf("livekit url: %s room: %s", livekitURL, call.LivekitRoomName)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	peerA := newLivekitCallPeer(ctx, t, livekitURL, tokenA.JWT, sessB.ProfileID)
	defer peerA.close()
	peerB := newLivekitCallPeer(ctx, t, livekitURL, tokenB.JWT, sessA.ProfileID)
	defer peerB.close()

	require.NoError(t, peerA.waitRemoteAudio(45*time.Second), "caller did not receive callee audio")
	require.NoError(t, peerB.waitRemoteAudio(45*time.Second), "callee did not receive caller audio")

	endComposeCall(t, client, base, sessA.AccessToken, call.RoomID)
}

func resolveComposeLivekitURL(from ...string) string {
	for _, u := range from {
		u = strings.TrimSpace(u)
		if u != "" && !isDockerInternalLivekitHost(u) {
			return u
		}
	}
	return composeLivekitFallbackURL()
}

func composeLivekitFallbackURL() string {
	for _, key := range []string{"VOICE_LIVEKIT_URL", "VOICE_LIVEKIT_PUBLIC_URL"} {
		if u := strings.TrimSpace(os.Getenv(key)); u != "" {
			return u
		}
	}
	return "ws://127.0.0.1:7880"
}

func isDockerInternalLivekitHost(raw string) bool {
	u, err := parseWSURLHost(raw)
	if err != nil || u == "" {
		return false
	}
	if u == "localhost" || u == "127.0.0.1" || u == "::1" {
		return false
	}
	if strings.Count(u, ".") == 3 && !strings.Contains(u, ":") {
		return false
	}
	return !strings.Contains(u, ".")
}

func parseWSURLHost(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if !strings.Contains(raw, "://") {
		raw = "ws://" + raw
	}
	// net/url accepts ws scheme in recent Go; fall back to http for parse.
	parse := raw
	if strings.HasPrefix(parse, "ws://") {
		parse = "http://" + strings.TrimPrefix(parse, "ws://")
	} else if strings.HasPrefix(parse, "wss://") {
		parse = "https://" + strings.TrimPrefix(parse, "wss://")
	}
	u, err := url.Parse(parse)
	if err != nil {
		return "", err
	}
	return strings.ToLower(strings.TrimSpace(u.Hostname())), nil
}

type livekitCallPeer struct {
	t              *testing.T
	ctx            context.Context
	room           *lksdk.Room
	pcmTrack       *lkmedia.PCMLocalTrack
	remoteAudio    atomic.Bool
	expectedRemote string
}

func newLivekitCallPeer(ctx context.Context, t *testing.T, livekitURL, jwt, expectedRemoteProfileID string) *livekitCallPeer {
	t.Helper()
	peer := &livekitCallPeer{t: t, ctx: ctx, expectedRemote: expectedRemoteProfileID}
	trackSubscribed := make(chan struct{}, 4)
	cb := &lksdk.RoomCallback{
		ParticipantCallback: lksdk.ParticipantCallback{
			OnTrackSubscribed: func(track *webrtc.TrackRemote, _ *lksdk.RemoteTrackPublication, rp *lksdk.RemoteParticipant) {
				if track == nil || track.Kind() != webrtc.RTPCodecTypeAudio {
					return
				}
				if peer.expectedRemote != "" && rp.Identity() != peer.expectedRemote {
					return
				}
				peer.remoteAudio.Store(true)
				select {
				case trackSubscribed <- struct{}{}:
				default:
				}
			},
		},
	}

	room, err := lksdk.ConnectToRoomWithToken(livekitURL, jwt, cb, lksdk.WithAutoSubscribe(true))
	require.NoError(t, err, "livekit connect")
	peer.room = room

	pcmTrack, err := lkmedia.NewPCMLocalTrack(48000, 1, nil)
	require.NoError(t, err)
	_, err = room.LocalParticipant.PublishTrack(pcmTrack, &lksdk.TrackPublicationOptions{
		Name:   "qa-mic",
		Source: livekit.TrackSource_MICROPHONE,
	})
	require.NoError(t, err, "publish pcm track")
	peer.pcmTrack = pcmTrack

	go peer.publishTone(ctx, pcmTrack, 440.0)

	// Drain initial subscription signals.
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-trackSubscribed:
			case <-time.After(200 * time.Millisecond):
				return
			}
		}
	}()

	return peer
}

func (p *livekitCallPeer) publishTone(ctx context.Context, track *lkmedia.PCMLocalTrack, hz float64) {
	const sampleRate = 48000
	const frameSamples = 480 // 10ms @ 48kHz
	phase := 0.0
	buf := make([]int16, frameSamples)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for i := range buf {
				sample := math.Sin(phase) * 0.25 * math.MaxInt16
				buf[i] = int16(sample)
				phase += 2 * math.Pi * hz / sampleRate
			}
			if err := track.WriteSample(buf); err != nil {
				return
			}
		}
	}
}

func (p *livekitCallPeer) waitRemoteAudio(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if p.remoteAudio.Load() {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return context.DeadlineExceeded
}

func (p *livekitCallPeer) close() {
	if p.pcmTrack != nil {
		_ = p.pcmTrack.Close()
	}
	if p.room != nil {
		p.room.Disconnect()
	}
}
