package main

import (
	"net/http"
	"strconv"
	"strings"
)

func (g *gateway) handleREST(w http.ResponseWriter, r *http.Request) {
	if blocked, updateURL := g.forceUpdateDecision(r); blocked {
		g.metrics.ObserveForceUpdateBlock(r.Header.Get("X-Voice-Client-Platform"))
		writeJSON(w, http.StatusUpgradeRequired, map[string]string{
			"error":      "client_outdated",
			"update_url": updateURL,
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
	botRoute := isBotTokenRESTRoute(r.URL.Path)
	if !publicRoute {
		if botRoute {
			if botBearerToken(r) == "" {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid_token"})
				return
			}
		} else {
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
			if retryAfter := rateLimitRetryAfterSeconds(group); retryAfter > 0 {
				w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
			}
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
	case "auth", "users", "friends", "chats", "messages", "spaces", "invites", "roles", "voice", "files", "notifications", "search", "matchmaking", "moderation", "subscription", "bots", "stories", "analytics", "links":
		return true
	default:
		return false
	}
}

func isPublicRESTRoute(method, path string) bool {
	if method == http.MethodPost && (path == "/api/v1/auth/login" ||
		path == "/api/v1/auth/register" ||
		path == "/api/v1/auth/refresh") {
		return true
	}
	if method == http.MethodPost && strings.HasPrefix(path, "/api/v1/auth/otp/") {
		return true
	}
	if method == http.MethodPost && strings.HasPrefix(path, "/api/v1/subscription/webhooks/") {
		return true
	}
	if path == "/api/v1/auth/oauth2/authorize" &&
		(method == http.MethodGet || method == http.MethodPost) {
		return true
	}
	if method == http.MethodPost && path == "/api/v1/auth/oauth2/token" {
		return true
	}
	if method == http.MethodGet && path == "/api/v1/auth/.well-known/openid-configuration" {
		return true
	}
	return method == http.MethodGet && path == "/api/v1/version"
}

func isBotTokenRESTRoute(path string) bool {
	return strings.HasPrefix(path, "/api/v1/bots/me/")
}

func publicRESTNamespaces() []string {
	return []string{"auth", "users", "friends", "chats", "messages", "spaces", "invites", "roles", "voice", "files", "notifications", "search", "matchmaking", "moderation", "subscription", "bots", "stories", "analytics", "links"}
}
