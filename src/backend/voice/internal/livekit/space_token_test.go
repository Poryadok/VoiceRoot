package livekit

import (
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestSpaceTokenIssuerBindsIdentityEpochsRoomAndExplicitGrants(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	issuer := NewSpaceTokenIssuer("api-key", "secret", "wss://livekit.example", 90*time.Second)
	grant := validSpaceRoomGrant()
	grant.CanPublishAudio = false

	token, expiresAt, err := issuer.JoinToken(grant, now)
	if err != nil {
		t.Fatal("issue token")
	}
	if got := expiresAt.Sub(now); got != maxSpaceTokenTTL {
		t.Fatalf("token lifetime was %s", got)
	}
	parts := splitJWT(t, token)
	var claims struct {
		Issuer       string         `json:"iss"`
		Subject      string         `json:"sub"`
		AccountID    string         `json:"account_id"`
		SpaceID      string         `json:"space_id"`
		VoiceRoomID  string         `json:"voice_room_id"`
		ProfileID    string         `json:"profile_id"`
		SessionEpoch uint64         `json:"session_epoch"`
		AccessEpoch  uint64         `json:"space_access_epoch"`
		PolicyEpoch  uint64         `json:"role_policy_epoch"`
		Expires      int64          `json:"exp"`
		Video        map[string]any `json:"video"`
	}
	if err := json.Unmarshal(parts, &claims); err != nil {
		t.Fatal("decode token claims")
	}
	wantIdentity, err := SpaceParticipantIdentity(grant.ProfileID, grant.MediaGeneration)
	if err != nil {
		t.Fatal("build expected LiveKit identity")
	}
	if claims.Issuer != "api-key" || claims.Subject != wantIdentity || claims.ProfileID != grant.ProfileID || claims.AccountID != grant.AccountID ||
		claims.SpaceID != grant.SpaceID || claims.VoiceRoomID != grant.VoiceRoomID ||
		claims.SessionEpoch != grant.SessionEpoch || claims.AccessEpoch != grant.AccessEpoch || claims.PolicyEpoch != grant.PolicyEpoch {
		t.Fatal("token did not bind the verified identity and current authority epochs")
	}
	if claims.Expires-now.Unix() > int64(maxSpaceTokenTTL/time.Second) {
		t.Fatal("JWT expiry exceeds the Space token lifetime")
	}
	if claims.Video["room"] != grant.LiveKitRoomName || claims.Video["roomJoin"] != true ||
		claims.Video["canPublish"] != false || claims.Video["canSubscribe"] != true {
		t.Fatal("token did not carry the explicit Space media grants")
	}
}

func TestSpaceTokenIssuerRejectsIncompleteOrDeniedGrant(t *testing.T) {
	issuer := NewSpaceTokenIssuer("api-key", "secret", "wss://livekit.example", time.Second)
	grant := validSpaceRoomGrant()
	grant.AccessEpoch = 0
	if _, _, err := issuer.JoinToken(grant, time.Now()); err == nil {
		t.Fatal("expected missing Space epoch to deny token issuance")
	}
	grant = validSpaceRoomGrant()
	grant.CanJoin = false
	if _, _, err := issuer.JoinToken(grant, time.Now()); err == nil {
		t.Fatal("expected join denial to reject token issuance")
	}
	grant = validSpaceRoomGrant()
	grant.CanSubscribe = false
	if _, _, err := issuer.JoinToken(grant, time.Now()); err == nil {
		t.Fatal("expected subscribe denial to reject token issuance")
	}
}

func validSpaceRoomGrant() SpaceRoomGrant {
	return SpaceRoomGrant{
		AccountID: uuid.NewString(), ProfileID: uuid.NewString(), SpaceID: uuid.NewString(),
		VoiceRoomID: uuid.NewString(), LiveKitRoomName: "space-room-voice-1", MediaGeneration: uuid.NewString(),
		SessionEpoch: 8, AccessEpoch: 12, PolicyEpoch: 14,
		CanJoin: true, CanPublishAudio: true, CanSubscribe: true,
	}
}

func splitJWT(t *testing.T, token string) []byte {
	t.Helper()
	parts := []byte(token)
	dotOne := -1
	dotTwo := -1
	for index, b := range parts {
		if b != '.' {
			continue
		}
		if dotOne == -1 {
			dotOne = index
		} else {
			dotTwo = index
			break
		}
	}
	if dotOne <= 0 || dotTwo <= dotOne+1 {
		t.Fatal("malformed JWT")
	}
	payload, err := base64.RawURLEncoding.DecodeString(string(parts[dotOne+1 : dotTwo]))
	if err != nil {
		t.Fatal("decode JWT payload")
	}
	return payload
}
