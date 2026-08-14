package main

import (
	"context"
	"net/http"
	"strings"

	voicejwt "voice/backend/pkg/jwt"
)

func (g *gateway) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	if blocked, updateURL := g.forceUpdateDecision(r); blocked {
		g.metrics.ObserveForceUpdateBlock(r.Header.Get("X-Voice-Client-Platform"))
		writeJSON(w, http.StatusUpgradeRequired, map[string]string{
			"error":      "client_outdated",
			"update_url": updateURL,
		})
		return
	}
	if !isWebSocketUpgrade(r) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "websocket_upgrade_required"})
		return
	}

	var claims tokenClaims
	var upstreamToken string
	if ticket := strings.TrimSpace(r.URL.Query().Get("ticket")); ticket != "" {
		record, ok, err := g.consumeWsTicket(r.Context(), ticket)
		if err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "ws_ticket_unavailable"})
			return
		}
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid_ticket"})
			return
		}
		claims = record.claims()
		upstreamToken = record.UpstreamToken
	} else {
		prepareWebSocketUpstreamAuth(r)
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
		upstreamToken = voicejwt.BearerToken(r)
	}
	setWebSocketUpstreamAuth(r, upstreamToken)
	applyClaims(r, claims)
	if g.config.realtimeUpstream == nil {
		http.NotFound(w, r)
		return
	}
	g.config.realtimeUpstream.ServeHTTP(w, r)
}

func (g *gateway) consumeWsTicket(ctx context.Context, ticket string) (wsTicketRecord, bool, error) {
	if g.config.wsTicketStore == nil {
		return wsTicketRecord{}, false, nil
	}
	return g.config.wsTicketStore.Consume(ctx, ticket)
}

// prepareWebSocketUpstreamAuth copies access_token query into Authorization for Realtime upstream.
// Legacy web path — prefer short-lived tickets from POST /api/v1/realtime/ws-ticket.
func prepareWebSocketUpstreamAuth(r *http.Request) {
	setWebSocketUpstreamAuth(r, voicejwt.BearerToken(r))
}

func setWebSocketUpstreamAuth(r *http.Request, token string) {
	token = strings.TrimSpace(token)
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
