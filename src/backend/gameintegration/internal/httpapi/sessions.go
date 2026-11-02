package httpapi

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"voice/backend/gameintegration/internal/registry"
)

type SessionCredentialVerifier interface {
	VerifyGameServer(*http.Request) (registry.SessionPrincipal, error)
}
type SessionRouteOrchestrator interface {
	CreateSession(context.Context, registry.SessionPrincipal, registry.CreateSessionInput) (registry.SessionOperation, error)
	GetOperation(context.Context, registry.SessionPrincipal, uuid.UUID) (registry.SessionOperation, error)
	CloseSession(context.Context, registry.SessionPrincipal, uuid.UUID, uuid.UUID) (registry.SessionOperation, error)
}
type SessionRosterUpdateRouteOrchestrator interface {
	UpdateSessionRoster(context.Context, registry.SessionPrincipal, uuid.UUID, registry.SessionRosterUpdateInput) (registry.SessionOperation, error)
}
type SessionEventRouteOrchestrator interface {
	ClaimSessionEvent(context.Context, registry.SessionPrincipal) (*registry.SessionEventClaim, error)
	AckSessionEvent(context.Context, registry.SessionPrincipal, uuid.UUID, uuid.UUID, []byte) error
}
type CommunityRosterRouteOrchestrator interface {
	ApplyCommunityRosterSnapshot(context.Context, registry.SessionPrincipal, registry.CommunityRosterSnapshotInput) (registry.CommunityRosterResult, error)
	GetCommunityRosterSnapshot(context.Context, registry.SessionPrincipal, string) (registry.CommunityRosterSnapshotView, error)
}
type sessionHandler struct {
	verifier     SessionCredentialVerifier
	orchestrator SessionRouteOrchestrator
}

