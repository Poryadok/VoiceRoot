package httpapi

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"voice/backend/gameintegration/internal/registry"
)

func (h *Handler) openCommunityOwnerRecovery(w http.ResponseWriter, r *http.Request, applicationID, replacementAccountID, replacementProfileID uuid.UUID) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	if h == nil || h.CommunityRecoveries == nil {
		writeError(w, http.StatusServiceUnavailable, "OWNER_RECOVERY_UNAVAILABLE")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var body struct {
		EnvironmentID      string `json:"environment_id"`
		CorporationKey     string `json:"corporation_key"`
		OperationID        string `json:"operation_id"`
		ExpectedGeneration int64  `json:"expected_generation"`
		ReasonCode         string `json:"reason_code"`
		EvidenceSHA256     string `json:"evidence_sha256"`
	}
	if err := decoder.Decode(&body); err != nil || decoder.Decode(new(any)) != io.EOF {
		writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT")
		return
	}
	environmentID, e1 := uuid.Parse(body.EnvironmentID)
	operationID, e2 := uuid.Parse(body.OperationID)
	evidence, e3 := hex.DecodeString(body.EvidenceSHA256)
	if e1 != nil || e2 != nil || e3 != nil || environmentID == uuid.Nil || operationID == uuid.Nil || strings.TrimSpace(body.CorporationKey) == "" || body.CorporationKey != strings.TrimSpace(body.CorporationKey) || len(evidence) != 32 || body.ExpectedGeneration <= 0 {
		writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT")
		return
	}
	op, err := h.CommunityRecoveries.OpenCommunityOwnerRecovery(r.Context(), registry.OpenOwnerRecoveryInput{OperationID: operationID, ApplicationID: applicationID, EnvironmentID: environmentID, CorporationKey: body.CorporationKey, ReplacementAccountID: replacementAccountID, ReplacementProfileID: replacementProfileID, ExpectedGeneration: body.ExpectedGeneration, ReasonCode: body.ReasonCode, EvidenceSHA256: evidence})
	if err != nil {
		switch {
		case errors.Is(err, registry.ErrOwnerRecoveryConflict), errors.Is(err, registry.ErrIdempotencyConflict):
			writeError(w, http.StatusConflict, "OPERATION_CONFLICT")
		case errors.Is(err, registry.ErrInvalidApplication):
			writeError(w, http.StatusForbidden, "COMMUNITY_RECOVERY_DENIED")
		default:
			writeError(w, http.StatusServiceUnavailable, "REGISTRY_UNAVAILABLE")
		}
		return
	}
	if op.Status == "succeeded" {
		writeOwnerRecoveryResponse(w, http.StatusOK, op)
		return
	}
	if op.Status == "cancelled" {
		writeOwnerRecoveryResponse(w, http.StatusConflict, op)
		return
	}
	if op.Status == "approved" {
		if op.ExpiresAt == nil || !time.Now().Before(*op.ExpiresAt) {
			writeError(w, http.StatusConflict, "APPROVAL_EXPIRED")
			return
		}
		if h.CommunityRecoveryProvisioner == nil {
			writeError(w, http.StatusServiceUnavailable, "SPACE_AUTHORITY_UNAVAILABLE")
			return
		}
		receipt, callErr := h.CommunityRecoveryProvisioner.ProvisionCommunityOwnerRecovery(r.Context(), op)
		if callErr != nil {
			writeError(w, http.StatusServiceUnavailable, "SPACE_AUTHORITY_UNAVAILABLE")
			return
		}
		op, err = h.CommunityRecoveries.CompleteCommunityOwnerRecovery(r.Context(), op.OperationID, replacementAccountID, replacementProfileID, receipt.SpaceID, receipt.Generation)
		if err != nil {
			if errors.Is(err, registry.ErrOwnerRecoveryExpired) {
				writeError(w, http.StatusConflict, "APPROVAL_EXPIRED")
			} else if errors.Is(err, registry.ErrOwnerRecoveryConflict) {
				writeError(w, http.StatusConflict, "OPERATION_CONFLICT")
			} else {
				writeError(w, http.StatusServiceUnavailable, "REGISTRY_UNAVAILABLE")
			}
			return
		}
		writeOwnerRecoveryResponse(w, http.StatusOK, op)
		return
	}
	writeOwnerRecoveryResponse(w, http.StatusAccepted, op)
}

func (h *Handler) approveCommunityOwnerRecovery(w http.ResponseWriter, r *http.Request, applicationID uuid.UUID, operationRaw string, operatorID uuid.UUID) {
	if h == nil || h.CommunityRecoveries == nil {
		writeError(w, http.StatusServiceUnavailable, "OWNER_RECOVERY_UNAVAILABLE")
		return
	}
	operationID, err := uuid.Parse(operationRaw)
	if err != nil || operationID == uuid.Nil {
		http.NotFound(w, r)
		return
	}
	if _, ok := h.OperatorAccounts[operatorID]; !ok {
		writeError(w, http.StatusForbidden, "OPERATOR_REQUIRED")
		return
	}
	if !emptyPostBody(w, r) {
		return
	}
	op, err := h.CommunityRecoveries.ApproveCommunityOwnerRecovery(r.Context(), applicationID, operationID, operatorID)
	if err != nil {
		if errors.Is(err, registry.ErrOwnerRecoveryConflict) {
			writeError(w, http.StatusConflict, "OPERATION_CONFLICT")
		} else {
			writeError(w, http.StatusServiceUnavailable, "REGISTRY_UNAVAILABLE")
		}
		return
	}
	writeOwnerRecoveryResponse(w, http.StatusOK, op)
}

func (h *Handler) cancelCommunityOwnerRecovery(w http.ResponseWriter, r *http.Request, applicationID uuid.UUID, operationRaw string, operatorID uuid.UUID) {
	if h == nil || h.CommunityRecoveries == nil {
		writeError(w, http.StatusServiceUnavailable, "OWNER_RECOVERY_UNAVAILABLE")
		return
	}
	operationID, err := uuid.Parse(operationRaw)
	if err != nil || operationID == uuid.Nil {
		http.NotFound(w, r)
		return
	}
	if _, ok := h.OperatorAccounts[operatorID]; !ok {
		writeError(w, http.StatusForbidden, "OPERATOR_REQUIRED")
		return
	}
	if !emptyPostBody(w, r) {
		return
	}
	op, err := h.CommunityRecoveries.CancelCommunityOwnerRecovery(r.Context(), applicationID, operationID, operatorID)
	if err != nil {
		if errors.Is(err, registry.ErrOwnerRecoveryConflict) {
			writeError(w, http.StatusConflict, "OPERATION_CONFLICT")
		} else {
			writeError(w, http.StatusServiceUnavailable, "REGISTRY_UNAVAILABLE")
		}
		return
	}
	writeOwnerRecoveryResponse(w, http.StatusOK, op)
}

func emptyPostBody(w http.ResponseWriter, r *http.Request) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1)
	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) != 0 {
		writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT")
		return false
	}
	return true
}

func writeOwnerRecoveryResponse(w http.ResponseWriter, status int, op registry.OwnerRecoveryOperation) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	response := map[string]any{"operation_id": op.OperationID.String(), "status": op.Status, "owner_generation": op.ExpectedGeneration}
	if op.ExpiresAt != nil {
		response["expires_at"] = op.ExpiresAt
	}
	if op.SpaceID != nil {
		response["space_id"] = op.SpaceID.String()
	}
	if op.ResultGeneration != nil {
		response["owner_generation"] = *op.ResultGeneration
	}
	_ = json.NewEncoder(w).Encode(response)
}
