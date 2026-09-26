package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"voice/backend/gameintegration/internal/registry"
)

func (h *Handler) serveCredentialRevoke(w http.ResponseWriter, r *http.Request) {
	const prefix = "/api/v1/game-integrations/applications/"
	const separator = "/environments/"
	const credentialSeparator = "/credentials/"
	if r.Method != http.MethodDelete {
		http.NotFound(w, r)
		return
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, prefix), separator)
	if len(parts) != 2 {
		http.NotFound(w, r)
		return
	}
	ids := strings.Split(parts[1], credentialSeparator)
	if len(ids) != 2 {
		http.NotFound(w, r)
		return
	}
	appID, aerr := uuid.Parse(parts[0])
	envID, eerr := uuid.Parse(ids[0])
	credentialID, cerr := uuid.Parse(ids[1])
	if aerr != nil || eerr != nil || cerr != nil || appID == uuid.Nil || envID == uuid.Nil || credentialID == uuid.Nil {
		http.NotFound(w, r)
		return
	}
	if h == nil || h.Tokens == nil || h.Credentials == nil {
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
	if r.ContentLength != 0 {
		writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT")
		return
	}
	if err := h.Credentials.RevokeCredential(r.Context(), ownerID, appID, envID, credentialID); err != nil {
		if errors.Is(err, registry.ErrAdmissionConflict) {
			writeError(w, http.StatusForbidden, "ENVIRONMENT_DENIED")
			return
		}
		writeError(w, http.StatusServiceUnavailable, "REGISTRY_UNAVAILABLE")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
