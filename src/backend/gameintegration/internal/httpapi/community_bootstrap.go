package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"voice/backend/gameintegration/internal/registry"
)

func (h *Handler) serveCommunityBootstrap(w http.ResponseWriter, r *http.Request) {
	const prefix = "/api/v1/game-integrations/applications/"
	path := strings.TrimPrefix(r.URL.Path, prefix)
	parts := strings.Split(path, "/")
	if len(parts) < 2 || parts[1] != "community-bindings" {
		http.NotFound(w, r)
		return
	}
	applicationID, err := uuid.Parse(parts[0])
	if err != nil || applicationID == uuid.Nil {
		http.NotFound(w, r)
		return
	}
	if h == nil || h.Tokens == nil {
		writeError(w, http.StatusServiceUnavailable, "COMMUNITY_BOOTSTRAP_UNAVAILABLE")
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
	accountID, accountErr := uuid.Parse(claims.UserID)
	profileID, profileErr := uuid.Parse(claims.ProfileID)
	if accountErr != nil || profileErr != nil || accountID == uuid.Nil || profileID == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "INVALID_TOKEN")
		return
	}
	if len(parts) == 2 && parts[1] == "community-bindings" {
		h.openCommunityBootstrap(w, r, applicationID, accountID, profileID)
		return
	}
	if len(parts) == 3 && parts[1] == "community-bindings" && parts[2] == "owner-recovery" {
		h.openCommunityOwnerRecovery(w, r, applicationID, accountID, profileID)
		return
	}
	if len(parts) == 5 && parts[1] == "community-bindings" && parts[2] == "owner-recoveries" && parts[4] == "approve" && r.Method == http.MethodPost {
		h.approveCommunityOwnerRecovery(w, r, applicationID, parts[3], accountID)
		return
	}
	if len(parts) == 5 && parts[1] == "community-bindings" && parts[2] == "owner-recoveries" && parts[4] == "cancel" && r.Method == http.MethodPost {
		h.cancelCommunityOwnerRecovery(w, r, applicationID, parts[3], accountID)
		return
	}
	if len(parts) == 4 && parts[1] == "community-bindings" && parts[3] == "approve" && r.Method == http.MethodPost {
		h.approveCommunityBootstrap(w, r, applicationID, parts[2], accountID)
		return
	}
	http.NotFound(w, r)
}

