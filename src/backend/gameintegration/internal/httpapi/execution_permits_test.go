package httpapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/gameintegration/internal/registry"
)

type executionPermitIssuerStub struct {
	called bool
	claims registry.GameDeviceAuthorityClaims
	permit registry.PlayerBindingExecutionPermit
	err    error
}

func (s *executionPermitIssuerStub) IssuePlayerBindingExecutionPermit(_ context.Context, bindingID, operationID uuid.UUID,
	claims registry.GameDeviceAuthorityClaims) (registry.PlayerBindingExecutionPermit, error) {
	s.called = true
	s.claims = claims
	s.permit.BindingID = bindingID
	s.permit.OperationID = operationID
	return s.permit, s.err
}

type executionPermitCompleterStub struct {
	completion registry.PlayerBindingPermitCompletion
	permitID   uuid.UUID
	operation  uuid.UUID
	outcome    string
	err        error
}

func (s *executionPermitCompleterStub) CompletePlayerBindingExecutionPermit(_ context.Context, permitID, operationID uuid.UUID,
	outcome string) (registry.PlayerBindingPermitCompletion, error) {
	s.permitID, s.operation, s.outcome = permitID, operationID, outcome
	return s.completion, s.err
}

func TestInternalExecutionPermitBindsAssertionAndReturnsGISOnlyFields(t *testing.T) {
	now := time.UnixMilli(1790435000000).UTC()
	key := []byte("0123456789abcdef0123456789abcdef")
	nonces := &nonceMemory{}
	bindingID, appID, envID := uuid.New(), uuid.New(), uuid.New()
	claims := registry.GameDeviceAuthorityClaims{Issuer: "auth", Audience: "voice.game-message", Version: 1,
		ApplicationID: appID, EnvironmentID: envID, AccountID: uuid.New(), ActorID: uuid.New(), BindingID: bindingID,
		DeviceID: uuid.New(), KeyID: uuid.New(), DeviceGeneration: 3, AuthorityRevision: 8, AssertionID: uuid.New(),
		Status: "active", NotAfter: now.Add(4 * time.Second), IssuedAt: now, ExpiresAt: now.Add(4 * time.Second)}
	assertion := makeTestDeviceAuthorityAssertion(t, claims)
	other := claims
	other.BindingID = uuid.New()
	otherAssertion := makeTestDeviceAuthorityAssertion(t, other)
	operationID := uuid.New()
	body := []byte(`{"operation_id":"` + operationID.String() + `"}`)
	permit := registry.PlayerBindingExecutionPermit{PermitID: uuid.New(), BindingID: bindingID,
		ApplicationID: appID, EnvironmentID: envID, BindingRevision: 4, AssertionID: claims.AssertionID,
		OperationID: operationID, ExpiresAt: now.Add(3750 * time.Millisecond)}
	store := &executionPermitIssuerStub{permit: permit}
	verifier := WorkloadVerifier{Key: key, Now: func() time.Time { return now }, Nonces: nonces}
	handler := NewInternalExecutionPermitHandler(verifier, store, nil)
	path := "/internal/v1/game-integrations/bindings/" + bindingID.String() + "/execution-permits"
	request := func(nonce string, token []byte) *http.Request {
		r := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
		r.Header.Set("X-Voice-Device-Authority", string(token))
		SignAssertionBoundWorkloadRequest(r, key, now, nonce, body, token)
		return r
	}

	tampered := request(uuid.NewString(), assertion)
	tampered.Header.Set("X-Voice-Device-Authority", string(otherAssertion))
	_, err := verifier.VerifyAssertionBound(tampered)
	var failure workloadProofFailure
	require.ErrorIs(t, err, ErrInvalidWorkloadProof)
	require.ErrorAs(t, err, &failure)
	require.Equal(t, workloadFailureSignature, failure)
	require.NotContains(t, failure.Error(), string(assertion))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, tampered)
	require.Equal(t, http.StatusUnauthorized, w.Code)
	require.False(t, store.called, "a substituted assertion must fail before GIS state is read")

	r := request(uuid.NewString(), assertion)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.True(t, store.called)
	require.Equal(t, claims.AssertionID, store.claims.AssertionID)
	require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	require.Equal(t, responseSignature(key, http.StatusOK, r.URL.EscapedPath(), r.Header.Get("X-Voice-Timestamp"),
		r.Header.Get("X-Voice-Nonce"), w.Body.Bytes()), w.Header().Get("X-Voice-Response-Signature"))
	var response map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Equal(t, map[string]any{
		"permit_id": permit.PermitID.String(), "binding_id": bindingID.String(),
		"application_id": appID.String(), "environment_id": envID.String(), "binding_revision": float64(4),
		"assertion_jti": claims.AssertionID.String(), "operation_id": operationID.String(),
		"expires_at": permit.ExpiresAt.Format(time.RFC3339Nano),
	}, response)
	require.NotContains(t, response, "account_id")
	require.NotContains(t, response, "profile_id")
	require.NotContains(t, response, "scope")
}

