package httpapi

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

const messagingServiceURI = "spiffe://voice/service/messaging"

type ResourceMappingAuthorizationStore interface {
	AuthorizeAppBindingChat(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID) (bool, int64, error)
}

type InternalResourceMappingAuthorizationHandler struct {
	Verifier WorkloadVerifier
	Store    ResourceMappingAuthorizationStore
}

func NewInternalResourceMappingAuthorizationHandler(verifier WorkloadVerifier, store ResourceMappingAuthorizationStore) *InternalResourceMappingAuthorizationHandler {
	verifier.Principal = "messaging"
	return &InternalResourceMappingAuthorizationHandler{Verifier: verifier, Store: store}
}

type authorizeResourceMappingRequest struct {
	ApplicationID string `json:"application_id"`
	EnvironmentID string `json:"environment_id"`
	BindingID     string `json:"binding_id"`
	ChatID        string `json:"chat_id"`
}

type authorizeResourceMappingResponse struct {
	Allowed         bool  `json:"allowed"`
	MappingRevision int64 `json:"mapping_revision,omitempty"`
}

func (h *InternalResourceMappingAuthorizationHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	const path = "/internal/v1/game-integrations/resource-mappings/authorize-chat"
	if r.Method != http.MethodPost || r.URL.Path != path {
		http.NotFound(w, r)
		return
	}
	if h == nil || h.Store == nil {
		writeError(w, http.StatusServiceUnavailable, "RESOURCE_MAPPING_UNAVAILABLE")
		return
	}
	if !hasVerifiedMessagingIdentity(r.TLS) {
		writeError(w, http.StatusUnauthorized, "INVALID_SERVICE_IDENTITY")
		return
	}
	body, responseKey, err := h.Verifier.VerifyBodyWithKey(r)
	if err != nil {
		if errors.Is(err, ErrWorkloadUnavailable) {
			writeError(w, http.StatusServiceUnavailable, "AUTHORITY_UNAVAILABLE")
			return
		}
		writeError(w, http.StatusUnauthorized, "INVALID_WORKLOAD_PROOF")
		return
	}
	var input authorizeResourceMappingRequest
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil || decoder.Decode(new(any)) != io.EOF {
		writeError(w, http.StatusBadRequest, "INVALID_RESOURCE_MAPPING_REQUEST")
		return
	}
	applicationID, okApp := parseCanonicalUUID(input.ApplicationID)
	environmentID, okEnv := parseCanonicalUUID(input.EnvironmentID)
	bindingID, okBinding := parseCanonicalUUID(input.BindingID)
	chatID, okChat := parseCanonicalUUID(input.ChatID)
	if !okApp || !okEnv || !okBinding || !okChat {
		writeError(w, http.StatusBadRequest, "INVALID_RESOURCE_MAPPING_REQUEST")
		return
	}
	allowed, revision, err := h.Store.AuthorizeAppBindingChat(r.Context(), applicationID, environmentID, bindingID, chatID)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "RESOURCE_MAPPING_UNAVAILABLE")
		return
	}
	response := authorizeResourceMappingResponse{Allowed: allowed}
	if allowed {
		if revision <= 0 {
			writeError(w, http.StatusServiceUnavailable, "RESOURCE_MAPPING_UNAVAILABLE")
			return
		}
		response.MappingRevision = revision
	}
	responseBody, err := json.Marshal(response)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "RESOURCE_MAPPING_UNAVAILABLE")
		return
	}
	responseBody = append(responseBody, '\n')
	timestamp, nonce := r.Header.Get("X-Voice-Timestamp"), r.Header.Get("X-Voice-Nonce")
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Voice-Response-Timestamp", timestamp)
	w.Header().Set("X-Voice-Response-Nonce", nonce)
	w.Header().Set("X-Voice-Response-Signature", responseSignature(responseKey, http.StatusOK,
		r.URL.EscapedPath(), timestamp, nonce, responseBody))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(responseBody)
}

func parseCanonicalUUID(value string) (uuid.UUID, bool) {
	parsed, err := uuid.Parse(value)
	return parsed, err == nil && parsed != uuid.Nil && parsed.String() == value
}

func hasVerifiedMessagingIdentity(state *tls.ConnectionState) bool {
	if state == nil || len(state.PeerCertificates) == 0 || len(state.VerifiedChains) == 0 {
		return false
	}
	for _, uri := range state.PeerCertificates[0].URIs {
		if uri != nil && uri.String() == messagingServiceURI {
			return true
		}
	}
	return false
}

// MessagingMTLSConfig creates the private listener policy. The caller loads
// its server keypair and client CA into the returned config before serving.
func MessagingMTLSConfig(clientCAs *x509.CertPool) *tls.Config {
	return &tls.Config{MinVersion: tls.VersionTLS12, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: clientCAs}
}