func (h *Handler) openCommunityBootstrap(w http.ResponseWriter, r *http.Request, applicationID, accountID, profileID uuid.UUID) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	if h == nil || h.CommunityBootstraps == nil {
		writeError(w, http.StatusServiceUnavailable, "COMMUNITY_BOOTSTRAP_UNAVAILABLE")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var body struct {
		EnvironmentID  string `json:"environment_id"`
		CorporationKey string `json:"corporation_key"`
		TemplateID     string `json:"space_template_id"`
		OperationID    string `json:"operation_id"`
	}
	if err := decoder.Decode(&body); err != nil || decoder.Decode(new(any)) != io.EOF {
		writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT")
		return
	}
	environmentID, err1 := uuid.Parse(body.EnvironmentID)
	operationID, err2 := uuid.Parse(body.OperationID)
	if err1 != nil || err2 != nil || environmentID == uuid.Nil || operationID == uuid.Nil ||
		strings.TrimSpace(body.CorporationKey) == "" || body.CorporationKey != strings.TrimSpace(body.CorporationKey) {
		writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT")
		return
	}
	if _, allowed := h.CommunityTemplates[body.TemplateID]; !allowed {
		writeError(w, http.StatusForbidden, "SPACE_TEMPLATE_NOT_ALLOWED")
		return
	}
	op, err := h.CommunityBootstraps.OpenCommunityBootstrap(r.Context(), registry.OpenCommunityBootstrapInput{
		OperationID: operationID, ApplicationID: applicationID, EnvironmentID: environmentID,
		OwnerAccountID: accountID, OwnerProfileID: profileID, CorporationKey: body.CorporationKey, TemplateID: body.TemplateID,
	})
	if err != nil {
		switch {
		case errors.Is(err, registry.ErrCommunityBootstrapConflict), errors.Is(err, registry.ErrIdempotencyConflict):
			writeError(w, http.StatusConflict, "OPERATION_CONFLICT")
		case errors.Is(err, registry.ErrInvalidApplication):
			writeError(w, http.StatusForbidden, "COMMUNITY_OWNER_REQUIRED")
		default:
			writeError(w, http.StatusServiceUnavailable, "REGISTRY_UNAVAILABLE")
		}
		return
	}
	if op.Status == "succeeded" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(communityBootstrapResponse(op))
		return
	}
	if op.Status == "approved" {
		if op.ExpiresAt == nil || !time.Now().Before(*op.ExpiresAt) {
			writeError(w, http.StatusConflict, "APPROVAL_EXPIRED")
			return
		}
		if h.CommunityProvisioner == nil {
			writeError(w, http.StatusServiceUnavailable, "SPACE_AUTHORITY_UNAVAILABLE")
			return
		}
		spaceID, provisionErr := h.CommunityProvisioner.ProvisionCommunityBootstrap(r.Context(), op)
		if provisionErr != nil || spaceID == uuid.Nil {
			writeError(w, http.StatusServiceUnavailable, "SPACE_AUTHORITY_UNAVAILABLE")
			return
		}
		op, err = h.CommunityBootstraps.CompleteCommunityBootstrap(r.Context(), op.OperationID, accountID, spaceID)
		if err != nil {
			switch {
			case errors.Is(err, registry.ErrCommunityBootstrapExpired):
				writeError(w, http.StatusConflict, "APPROVAL_EXPIRED")
			case errors.Is(err, registry.ErrCommunityBootstrapConflict):
				writeError(w, http.StatusConflict, "OPERATION_CONFLICT")
			default:
				writeError(w, http.StatusServiceUnavailable, "REGISTRY_UNAVAILABLE")
			}
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(communityBootstrapResponse(op))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(communityBootstrapResponse(op))
}

func (h *Handler) approveCommunityBootstrap(w http.ResponseWriter, r *http.Request, applicationID uuid.UUID, operationRaw string, operatorID uuid.UUID) {
	if h == nil || h.CommunityBootstraps == nil {
		writeError(w, http.StatusServiceUnavailable, "COMMUNITY_BOOTSTRAP_UNAVAILABLE")
		return
	}
	operationID, err := uuid.Parse(operationRaw)
	if err != nil || operationID == uuid.Nil {
		http.NotFound(w, r)
		return
	}
	if _, allowed := h.OperatorAccounts[operatorID]; !allowed {
		writeError(w, http.StatusForbidden, "OPERATOR_REQUIRED")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1)
	body, bodyErr := io.ReadAll(r.Body)
	if bodyErr != nil || len(body) != 0 {
		writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT")
		return
	}
	op, err := h.CommunityBootstraps.ApproveCommunityBootstrap(r.Context(), applicationID, operationID, operatorID)
	if err != nil {
		switch {
		case errors.Is(err, registry.ErrCommunityBootstrapConflict), errors.Is(err, registry.ErrIdempotencyConflict):
			writeError(w, http.StatusConflict, "OPERATION_CONFLICT")
		case errors.Is(err, registry.ErrInvalidApplication):
			writeError(w, http.StatusForbidden, "OPERATOR_OR_OPERATION_DENIED")
		default:
			writeError(w, http.StatusServiceUnavailable, "REGISTRY_UNAVAILABLE")
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(communityBootstrapResponse(op))
}

func communityBootstrapResponse(op registry.CommunityBootstrapOperation) map[string]any {
	result := map[string]any{"operation_id": op.OperationID.String(), "status": op.Status, "expires_at": op.ExpiresAt}
	if op.SpaceID != nil {
		result["space_id"] = op.SpaceID.String()
	}
	return result
}
