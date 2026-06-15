package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	headerSignature = "X-Voice-Signature"
	headerTimestamp = "X-Voice-Timestamp"
	maxSkew         = 5 * time.Minute
)

type interactionPayload struct {
	Type             string         `json:"type"`
	InteractionToken string         `json:"interaction_token"`
	CommandName      string         `json:"command_name"`
	Options          map[string]any `json:"options"`
	ChatID           string         `json:"chat_id"`
	ChatType         string         `json:"chat_type"`
	InvokerProfileID string         `json:"invoker_profile_id"`
}

// NewWebhookHandler serves signed slash interactions (webhook mode).
func NewWebhookHandler(secret string, ephemeral bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if !verifySignature(secret, r.Header.Get(headerSignature), r.Header.Get(headerTimestamp), body, time.Now()) {
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}
		var payload interactionPayload
		if err := json.Unmarshal(body, &payload); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"content":   "pong",
			"ephemeral": ephemeral,
		})
	})
}

func verifySignature(secret, signatureHeader, timestampHeader string, body []byte, now time.Time) bool {
	sig := strings.TrimSpace(signatureHeader)
	if !strings.HasPrefix(sig, "v1=") {
		return false
	}
	gotHex := strings.TrimPrefix(sig, "v1=")
	ts, err := strconv.ParseInt(strings.TrimSpace(timestampHeader), 10, 64)
	if err != nil {
		return false
	}
	eventTime := time.Unix(ts, 0)
	if now.Sub(eventTime) > maxSkew || eventTime.Sub(now) > maxSkew {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(fmt.Sprintf("%d.", ts)))
	_, _ = mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(gotHex), []byte(expected))
}
