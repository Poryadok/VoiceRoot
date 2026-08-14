package main

import (
	"net/http"

	voicejwt "voice/backend/pkg/jwt"
)

func (g *gateway) handleRealtimeWsTicket(w http.ResponseWriter, r *http.Request) {
	if blocked, updateURL := g.forceUpdateDecision(r); blocked {
		g.metrics.ObserveForceUpdateBlock(r.Header.Get("X-Voice-Client-Platform"))
		writeJSON(w, http.StatusUpgradeRequired, map[string]string{
			"error":      "client_outdated",
			"update_url": updateURL,
		})
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	if g.config.wsTicketStore == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "ws_ticket_unavailable"})
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
	upstreamToken := voicejwt.BearerToken(r)
	if upstreamToken == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid_token"})
		return
	}

	ttl := wsTicketTTLFromEnv()
	record := wsTicketRecord{
		UserID:           claims.UserID,
		ProfileID:        claims.ProfileID,
		Roles:            claims.Roles,
		SubscriptionTier: claims.SubscriptionTier,
		AccountType:      claims.AccountType,
		UpstreamToken:    upstreamToken,
	}
	ticket, err := g.config.wsTicketStore.Issue(r.Context(), record, ttl)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "ws_ticket_unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ticket":               ticket,
		"expires_in_seconds":   int(ttl.Seconds()),
	})
}
