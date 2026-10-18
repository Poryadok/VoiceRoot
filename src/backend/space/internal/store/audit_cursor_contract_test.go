package store

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"github.com/google/uuid"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAuditCursorRejectsIncompleteOrInvalidLifetime(t *testing.T) {
	key := bytes.Repeat([]byte{0x42}, 32)
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	digest := sha256.Sum256(key)
	for _, tc := range []struct {
		name   string
		change func(map[string]any)
	}{
		{"missing key id", func(p map[string]any) { delete(p, "kid") }},
		{"unknown key id", func(p map[string]any) { p["kid"] = "unknown" }},
		{"missing issue time", func(p map[string]any) { delete(p, "iat") }},
		{"future issue time", func(p map[string]any) { p["iat"] = now.Add(time.Second).UnixNano() }},
		{"extended lifetime", func(p map[string]any) { p["e"] = now.Add(16 * time.Minute).UnixNano() }},
		{"expired", func(p map[string]any) { p["iat"] = now.Add(-15 * time.Minute).UnixNano(); p["e"] = now.UnixNano() }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload := map[string]any{"v": 1, "b": "binding", "kid": base64.RawURLEncoding.EncodeToString(digest[:]), "iat": now.UnixNano(), "e": now.Add(15 * time.Minute).UnixNano()}
			tc.change(payload)
			raw, err := json.Marshal(payload)
			require.NoError(t, err)
			mac := hmac.New(sha256.New, key)
			_, _ = mac.Write(raw)
			cursor := base64.RawURLEncoding.EncodeToString(raw) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
			_, err = decodeSignedAuditCursor(cursor, key, "binding", now)
			require.ErrorIs(t, err, ErrInvalidAuditCursor)
		})
	}
}

func TestAuditCursorRoundTripAndViewerBinding(t *testing.T) {
	key := bytes.Repeat([]byte{0x42}, 32)
	now := time.Now().UTC()
	query := AuditLogQuery{SpaceID: uuid.New(), ViewerProfileID: uuid.New(), PageSize: 1, AccessScope: "space.audit.read.v1"}
	binding, err := auditQueryBindingHash(query)
	require.NoError(t, err)
	payload := signedAuditCursorPayload{Version: 1, KeyID: auditCursorKeyID(key), IssuedAt: now.UnixNano(), ExpiresAt: now.Add(15 * time.Minute).UnixNano(), BindingHash: binding}
	cursor, err := encodeSignedAuditCursor(payload, key)
	require.NoError(t, err)
	decoded, err := decodeSignedAuditCursor(cursor, key, binding, now.Add(14*time.Minute))
	require.NoError(t, err)
	require.Equal(t, payload, decoded)
	query.ViewerProfileID = uuid.New()
	changedBinding, err := auditQueryBindingHash(query)
	require.NoError(t, err)
	_, err = decodeSignedAuditCursor(cursor, key, changedBinding, now)
	require.ErrorIs(t, err, ErrInvalidAuditCursor)
	_, err = decodeSignedAuditCursor(cursor, key, binding, now.Add(15*time.Minute))
	require.ErrorIs(t, err, ErrInvalidAuditCursor)
}
