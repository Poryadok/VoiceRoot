package httpapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"voice/backend/gameintegration/internal/authbinding"
	"voice/backend/gameintegration/internal/registry"
)

type BindingExchangeStore interface {
	BeginPlayerBindingExchange(context.Context, uuid.UUID, string) (registry.BindingExchangeOperation, error)
	RecordPlayerBindingExchangeClaim(context.Context, uuid.UUID, string, string, uuid.UUID, uuid.UUID) error
	CommitPlayerBindingExchange(context.Context, uuid.UUID, string, uuid.UUID, uuid.UUID, registry.CreatePlayerBindingInput) (registry.BindingExchangeResult, error)
	FailPlayerBindingExchange(context.Context, uuid.UUID, string, uuid.UUID, uuid.UUID) error
	NextPlayerBindingCompletion(context.Context) (registry.BindingExchangeCompletion, error)
	RecordPlayerBindingCompletionAttempt(context.Context, uuid.UUID) error
	CompletePlayerBindingExchange(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID) error
	CompleteFailedPlayerBindingExchange(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error
}

type BindingExchangeAuth interface {
	Exchange(context.Context, authbinding.ExchangeRequest) (authbinding.ExchangeResponse, error)
	Claim(context.Context, authbinding.ClaimRequest) (authbinding.ClaimReceipt, error)
	Complete(context.Context, authbinding.CompletionRequest) (authbinding.CompletionReceipt, error)
}

type BindingExchangeHandler struct {
	Tokens TokenValidator
	Store  BindingExchangeStore
	Auth   BindingExchangeAuth
}

// RunBindingCompletionOutbox retries Auth completion receipts until GIS can
// durably activate the binding. The caller cancels ctx during shutdown.
func RunBindingCompletionOutbox(ctx context.Context, store BindingExchangeStore, auth BindingExchangeAuth) {
	if store == nil || auth == nil {
		return
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		completion, err := store.NextPlayerBindingCompletion(ctx)
		if err == nil {
			_ = store.RecordPlayerBindingCompletionAttempt(ctx, completion.OperationID)
			ack, callErr := auth.Complete(ctx, authbinding.CompletionRequest{ClaimID: completion.ClaimID,
				OperationID: completion.OperationID, Outcome: completion.Outcome, BindingID: completion.BindingID})
			if callErr == nil && ack.ClaimID == completion.ClaimID && ack.OperationID == completion.OperationID &&
				ack.Outcome == completion.Outcome && ack.BindingID == completion.BindingID && ack.Status == "completed" {
				if completion.Outcome == "failed" {
					_ = store.CompleteFailedPlayerBindingExchange(ctx, completion.OperationID, completion.ClaimID,
						completion.AssertionJTI)
				} else {
					_ = store.CompletePlayerBindingExchange(ctx, completion.OperationID, completion.ClaimID,
						completion.AssertionJTI, completion.BindingID)
				}
			}
			continue
		}
		if ctx.Err() != nil {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

type bindingExchangeRequest struct {
	ChallengeID  uuid.UUID `json:"challenge_id"`
	Code         string    `json:"code"`
	CodeVerifier string    `json:"code_verifier"`
	DeviceProof  string    `json:"device_proof"`
}

var bindingExchangeCodePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)
var bindingExchangeVerifierPattern = regexp.MustCompile(`^[A-Za-z0-9._~-]{43,128}$`)

type bindingHandoffClaims struct {
	Issuer                 string   `json:"iss"`
	Subject                string   `json:"sub"`
	Audience               []string `json:"aud"`
	IssuedAt               int64    `json:"iat"`
	NotBefore              int64    `json:"nbf"`
	Expiration             int64    `json:"exp"`
	JTI                    string   `json:"jti"`
	Version                int      `json:"version"`
	AuthorizationRequestID string   `json:"authorization_request_id"`
	OperationID            string   `json:"operation_id"`
	ChallengeID            string   `json:"challenge_id"`
	ChallengeNonce         string   `json:"challenge_nonce"`
	ApplicationID          string   `json:"application_id"`
	EnvironmentID          string   `json:"environment_id"`
	Provider               string   `json:"provider"`
	ProviderSubjectDigest  string   `json:"provider_subject_digest"`
	SourceAccountID        string   `json:"source_account_id"`
	SourceActorID          string   `json:"source_actor_id"`
	SourceDeviceID         string   `json:"source_device_id"`
	TargetAccountID        string   `json:"target_account_id"`
	TargetProfileID        string   `json:"target_profile_id"`
	DeviceGeneration       int64    `json:"device_generation"`
	ProfileRevision        int64    `json:"profile_revision"`
	ConsentRevision        int64    `json:"consent_revision"`
	PolicyRevision         int64    `json:"policy_revision"`
	Scopes                 []string `json:"scopes"`
	DeviceKeyID            string   `json:"device_key_id"`
	DeviceKeyThumbprint    string   `json:"device_key_thumbprint"`
	RedirectURIHash        string   `json:"redirect_uri_sha256"`
	PKCEChallenge          string   `json:"pkce_challenge"`
}

func (h *BindingExchangeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if h == nil || h.Tokens == nil || h.Store == nil || h.Auth == nil {
		writeError(w, http.StatusServiceUnavailable, "BINDING_EXCHANGE_UNAVAILABLE")
		return
	}
	if r.Method != http.MethodPost || r.URL.RawQuery != "" || r.URL.Path != "/api/v1/game-integrations/bindings/exchange" {
		http.NotFound(w, r)
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
	if r.Header.Get("Content-Type") != "application/json" || len(r.Header.Values("Content-Type")) != 1 {
		writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var body bindingExchangeRequest
	if err := decoder.Decode(&body); err != nil || decoder.Decode(new(any)) != io.EOF ||
		body.ChallengeID == uuid.Nil || !bindingExchangeCodePattern.MatchString(body.Code) ||
		!bindingExchangeVerifierPattern.MatchString(body.CodeVerifier) || strings.TrimSpace(body.DeviceProof) != body.DeviceProof ||
		body.DeviceProof == "" || len(body.DeviceProof) > 8<<10 {
		writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT")
		return
	}
	canonicalBody, err := json.Marshal(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT")
		return
	}
	digest := sha256.Sum256(canonicalBody)
	requestHash := hex.EncodeToString(digest[:])
	operation, err := h.Store.BeginPlayerBindingExchange(r.Context(), body.ChallengeID, requestHash)
	if err != nil {
		h.writeStoreError(w, err)
		return
	}
	handoffJWS := operation.HandoffJWS
	if handoffJWS == "" {
		exchanged, exchangeErr := h.Auth.Exchange(r.Context(), authbinding.ExchangeRequest{ChallengeID: body.ChallengeID,
			OperationID: operation.OperationID, Code: body.Code, CodeVerifier: body.CodeVerifier, DeviceProof: body.DeviceProof})
		if exchangeErr != nil {
			writeError(w, http.StatusServiceUnavailable, "AUTHORITY_UNAVAILABLE")
			return
		}
		handoffJWS = exchanged.HandoffJWS
	}
	receipt, err := h.Auth.Claim(r.Context(), authbinding.ClaimRequest{OperationID: operation.OperationID,
		RequestSHA256: requestHash, HandoffJWS: handoffJWS, DeviceProof: body.DeviceProof})
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "AUTHORITY_UNAVAILABLE")
		return
	}
	parsed, err := parseBindingHandoff(handoffJWS)
	if err != nil || !handoffMatchesChallenge(parsed, operation.Challenge) ||
		parsed.Issuer != "auth" || parsed.Subject != "service:auth" || len(parsed.Audience) != 1 || parsed.Audience[0] != "voice.game-binding" ||
		parsed.Version != 1 || parsed.Expiration <= 0 || receipt.OperationID != operation.OperationID ||
		parsed.IssuedAt <= 0 || parsed.NotBefore != parsed.IssuedAt || parsed.Expiration <= parsed.IssuedAt ||
		parsed.DeviceGeneration <= 0 || parsed.ProfileRevision <= 0 || parsed.ConsentRevision <= 0 || parsed.PolicyRevision <= 0 ||
		receipt.AssertionJTI.String() != parsed.JTI {
		writeError(w, http.StatusServiceUnavailable, "AUTHORITY_UNAVAILABLE")
		return
	}
	if err := h.Store.RecordPlayerBindingExchangeClaim(r.Context(), operation.OperationID, requestHash,
		handoffJWS, receipt.ClaimID, receipt.AssertionJTI); err != nil {
		h.writeStoreError(w, err)
		return
	}
	input, err := playerBindingFromHandoff(parsed)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "AUTHORITY_UNAVAILABLE")
		return
	}
	result, err := h.Store.CommitPlayerBindingExchange(r.Context(), operation.OperationID, requestHash,
		receipt.ClaimID, receipt.AssertionJTI, input)
	if errors.Is(err, registry.ErrInvalidPlayerBinding) {
		if failErr := h.Store.FailPlayerBindingExchange(r.Context(), operation.OperationID, requestHash,
			receipt.ClaimID, receipt.AssertionJTI); failErr == nil {
			result = registry.BindingExchangeResult{OperationID: operation.OperationID, Status: "failed"}
			err = nil
		} else {
			err = failErr
		}
	}
	if err != nil {
		h.writeStoreError(w, err)
		return
	}
	if result.Status == "pending" || result.Status == "failed" {
		completion, nextErr := h.Store.NextPlayerBindingCompletion(r.Context())
		if nextErr == nil && completion.OperationID == result.OperationID {
			_ = h.Store.RecordPlayerBindingCompletionAttempt(r.Context(), completion.OperationID)
			ack, completeErr := h.Auth.Complete(r.Context(), authbinding.CompletionRequest{
				ClaimID: completion.ClaimID, OperationID: completion.OperationID, Outcome: completion.Outcome, BindingID: completion.BindingID,
			})
			if completeErr == nil && ack.ClaimID == completion.ClaimID && ack.OperationID == completion.OperationID &&
				ack.Outcome == completion.Outcome && ack.BindingID == completion.BindingID && ack.Status == "completed" {
				var completeErr error
				if completion.Outcome == "failed" {
					completeErr = h.Store.CompleteFailedPlayerBindingExchange(r.Context(), completion.OperationID,
						completion.ClaimID, completion.AssertionJTI)
				} else {
					completeErr = h.Store.CompletePlayerBindingExchange(r.Context(), completion.OperationID,
						completion.ClaimID, completion.AssertionJTI, completion.BindingID)
				}
				if completeErr != nil {
					writeError(w, http.StatusServiceUnavailable, "BINDING_EXCHANGE_PENDING")
					return
				}
				if completion.Outcome == "succeeded" {
					result.Status = "active"
				} else {
					result.Status = "failed"
				}
			}
			if result.Status == "active" || result.Status == "succeeded" {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Cache-Control", "no-store")
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(result)
				return
			}
			if result.Status == "failed" {
				writeError(w, http.StatusForbidden, "BINDING_EXCHANGE_DENIED")
				return
			}
		}
		// The durable outbox owns recovery after an uncertain or lost ACK.
		writeError(w, http.StatusServiceUnavailable, "BINDING_EXCHANGE_PENDING")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(result)
}

func (h *BindingExchangeHandler) writeStoreError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, registry.ErrBindingExchangeConflict), errors.Is(err, registry.ErrPlayerBindingConflict):
		writeError(w, http.StatusConflict, "BINDING_EXCHANGE_CONFLICT")
	case errors.Is(err, registry.ErrInvalidPlayerBinding):
		writeError(w, http.StatusForbidden, "BINDING_EXCHANGE_DENIED")
	case errors.Is(err, registry.ErrBindingChallengeNotFound):
		writeError(w, http.StatusNotFound, "BINDING_CHALLENGE_NOT_FOUND")
	default:
		writeError(w, http.StatusServiceUnavailable, "BINDING_EXCHANGE_UNAVAILABLE")
	}
}

