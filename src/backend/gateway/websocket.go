package main

import (
	"net/http"
	"strings"
)

func (g *gateway) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	if !isWebSocketUpgrade(r) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "websocket_upgrade_required"})
		return
	}
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
