package livekit

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type TokenIssuer interface {
	JoinToken(profileID, roomName string, canPublish *bool, now time.Time) (jwt string, expiresAt time.Time, err error)
	LivekitURL() string
}

type HS256TokenIssuer struct {
	apiKey   string
	secret   string
	url      string
	tokenTTL time.Duration
}

// MatchSquadIdentity binds a media participant target to one immutable member
// generation. Ordinary calls continue to use profile IDs directly.
func MatchSquadIdentity(profileID, mediaEpoch string) (string, error) {
	profile, err := uuid.Parse(profileID)
	if err != nil || profile == uuid.Nil || profile.String() != profileID {
		return "", fmt.Errorf("invalid MatchSquad profile identity")
	}
	epoch, err := uuid.Parse(mediaEpoch)
	if err != nil || epoch == uuid.Nil || epoch.String() != mediaEpoch {
		return "", fmt.Errorf("invalid MatchSquad media generation")
	}
	return "ms:" + profile.String() + ":" + epoch.String(), nil
}

func (i *HS256TokenIssuer) LivekitURL() string {
	if i == nil {
		return ""
	}
	return strings.TrimSpace(i.url)
}

func NewHS256TokenIssuer(apiKey, secret, url string, tokenTTL time.Duration) *HS256TokenIssuer {
	if tokenTTL <= 0 {
		tokenTTL = time.Hour
	}
	return &HS256TokenIssuer{apiKey: apiKey, secret: secret, url: url, tokenTTL: tokenTTL}
}

func (i *HS256TokenIssuer) JoinToken(profileID, roomName string, canPublish *bool, now time.Time) (string, time.Time, error) {
	if i == nil {
		return "", time.Time{}, fmt.Errorf("livekit credentials not configured")
	}
	return i.joinTokenUntil(profileID, roomName, canPublish, now, now.UTC().Add(i.tokenTTL))
}

// MatchSquadTokenWindow chooses an integer-second issuance window from the
// database clock. Its expiry is capped by both the verified principal and the
// fixed MatchSquad bearer bound, so it can be durably reserved before signing.
func (i *HS256TokenIssuer) MatchSquadTokenWindow(now, actorExpiresAt time.Time) (time.Time, time.Time, error) {
	now = now.UTC()
	if !actorExpiresAt.After(now) {
		return time.Time{}, time.Time{}, fmt.Errorf("delegated user credential expired")
	}
	issuedAt := time.Unix(now.Unix(), 0).UTC()
	expiresAt := issuedAt.Add(time.Minute)
	actorExpiry := time.Unix(actorExpiresAt.UTC().Unix(), 0).UTC()
	if actorExpiry.Before(expiresAt) {
		expiresAt = actorExpiry
	}
	if !expiresAt.After(issuedAt) || !expiresAt.After(now) {
		return time.Time{}, time.Time{}, fmt.Errorf("delegated user credential expires too soon")
	}
	return issuedAt, expiresAt, nil
}

// MatchSquadJoinTokenUntil signs exactly the previously reserved generation
// window. It never samples a new issue time or extends the durable expiry.
func (i *HS256TokenIssuer) MatchSquadJoinTokenUntil(profileID, roomName string, canPublish *bool, issuedAt, expiresAt time.Time) (string, error) {
	issuedAt = issuedAt.UTC()
	expiresAt = expiresAt.UTC()
	if !expiresAt.After(issuedAt) || expiresAt.Sub(issuedAt) > time.Minute {
		return "", fmt.Errorf("invalid MatchSquad token window")
	}
	jwt, _, err := i.joinTokenUntil(profileID, roomName, canPublish, issuedAt, expiresAt)
	return jwt, err
}

// MatchSquadJoinToken is a compatibility wrapper for direct issuer callers.
// The member service reserves MatchSquadTokenWindow before using the fixed
// expiry signing method above.
func (i *HS256TokenIssuer) MatchSquadJoinToken(profileID, roomName string, canPublish *bool, now, actorExpiresAt time.Time) (string, time.Time, error) {
	issuedAt, expiresAt, err := i.MatchSquadTokenWindow(now, actorExpiresAt)
	if err != nil {
		return "", time.Time{}, err
	}
	jwt, err := i.MatchSquadJoinTokenUntil(profileID, roomName, canPublish, issuedAt, expiresAt)
	return jwt, expiresAt, err
}

func (i *HS256TokenIssuer) joinTokenUntil(profileID, roomName string, canPublish *bool, now, expiresAt time.Time) (string, time.Time, error) {
	if i == nil || strings.TrimSpace(i.apiKey) == "" || strings.TrimSpace(i.secret) == "" {
		return "", time.Time{}, fmt.Errorf("livekit credentials not configured")
	}
	if strings.TrimSpace(profileID) == "" || strings.TrimSpace(roomName) == "" {
		return "", time.Time{}, fmt.Errorf("profile and room are required")
	}
	now = now.UTC()
	expiresAt = expiresAt.UTC()
	if !expiresAt.After(now) {
		return "", time.Time{}, fmt.Errorf("token expiry must be after issue time")
	}
	issuedAt := now.UTC().Unix()
	header := map[string]string{"alg": "HS256", "typ": "JWT"}
	video := map[string]any{
		"roomJoin": true,
		"room":     roomName,
	}
	if canPublish != nil {
		video["canPublish"] = *canPublish
	}
	claims := map[string]any{
		"iss":   i.apiKey,
		"sub":   profileID,
		"iat":   issuedAt,
		"nbf":   issuedAt,
		"exp":   expiresAt.Unix(),
		"video": video,
	}
	// livekit_url is returned via GetJoinTokenResponse; do not embed an object in
	// JWT "metadata" — LiveKit expects metadata to be a string claim.
	head, err := encodeJWTPart(header)
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
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return unsigned + "." + sig, expiresAt, nil
}

func encodeJWTPart(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
