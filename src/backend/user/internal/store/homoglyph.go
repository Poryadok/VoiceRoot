package store

import "voice/backend/pkg/searchnormalization"

// NormalizeUsernameKey applies NFKC and homoglyph folding for uniqueness/spoof checks.
func NormalizeUsernameKey(username string) string {
	return searchnormalization.V1.Normalize(username)
}
