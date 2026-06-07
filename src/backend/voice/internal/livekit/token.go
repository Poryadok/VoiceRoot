package livekit

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type TokenIssuer interface {
	JoinToken(profileID, roomName string, now time.Time) (jwt string, expiresAt time.Time, err error)
	LivekitURL() string
}

type HS256TokenIssuer struct {
	apiKey   string
	secret   string
	url      string
	tokenTTL time.Duration
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

func (i *HS256TokenIssuer) JoinToken(profileID, roomName string, now time.Time) (string, time.Time, error) {
	if i == nil || strings.TrimSpace(i.apiKey) == "" || strings.TrimSpace(i.secret) == "" {
		return "", time.Time{}, fmt.Errorf("livekit credentials not configured")
	}
	if strings.TrimSpace(profileID) == "" || strings.TrimSpace(roomName) == "" {
		return "", time.Time{}, fmt.Errorf("profile and room are required")
	}
	expiresAt := now.UTC().Add(i.tokenTTL)
	header := map[string]string{"alg": "HS256", "typ": "JWT"}
	claims := map[string]any{
		"iss": i.apiKey,
		"sub": profileID,
		"nbf": now.UTC().Unix(),
		"exp": expiresAt.Unix(),
		"video": map[string]any{
			"roomJoin": true,
			"room":     roomName,
		},
	}
	if strings.TrimSpace(i.url) != "" {
		claims["metadata"] = map[string]any{"livekit_url": i.url}
	}
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
