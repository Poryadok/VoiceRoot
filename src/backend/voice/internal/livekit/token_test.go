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

func TestMatchSquadFixedWindowSignerUsesReservedJWTClaims(t *testing.T) {
	t.Parallel()
	databaseNow := time.Unix(1_700_000_000, 850_000_000).UTC()
	actorExpiry := databaseNow.Add(25*time.Second + 700*time.Millisecond)
	issuer := NewHS256TokenIssuer("devkey", "secret", "ws://127.0.0.1:7880", time.Hour)
	issuedAt, expiresAt, err := issuer.MatchSquadTokenWindow(databaseNow, actorExpiry)
	require.NoError(t, err)
	require.Equal(t, time.Unix(databaseNow.Unix(), 0).UTC(), issuedAt)
	require.Equal(t, time.Unix(actorExpiry.Unix(), 0).UTC(), expiresAt)

	jwt, err := issuer.MatchSquadJoinTokenUntil("ms:profile:epoch", "match-squad-room", nil, issuedAt, expiresAt)
	require.NoError(t, err)
	parts := strings.Split(jwt, ".")
	require.Len(t, parts, 3)
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	require.NoError(t, err)
	var claims map[string]any
	require.NoError(t, json.Unmarshal(payload, &claims))
	require.Equal(t, issuedAt.Unix(), int64(claims["iat"].(float64)))
	require.Equal(t, expiresAt.Unix(), int64(claims["exp"].(float64)))
	require.LessOrEqual(t, int64(claims["exp"].(float64)-claims["iat"].(float64)), int64(60))

	_, err = issuer.MatchSquadJoinTokenUntil("ms:profile:epoch", "match-squad-room", nil, issuedAt, issuedAt.Add(61*time.Second))
	require.Error(t, err, "the fixed signer rejects a reservation above the bearer bound")
}
func TestMatchSquadTokenUsesCanonicalMediaEpochIdentity(t *testing.T) {
	t.Parallel()
	profileID := "00000000-0000-4000-8000-000000000011"
	epoch := "00000000-0000-4000-8000-000000000022"
	identity, err := MatchSquadIdentity(profileID, epoch)
	require.NoError(t, err)
	require.Equal(t, "ms:"+profileID+":"+epoch, identity)
	for _, pair := range [][2]string{
		{"AAAAAAAA-AAAA-4AAA-8AAA-AAAAAAAAAAAA", epoch},
		{profileID, "00000000-0000-0000-0000-000000000000"},
		{"not-a-profile", epoch},
	} {
		_, err := MatchSquadIdentity(pair[0], pair[1])
		require.Error(t, err)
	}

	issuer := NewHS256TokenIssuer("devkey", "secret", "ws://127.0.0.1:7880", time.Hour)
	now := time.Unix(1_700_000_000, 0)
	matched, _, err := issuer.MatchSquadJoinToken(identity, "match-squad-room", nil, now, now.Add(time.Minute))
	require.NoError(t, err)
	ordinary, _, err := issuer.JoinToken(profileID, "ordinary-room", nil, now)
	require.NoError(t, err)
	require.Equal(t, identity, tokenSubject(t, matched))
	require.Equal(t, profileID, tokenSubject(t, ordinary), "ordinary media identities stay profile based")
}

func tokenSubject(t *testing.T, token string) string {
	t.Helper()
	parts := strings.Split(token, ".")
	require.Len(t, parts, 3)
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	require.NoError(t, err)
	var claims map[string]any
	require.NoError(t, json.Unmarshal(payload, &claims))
	sub, ok := claims["sub"].(string)
	require.True(t, ok)
	return sub
}

func TestMatchSquadJoinTokenCapsLongActorValidity(t *testing.T) {
	t.Parallel()
	now := time.Unix(1_700_000_000, 0)
	issuer := NewHS256TokenIssuer("devkey", "secret", "ws://127.0.0.1:7880", time.Hour)
	_, expiry, err := issuer.MatchSquadJoinToken("profile-a", "match-squad-room-1", nil, now, now.Add(5*time.Minute))
	require.NoError(t, err)
	require.Equal(t, now.UTC().Add(time.Minute), expiry)
}
