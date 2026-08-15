package guestprofile

import "strings"

// IsPlaceholderDisplayName reports whether displayName is the auth guest bootstrap
// placeholder (account UUID without dashes, case-insensitive).
func IsPlaceholderDisplayName(accountID, displayName string) bool {
	return normalizeID(accountID) == normalizeID(displayName)
}

func normalizeID(s string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(s)), "-", "")
}