func parseBindingHandoff(compact string) (bindingHandoffClaims, error) {
	var claims bindingHandoffClaims
	parts := strings.Split(compact, ".")
	if len(parts) != 3 || len(compact) > 16<<10 {
		return claims, errors.New("invalid handoff")
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return claims, err
	}
	var header struct {
		Type      string `json:"typ"`
		Algorithm string `json:"alg"`
	}
	if json.Unmarshal(headerBytes, &header) != nil || header.Type != "voice.game-binding-handoff+jwt" || header.Algorithm != "RS256" {
		return claims, errors.New("invalid handoff header")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return claims, err
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&claims); err != nil || decoder.Decode(new(any)) != io.EOF {
		return claims, errors.New("invalid handoff claims")
	}
	return claims, nil
}

func handoffMatchesChallenge(c bindingHandoffClaims, challenge registry.BindingChallenge) bool {
	return c.ChallengeID == challenge.ChallengeID.String() && c.ChallengeNonce == challenge.Nonce &&
		c.OperationID == challenge.OperationID.String() && c.ApplicationID == challenge.ApplicationID.String() &&
		c.EnvironmentID == challenge.EnvironmentID.String() && c.Provider == challenge.Provider &&
		c.RedirectURIHash == challenge.RedirectURIHash && c.PKCEChallenge == challenge.PKCEChallenge &&
		c.DeviceKeyID == challenge.DeviceKeyID.String() && c.DeviceKeyThumbprint == challenge.DeviceKeyThumbprint &&
		c.SourceAccountID == challenge.SourceAccountID.String() && c.SourceActorID == challenge.SourceActorID.String() &&
		c.SourceDeviceID == challenge.SourceDeviceID.String() && c.DeviceGeneration == challenge.SourceGeneration &&
		c.TargetAccountID == challenge.TargetAccountID.String() && c.TargetProfileID == challenge.TargetProfileID.String() &&
		c.ProfileRevision == challenge.ProfileRevision && c.ConsentRevision == challenge.ConsentRevision &&
		c.PolicyRevision == challenge.PolicyRevision && equalSortedScopes(c.Scopes, challenge.Scopes)
}

