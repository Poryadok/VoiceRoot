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

func (h *Handler) serveSandboxPolicyUpdate(w http.ResponseWriter, r *http.Request) {
	const prefix = "/api/v1/game-integrations/applications/"
	const separator = "/environments/"
	const suffix = "/policy"
	if r.Method != http.MethodPut {
		http.NotFound(w, r)
		return
	}
	parts := strings.Split(strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, prefix), suffix), separator)
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
	if h == nil || h.Tokens == nil || h.Policies == nil {
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
		ExpectedRevision int64    `json:"expected_revision"`
		RedirectURIs     []string `json:"redirect_uris"`
		AllowedOrigins   []string `json:"allowed_origins"`
		Providers        []string `json:"providers"`
		PlayerScopes     []string `json:"player_scopes"`
	}
	if err := decoder.Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT")
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT")
		return
	}
	env, err := h.Policies.UpdateSandboxPolicy(r.Context(), registry.UpdateSandboxPolicyInput{
		OwnerAccountID: ownerID, ApplicationID: appID, EnvironmentID: envID,
		ExpectedRevision: body.ExpectedRevision, RedirectURIs: body.RedirectURIs,
		AllowedOrigins: body.AllowedOrigins, Providers: body.Providers,
		PlayerScopes: body.PlayerScopes, IdempotencyKey: key,
	})
	if err != nil {
		switch {
		case errors.Is(err, registry.ErrInvalidPolicy):
			writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT")
		case errors.Is(err, registry.ErrAdmissionConflict):
			writeError(w, http.StatusForbidden, "ENVIRONMENT_DENIED")
		case errors.Is(err, registry.ErrPolicyConflict), errors.Is(err, registry.ErrIdempotencyConflict):
			writeError(w, http.StatusConflict, "POLICY_CONFLICT")
		default:
			writeError(w, http.StatusServiceUnavailable, "REGISTRY_UNAVAILABLE")
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"environment_id": env.ID.String(), "revision": env.Revision, "status": env.Status,
	})
}
