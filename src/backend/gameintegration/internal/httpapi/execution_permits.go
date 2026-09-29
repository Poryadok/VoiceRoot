package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/google/uuid"
	"voice/backend/gameintegration/internal/registry"
)

type PlayerBindingExecutionPermitStore interface {
	IssuePlayerBindingExecutionPermit(context.Context, uuid.UUID, uuid.UUID, registry.GameDeviceAuthorityClaims) (registry.PlayerBindingExecutionPermit, error)
}

type PlayerBindingExecutionPermitCompleter interface {
	CompletePlayerBindingExecutionPermit(context.Context, uuid.UUID, uuid.UUID, string) (registry.PlayerBindingPermitCompletion, error)
}

type InternalExecutionPermitHandler struct {
	Verifier  WorkloadVerifier
	Issuer    PlayerBindingExecutionPermitStore
	Completer PlayerBindingExecutionPermitCompleter
}

func NewInternalExecutionPermitHandler(verifier WorkloadVerifier, issuer PlayerBindingExecutionPermitStore,
	completer PlayerBindingExecutionPermitCompleter) *InternalExecutionPermitHandler {
	return &InternalExecutionPermitHandler{Verifier: verifier, Issuer: issuer, Completer: completer}
}

func (h *InternalExecutionPermitHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/internal/v1/game-integrations/bindings/") {
		h.issue(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/internal/v1/game-integrations/execution-permits/") {
		h.complete(w, r)
		return
	}
	http.NotFound(w, r)
}

func (h *InternalExecutionPermitHandler) issue(w http.ResponseWriter, r *http.Request) {
	const prefix = "/internal/v1/game-integrations/bindings/"
	const suffix = "/execution-permits"
	if r.Method != http.MethodPost || !strings.HasPrefix(r.URL.Path, prefix) || !strings.HasSuffix(r.URL.Path, suffix) {
		http.NotFound(w, r)
		return
	}
	if h == nil || h.Issuer == nil {
		writeError(w, http.StatusServiceUnavailable, "PERMIT_UNAVAILABLE")
		return
	}
	assertion, err := h.Verifier.VerifyAssertionBound(r)
	if err != nil {
		if os.Getenv("T16_ACCEPTANCE_DIAGNOSTICS") == "1" {
			slog.Info("T16 execution permit workload proof rejected", "stage", workloadProofFailureStage(err))
		}
		if errors.Is(err, ErrWorkloadUnavailable) {
			writeError(w, http.StatusServiceUnavailable, "AUTHORITY_UNAVAILABLE")
		} else {
			writeError(w, http.StatusUnauthorized, "INVALID_WORKLOAD_PROOF")
		}
		return
	}
	operationID, body, err := parseOperationBody(r.Body, false)
	if err != nil {
		signedError(w, r, h.Verifier.Key, http.StatusBadRequest, "INVALID_ARGUMENT")
		return
	}
	_ = body
	claims, err := parseForwardedDeviceAuthority(assertion)
	if err != nil {
		signedError(w, r, h.Verifier.Key, http.StatusUnauthorized, "INVALID_DEVICE_AUTHORITY")
		return
	}
	value := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, prefix), suffix)
	bindingID, err := canonicalPathUUID(value)
	if err != nil || bindingID != claims.BindingID {
		signedError(w, r, h.Verifier.Key, http.StatusForbidden, "PERMIT_DENIED")
		return
	}
	permit, err := h.Issuer.IssuePlayerBindingExecutionPermit(r.Context(), bindingID, operationID, claims)
	if err != nil {
		switch {
		case errors.Is(err, registry.ErrPlayerBindingNotFound):
			signedError(w, r, h.Verifier.Key, http.StatusNotFound, "BINDING_NOT_FOUND")
		case errors.Is(err, registry.ErrPlayerBindingConflict):
			signedError(w, r, h.Verifier.Key, http.StatusConflict, "PERMIT_CONFLICT")
		case errors.Is(err, registry.ErrPlayerBindingPermitDenied), errors.Is(err, registry.ErrInvalidPlayerBinding):
			signedError(w, r, h.Verifier.Key, http.StatusForbidden, "PERMIT_DENIED")
		default:
			signedError(w, r, h.Verifier.Key, http.StatusServiceUnavailable, "PERMIT_UNAVAILABLE")
		}
		return
	}
	writeSignedJSON(w, r, h.Verifier.Key, http.StatusOK, permit)
}

