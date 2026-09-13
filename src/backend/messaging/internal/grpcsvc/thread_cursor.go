package grpcsvc

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

const (
	threadCursorVersion = "v1"
	threadCursorTTL     = 15 * time.Minute
)

var errInvalidThreadCursor = errors.New("invalid thread cursor")

type threadCursor struct {
	ChatID       uuid.UUID `json:"c"`
	ProfileID    uuid.UUID `json:"p"`
	PageSize     int       `json:"s"`
	LastReplyAt  time.Time `json:"l"`
	LastParentID uuid.UUID `json:"i"`
	CeilingAt    time.Time `json:"a"`
	CeilingID    uuid.UUID `json:"z"`
	CeilingMsgID uuid.UUID `json:"m"`
	ExpiresAt    time.Time `json:"e"`
}

func signThreadCursor(secret []byte, cursor threadCursor) (string, error) {
	payload, err := json.Marshal(cursor)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write(payload)
	return threadCursorVersion + "." + base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func verifyThreadCursor(secret []byte, raw string, now time.Time) (threadCursor, error) {
	var cursor threadCursor
	parts := splitThreadCursor(raw)
	if len(parts) != 3 || parts[0] != threadCursorVersion {
		return cursor, errInvalidThreadCursor
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return cursor, errInvalidThreadCursor
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return cursor, errInvalidThreadCursor
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write(payload)
	if !hmac.Equal(signature, mac.Sum(nil)) || json.Unmarshal(payload, &cursor) != nil || !now.Before(cursor.ExpiresAt) {
		return threadCursor{}, errInvalidThreadCursor
	}
	return cursor, nil
}

func splitThreadCursor(raw string) []string {
	var parts []string
	start := 0
	for i := range raw {
		if raw[i] == '.' {
			parts = append(parts, raw[start:i])
			start = i + 1
		}
	}
	return append(parts, raw[start:])
}
