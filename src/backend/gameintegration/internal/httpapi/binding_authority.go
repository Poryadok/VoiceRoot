package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"voice/backend/gameintegration/internal/registry"
)

type PlayerBindingAuthorityStore interface {
	LoadPlayerBindingAuthority(context.Context, uuid.UUID) (registry.PlayerBindingAuthority, error)
}

type InternalBindingAuthorityHandler struct {
	Verifier WorkloadVerifier
	Store    PlayerBindingAuthorityStore
}

func NewInternalBindingAuthorityHandler(verifier WorkloadVerifier, store PlayerBindingAuthorityStore) *InternalBindingAuthorityHandler {
	return &InternalBindingAuthorityHandler{Verifier: verifier, Store: store}
}

func (h *InternalBindingAuthorityHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	const prefix = "/internal/v1/bindings/"
	if h == nil || h.Store == nil {
		writeError(w, http.StatusServiceUnavailable, "BINDING_AUTHORITY_UNAVAILABLE")
		return
	}
	if r.Method != http.MethodGet || !strings.HasPrefix(r.URL.Path, prefix) || !strings.HasSuffix(r.URL.Path, "/authority") {
		http.NotFound(w, r)
		return
	}
	if err := h.Verifier.Verify(r); err != nil {
		if errors.Is(err, ErrWorkloadUnavailable) {
			writeError(w, http.StatusServiceUnavailable, "AUTHORITY_UNAVAILABLE")
			return
		}
		writeError(w, http.StatusUnauthorized, "INVALID_WORKLOAD_PROOF")
		return
	}
	value := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, prefix), "/authority")
	id, err := uuid.Parse(value)
	if err != nil || id == uuid.Nil || id.String() != value {
		http.NotFound(w, r)
		return
	}
	authority, err := h.Store.LoadPlayerBindingAuthority(r.Context(), id)
	if err != nil {
		if errors.Is(err, registry.ErrPlayerBindingNotFound) {
			writeError(w, http.StatusNotFound, "BINDING_NOT_FOUND")
		} else {
			writeError(w, http.StatusServiceUnavailable, "BINDING_AUTHORITY_UNAVAILABLE")
		}
		return
	}
	if authority.BindingID != id || authority.ApplicationID == uuid.Nil || authority.EnvironmentID == uuid.Nil ||
		authority.BindingRevision <= 0 || (authority.Status != "active" && authority.Status != "revoking" && authority.Status != "revoked") ||
		len(authority.CharacterContext) == 0 || !json.Valid(authority.CharacterContext) {
		writeError(w, http.StatusServiceUnavailable, "BINDING_AUTHORITY_UNAVAILABLE")
		return
	}
	body, err := json.Marshal(authority)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "BINDING_AUTHORITY_UNAVAILABLE")
		return
	}
	body = append(body, '\n')
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Voice-Response-Timestamp", r.Header.Get("X-Voice-Timestamp"))
	w.Header().Set("X-Voice-Response-Nonce", r.Header.Get("X-Voice-Nonce"))
	w.Header().Set("X-Voice-Response-Signature", responseSignature(h.Verifier.Key, http.StatusOK,
		r.URL.EscapedPath(), r.Header.Get("X-Voice-Timestamp"), r.Header.Get("X-Voice-Nonce"), body))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}
