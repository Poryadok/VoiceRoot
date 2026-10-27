package main

import (
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// linkedOAuthRoute recognizes only the frozen linked-provider transport. Auth
// owns attempt/session binding, eligibility and the default-off activation gate.
func linkedOAuthRoute(method, path string) (public bool, matched bool) {
	const prefix = "/api/v1/auth/linked-accounts/"
	if !strings.HasPrefix(path, prefix) {
		return false, false
	}
	parts := strings.Split(strings.TrimPrefix(path, prefix), "/")
	if len(parts) < 2 || (parts[0] != "twitch" && parts[0] != "youtube") {
		return false, false
	}
	if len(parts) == 2 && method == http.MethodGet && parts[1] == "oauth-callback" {
		return true, true
	}
	if method != http.MethodPost {
		return false, false
	}
	if len(parts) == 2 && parts[1] == "link" ||
		len(parts) == 3 && parts[1] == "results" && parts[2] == "consume" {
		return false, true
	}
	if len(parts) == 4 && parts[1] == "attempts" && parts[3] == "cancel" {
		_, err := uuid.Parse(parts[2])
		return false, len(parts[2]) == 36 && err == nil
	}
	return false, false
}
