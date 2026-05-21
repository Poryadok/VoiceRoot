package main

import (
	"net/http"
	"strings"

	voicejwt "voice/backend/pkg/jwt"
)

func (g *gateway) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	if !isWebSocketUpgrade(r) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "websocket_upgrade_required"})
		return
	}
	prepareWebSocketUpstreamAuth(r)
	claims, code := g.authenticate(r)
	if code != "" {
		status := http.StatusUnauthorized
		if code == "auth_unavailable" {
			status = http.StatusServiceUnavailable
		}
		writeJSON(w, status, map[string]string{"error": code})
		return
	}
	applyClaims(r, claims)
	if g.config.realtimeUpstream == nil {
		http.NotFound(w, r)
		return
	}
	g.config.realtimeUpstream.ServeHTTP(w, r)
}

// prepareWebSocketUpstreamAuth copies access_token query into Authorization for Realtime upstream.
func prepareWebSocketUpstreamAuth(r *http.Request) {
	token := voicejwt.BearerToken(r)
	if token == "" {
		return
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(r.Header.Get("Authorization"), prefix) {
		r.Header.Set("Authorization", prefix+token)
	}
}

func isWebSocketUpgrade(r *http.Request) bool {
	if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		return false
	}
	for _, value := range strings.Split(r.Header.Get("Connection"), ",") {
		if strings.EqualFold(strings.TrimSpace(value), "upgrade") {
			return true
		}
	}
	return false
}
