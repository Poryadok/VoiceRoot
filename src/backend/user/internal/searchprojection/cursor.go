// Package searchprojection owns the protected Search bootstrap cursor format.
package searchprojection

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const cursorKeyEnv = "USER_SEARCH_PROJECTION_CURSOR_HMAC_KEY"

const snapshotCursorTTL = 15 * time.Minute

// SnapshotCursor binds pagination to an immutable journal high watermark.
// Versioning and expiry prevent a cursor from being repurposed for a different
// snapshot protocol or retained indefinitely.
type SnapshotCursor struct {
	Version    int    `json:"v"`
	High       uint64 `json:"h"`
	LastOffset uint64 `json:"o"`
	ExpiresAt  int64  `json:"e"`
}

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

func EncodeSnapshotCursor(key []byte, high, lastOffset uint64, now time.Time) (string, error) {
	if high == 0 || lastOffset == 0 {
		return "", errors.New("invalid snapshot cursor input")
	}
	payload, err := json.Marshal(SnapshotCursor{Version: 1, High: high, LastOffset: lastOffset, ExpiresAt: now.Add(snapshotCursorTTL).Unix()})
	if err != nil {
		return "", err
	}
	return EncodeCursor(key, string(payload))
}

func DecodeSnapshotCursor(key []byte, cursor string, now time.Time) (SnapshotCursor, error) {
	raw, err := DecodeCursor(key, cursor)
	if err != nil {
		return SnapshotCursor{}, err
	}
	var value SnapshotCursor
	if err := json.Unmarshal([]byte(raw), &value); err != nil || value.Version != 1 || value.High == 0 || value.LastOffset == 0 || value.ExpiresAt <= now.Unix() {
		return SnapshotCursor{}, errors.New("invalid snapshot cursor")
	}
	return value, nil
}
