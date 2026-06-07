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
	jwt, _, err := issuer.JoinToken("profile-a", "voice-dm-room-1", time.Unix(1_700_000_000, 0))
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
