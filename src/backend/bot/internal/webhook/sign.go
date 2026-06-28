package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	HeaderSignature = "X-Voice-Signature"
	HeaderTimestamp = "X-Voice-Timestamp"
	maxSkew         = 5 * time.Minute
)

// Sign produces v1 HMAC-SHA256 signature for outbound webhook payloads.
func Sign(secret string, timestamp int64, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = fmt.Fprintf(mac, "%d.", timestamp)
	_, _ = mac.Write(body)
	return "v1=" + hex.EncodeToString(mac.Sum(nil))
}

// Verify checks bot-side webhook signature and timestamp skew.
func Verify(secret, signatureHeader, timestampHeader string, body []byte, now time.Time) bool {
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
	_, _ = fmt.Fprintf(mac, "%d.", ts)
	_, _ = mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(gotHex), []byte(expected))
}