func NewSessionHandler(verifier SessionCredentialVerifier, orchestrator SessionRouteOrchestrator) http.Handler {
	return &sessionHandler{verifier: verifier, orchestrator: orchestrator}
}
func (h *sessionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.verifier == nil || h.orchestrator == nil {
		writeError(w, http.StatusServiceUnavailable, "SESSION_ORCHESTRATION_UNAVAILABLE")
		return
	}
	principal, err := h.verifier.VerifyGameServer(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "INVALID_GAME_SERVER_CREDENTIAL")
		return
	}
	if principal.ApplicationID == uuid.Nil || principal.EnvironmentID == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "INVALID_GAME_SERVER_CREDENTIAL")
		return
	}
	requiredScope := "game.sessions.manage"
	if strings.HasPrefix(r.URL.Path, "/api/v1/community-bindings/") && strings.HasSuffix(r.URL.Path, "/roster") {
		requiredScope = "game.roster.write"
	}
	if !sessionPrincipalHasScope(principal, requiredScope) {
		writeError(w, http.StatusForbidden, "SCOPE_REQUIRED")
		return
	}
	switch {
	case strings.HasPrefix(r.URL.Path, "/api/v1/community-bindings/") && strings.HasSuffix(r.URL.Path, "/roster") && r.Method == http.MethodGet:
		rosters, ok := h.orchestrator.(CommunityRosterRouteOrchestrator)
		if !ok {
			writeError(w, http.StatusServiceUnavailable, "COMMUNITY_ROSTER_UNAVAILABLE")
			return
		}
		corporationKey, valid := communityRosterCorporationKey(r.URL)
		if !valid {
			writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT")
			return
		}
		view, err := rosters.GetCommunityRosterSnapshot(r.Context(), principal, corporationKey)
		if err != nil {
			writeCommunityRosterError(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("ETag", `"roster-`+view.SnapshotSHA256String()+`"`)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(view)
	case strings.HasPrefix(r.URL.Path, "/api/v1/community-bindings/") && strings.HasSuffix(r.URL.Path, "/roster") && r.Method == http.MethodPut:
		rosters, ok := h.orchestrator.(CommunityRosterRouteOrchestrator)
		if !ok {
			writeError(w, http.StatusServiceUnavailable, "COMMUNITY_ROSTER_UNAVAILABLE")
			return
		}
		corporationKey, valid := communityRosterCorporationKey(r.URL)
		if !valid {
			writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT")
			return
		}
		var request communityRosterSnapshotRequest
		if err := decodeStrictSessionJSON(w, r, &request); err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT")
			return
		}
		input, err := request.toInput(corporationKey)
		if err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT")
			return
		}
		result, err := rosters.ApplyCommunityRosterSnapshot(r.Context(), principal, input)
		if err != nil {
			writeCommunityRosterError(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json")
		status := http.StatusAccepted
		if result.Replayed {
			status = http.StatusOK
		}
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(result)
	case r.URL.Path == "/api/v1/session-events/claim" && r.Method == http.MethodPost:
		delivery, ok := h.orchestrator.(SessionEventRouteOrchestrator)
		if !ok {
			writeError(w, http.StatusServiceUnavailable, "SESSION_EVENT_DELIVERY_UNAVAILABLE")
			return
		}
		body, readErr := io.ReadAll(http.MaxBytesReader(w, r.Body, 1))
		if readErr != nil || len(body) != 0 {
			writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT")
			return
		}
		claim, claimErr := delivery.ClaimSessionEvent(r.Context(), principal)
		if claimErr != nil {
			writeSessionEventError(w, claimErr)
			return
		}
		if claim == nil {
			w.Header().Set("Retry-After", "1")
			w.Header().Set("Cache-Control", "no-store")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Encoding", "identity")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Voice-Event-Id", claim.EventID.String())
		w.Header().Set("X-Voice-Payload-SHA256", hex.EncodeToString(claim.PayloadSHA256))
		w.Header().Set("X-Voice-Claim-Lease-Id", claim.LeaseID.String())
		w.Header().Set("X-Voice-Claim-Lease-Expires-At", claim.LeaseExpiresAt.UTC().Format(time.RFC3339Nano))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(claim.PayloadBytes)
	case strings.HasPrefix(r.URL.Path, "/api/v1/session-events/") && strings.HasSuffix(r.URL.Path, "/ack") && r.Method == http.MethodPost:
		delivery, ok := h.orchestrator.(SessionEventRouteOrchestrator)
		if !ok {
			writeError(w, http.StatusServiceUnavailable, "SESSION_EVENT_DELIVERY_UNAVAILABLE")
			return
		}
		raw := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/v1/session-events/"), "/ack")
		eventID, ok := parseCanonicalSessionUUID(raw)
		if !ok {
			writeError(w, http.StatusNotFound, "NOT_FOUND")
			return
		}
		var body struct {
			LeaseID     string `json:"lease_id"`
			PayloadHash string `json:"payload_sha256"`
		}
		if err := decodeStrictSessionJSON(w, r, &body); err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT")
			return
		}
		leaseID, leaseOK := parseCanonicalSessionUUID(body.LeaseID)
		digest, digestErr := hex.DecodeString(body.PayloadHash)
		if !leaseOK || digestErr != nil || len(body.PayloadHash) != 64 || body.PayloadHash != strings.ToLower(body.PayloadHash) || len(digest) != 32 {
			writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT")
			return
		}
		if err := delivery.AckSessionEvent(r.Context(), principal, eventID, leaseID, digest); err != nil {
			writeSessionEventError(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
	case r.URL.Path == "/api/v1/sessions" && r.Method == http.MethodPost:
		var request createSessionRequest
		if err := decodeStrictSessionJSON(w, r, &request); err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT")
			return
		}
		input, err := request.toInput()
		if err != nil || registry.ValidateCreateSessionInput(input) != nil {
			writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT")
			return
		}
		operation, err := h.orchestrator.CreateSession(r.Context(), principal, input)
		if err != nil {
			writeSessionError(w, err)
			return
		}
		writeSessionOperation(w, http.StatusAccepted, operation)
	case strings.HasPrefix(r.URL.Path, "/api/v1/operations/") && r.Method == http.MethodGet:
		id, ok := parseCanonicalSessionUUID(strings.TrimPrefix(r.URL.Path, "/api/v1/operations/"))
		if !ok {
			writeError(w, http.StatusNotFound, "NOT_FOUND")
			return
		}
		operation, err := h.orchestrator.GetOperation(r.Context(), principal, id)
		if err != nil {
			writeSessionError(w, err)
			return
		}
		writeSessionOperation(w, http.StatusOK, operation)
	case strings.HasPrefix(r.URL.Path, "/api/v1/sessions/") && strings.HasSuffix(r.URL.Path, "/roster") && r.Method == http.MethodPut:
		updater, ok := h.orchestrator.(SessionRosterUpdateRouteOrchestrator)
		if !ok {
			writeError(w, http.StatusServiceUnavailable, "SESSION_ROSTER_UPDATE_UNAVAILABLE")
			return
		}
		raw := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/v1/sessions/"), "/roster")
		sessionID, ok := parseCanonicalSessionUUID(raw)
		if !ok {
			writeError(w, http.StatusNotFound, "NOT_FOUND")
			return
		}
		var request sessionRosterUpdateRequest
		if err := decodeStrictSessionJSON(w, r, &request); err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT")
			return
		}
		input, err := request.toInput()
		if err != nil || registry.ValidateSessionRosterUpdateInput(input) != nil {
			writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT")
			return
		}
		operation, err := updater.UpdateSessionRoster(r.Context(), principal, sessionID, input)
		if err != nil {
			writeSessionRosterError(w, err)
			return
		}
		writeSessionOperation(w, http.StatusAccepted, operation)
	case strings.HasPrefix(r.URL.Path, "/api/v1/sessions/") && strings.HasSuffix(r.URL.Path, "/close") && r.Method == http.MethodPost:
		raw := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/v1/sessions/"), "/close")
		sessionID, ok := parseCanonicalSessionUUID(raw)
		if !ok {
			writeError(w, http.StatusNotFound, "NOT_FOUND")
			return
		}
		var body struct {
			OperationID string `json:"operation_id"`
		}
		if err := decodeStrictSessionJSON(w, r, &body); err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT")
			return
		}
		opID, ok := parseCanonicalSessionUUID(body.OperationID)
		if !ok {
			writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT")
			return
		}
		operation, err := h.orchestrator.CloseSession(r.Context(), principal, sessionID, opID)
		if err != nil {
			writeSessionError(w, err)
			return
		}
		writeSessionOperation(w, http.StatusAccepted, operation)
	default:
		http.NotFound(w, r)
	}
}

