package hash

import "voice/backend/pkg/analyticshash"

// ID returns HMAC-SHA256 hex of raw using key (no PII in analytics store).
func ID(key, raw string) string {
	return analyticshash.ID(key, raw)
}
