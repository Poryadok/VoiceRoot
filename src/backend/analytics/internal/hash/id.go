package hash

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// ID returns HMAC-SHA256 hex of raw using key (no PII in analytics store).
func ID(key, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(key))
	_, _ = mac.Write([]byte(raw))
	return hex.EncodeToString(mac.Sum(nil))
}
