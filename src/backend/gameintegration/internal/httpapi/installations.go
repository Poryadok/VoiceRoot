package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"voice/backend/gameintegration/internal/registry"
)

func (h *Handler) serveInstallationRegistration(w http.ResponseWriter, r *http.Request) {
	const prefix = "/api/v1/game-integrations/applications/"
	const separator = "/environments/"
	const suffix = "/installations"
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, prefix)
	parts := strings.Split(strings.TrimSuffix(path, suffix), separator)
	if len(parts) != 2 {
		http.NotFound(w, r)
		return
	}
	appID, appErr := uuid.Parse(parts[0])
	envID, envErr := uuid.Parse(parts[1])
	if appErr != nil || envErr != nil || appID == uuid.Nil || envID == uuid.Nil {
		http.NotFound(w, r)
		return
	}
	if h == nil || h.Tokens == nil || h.Installations == nil {
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
	ownerID, err := uuid.Parse(claims.UserID)
	if err != nil || ownerID == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "INVALID_TOKEN")
		return
	}
	if claims.AccountType != "regular" {
		writeError(w, http.StatusForbidden, "ACCOUNT_TYPE_DENIED")
		return
	}
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key == "" {
		writeError(w, http.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var body struct {
		CallbackURL string `json:"callback_url"`
		BotID       string `json:"bot_id"`
	}
	if err := decoder.Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT")
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT")
		return
	}
	botID, err := uuid.Parse(body.BotID)
	if err != nil || botID == uuid.Nil || botID.String() != body.BotID {
		writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT")
		return
	}
	installation, err := h.Installations.CreateInstallation(r.Context(), registry.CreateInstallationInput{
		OwnerAccountID: ownerID, ApplicationID: appID, EnvironmentID: envID,
		BotID: botID, CallbackURL: body.CallbackURL, IdempotencyKey: key,
	})
	if err != nil {
		switch {
		case errors.Is(err, registry.ErrInvalidApplication):
			writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT")
		case errors.Is(err, registry.ErrUnsafeCallbackURL):
			writeError(w, http.StatusBadRequest, "CALLBACK_URL_REJECTED")
		case errors.Is(err, registry.ErrInstallationConflict):
			writeError(w, http.StatusForbidden, "INSTALLATION_DENIED")
		case errors.Is(err, registry.ErrBotAuthorityDenied):
			writeError(w, http.StatusForbidden, "BOT_AUTHORITY_DENIED")
		case errors.Is(err, registry.ErrBotAuthorityUnavailable):
			writeError(w, http.StatusServiceUnavailable, "BOT_AUTHORITY_UNAVAILABLE")
		case errors.Is(err, registry.ErrApplicationSuspended):
			writeError(w, http.StatusServiceUnavailable, "APP_SUSPENDED")
		case errors.Is(err, registry.ErrRateLimited):
			var limited *registry.RateLimitError
			if errors.As(err, &limited) {
				seconds := int64((limited.RetryAfter + time.Second - 1) / time.Second)
				if seconds < 1 {
					seconds = 1
				}
				if seconds > 60 {
					seconds = 60
				}
				w.Header().Set("Retry-After", strconv.FormatInt(seconds, 10))
			}
			writeError(w, http.StatusTooManyRequests, "RATE_LIMITED")
		case errors.Is(err, registry.ErrIdempotencyConflict):
			writeError(w, http.StatusConflict, "IDEMPOTENCY_CONFLICT")
		default:
			writeError(w, http.StatusServiceUnavailable, "REGISTRY_UNAVAILABLE")
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"installation_id": installation.ID.String(),
		"application_id":  installation.ApplicationID.String(),
		"environment_id":  installation.EnvironmentID.String(),
		"bot_id":          installation.BotID.String(),
		"status":          installation.Status,
	})
}
