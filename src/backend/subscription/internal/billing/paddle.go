package billing

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

const defaultWebhookSecret = "test-webhook-secret"

// WebhookSecret returns PADDLE_WEBHOOK_SECRET or the test default.
func WebhookSecret() string {
	if v := strings.TrimSpace(os.Getenv("PADDLE_WEBHOOK_SECRET")); v != "" {
		return v
	}
	return defaultWebhookSecret
}

// PaddleEvent is a minimal Paddle Billing webhook payload.
type PaddleEvent struct {
	EventID   string         `json:"event_id"`
	EventType string         `json:"event_type"`
	Data      PaddleEventData `json:"data"`
}

type PaddleEventData struct {
	Status     string            `json:"status"`
	CustomData map[string]string `json:"custom_data"`
}

// ParseWebhook unmarshals raw webhook JSON.
func ParseWebhook(rawBody string) (*PaddleEvent, error) {
	var ev PaddleEvent
	if err := json.Unmarshal([]byte(rawBody), &ev); err != nil {
		return nil, fmt.Errorf("invalid webhook json: %w", err)
	}
	if strings.TrimSpace(ev.EventID) == "" {
		return nil, fmt.Errorf("missing event_id")
	}
	if strings.TrimSpace(ev.EventType) == "" {
		return nil, fmt.Errorf("missing event_type")
	}
	return &ev, nil
}

// VerifySignature checks Paddle ts/h1 HMAC signature.
func VerifySignature(rawBody, signature string) error {
	ts, h1, err := parseSignatureHeader(signature)
	if err != nil {
		return err
	}
	mac := hmac.New(sha256.New, []byte(WebhookSecret()))
	_, _ = mac.Write([]byte(ts + ":" + rawBody))
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(strings.ToLower(h1)), []byte(strings.ToLower(expected))) {
		return fmt.Errorf("invalid webhook signature")
	}
	return nil
}

func parseSignatureHeader(signature string) (ts, h1 string, err error) {
	for _, part := range strings.Split(signature, ",") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch strings.TrimSpace(kv[0]) {
		case "ts":
			ts = strings.TrimSpace(kv[1])
		case "h1":
			h1 = strings.TrimSpace(kv[1])
		}
	}
	if ts == "" || h1 == "" {
		return "", "", fmt.Errorf("malformed signature header")
	}
	if _, err := strconv.ParseInt(ts, 10, 64); err != nil {
		return "", "", fmt.Errorf("invalid signature timestamp")
	}
	return ts, h1, nil
}

// SignWebhookForTest builds a valid Paddle signature for integration tests.
func SignWebhookForTest(rawBody string) string {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, []byte(WebhookSecret()))
	_, _ = mac.Write([]byte(ts + ":" + rawBody))
	return fmt.Sprintf("ts=%s,h1=%s", ts, hex.EncodeToString(mac.Sum(nil)))
}

// AccountIDFromCustomData parses account_id from webhook custom_data.
func AccountIDFromCustomData(data map[string]string) (uuid.UUID, error) {
	raw := strings.TrimSpace(data["account_id"])
	if raw == "" {
		return uuid.Nil, fmt.Errorf("missing account_id in custom_data")
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid account_id")
	}
	return id, nil
}

// SpaceProFromCustomData parses space_id and purchaser_id for Space Pro activation.
func SpaceProFromCustomData(data map[string]string) (spaceID, purchaserID uuid.UUID, err error) {
	spaceRaw := strings.TrimSpace(data["space_id"])
	purchaserRaw := strings.TrimSpace(data["purchaser_id"])
	if spaceRaw == "" || purchaserRaw == "" {
		return uuid.Nil, uuid.Nil, fmt.Errorf("missing space_id or purchaser_id in custom_data")
	}
	spaceID, err = uuid.Parse(spaceRaw)
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("invalid space_id")
	}
	purchaserID, err = uuid.Parse(purchaserRaw)
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("invalid purchaser_id")
	}
	return spaceID, purchaserID, nil
}
