package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"voice/backend/gameintegration/internal/registry"
)

type PlayerSessionSnapshotStore interface {
	GetPlayerSessionSnapshot(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (registry.PlayerSessionSnapshot, error)
}

type PlayerSessionListStore interface {
	ListPlayerSessionSnapshots(context.Context, uuid.UUID, uuid.UUID) ([]registry.PlayerSessionSnapshot, error)
}

type playerSessionSnapshotHandler struct {
	authorizer Authorizer
	store      PlayerSessionSnapshotStore
}

func NewPlayerSessionSnapshotHandler(authorizer Authorizer, store PlayerSessionSnapshotStore) http.Handler {
	return &playerSessionSnapshotHandler{authorizer: authorizer, store: store}
}

func (h *playerSessionSnapshotHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED")
		return
	}
	if h == nil || h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "SESSION_SNAPSHOT_UNAVAILABLE")
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
	accountID, accountErr := uuid.Parse(claims.UserID)
	profileID, profileErr := uuid.Parse(claims.ProfileID)
	if accountErr != nil || profileErr != nil || accountID == uuid.Nil || profileID == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "INVALID_TOKEN")
		return
	}
	const prefix = "/api/v1/sessions/"
	rawSessionID := strings.TrimPrefix(r.URL.Path, prefix)
	sessionID, ok := parseCanonicalSessionUUID(rawSessionID)
	if !strings.HasPrefix(r.URL.Path, prefix) || !ok || strings.Contains(rawSessionID, "/") || r.URL.RawQuery != "" {
		http.NotFound(w, r)
		return
	}
	snapshot, err := h.store.GetPlayerSessionSnapshot(r.Context(), accountID, profileID, sessionID)
	if err != nil {
		if errors.Is(err, registry.ErrPlayerSessionDenied) {
			writeError(w, http.StatusNotFound, "NOT_FOUND")
		} else {
			writeError(w, http.StatusServiceUnavailable, "SESSION_SNAPSHOT_UNAVAILABLE")
		}
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("ETag", `"roster-`+strconv.FormatInt(snapshot.RosterRevision, 10)+`"`)
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(snapshot)
}

type playerSessionListHandler struct {
	authorizer Authorizer
	store      PlayerSessionListStore
}

func NewPlayerSessionListHandler(authorizer Authorizer, store PlayerSessionListStore) http.Handler {
	return &playerSessionListHandler{authorizer: authorizer, store: store}
}

func (h *playerSessionListHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED")
		return
	}
	if h == nil || h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "SESSION_SNAPSHOT_UNAVAILABLE")
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
	accountID, accountErr := uuid.Parse(claims.UserID)
	profileID, profileErr := uuid.Parse(claims.ProfileID)
	if accountErr != nil || profileErr != nil || accountID == uuid.Nil || profileID == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "INVALID_TOKEN")
		return
	}
	if r.URL.Path != "/api/v1/player/sessions/me" || r.URL.RawQuery != "" {
		http.NotFound(w, r)
		return
	}
	sessions, err := h.store.ListPlayerSessionSnapshots(r.Context(), accountID, profileID)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "SESSION_SNAPSHOT_UNAVAILABLE")
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{"sessions": sessions})
}
