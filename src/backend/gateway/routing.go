package main

import (
	"net/http"
	"strings"
)

func (g *gateway) handleREST(w http.ResponseWriter, r *http.Request) {
	if g.isForceUpdateBlocked(r) {
		writeJSON(w, http.StatusUpgradeRequired, map[string]string{
			"error":      "client_outdated",
			"update_url": g.config.forceUpdate.UpdateURL,
		})
		return
	}

	namespace := restNamespace(r.URL.Path)
	if !isPublicRESTNamespace(namespace) {
		http.NotFound(w, r)
		return
	}

	var claims tokenClaims
	publicRoute := isPublicRESTRoute(r.Method, r.URL.Path)
	if !publicRoute {
		var code string
		claims, code = g.authenticate(r)
		if code != "" {
			status := http.StatusUnauthorized
			if code == "auth_unavailable" {
				status = http.StatusServiceUnavailable
			}
			writeJSON(w, status, map[string]string{"error": code})
			return
		}
		if namespace == "analytics" && !hasRole(claims, "staff") {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
			return
		}
		applyClaims(r, claims)
	}

	if group := rateLimitGroup(r.Method, r.URL.Path); group != "" {
		key := g.rateLimitKey(r, claims, publicRoute)
		allowed, err := g.rateLimiter.Allow(r.Context(), key, group)
		if err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "rate_limit_unavailable"})
			return
		}
		if !allowed {
			g.metrics.ObserveRateLimitHit(group)
			writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate_limited"})
			return
		}
	}

	if g.config.transcoder != nil && g.config.transcoder.serveNamespace(w, r, namespace) {
		return
	}

	upstream, ok := g.config.restUpstreams[namespace]
	if !ok {
		http.NotFound(w, r)
		return
	}
	upstream.ServeHTTP(w, r)
}

func restNamespace(path string) string {
	rest := strings.TrimPrefix(path, "/api/v1/")
	if rest == "" {
		return ""
	}
	namespace, _, _ := strings.Cut(rest, "/")
	return namespace
}

func isPublicRESTNamespace(namespace string) bool {
	switch namespace {
	case "auth", "users", "friends", "chats", "messages", "spaces", "roles", "voice", "files", "notifications", "search", "matchmaking", "moderation", "subscription", "bots", "stories", "analytics":
		return true
	default:
		return false
	}
}

func isPublicRESTRoute(method, path string) bool {
	if method == http.MethodPost && (path == "/api/v1/auth/login" || path == "/api/v1/auth/register") {
		return true
	}
	if method == http.MethodPost && strings.HasPrefix(path, "/api/v1/auth/otp/") {
		return true
	}
	return method == http.MethodGet && path == "/api/v1/version"
}

func publicRESTNamespaces() []string {
	return []string{"auth", "users", "friends", "chats", "messages", "spaces", "roles", "voice", "files", "notifications", "search", "matchmaking", "moderation", "subscription", "bots", "stories", "analytics"}
}
