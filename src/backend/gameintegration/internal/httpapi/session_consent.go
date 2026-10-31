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

type SessionConsentStore interface {
	RecordSessionConsent(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID, int64) (registry.SessionConsentReceipt, error)
}

type SessionKeepGroupStore interface {
	KeepSessionGroup(context.Context, registry.SessionKeepGroupInput) (registry.SessionKeepGroupReceipt, error)
}

type sessionConsentHandler struct {
	authorizer Authorizer
	store      SessionConsentStore
}

func NewSessionConsentHandler(authorizer Authorizer, store SessionConsentStore) http.Handler {
	return &sessionConsentHandler{authorizer: authorizer, store: store}
}

type sessionKeepGroupHandler struct {
	authorizer Authorizer
	store      SessionKeepGroupStore
}

func NewSessionKeepGroupHandler(authorizer Authorizer, store SessionKeepGroupStore) http.Handler {
	return &sessionKeepGroupHandler{authorizer: authorizer, store: store}
}

func (h *sessionKeepGroupHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED")
		return
	}
	if h == nil || h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "KEEP_GROUP_UNAVAILABLE")
		return
	}
	claims, code := h.authorizer.Validate(r)
	if code != "" {
		if code == "auth_unavailable" {
			writeError(w, http.StatusServiceUnavailable, "AUTHORITY_UNAVAILABLE")
		} else {
			writeError(w, http.StatusUnauthorized, "INVALID_TOKEN")
		}
		return
	}
	if claims.AccountType == "disabled" {
		writeError(w, http.StatusForbidden, "ACCOUNT_STATE_DENIED")
		return
	}
	accountID, accountErr := uuid.Parse(claims.UserID)
	profileID, profileErr := uuid.Parse(claims.ProfileID)
	if accountErr != nil || profileErr != nil || accountID == uuid.Nil || profileID == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "INVALID_TOKEN")
		return
	}
	const prefix, suffix = "/api/v1/sessions/", "/keep-group"
	if !strings.HasPrefix(r.URL.Path, prefix) || !strings.HasSuffix(r.URL.Path, suffix) {
		http.NotFound(w, r)
		return
	}
	rawSessionID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, prefix), suffix)
	sessionID, ok := parseCanonicalSessionUUID(rawSessionID)
	if !ok || strings.Contains(rawSessionID, "/") {
		http.NotFound(w, r)
		return
	}
	var body struct {
		OperationID    string `json:"operation_id"`
		RosterRevision int64  `json:"roster_revision"`
	}
	if err := decodeStrictSessionJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT")
		return
	}
	operationID, validOperation := parseCanonicalSessionUUID(body.OperationID)
	if !validOperation || body.RosterRevision <= 0 {
		writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT")
		return
	}
	receipt, err := h.store.KeepSessionGroup(r.Context(), registry.SessionKeepGroupInput{
		OperationID: operationID, SessionID: sessionID, AccountID: accountID, ProfileID: profileID, RosterRevision: body.RosterRevision,
	})
	if err != nil {
		switch {
		case errors.Is(err, registry.ErrSessionKeepGroupDenied):
			writeError(w, http.StatusNotFound, "NOT_FOUND")
		case errors.Is(err, registry.ErrSessionKeepGroupConflict), errors.Is(err, registry.ErrIdempotencyConflict):
			writeError(w, http.StatusConflict, "KEEP_GROUP_CONFLICT")
		default:
			writeError(w, http.StatusServiceUnavailable, "KEEP_GROUP_UNAVAILABLE")
		}
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(receipt)
}

func (h *sessionConsentHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED")
		return
	}
	if h == nil || h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "CONSENT_UNAVAILABLE")
		return
	}
	claims, code := h.authorizer.Validate(r)
	if code != "" {
		if code == "auth_unavailable" {
			writeError(w, http.StatusServiceUnavailable, "AUTHORITY_UNAVAILABLE")
		} else {
			writeError(w, http.StatusUnauthorized, "INVALID_TOKEN")
		}
		return
	}
	if claims.AccountType == "disabled" {
		writeError(w, http.StatusForbidden, "ACCOUNT_STATE_DENIED")
		return
	}
	accountID, accountErr := uuid.Parse(claims.UserID)
	profileID, profileErr := uuid.Parse(claims.ProfileID)
	if accountErr != nil || profileErr != nil || accountID == uuid.Nil || profileID == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "INVALID_TOKEN")
		return
	}
	const prefix, suffix = "/api/v1/sessions/", "/consent"
	if !strings.HasPrefix(r.URL.Path, prefix) || !strings.HasSuffix(r.URL.Path, suffix) {
		http.NotFound(w, r)
		return
	}
	rawSessionID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, prefix), suffix)
	sessionID, ok := parseCanonicalSessionUUID(rawSessionID)
	if !ok || rawSessionID == "" || strings.Contains(rawSessionID, "/") {
		http.NotFound(w, r)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	var body struct {
		OperationID    string `json:"operation_id"`
		RosterRevision int64  `json:"roster_revision"`
	}
	if err := decodeStrictSessionJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT")
		return
	}
	operationID, validOperation := parseCanonicalSessionUUID(body.OperationID)
	if !validOperation || body.RosterRevision <= 0 {
		writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT")
		return
	}
	receipt, err := h.store.RecordSessionConsent(r.Context(), accountID, profileID, sessionID, operationID, body.RosterRevision)
	if err != nil {
		switch {
		case errors.Is(err, registry.ErrSessionConsentDenied):
			writeError(w, http.StatusNotFound, "NOT_FOUND")
		case errors.Is(err, registry.ErrSessionConsentConflict), errors.Is(err, registry.ErrIdempotencyConflict):
			writeError(w, http.StatusConflict, "CONSENT_CONFLICT")
		default:
			writeError(w, http.StatusServiceUnavailable, "CONSENT_UNAVAILABLE")
		}
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(receipt)
}

type sessionRouteDispatcher struct {
	playerConsent   http.Handler
	playerKeepGroup http.Handler
	playerSnapshot  http.Handler
	gameServer      http.Handler
}

// NewSessionRoutes separates the authenticated player's consent route from
// game-server routes, which retain their app/environment workload credential.
func NewSessionRoutes(gameServer, playerConsent, playerKeepGroup, playerSnapshot http.Handler) http.Handler {
	return &sessionRouteDispatcher{playerConsent: playerConsent, playerKeepGroup: playerKeepGroup, playerSnapshot: playerSnapshot, gameServer: gameServer}
}

func (d *sessionRouteDispatcher) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if d != nil && d.playerConsent != nil && r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/api/v1/sessions/") && strings.HasSuffix(r.URL.Path, "/consent") {
		d.playerConsent.ServeHTTP(w, r)
		return
	}
	if d != nil && d.playerKeepGroup != nil && r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/api/v1/sessions/") && strings.HasSuffix(r.URL.Path, "/keep-group") {
		d.playerKeepGroup.ServeHTTP(w, r)
		return
	}
	if d != nil && d.playerSnapshot != nil && r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v1/sessions/") &&
		!strings.HasPrefix(r.Header.Get("Authorization"), "Bearer vgi1_") {
		d.playerSnapshot.ServeHTTP(w, r)
		return
	}
	if d == nil || d.gameServer == nil {
		http.NotFound(w, r)
		return
	}
	d.gameServer.ServeHTTP(w, r)
}
