package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"voice/backend/gameintegration/internal/registry"
	voicejwt "voice/backend/pkg/jwt"
)

type TokenValidator interface {
	Validate(*http.Request) (voicejwt.Claims, string)
}

type ApplicationStore interface {
	CreateApplication(context.Context, registry.CreateApplicationInput) (registry.Application, error)
}

type Handler struct {
	Tokens       TokenValidator
	Applications ApplicationStore
}

func NewHandler(tokens TokenValidator, applications ApplicationStore) *Handler {
	return &Handler{Tokens: tokens, Applications: applications}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/v1/game-integrations/applications" || r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	if h == nil || h.Tokens == nil || h.Applications == nil {
		writeError(w, http.StatusServiceUnavailable, "REGISTRY_UNAVAILABLE")
		return
	}
	claims, code := h.Tokens.Validate(r)
	if code != "" {
		if code == "auth_unavailable" {
			writeError(w, http.StatusServiceUnavailable, "AUTHORITY_UNAVAILABLE")
			return
		}
		writeError(w, http.StatusUnauthorized, "INVALID_TOKEN")
		return
	}
	if claims.AccountType != "regular" {
		writeError(w, http.StatusForbidden, "ACCOUNT_TYPE_DENIED")
		return
	}
	ownerID, err := uuid.Parse(claims.UserID)
	if err != nil || ownerID == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "INVALID_TOKEN")
		return
	}
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key == "" {
		writeError(w, http.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var body struct {
		Name   string `json:"name"`
		GameID string `json:"game_id"`
	}
	if err := decoder.Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT")
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT")
		return
	}
	input := registry.CreateApplicationInput{
		OwnerAccountID: ownerID,
		Name:           body.Name,
		IdempotencyKey: key,
	}
	if body.GameID != "" {
		gameID, err := uuid.Parse(body.GameID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT")
			return
		}
		input.GameID = &gameID
	}
	application, err := h.Applications.CreateApplication(r.Context(), input)
	if err != nil {
		switch {
		case errors.Is(err, registry.ErrInvalidApplication):
			writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT")
		case errors.Is(err, registry.ErrIdempotencyConflict):
			writeError(w, http.StatusConflict, "IDEMPOTENCY_CONFLICT")
		default:
			writeError(w, http.StatusServiceUnavailable, "REGISTRY_UNAVAILABLE")
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"application_id": application.ID.String(),
		"name":           application.Name,
		"status":         application.Status,
		"revision":       application.Revision,
	})
}

func writeError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error_code": code,
		"retryable":  status >= 500,
	})
}
