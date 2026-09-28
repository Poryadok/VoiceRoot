package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"voice/backend/gameintegration/internal/registry"
)

type BindingChallengeStore interface {
	LoadBindingChallenge(context.Context, uuid.UUID) (registry.BindingChallenge, error)
}

type BindingChallengeCreator interface {
	CreateBindingChallengeForAuth(context.Context, registry.BindingChallengeCreate, string, uuid.UUID, string) (registry.BindingChallenge, error)
}

// InternalBindingChallengeCreateHandler is the only challenge writer. It is
// private to Auth's signed WorkloadProof principal; GIS allocates ID and nonce.
type InternalBindingChallengeCreateHandler struct {
	Verifier WorkloadVerifier
	Store    BindingChallengeCreator
}

func (h *InternalBindingChallengeCreateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Store == nil {
		writeError(w, http.StatusServiceUnavailable, "BINDING_CHALLENGE_UNAVAILABLE")
		return
	}
	if r.URL.Path != "/internal/v1/bindings/challenges" {
		http.NotFound(w, r)
		return
	}
	body, err := h.Verifier.VerifyBody(r)
	if err != nil {
		if errors.Is(err, ErrWorkloadUnavailable) {
			writeError(w, http.StatusServiceUnavailable, "BINDING_CHALLENGE_UNAVAILABLE")
		} else {
			writeError(w, http.StatusUnauthorized, "INVALID_WORKLOAD_PROOF")
		}
		return
	}
	var request registry.BindingChallengeCreate
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_BINDING_CHALLENGE")
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		writeError(w, http.StatusBadRequest, "INVALID_BINDING_CHALLENGE")
		return
	}
	id, err := uuid.NewRandom()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "BINDING_CHALLENGE_UNAVAILABLE")
		return
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		writeError(w, http.StatusServiceUnavailable, "BINDING_CHALLENGE_UNAVAILABLE")
		return
	}
	requestHashBytes := sha256.Sum256(body)
	challenge, err := h.Store.CreateBindingChallengeForAuth(r.Context(), request, hex.EncodeToString(requestHashBytes[:]),
		id, base64.RawURLEncoding.EncodeToString(secret))
	if err != nil {
		if errors.Is(err, registry.ErrInvalidBindingChallenge) {
			writeError(w, http.StatusConflict, "BINDING_CHALLENGE_CONFLICT")
		} else {
			writeError(w, http.StatusServiceUnavailable, "BINDING_CHALLENGE_UNAVAILABLE")
		}
		return
	}
	writeSignedChallenge(w, r, h.Verifier, challenge)
}

type InternalBindingChallengeHandler struct {
	Verifier WorkloadVerifier
	Store    BindingChallengeStore
}

func NewInternalBindingChallengeHandler(verifier WorkloadVerifier, store BindingChallengeStore) *InternalBindingChallengeHandler {
	return &InternalBindingChallengeHandler{Verifier: verifier, Store: store}
}

func (h *InternalBindingChallengeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	const prefix = "/internal/v1/bindings/challenges/"
	if h == nil || h.Store == nil {
		writeError(w, http.StatusServiceUnavailable, "BINDING_CHALLENGE_UNAVAILABLE")
		return
	}
	if r.Method != http.MethodGet || !strings.HasPrefix(r.URL.Path, prefix) {
		http.NotFound(w, r)
		return
	}
	if err := h.Verifier.Verify(r); err != nil {
		if errors.Is(err, ErrWorkloadUnavailable) {
			writeError(w, http.StatusServiceUnavailable, "BINDING_CHALLENGE_UNAVAILABLE")
		} else {
			writeError(w, http.StatusUnauthorized, "INVALID_WORKLOAD_PROOF")
		}
		return
	}
	value := strings.TrimPrefix(r.URL.Path, prefix)
	id, err := uuid.Parse(value)
	if err != nil || id == uuid.Nil || id.String() != value {
		http.NotFound(w, r)
		return
	}
	challenge, err := h.Store.LoadBindingChallenge(r.Context(), id)
	if err != nil {
		if errors.Is(err, registry.ErrBindingChallengeNotFound) {
			writeError(w, http.StatusNotFound, "BINDING_CHALLENGE_NOT_FOUND")
		} else {
			writeError(w, http.StatusServiceUnavailable, "BINDING_CHALLENGE_UNAVAILABLE")
		}
		return
	}
	if challenge.ChallengeID != id || challenge.ApplicationID == uuid.Nil || challenge.EnvironmentID == uuid.Nil ||
		challenge.DeviceKeyID == uuid.Nil || challenge.OperationID == uuid.Nil || challenge.Status != "pending" ||
		h.Verifier.Now == nil || !challenge.ExpiresAt.After(h.Verifier.Now()) {
		writeError(w, http.StatusServiceUnavailable, "BINDING_CHALLENGE_UNAVAILABLE")
		return
	}
	writeSignedChallenge(w, r, h.Verifier, challenge)
}

func writeSignedChallenge(w http.ResponseWriter, r *http.Request, verifier WorkloadVerifier, challenge registry.BindingChallenge) {
	body, err := json.Marshal(challenge)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "BINDING_CHALLENGE_UNAVAILABLE")
		return
	}
	body = append(body, '\n')
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Voice-Response-Timestamp", r.Header.Get("X-Voice-Timestamp"))
	w.Header().Set("X-Voice-Response-Nonce", r.Header.Get("X-Voice-Nonce"))
	w.Header().Set("X-Voice-Response-Signature", responseSignature(verifier.Key, http.StatusOK,
		r.URL.EscapedPath(), r.Header.Get("X-Voice-Timestamp"), r.Header.Get("X-Voice-Nonce"), body))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}
