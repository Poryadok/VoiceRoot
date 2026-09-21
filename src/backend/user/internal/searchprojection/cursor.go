// Package searchprojection owns the protected Search bootstrap cursor format.
package searchprojection

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
)

const cursorKeyEnv = "USER_SEARCH_PROJECTION_CURSOR_HMAC_KEY"

// CursorKeyFromEnv requires a dedicated signing key whenever the protected
// listener is enabled. It deliberately never reuses TLS or JWT material.
func CursorKeyFromEnv(enabled bool, lookup func(string) string) ([]byte, error) {
	if !enabled {
		return nil, nil
	}
	key := []byte(strings.TrimSpace(lookup(cursorKeyEnv)))
	if len(key) < 32 {
		return nil, errors.New(cursorKeyEnv + " must contain at least 32 bytes")
	}
	return key, nil
}

func EncodeCursor(key []byte, value string) (string, error) {
	if len(key) < 32 || strings.TrimSpace(value) == "" {
		return "", errors.New("invalid signed cursor input")
	}
	payload := base64.RawURLEncoding.EncodeToString([]byte(value))
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(payload))
	return payload + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func DecodeCursor(key []byte, cursor string) (string, error) {
	parts := strings.Split(cursor, ".")
	if len(key) < 32 || len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", errors.New("invalid signed cursor")
	}
	provided, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", errors.New("invalid signed cursor")
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(parts[0]))
	if !hmac.Equal(provided, mac.Sum(nil)) {
		return "", errors.New("invalid signed cursor")
	}
	value, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil || strings.TrimSpace(string(value)) == "" {
		return "", errors.New("invalid signed cursor")
	}
	return string(value), nil
}