func TestInternalExecutionPermitCompletionRequiresWorkloadAndIsExact(t *testing.T) {
	now := time.Unix(1790435000, 0)
	key := []byte("0123456789abcdef0123456789abcdef")
	permitID, operationID := uuid.New(), uuid.New()
	body := []byte(`{"operation_id":"` + operationID.String() + `","outcome":"committed"}`)
	completer := &executionPermitCompleterStub{completion: registry.PlayerBindingPermitCompletion{
		PermitID: permitID, OperationID: operationID, Outcome: "committed", Status: "completed",
	}}
	handler := NewInternalExecutionPermitHandler(WorkloadVerifier{Key: key, Now: func() time.Time { return now }, Nonces: &nonceMemory{}}, nil, completer)
	path := "/internal/v1/game-integrations/execution-permits/" + permitID.String() + "/completion"
	r := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	SignBodyWorkloadRequest(r, key, now, uuid.NewString(), body)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, permitID, completer.permitID)
	require.Equal(t, operationID, completer.operation)
	require.Equal(t, "committed", completer.outcome)

	bad := httptest.NewRequest(http.MethodPost, path, bytes.NewReader([]byte(`{"operation_id":"`+operationID.String()+`","outcome":"committed","profile_id":"forbidden"}`)))
	SignBodyWorkloadRequest(bad, key, now, uuid.NewString(), []byte(`{"operation_id":"`+operationID.String()+`","outcome":"committed","profile_id":"forbidden"}`))
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, bad)
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func makeTestDeviceAuthorityAssertion(t *testing.T, claims registry.GameDeviceAuthorityClaims) []byte {
	t.Helper()
	header, err := json.Marshal(map[string]string{"alg": "RS256", "kid": "current", "typ": "voice.game-device-status+jwt"})
	require.NoError(t, err)
	payload := map[string]any{
		"version": 1, "iss": claims.Issuer, "aud": claims.Audience, "jti": claims.AssertionID.String(),
		"application_id": claims.ApplicationID.String(), "environment_id": claims.EnvironmentID.String(),
		"account_id": claims.AccountID.String(), "actor_id": claims.ActorID.String(), "binding_id": claims.BindingID.String(),
		"device_id": claims.DeviceID.String(), "key_id": claims.KeyID.String(), "public_jwk": map[string]any{"kty": "EC", "crv": "P-256", "x": "x", "y": "y"},
		"key_thumbprint": "test-thumbprint", "device_generation": claims.DeviceGeneration,
		"authority_revision": claims.AuthorityRevision, "status": claims.Status,
		"not_after": claims.NotAfter.UnixMilli(), "iat": claims.IssuedAt.UnixMilli(), "exp": claims.ExpiresAt.UnixMilli(),
	}
	payloadBytes, err := json.Marshal(payload)
	require.NoError(t, err)
	return []byte(base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payloadBytes) + "." +
		base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{7}, 256)))
}