func equalSortedScopes(a, b []string) bool {
	if len(a) != len(b) { return false }
	for i := range a { if a[i] != b[i] { return false } }
	return true
}

func playerBindingFromHandoff(c bindingHandoffClaims) (registry.CreatePlayerBindingInput, error) {
	parse := func(raw string) (uuid.UUID, error) {
		id, err := uuid.Parse(raw)
		if err != nil || id == uuid.Nil || id.String() != raw {
			return uuid.Nil, errors.New("invalid handoff identity")
		}
		return id, nil
	}
	app, err := parse(c.ApplicationID)
	if err != nil {
		return registry.CreatePlayerBindingInput{}, err
	}
	env, err := parse(c.EnvironmentID)
	if err != nil {
		return registry.CreatePlayerBindingInput{}, err
	}
	account, err := parse(c.TargetAccountID)
	if err != nil {
		return registry.CreatePlayerBindingInput{}, err
	}
	actor, err := parse(c.SourceActorID)
	if err != nil {
		return registry.CreatePlayerBindingInput{}, err
	}
	profile, err := parse(c.TargetProfileID)
	if err != nil {
		return registry.CreatePlayerBindingInput{}, err
	}
	device, err := parse(c.SourceDeviceID)
	if err != nil {
		return registry.CreatePlayerBindingInput{}, err
	}
	return registry.CreatePlayerBindingInput{ApplicationID: app, EnvironmentID: env, Provider: c.Provider,
		ProviderSubjectDigest: c.ProviderSubjectDigest, AccountID: account, ActorID: actor, ProfileID: profile, DeviceID: device}, nil
}
