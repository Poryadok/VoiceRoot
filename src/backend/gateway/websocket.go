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
	claims, ok := g.authenticate(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
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