func (h *InternalExecutionPermitHandler) complete(w http.ResponseWriter, r *http.Request) {
	const prefix = "/internal/v1/game-integrations/execution-permits/"
	const suffix = "/completion"
	if r.Method != http.MethodPost || !strings.HasPrefix(r.URL.Path, prefix) || !strings.HasSuffix(r.URL.Path, suffix) {
		http.NotFound(w, r)
		return
	}
	if h == nil || h.Completer == nil {
		writeError(w, http.StatusServiceUnavailable, "PERMIT_UNAVAILABLE")
		return
	}
	body, err := h.Verifier.VerifyBody(r)
	if err != nil {
		if errors.Is(err, ErrWorkloadUnavailable) {
			writeError(w, http.StatusServiceUnavailable, "AUTHORITY_UNAVAILABLE")
		} else {
			writeError(w, http.StatusUnauthorized, "INVALID_WORKLOAD_PROOF")
		}
		return
	}
	operationID, body, err := parseOperationBody(io.NopCloser(bytes.NewReader(body)), true)
	if err != nil {
		signedError(w, r, h.Verifier.Key, http.StatusBadRequest, "INVALID_ARGUMENT")
		return
	}
	var request struct {
		OperationID string `json:"operation_id"`
		Outcome     string `json:"outcome"`
	}
	if err := json.Unmarshal(body, &request); err != nil {
		signedError(w, r, h.Verifier.Key, http.StatusBadRequest, "INVALID_ARGUMENT")
		return
	}
	permitID, err := canonicalPathUUID(strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, prefix), suffix))
	if err != nil {
		signedError(w, r, h.Verifier.Key, http.StatusNotFound, "PERMIT_NOT_FOUND")
		return
	}
	completion, err := h.Completer.CompletePlayerBindingExecutionPermit(r.Context(), permitID, operationID, request.Outcome)
	if err != nil {
		switch {
		case errors.Is(err, registry.ErrPlayerBindingNotFound):
			signedError(w, r, h.Verifier.Key, http.StatusNotFound, "PERMIT_NOT_FOUND")
		case errors.Is(err, registry.ErrPlayerBindingConflict):
			signedError(w, r, h.Verifier.Key, http.StatusConflict, "PERMIT_CONFLICT")
		case errors.Is(err, registry.ErrInvalidPlayerBinding):
			signedError(w, r, h.Verifier.Key, http.StatusBadRequest, "INVALID_ARGUMENT")
		default:
			signedError(w, r, h.Verifier.Key, http.StatusServiceUnavailable, "PERMIT_UNAVAILABLE")
		}
		return
	}
	writeSignedJSON(w, r, h.Verifier.Key, http.StatusOK, completion)
}

func parseOperationBody(body io.Reader, requireOutcome bool) (uuid.UUID, []byte, error) {
	if body == nil {
		return uuid.Nil, nil, errInvalidDeviceAuthority
	}
	raw, err := io.ReadAll(io.LimitReader(body, (2<<10)+1))
	if err != nil || len(raw) > 2<<10 {
		return uuid.Nil, nil, errInvalidDeviceAuthority
	}
	var operationID string
	if requireOutcome {
		var request struct {
			OperationID string `json:"operation_id"`
			Outcome     string `json:"outcome"`
			Unknown     any    `json:"-"`
		}
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil || decoder.Decode(new(any)) != io.EOF ||
			(request.Outcome != "committed" && request.Outcome != "aborted") {
			return uuid.Nil, nil, errInvalidDeviceAuthority
		}
		operationID = request.OperationID
		canonical, _ := json.Marshal(struct {
			OperationID string `json:"operation_id"`
			Outcome     string `json:"outcome"`
		}{request.OperationID, request.Outcome})
		if !bytes.Equal(canonical, raw) {
			return uuid.Nil, nil, errInvalidDeviceAuthority
		}
	} else {
		var request struct {
			OperationID string `json:"operation_id"`
		}
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil || decoder.Decode(new(any)) != io.EOF {
			return uuid.Nil, nil, errInvalidDeviceAuthority
		}
		operationID = request.OperationID
		canonical, _ := json.Marshal(request)
		if !bytes.Equal(canonical, raw) {
			return uuid.Nil, nil, errInvalidDeviceAuthority
		}
	}
	id, err := uuid.Parse(operationID)
	if err != nil || id == uuid.Nil || id.String() != operationID {
		return uuid.Nil, nil, errInvalidDeviceAuthority
	}
	return id, raw, nil
}

func canonicalPathUUID(value string) (uuid.UUID, error) {
	id, err := uuid.Parse(value)
	if err != nil || id == uuid.Nil || id.String() != value {
		return uuid.Nil, errInvalidDeviceAuthority
	}
	return id, nil
}

func signedError(w http.ResponseWriter, r *http.Request, key []byte, status int, code string) {
	body, _ := json.Marshal(map[string]any{"error_code": code, "retryable": status >= 500})
	body = append(body, '\n')
	writeSignedBytes(w, r, key, status, body)
}

func writeSignedJSON(w http.ResponseWriter, r *http.Request, key []byte, status int, value any) {
	body, err := json.Marshal(value)
	if err != nil {
		signedError(w, r, key, http.StatusServiceUnavailable, "PERMIT_UNAVAILABLE")
		return
	}
	body = append(body, '\n')
	writeSignedBytes(w, r, key, status, body)
}

func writeSignedBytes(w http.ResponseWriter, r *http.Request, key []byte, status int, body []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Voice-Response-Timestamp", r.Header.Get("X-Voice-Timestamp"))
	w.Header().Set("X-Voice-Response-Nonce", r.Header.Get("X-Voice-Nonce"))
	w.Header().Set("X-Voice-Response-Signature", responseSignature(key, status, r.URL.EscapedPath(),
		r.Header.Get("X-Voice-Timestamp"), r.Header.Get("X-Voice-Nonce"), body))
	w.WriteHeader(status)
	_, _ = w.Write(body)
}
