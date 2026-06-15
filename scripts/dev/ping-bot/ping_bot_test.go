package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestVerifySignature_acceptsValidHMAC(t *testing.T) {
	secret := "whsec_test"
	body := []byte(`{"type":"slash_command","command_name":"ping"}`)
	ts := time.Now().Unix()
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(fmt.Sprintf("%d.", ts)))
	_, _ = mac.Write(body)
	sig := "v1=" + hex.EncodeToString(mac.Sum(nil))

	ok := verifySignature(secret, sig, fmt.Sprintf("%d", ts), body, time.Now())
	require.True(t, ok)
}

func TestVerifySignature_rejectsStaleTimestamp(t *testing.T) {
	secret := "whsec_test"
	body := []byte(`{}`)
	ts := time.Now().Add(-10 * time.Minute).Unix()
	sig := "v1=00"
	require.False(t, verifySignature(secret, sig, fmt.Sprintf("%d", ts), body, time.Now()))
}

func TestWebhookHandler_returnsPong(t *testing.T) {
	secret := "whsec_test"
	handler := NewWebhookHandler(secret, true)

	body, err := json.Marshal(map[string]any{
		"type":              "slash_command",
		"interaction_token": "tok-1",
		"command_name":      "ping",
	})
	require.NoError(t, err)
	ts := time.Now().Unix()
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(fmt.Sprintf("%d.", ts)))
	_, _ = mac.Write(body)
	sig := "v1=" + hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	req.Header.Set(headerSignature, sig)
	req.Header.Set(headerTimestamp, fmt.Sprintf("%d", ts))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &parsed))
	require.Equal(t, "pong", parsed["content"])
	require.Equal(t, true, parsed["ephemeral"])
}