type createSessionRequest struct {
	OperationID    string   `json:"operation_id"`
	Kind           string   `json:"kind"`
	ExternalKey    string   `json:"external_key"`
	ParentPartyKey string   `json:"parent_party_key,omitempty"`
	DisplayName    string   `json:"display_name,omitempty"`
	RosterRevision int64    `json:"roster_revision"`
	RosterComplete bool     `json:"roster_complete"`
	Members        []string `json:"members"`
	HostProfileID  *string  `json:"host_profile_id,omitempty"`
}

type sessionRosterUpdateRequest struct {
	OperationID    string   `json:"operation_id"`
	RosterRevision int64    `json:"roster_revision"`
	RosterComplete bool     `json:"roster_complete"`
	Members        []string `json:"members"`
	HostProfileID  *string  `json:"host_profile_id,omitempty"`
}

func (r sessionRosterUpdateRequest) toInput() (registry.SessionRosterUpdateInput, error) {
	operationID, ok := parseCanonicalSessionUUID(r.OperationID)
	if !ok || r.Members == nil {
		return registry.SessionRosterUpdateInput{}, errors.New("noncanonical roster update")
	}
	members := make([]uuid.UUID, len(r.Members))
	for i, raw := range r.Members {
		member, ok := parseCanonicalSessionUUID(raw)
		if !ok {
			return registry.SessionRosterUpdateInput{}, errors.New("noncanonical member profile ID")
		}
		members[i] = member
	}
	var hostProfileID *uuid.UUID
	if r.HostProfileID != nil {
		host, ok := parseCanonicalSessionUUID(*r.HostProfileID)
		if !ok {
			return registry.SessionRosterUpdateInput{}, errors.New("noncanonical host profile ID")
		}
		hostProfileID = &host
	}
	return registry.SessionRosterUpdateInput{
		OperationID: operationID, RosterRevision: r.RosterRevision,
		RosterComplete: r.RosterComplete, Members: members, HostProfileID: hostProfileID,
	}, nil
}

