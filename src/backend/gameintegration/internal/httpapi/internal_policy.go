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

type AuthorizationPolicyStore interface {
	LoadAuthorizationPolicy(context.Context, uuid.UUID) (registry.AuthorizationPolicy, error)
}

type InternalPolicyHandler struct {
	Verifier WorkloadVerifier
	Store    AuthorizationPolicyStore
}

func NewInternalPolicyHandler(verifier WorkloadVerifier, store AuthorizationPolicyStore) *InternalPolicyHandler {
	return &InternalPolicyHandler{Verifier: verifier, Store: store}
}

func (h *InternalPolicyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	const prefix = "/internal/v1/authorizations/environments/"
	if h == nil || h.Store == nil {
		writeError(w, http.StatusServiceUnavailable, "POLICY_UNAVAILABLE")
		return
	}
	if r.Method != http.MethodGet || !strings.HasPrefix(r.URL.Path, prefix) {
		http.NotFound(w, r)
		return
	}
	envID, err := uuid.Parse(strings.TrimPrefix(r.URL.Path, prefix))
	if err != nil || envID == uuid.Nil {
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
	policy, err := h.Store.LoadAuthorizationPolicy(r.Context(), envID)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "POLICY_UNAVAILABLE")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(policy)
}
