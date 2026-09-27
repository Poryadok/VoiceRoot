package gameintegrationproof

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"voice/backend/pkg/workloadproof"
)

const routePrefix = "/internal/v1/game-integrations/bots/"

type BotLookup interface {
	LookupGameIntegrationBotAuthority(context.Context, uuid.UUID) (uuid.UUID, string, bool, error)
}

type authorityRequest struct {
	ApplicationOwnerAccountID string `json:"application_owner_account_id"`
}

type authorityResponse struct {
	BotID          string `json:"bot_id"`
	OwnerAccountID string `json:"owner_account_id"`
	Status         string `json:"status"`
}

type Handler struct {
	Bots     BotLookup
	Key      []byte
	Nonces   workloadproof.NonceStore
	Now      func() time.Time
	verifier workloadproof.Verifier
}

func NewHandler(bots BotLookup, key []byte, nonces workloadproof.NonceStore, now func() time.Time) *Handler {
	if now == nil {
		now = time.Now
	}
	return &Handler{Bots: bots, Key: append([]byte(nil), key...), Nonces: nonces, Now: now,
		verifier: workloadproof.Verifier{Key: append([]byte(nil), key...), Principal: "gameintegration", Audience: "bot", Now: now, Nonces: nonces}}
}

func DecodeWorkloadKey(encoded string) ([]byte, error) {
	if strings.TrimSpace(encoded) == "" {
		return nil, nil
	}
	key, err := base64.StdEncoding.Strict().DecodeString(encoded)
	if err != nil || len(key) != 32 {
		return nil, errors.New("GAME_INTEGRATION_BOT_WORKLOAD_KEY_B64 must be a base64-encoded 32-byte key")
	}
	return key, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r == nil || r.Method != http.MethodPost || r.URL == nil || r.URL.RawQuery != "" {
		http.NotFound(w, r)
		return
	}
	botID, ok := parseRouteBotID(r.URL.EscapedPath())
	if !ok {
		http.NotFound(w, r)
		return
	}
	if h == nil || h.Bots == nil {
		writeProofError(w, http.StatusServiceUnavailable, "BOT_AUTHORITY_UNAVAILABLE")
		return
	}
	proof, err := h.verifier.VerifyRequest(r, 1024)
	if err != nil {
		if errors.Is(err, workloadproof.ErrUnavailable) {
			writeProofError(w, http.StatusServiceUnavailable, "BOT_AUTHORITY_UNAVAILABLE")
		} else {
			writeProofError(w, http.StatusUnauthorized, "INVALID_WORKLOAD_PROOF")
		}
		return
	}
	var body authorityRequest
	decoder := json.NewDecoder(bytes.NewReader(proof.Body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil || decoder.Decode(new(any)) != io.EOF {
		writeProofError(w, http.StatusBadRequest, "INVALID_ARGUMENT")
		return
	}
	canonicalOwner, err := uuid.Parse(body.ApplicationOwnerAccountID)
	if err != nil || canonicalOwner == uuid.Nil || canonicalOwner.String() != body.ApplicationOwnerAccountID {
		writeProofError(w, http.StatusBadRequest, "INVALID_ARGUMENT")
		return
	}
	canonicalBody, err := json.Marshal(body)
	if err != nil || !bytes.Equal(canonicalBody, proof.Body) {
		writeProofError(w, http.StatusBadRequest, "INVALID_ARGUMENT")
		return
	}
	if proof.Principal != "gameintegration" || proof.Audience != "bot" {
		writeProofError(w, http.StatusUnauthorized, "INVALID_WORKLOAD_PROOF")
		return
	}
	ownerID, status, found, err := h.Bots.LookupGameIntegrationBotAuthority(r.Context(), botID)
	if err != nil {
		writeProofError(w, http.StatusServiceUnavailable, "BOT_AUTHORITY_UNAVAILABLE")
		return
	}
	if !found || ownerID != canonicalOwner || status != "live" {
		writeProofError(w, http.StatusForbidden, "BOT_AUTHORITY_DENIED")
		return
	}
	response := authorityResponse{BotID: botID.String(), OwnerAccountID: ownerID.String(), Status: status}
	responseBody, err := json.Marshal(response)
	if err != nil {
		writeProofError(w, http.StatusInternalServerError, "BOT_AUTHORITY_UNAVAILABLE")
		return
	}
	path := r.URL.EscapedPath()
	timestamp := strconv.FormatInt(proof.Timestamp, 10)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	workloadproof.SignResponse(w, h.Key, http.StatusOK, path, timestamp, proof.Nonce, responseBody)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(responseBody)
}

func parseRouteBotID(path string) (uuid.UUID, bool) {
	if !strings.HasPrefix(path, routePrefix) || !strings.HasSuffix(path, "/authority") {
		return uuid.Nil, false
	}
	raw := strings.TrimSuffix(strings.TrimPrefix(path, routePrefix), "/authority")
	if strings.Contains(raw, "/") {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(raw)
	return id, err == nil && id != uuid.Nil && id.String() == raw
}

func writeProofError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error_code": code})
}
