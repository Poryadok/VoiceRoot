package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"voice/backend/gameintegration/internal/registry"
)

func (h *Handler) serveOwnerDiagnostics(w http.ResponseWriter, r *http.Request) {
	const prefix = "/api/v1/game-integrations/applications/"
	const suffix = "/diagnostics"
	if r.Method != http.MethodGet {
		http.NotFound(w, r)
		return
	}
	appID, err := uuid.Parse(strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, prefix), suffix))
	if err != nil || appID == uuid.Nil {
		http.NotFound(w, r)
		return
	}
	if h == nil || h.Tokens == nil || h.Diagnostics == nil {
		writeError(w, http.StatusServiceUnavailable, "REGISTRY_UNAVAILABLE")
		return
	}
	claims, code := h.Tokens.Validate(r)
	if code != "" {
		if code == "auth_unavailable" {
			writeError(w, http.StatusServiceUnavailable, "AUTHORITY_UNAVAILABLE")
			return
		}
		writeError(w, http.StatusUnauthorized, "INVALID_TOKEN")
		return
	}
	ownerID, err := uuid.Parse(claims.UserID)
	if err != nil || ownerID == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "INVALID_TOKEN")
		return
	}
	if claims.AccountType != "regular" {
		writeError(w, http.StatusForbidden, "ACCOUNT_TYPE_DENIED")
		return
	}
	diagnostics, err := h.Diagnostics.LoadOwnerDiagnostics(r.Context(), ownerID, appID)
	if err != nil {
		if errors.Is(err, registry.ErrDiagnosticsDenied) {
			writeError(w, http.StatusForbidden, "DIAGNOSTICS_DENIED")
			return
		}
		writeError(w, http.StatusServiceUnavailable, "REGISTRY_UNAVAILABLE")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(diagnostics)
}
