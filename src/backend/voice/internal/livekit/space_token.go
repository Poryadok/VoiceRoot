package livekit

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

const maxSpaceTokenTTL = 60 * time.Second

type SpaceRoomGrant struct {
	AccountID       string
	ProfileID       string
	SpaceID         string
	VoiceRoomID     string
	LiveKitRoomName string
	MediaGeneration string
	SessionEpoch    uint64
	AccessEpoch     uint64
	PolicyEpoch     uint64
	CanJoin         bool
	CanPublishAudio bool
	CanSubscribe    bool
}

// SpaceTokenIssuer is intentionally separate from the shared issuer because
// direct/group and managed MatchFound tokens retain their existing lifetime.
type SpaceTokenIssuer struct {
	apiKey string
	secret string
	url    string
	ttl    time.Duration
}

func NewSpaceTokenIssuer(apiKey, secret, url string, ttl time.Duration) *SpaceTokenIssuer {
	if ttl <= 0 || ttl > maxSpaceTokenTTL {
		ttl = maxSpaceTokenTTL
	}
	return &SpaceTokenIssuer{apiKey: strings.TrimSpace(apiKey), secret: strings.TrimSpace(secret), url: strings.TrimSpace(url), ttl: ttl}
}

func (i *SpaceTokenIssuer) LivekitURL() string {
	if i == nil {
		return ""
	}
	return i.url
}

func (i *SpaceTokenIssuer) JoinToken(grant SpaceRoomGrant, now time.Time) (string, time.Time, error) {
	if i == nil || i.apiKey == "" || i.secret == "" || i.ttl <= 0 || i.ttl > maxSpaceTokenTTL {
		return "", time.Time{}, errors.New("space livekit token issuer not configured")
	}
	for _, raw := range []string{grant.AccountID, grant.ProfileID, grant.SpaceID, grant.VoiceRoomID} {
		id, err := uuid.Parse(strings.TrimSpace(raw))
		if err != nil || id.String() != raw {
			return "", time.Time{}, errors.New("space media identity is invalid")
		}
	}
	if strings.TrimSpace(grant.LiveKitRoomName) == "" || grant.SessionEpoch == 0 || grant.AccessEpoch == 0 || grant.PolicyEpoch == 0 || !grant.CanJoin || !grant.CanSubscribe {
		return "", time.Time{}, errors.New("space media grant is incomplete")
	}
	identity, err := SpaceParticipantIdentity(grant.ProfileID, grant.MediaGeneration)
	if err != nil {
		return "", time.Time{}, err
	}
	expiresAt := now.UTC().Add(i.ttl)
	issuedAt := now.UTC().Unix()
	claims := map[string]any{
		"iss": i.apiKey, "sub": identity, "iat": issuedAt, "nbf": issuedAt,
		"exp": expiresAt.Unix(), "account_id": grant.AccountID, "profile_id": grant.ProfileID, "space_id": grant.SpaceID,
		"voice_room_id": grant.VoiceRoomID, "session_epoch": grant.SessionEpoch,
		"space_access_epoch": grant.AccessEpoch, "role_policy_epoch": grant.PolicyEpoch,
		"video": map[string]any{
			"roomJoin": true, "room": grant.LiveKitRoomName,
			"canPublish": grant.CanPublishAudio, "canSubscribe": grant.CanSubscribe,
		},
	}
	head, err := encodeJWTPart(map[string]string{"alg": "HS256", "typ": "JWT"})
	if err != nil {
		return "", time.Time{}, err
	}
	body, err := encodeJWTPart(claims)
	if err != nil {
		return "", time.Time{}, err
	}
	unsigned := head + "." + body
	mac := hmac.New(sha256.New, []byte(i.secret))
	_, _ = mac.Write([]byte(unsigned))
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), expiresAt, nil
}

// SpaceParticipantIdentity returns the existing server-owned identity shape
// with an incarnation UUID so a stale revocation cannot eject a later session.
func SpaceParticipantIdentity(profileID, generation string) (string, error) {
	profile, profileErr := uuid.Parse(strings.TrimSpace(profileID))
	incarnation, generationErr := uuid.Parse(strings.TrimSpace(generation))
	if profileErr != nil || generationErr != nil || profile.String() != profileID || incarnation == uuid.Nil || incarnation.String() != generation {
		return "", errors.New("space media participant identity is invalid")
	}
	return "profile:" + profile.String() + ":media:" + incarnation.String(), nil
}
