package livekit

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestJoinTokenOmitsObjectMetadataClaim(t *testing.T) {
	t.Parallel()

	issuer := NewHS256TokenIssuer("devkey", "secret", "ws://127.0.0.1:7880", time.Hour)
	jwt, _, err := issuer.JoinToken("profile-a", "voice-dm-room-1", nil, time.Unix(1_700_000_000, 0))
	require.NoError(t, err)

	parts := strings.Split(jwt, ".")
	require.Len(t, parts, 3)

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	require.NoError(t, err)

	var claims map[string]any
	require.NoError(t, json.Unmarshal(payload, &claims))
	require.NotContains(t, claims, "metadata")
	require.Equal(t, "devkey", claims["iss"])
	require.Equal(t, "profile-a", claims["sub"])

	video, ok := claims["video"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, true, video["roomJoin"])
	require.Equal(t, "voice-dm-room-1", video["room"])
}

func TestMatchSquadJoinTokenIsBoundedByActorValidity(t *testing.T) {
	t.Parallel()
	now := time.Unix(1_700_000_000, 0)
	actorExpiry := now.Add(17 * time.Second)
	issuer := NewHS256TokenIssuer("devkey", "secret", "ws://127.0.0.1:7880", time.Hour)
	publish := true
	token, expiry, err := issuer.MatchSquadJoinToken("profile-a", "match-squad-room-1", &publish, now, actorExpiry)
	require.NoError(t, err)
	require.Equal(t, actorExpiry.UTC(), expiry)
	parts := strings.Split(token, ".")
	require.Len(t, parts, 3)
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	require.NoError(t, err)
	var claims map[string]any
	require.NoError(t, json.Unmarshal(payload, &claims))
	require.LessOrEqual(t, int64(claims["exp"].(float64)), actorExpiry.Unix())
	require.LessOrEqual(t, int64(claims["exp"].(float64)-claims["iat"].(float64)), int64(60))
	video := claims["video"].(map[string]any)
	require.Equal(t, true, video["canPublish"])
	_, _, err = issuer.MatchSquadJoinToken("profile-a", "match-squad-room-1", nil, now, now)
	require.Error(t, err, "expired actor must not receive a bearer")
}

func TestMatchSquadJoinTokenCapsLongActorValidity(t *testing.T) {
	t.Parallel()
	now := time.Unix(1_700_000_000, 0)
	issuer := NewHS256TokenIssuer("devkey", "secret", "ws://127.0.0.1:7880", time.Hour)
	_, expiry, err := issuer.MatchSquadJoinToken("profile-a", "match-squad-room-1", nil, now, now.Add(5*time.Minute))
	require.NoError(t, err)
	require.Equal(t, now.UTC().Add(time.Minute), expiry)
}
