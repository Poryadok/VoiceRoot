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

func (h *Handler) serveApplicationSuspension(w http.ResponseWriter, r *http.Request) {
	const prefix = "/api/v1/game-integrations/applications/"
	const suffix = "/suspension"
	if r.Method != http.MethodPut {
		http.NotFound(w, r)
		return
	}
	appID, err := uuid.Parse(strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, prefix), suffix))
	if err != nil || appID == uuid.Nil {
		http.NotFound(w, r)
		return
	}
	if h == nil || h.Tokens == nil || h.Suspensions == nil {
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
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key == "" {
		writeError(w, http.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1024)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var body struct {
		Suspended *bool `json:"suspended"`
	}
	if err := decoder.Decode(&body); err != nil || body.Suspended == nil {
		writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT")
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT")
		return
	}
	application, err := h.Suspensions.SetApplicationSuspension(r.Context(), registry.SetApplicationSuspensionInput{
		ApplicationID: appID, OperatorAccountID: operatorID, Suspended: *body.Suspended, IdempotencyKey: key,
	})
	if err != nil {
		switch {
		case errors.Is(err, registry.ErrInvalidApplication):
			writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT")
		case errors.Is(err, registry.ErrSelfApproval):
			writeError(w, http.StatusForbidden, "OPERATOR_REQUIRED")
		case errors.Is(err, registry.ErrApplicationStateConflict):
			writeError(w, http.StatusConflict, "APPLICATION_STATE_CONFLICT")
		case errors.Is(err, registry.ErrAdmissionConflict):
			writeError(w, http.StatusNotFound, "APPLICATION_NOT_FOUND")
		case errors.Is(err, registry.ErrIdempotencyConflict):
			writeError(w, http.StatusConflict, "IDEMPOTENCY_CONFLICT")
		default:
			writeError(w, http.StatusServiceUnavailable, "REGISTRY_UNAVAILABLE")
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"application_id": application.ID.String(), "status": application.Status,
		"revision": application.Revision,
	})
}
