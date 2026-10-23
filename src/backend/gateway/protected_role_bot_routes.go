package main

import (
	"net/http"
	"strings"
)

func isBotInstallationRoute(method, rest string) bool {
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	return len(parts) == 4 && method == http.MethodPost && parts[1] == "spaces" && parts[3] == "install" && parts[0] != "" && parts[2] != "" ||
		len(parts) == 3 && method == http.MethodDelete && parts[1] == "spaces" && parts[0] != "" && parts[2] != ""
}

func isProtectedRoleOrBotRequest(r *http.Request) bool {
	if r == nil {
		return false
	}
	if strings.HasPrefix(r.URL.Path, "/api/v1/bots/") {
		return isBotInstallationRoute(r.Method, strings.TrimPrefix(r.URL.Path, "/api/v1/bots/"))
	}
	if r.URL.Path != "/api/v1/roles" && !strings.HasPrefix(r.URL.Path, "/api/v1/roles/") {
		return false
	}
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/roles"), "/")
	switch r.Method {
	case http.MethodGet:
		switch rest {
		case "", "members", "effective", "default-join", "chat-overrides", "voice-overrides":
			return true
		}
	case http.MethodPost:
		switch rest {
		case "", "assign", "revoke", "reorder", "chat-overrides", "voice-overrides":
			return true
		}
	case http.MethodPut:
		return rest == "default-join"
	case http.MethodPatch, http.MethodDelete:
		return rest != "" && !strings.Contains(rest, "/")
	}
	return false
}
