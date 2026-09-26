package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"voice/backend/gameintegration/internal/registry"
)

func (h *Handler) serveCredentialIssue(w http.ResponseWriter, r *http.Request) {
	const prefix = "/api/v1/game-integrations/applications/"
	const separator = "/environments/"
	const suffix = "/credentials"
	path := strings.TrimPrefix(r.URL.Path, prefix)
	if r.Method != http.MethodPost || !strings.HasSuffix(path, suffix) {
		http.NotFound(w, r)
		return
	}
	parts := strings.Split(strings.TrimSuffix(path, suffix), separator)
	if len(parts) != 2 {
		http.NotFound(w, r)
		return
	}
	appID, appErr := uuid.Parse(parts[0])
	envID, envErr := uuid.Parse(parts[1])
	if appErr != nil || envErr != nil || appID == uuid.Nil || envID == uuid.Nil {
		http.NotFound(w, r)
		return
	}
	if h == nil || h.Tokens == nil || h.Credentials == nil || len(h.CredentialKey) != 32 {
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
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key == "" {
		writeError(w, http.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var body struct {
		Scopes []string `json:"scopes"`
	}
	if err := decoder.Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT")
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT")
		return
	}
	credential, err := h.Credentials.IssueCredential(r.Context(), registry.IssueCredentialInput{
		OwnerAccountID: ownerID, ApplicationID: appID, EnvironmentID: envID,
		Scopes: body.Scopes, IdempotencyKey: key, SecretKey: h.CredentialKey,
	})
	if err != nil {
		switch {
		case errors.Is(err, registry.ErrInvalidCredentialRequest):
			writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT")
		case errors.Is(err, registry.ErrAdmissionConflict):
			writeError(w, http.StatusForbidden, "ENVIRONMENT_DENIED")
		case errors.Is(err, registry.ErrIdempotencyConflict):
			writeError(w, http.StatusConflict, "IDEMPOTENCY_CONFLICT")
		case errors.Is(err, registry.ErrCredentialRevealExpired):
			writeError(w, http.StatusConflict, "CREDENTIAL_REVEAL_EXPIRED")
		default:
			writeError(w, http.StatusServiceUnavailable, "REGISTRY_UNAVAILABLE")
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"credential_id": credential.ID.String(), "environment_id": credential.EnvironmentID.String(),
		"scopes": credential.Scopes, "generation": credential.Generation,
		"secret":     "vgi1_" + credential.ID.String() + "_" + credential.Secret,
		"expires_at": credential.ExpiresAt,
	})
}