func (r createSessionRequest) toInput() (registry.CreateSessionInput, error) {
	operationID, ok := parseCanonicalSessionUUID(r.OperationID)
	if !ok || r.Members == nil {
		return registry.CreateSessionInput{}, errors.New("noncanonical session request")
	}
	members := make([]uuid.UUID, len(r.Members))
	for i, raw := range r.Members {
		member, ok := parseCanonicalSessionUUID(raw)
		if !ok {
			return registry.CreateSessionInput{}, errors.New("noncanonical member profile ID")
		}
		members[i] = member
	}
	var hostProfileID *uuid.UUID
	if r.HostProfileID != nil {
		host, ok := parseCanonicalSessionUUID(*r.HostProfileID)
		if !ok {
			return registry.CreateSessionInput{}, errors.New("noncanonical host profile ID")
		}
		hostProfileID = &host
	}
	return registry.CreateSessionInput{
		OperationID: operationID, Kind: r.Kind, ExternalKey: r.ExternalKey,
		ParentPartyKey: r.ParentPartyKey, DisplayName: r.DisplayName,
		RosterRevision: r.RosterRevision, RosterComplete: r.RosterComplete, Members: members,
		HostProfileID: hostProfileID,
	}, nil
}

func decodeStrictSessionJSON(w http.ResponseWriter, r *http.Request, out any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	if !utf8.Valid(raw) || !bytes.Equal(bytes.TrimSpace(raw), raw) || !json.Valid(raw) {
		return errors.New("invalid JSON")
	}
	if err = rejectDuplicateJSONKeys(raw); err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err = dec.Decode(out); err != nil {
		return err
	}
	if dec.Decode(new(any)) != io.EOF {
		return errors.New("trailing JSON")
	}
	return nil
}
func rejectDuplicateJSONKeys(raw []byte) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	var walk func() error
	walk = func() error {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		d, ok := tok.(json.Delim)
		if !ok {
			return nil
		}
		switch d {
		case '{':
			seen := map[string]bool{}
			for dec.More() {
				k, err := dec.Token()
				if err != nil {
					return err
				}
				key, ok := k.(string)
				if !ok || seen[key] {
					return errors.New("duplicate JSON key")
				}
				seen[key] = true
				if err = walk(); err != nil {
					return err
				}
			}
			_, err = dec.Token()
			return err
		case '[':
			for dec.More() {
				if err := walk(); err != nil {
					return err
				}
			}
			_, err := dec.Token()
			return err
		default:
			return fmt.Errorf("unexpected delimiter")
		}
	}
	return walk()
}
func parseCanonicalSessionUUID(raw string) (uuid.UUID, bool) {
	id, err := uuid.Parse(raw)
	return id, err == nil && id != uuid.Nil && id.String() == raw
}
func writeSessionOperation(w http.ResponseWriter, status int, o registry.SessionOperation) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(o)
}
func writeSessionError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, registry.ErrSessionNotFound):
		writeError(w, http.StatusNotFound, "NOT_FOUND")
	case errors.Is(err, registry.ErrIdempotencyConflict):
		writeError(w, http.StatusConflict, "OPERATION_ID_CONFLICT")
	default:
		writeError(w, http.StatusServiceUnavailable, "SESSION_ORCHESTRATION_UNAVAILABLE")
	}
}

func writeSessionRosterError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, registry.ErrSessionNotFound):
		writeError(w, http.StatusNotFound, "NOT_FOUND")
	case errors.Is(err, registry.ErrIdempotencyConflict):
		writeError(w, http.StatusConflict, "ROSTER_REVISION_CONFLICT")
	case errors.Is(err, registry.ErrRosterUpdatePending):
		writeError(w, http.StatusConflict, "ROSTER_UPDATE_PENDING")
	case errors.Is(err, registry.ErrHostTransferRequiresNextRev):
		writeError(w, http.StatusConflict, "HOST_TRANSFER_REQUIRES_NEXT_REVISION")
	case errors.Is(err, registry.ErrRosterLeaseExpired):
		writeError(w, http.StatusConflict, "SESSION_LEASE_EXPIRED")
	default:
		writeError(w, http.StatusServiceUnavailable, "SESSION_ROSTER_UPDATE_UNAVAILABLE")
	}
}

func writeSessionEventError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, registry.ErrSessionNotFound):
		writeError(w, http.StatusNotFound, "NOT_FOUND")
	case errors.Is(err, registry.ErrIdempotencyConflict):
		writeError(w, http.StatusConflict, "SESSION_EVENT_ACK_CONFLICT")
	default:
		writeError(w, http.StatusServiceUnavailable, "SESSION_EVENT_DELIVERY_UNAVAILABLE")
	}
}
