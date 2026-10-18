package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

type AppNotificationChatAuthority interface {
	ResolveAppNotificationChat(context.Context, uuid.UUID, uuid.UUID, string, string) (bool, bool, error)
}

type InternalAppNotificationConsentHandler struct {
	PushVerifier  WorkloadVerifier
	InAppVerifier WorkloadVerifier
	Authority     AppNotificationChatAuthority
}

func (h *InternalAppNotificationConsentHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Authority == nil {
		writeError(w, http.StatusServiceUnavailable, "CONSENT_UNAVAILABLE")
		return
	}
	const prefix = "/internal/v1/notification-consents/"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		http.NotFound(w, r)
		return
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, prefix), "/")
	if len(parts) != 5 || parts[0] != "chat" || r.Method != http.MethodGet || r.URL.RawQuery != "" {
		http.NotFound(w, r)
		return
	}
	profileID, profileErr := uuid.Parse(parts[1])
	chatID, chatErr := uuid.Parse(parts[2])
	if profileErr != nil || chatErr != nil || profileID == uuid.Nil || chatID == uuid.Nil ||
		profileID.String() != parts[1] || chatID.String() != parts[2] || !validAppNotificationCategory(parts[3]) {
		http.NotFound(w, r)
		return
	}
	verifier := h.PushVerifier
	if parts[4] == "in_app" {
		verifier = h.InAppVerifier
	} else if parts[4] != "push" {
		http.NotFound(w, r)
		return
	}
	principal := verifier.Principal
	if principal == "" {
		principal = "auth"
	}
	if err := verifier.VerifyForPrincipal(r, principal); err != nil {
		if errors.Is(err, ErrWorkloadUnavailable) {
			writeError(w, http.StatusServiceUnavailable, "AUTHORITY_UNAVAILABLE")
		} else {
			writeError(w, http.StatusUnauthorized, "INVALID_WORKLOAD_PROOF")
		}
		return
	}
	gameScoped, allowed, err := h.Authority.ResolveAppNotificationChat(r.Context(), profileID, chatID, parts[3], parts[4])
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "CONSENT_UNAVAILABLE")
		return
	}
	writeSignedAppNotificationConsent(w, r, verifier, struct {
		GameScoped bool `json:"game_scoped"`
		Allowed    bool `json:"allowed"`
	}{GameScoped: gameScoped, Allowed: gameScoped && allowed})
}

func validAppNotificationCategory(category string) bool {
	switch category {
	case "game_activity", "game_actions", "game_social":
		return true
	default:
		return false
	}
}

func writeSignedAppNotificationConsent(w http.ResponseWriter, r *http.Request, verifier WorkloadVerifier, response any) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	body, err := json.Marshal(response)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "CONSENT_UNAVAILABLE")
		return
	}
	timestamp, nonce := r.Header.Get("X-Voice-Timestamp"), r.Header.Get("X-Voice-Nonce")
	w.Header().Set("X-Voice-Response-Timestamp", timestamp)
	w.Header().Set("X-Voice-Response-Nonce", nonce)
	w.Header().Set("X-Voice-Response-Signature", responseSignature(verifier.Key, http.StatusOK,
		r.URL.EscapedPath(), timestamp, nonce, body))
	_, _ = w.Write(body)
}
