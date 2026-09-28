package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"voice/backend/gameintegration/internal/authbinding"
	"voice/backend/gameintegration/internal/registry"
)

type BindingRevocationStore interface {
	RevokePlayerBindingForOwner(context.Context, uuid.UUID, uuid.UUID, int64, uuid.UUID) (registry.PlayerBindingAuthority, error)
	LoadPlayerBindingOperationID(context.Context, uuid.UUID) (uuid.UUID, error)
}

type BindingRevocationAuth interface {
	Revoke(context.Context, uuid.UUID) (authbinding.RevokeReceipt, error)
}

type BindingRevocationHandler struct {
	Tokens TokenValidator
	Store  BindingRevocationStore
	Auth   BindingRevocationAuth
}

func (h *BindingRevocationHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if h == nil || h.Tokens == nil || h.Store == nil || h.Auth == nil {
		writeError(w, http.StatusServiceUnavailable, "BINDING_REVOCATION_UNAVAILABLE")
		return
	}
	const prefix = "/api/v1/game-integrations/bindings/"
	if r.Method != http.MethodDelete || r.URL.RawQuery != "" || !strings.HasPrefix(r.URL.Path, prefix) {
		http.NotFound(w, r)
		return
	}
	value := strings.TrimPrefix(r.URL.Path, prefix)
	bindingID, err := uuid.Parse(value)
	if err != nil || bindingID == uuid.Nil || bindingID.String() != value {
		http.NotFound(w, r)
		return
	}
	claims, code := h.Tokens.Validate(r)
	if code != "" {
		if code == "auth_unavailable" {
			writeError(w, http.StatusServiceUnavailable, "AUTHORITY_UNAVAILABLE")
		} else {
			writeError(w, http.StatusUnauthorized, "INVALID_TOKEN")
		}
		return
	}
	if claims.AccountType != "regular" {
		writeError(w, http.StatusForbidden, "ACCOUNT_TYPE_DENIED")
		return
	}
	accountID, err := uuid.Parse(claims.UserID)
	if err != nil || accountID == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "INVALID_TOKEN")
		return
	}
	operationID, err := uuid.Parse(r.Header.Get("Idempotency-Key"))
	if err != nil || operationID == uuid.Nil || operationID.String() != r.Header.Get("Idempotency-Key") {
		writeError(w, http.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED")
		return
	}
	if r.Header.Get("Content-Type") != "application/json" || len(r.Header.Values("Content-Type")) != 1 {
		writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 2<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var body struct {
		ExpectedRevision int64 `json:"expected_revision"`
	}
	if err := decoder.Decode(&body); err != nil || decoder.Decode(new(any)) != io.EOF || body.ExpectedRevision <= 0 {
		writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT")
		return
	}
	authority, err := h.Store.RevokePlayerBindingForOwner(r.Context(), bindingID, accountID,
		body.ExpectedRevision, operationID)
	if err != nil {
		switch {
		case err == registry.ErrPlayerBindingNotFound:
			writeError(w, http.StatusNotFound, "BINDING_NOT_FOUND")
		case err == registry.ErrPlayerBindingConflict:
			writeError(w, http.StatusConflict, "BINDING_REVISION_CONFLICT")
		case err == registry.ErrPlayerBindingDrainPending:
			writeError(w, http.StatusServiceUnavailable, "BINDING_REVOCATION_PENDING")
		default:
			writeError(w, http.StatusServiceUnavailable, "BINDING_REVOCATION_UNAVAILABLE")
		}
		return
	}
	grantOperationID, err := h.Store.LoadPlayerBindingOperationID(r.Context(), bindingID)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "BINDING_REVOCATION_UNAVAILABLE")
		return
	}
	receipt, err := h.Auth.Revoke(r.Context(), grantOperationID)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "BINDING_REVOCATION_PENDING")
		return
	}
	if receipt.OperationID != grantOperationID || (receipt.Status != "revoking" && receipt.Status != "revoked") {
		writeError(w, http.StatusServiceUnavailable, "BINDING_REVOCATION_PENDING")
		return
	}
	if receipt.Status != "revoked" {
		writeError(w, http.StatusServiceUnavailable, "BINDING_REVOCATION_PENDING")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{"binding_id": authority.BindingID,
		"status": authority.Status, "binding_revision": authority.BindingRevision})
}
