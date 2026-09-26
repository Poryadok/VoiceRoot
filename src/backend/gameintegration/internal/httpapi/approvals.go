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

func (h *Handler) serveSandboxApproval(w http.ResponseWriter, r *http.Request) {
	const prefix = "/api/v1/game-integrations/applications/"
	const suffix = "/admissions/sandbox"
	path := strings.TrimPrefix(r.URL.Path, prefix)
	if r.Method != http.MethodPost || !strings.HasSuffix(path, suffix) {
		http.NotFound(w, r)
		return
	}
	appID, err := uuid.Parse(strings.TrimSuffix(path, suffix))
	if err != nil || appID == uuid.Nil {
		http.NotFound(w, r)
		return
	}
	if h == nil || h.Tokens == nil || h.Approvals == nil {
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
	operatorID, err := uuid.Parse(claims.UserID)
	if err != nil || operatorID == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "INVALID_TOKEN")
		return
	}
	if claims.AccountType != "regular" {
		writeError(w, http.StatusForbidden, "ACCOUNT_TYPE_DENIED")
		return
	}
	if _, allowed := h.OperatorAccounts[operatorID]; !allowed {
		writeError(w, http.StatusForbidden, "OPERATOR_REQUIRED")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1)
	if _, err := io.ReadAll(r.Body); err != nil || r.ContentLength != 0 {
		writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT")
		return
	}
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key == "" {
		writeError(w, http.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED")
		return
	}
	env, err := h.Approvals.ApproveSandbox(r.Context(), registry.ApproveSandboxInput{
		ApplicationID: appID, OperatorAccountID: operatorID, IdempotencyKey: key,
	})
	if err != nil {
		switch {
		case errors.Is(err, registry.ErrSelfApproval):
			writeError(w, http.StatusForbidden, "SELF_APPROVAL_DENIED")
		case errors.Is(err, registry.ErrAdmissionConflict), errors.Is(err, registry.ErrIdempotencyConflict):
			writeError(w, http.StatusConflict, "ADMISSION_CONFLICT")
		case errors.Is(err, registry.ErrInvalidApplication):
			writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT")
		default:
			writeError(w, http.StatusServiceUnavailable, "REGISTRY_UNAVAILABLE")
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"environment_id": env.ID.String(), "application_id": env.ApplicationID.String(),
		"kind": env.Kind, "status": env.Status, "revision": env.Revision,
	})
}
