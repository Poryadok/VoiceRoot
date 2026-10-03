package httpapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/gameintegration/internal/authbinding"
	"voice/backend/gameintegration/internal/registry"
	voicejwt "voice/backend/pkg/jwt"
)

type bindingExchangeStoreStub struct {
	operation      registry.BindingExchangeOperation
	completion     registry.BindingExchangeCompletion
	result         registry.BindingExchangeResult
	committedInput registry.CreatePlayerBindingInput
	completed      bool
}

func (s *bindingExchangeStoreStub) BeginPlayerBindingExchange(_ context.Context, challenge uuid.UUID, hash string) (registry.BindingExchangeOperation, error) {
	s.operation.RequestSHA256 = hash
	return s.operation, nil
}
func (s *bindingExchangeStoreStub) RecordPlayerBindingExchangeClaim(_ context.Context, _ uuid.UUID, _, jws string, claim, jti uuid.UUID) error {
	s.operation.HandoffJWS, s.operation.ClaimID, s.operation.AssertionJTI = jws, claim, jti
	return nil
}
func (s *bindingExchangeStoreStub) CommitPlayerBindingExchange(_ context.Context, op uuid.UUID, _ string, claim, jti uuid.UUID, in registry.CreatePlayerBindingInput) (registry.BindingExchangeResult, error) {
	s.committedInput = in
	s.completion = registry.BindingExchangeCompletion{OperationID: op, ClaimID: claim, AssertionJTI: jti,
		BindingID: s.result.BindingID, Outcome: "succeeded"}
	return s.result, nil
}
func (s *bindingExchangeStoreStub) FailPlayerBindingExchange(context.Context, uuid.UUID, string, uuid.UUID, uuid.UUID) error {
	return nil
}
func (s *bindingExchangeStoreStub) NextPlayerBindingCompletion(context.Context) (registry.BindingExchangeCompletion, error) {
	if s.completed {
		return registry.BindingExchangeCompletion{}, context.Canceled
	}
	return s.completion, nil
}
func (s *bindingExchangeStoreStub) RecordPlayerBindingCompletionAttempt(context.Context, uuid.UUID) error {
	return nil
}
func (s *bindingExchangeStoreStub) CompletePlayerBindingExchange(_ context.Context, _, _, _, _ uuid.UUID) error {
	s.completed = true
	return nil
}
func (s *bindingExchangeStoreStub) CompleteFailedPlayerBindingExchange(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error {
	s.completed = true
	return nil
}

type bindingExchangeAuthStub struct {
	handoff                                  string
	exchangeCalls, claimCalls, completeCalls int
	completeErr                              error
}

func (s *bindingExchangeAuthStub) Exchange(context.Context, authbinding.ExchangeRequest) (authbinding.ExchangeResponse, error) {
	s.exchangeCalls++
	return authbinding.ExchangeResponse{HandoffJWS: s.handoff}, nil
}
func (s *bindingExchangeAuthStub) Claim(_ context.Context, in authbinding.ClaimRequest) (authbinding.ClaimReceipt, error) {
	s.claimCalls++
	return authbinding.ClaimReceipt{ClaimID: uuid.MustParse("11111111-1111-4111-8111-111111111111"),
		OperationID: in.OperationID, AssertionJTI: uuid.MustParse("22222222-2222-4222-8222-222222222222"),
		ExpiresAt: time.Now().Add(30 * time.Second)}, nil
}
func (s *bindingExchangeAuthStub) Complete(_ context.Context, in authbinding.CompletionRequest) (authbinding.CompletionReceipt, error) {
	s.completeCalls++
	if s.completeErr != nil {
		return authbinding.CompletionReceipt{}, s.completeErr
	}
	return authbinding.CompletionReceipt{ClaimID: in.ClaimID, OperationID: in.OperationID, Outcome: in.Outcome,
		BindingID: in.BindingID, Status: "completed"}, nil
}

func TestBindingExchangeHandlerRunsClaimCommitAndCompletionInOrder(t *testing.T) {
	challenge := registry.BindingChallenge{ChallengeID: uuid.New(), Nonce: strings.Repeat("n", 43),
		ApplicationID: uuid.New(), EnvironmentID: uuid.New(), Provider: "google", RedirectURIHash: strings.Repeat("a", 64),
		PKCEChallenge: strings.Repeat("b", 43), DeviceKeyID: uuid.New(), DeviceKeyThumbprint: strings.Repeat("c", 43),
		OperationID: uuid.New(), ExpiresAt: time.Now().Add(5 * time.Minute), Status: "pending"}
	accountID, actorID, profileID := uuid.New(), uuid.New(), uuid.New()
	deviceID := challenge.DeviceKeyID
	challenge.SourceAccountID, challenge.SourceActorID, challenge.SourceDeviceID = accountID, actorID, deviceID
	challenge.SourceGeneration = 1
	challenge.TargetAccountID, challenge.TargetProfileID = accountID, profileID
	challenge.ProfileRevision, challenge.ConsentRevision, challenge.PolicyRevision = 1, 1, 1
	challenge.Scopes = []string{"game.chat.send"}
	now := time.Now().Unix()
	claims := map[string]any{"iss": "auth", "sub": "service:auth", "aud": []string{"voice.game-binding"},
		"iat": now, "nbf": now, "exp": time.Now().Add(20 * time.Second).Unix(),
		"authorization_request_id": uuid.NewString(),
		"jti":                      "22222222-2222-4222-8222-222222222222", "version": 1,
		"operation_id": challenge.OperationID.String(), "challenge_id": challenge.ChallengeID.String(),
		"challenge_nonce": challenge.Nonce, "application_id": challenge.ApplicationID.String(),
		"environment_id": challenge.EnvironmentID.String(), "provider": "google",
		"provider_subject_digest": "hmac-sha256-v1:test:" + strings.Repeat("d", 64),
		"source_account_id":       accountID.String(), "source_actor_id": actorID.String(), "source_device_id": deviceID.String(),
		"target_account_id": accountID.String(), "target_profile_id": profileID.String(),
		"device_generation": 1, "profile_revision": 1, "consent_revision": 1, "policy_revision": 1,
		"scopes":        []string{"game.chat.send"},
		"device_key_id": challenge.DeviceKeyID.String(), "device_key_thumbprint": challenge.DeviceKeyThumbprint,
		"redirect_uri_sha256": challenge.RedirectURIHash, "pkce_challenge": challenge.PKCEChallenge}
	payload, err := json.Marshal(claims)
	require.NoError(t, err)
	header, err := json.Marshal(map[string]string{"typ": "voice.game-binding-handoff+jwt", "alg": "RS256"})
	require.NoError(t, err)
	handoff := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload) + ".signature"
	store := &bindingExchangeStoreStub{operation: registry.BindingExchangeOperation{Challenge: challenge,
		OperationID: challenge.OperationID, Status: "pending"}, result: registry.BindingExchangeResult{
		OperationID: challenge.OperationID, BindingID: uuid.New(), Status: "pending"}}
	auth := &bindingExchangeAuthStub{handoff: handoff}
	handler := NewHandler(testValidator{claims: voicejwt.Claims{AccountType: "regular"}}, &testInstallationStore{})
	handler.BindingExchanges = &BindingExchangeHandler{Tokens: handler.Tokens, Store: store, Auth: auth}
	body, err := json.Marshal(bindingExchangeRequest{ChallengeID: challenge.ChallengeID, Code: strings.Repeat("x", 43),
		CodeVerifier: strings.Repeat("v", 43), DeviceProof: "proof-jws-bytes"})
	require.NoError(t, err)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/game-integrations/bindings/exchange", strings.NewReader(string(body)))
	request.Header.Set("Authorization", "Bearer player-token")
	request.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	auth.completeErr = errors.New("Auth completion outcome unavailable")
	handler.ServeHTTP(w, request)
	require.Equal(t, http.StatusServiceUnavailable, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "BINDING_EXCHANGE_PENDING")
	require.False(t, store.completed, "GIS binding remains pending while Auth completion is uncertain")

	auth.completeErr = nil
	request = httptest.NewRequest(http.MethodPost, "/api/v1/game-integrations/bindings/exchange", strings.NewReader(string(body)))
	request.Header.Set("Authorization", "Bearer player-token")
	request.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, request)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.True(t, store.completed)
	require.Equal(t, 1, auth.exchangeCalls, "durable handoff is reused when retrying uncertain completion")
	require.Equal(t, 2, auth.claimCalls, "same handoff/operation claim is retried idempotently")
	require.Equal(t, 2, auth.completeCalls, "Auth completion is retried after the uncertain result")
	require.Equal(t, challenge.ApplicationID, store.committedInput.ApplicationID)
	require.Equal(t, challenge.EnvironmentID, store.committedInput.EnvironmentID)
	require.Equal(t, accountID, store.committedInput.AccountID)
	require.Equal(t, actorID, store.committedInput.ActorID)
	require.Equal(t, profileID, store.committedInput.ProfileID)
	require.Equal(t, deviceID, store.committedInput.DeviceID)
	require.NotContains(t, w.Body.String(), "provider_subject_digest")
}
