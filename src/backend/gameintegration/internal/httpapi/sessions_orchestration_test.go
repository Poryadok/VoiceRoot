package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/gameintegration/internal/registry"
)

type sessionServerValidator struct {
	principal registry.SessionPrincipal
}

func (v sessionServerValidator) VerifyGameServer(r *http.Request) (registry.SessionPrincipal, error) {
	if r.Header.Get("Authorization") != "Bearer game-server-token" {
		return registry.SessionPrincipal{}, errors.New("invalid game-server credential")
	}
	return v.principal, nil
}

type sessionOrchestratorStub struct {
	principal registry.SessionPrincipal
	created   registry.CreateSessionInput
	operation registry.SessionOperation
	createN   int
}

func (s *sessionOrchestratorStub) CreateSession(_ context.Context, principal registry.SessionPrincipal, in registry.CreateSessionInput) (registry.SessionOperation, error) {
	s.principal = principal
	s.created = in
	s.createN++
	return s.operation, nil
}

func (s *sessionOrchestratorStub) GetOperation(_ context.Context, principal registry.SessionPrincipal, _ uuid.UUID) (registry.SessionOperation, error) {
	s.principal = principal
	return s.operation, nil
}

func (s *sessionOrchestratorStub) CloseSession(_ context.Context, principal registry.SessionPrincipal, _, _ uuid.UUID) (registry.SessionOperation, error) {
	s.principal = principal
	return s.operation, nil
}

func sessionRequest(handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer game-server-token")
	req.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	return response
}

func sessionResponse(t *testing.T, response *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &decoded), response.Body.String())
	return decoded
}

func TestSessionRoutesUseVerifiedScopedPrincipal(t *testing.T) {
	appID, envID := uuid.New(), uuid.New()
	principal := registry.SessionPrincipal{
		ApplicationID: appID, EnvironmentID: envID, Scopes: []string{"game.sessions.manage"},
	}
	operationID, sessionID := uuid.New(), uuid.New()
	orchestrator := &sessionOrchestratorStub{operation: registry.SessionOperation{
		OperationID: operationID, SessionID: sessionID, Status: "pending",
		SessionStatus: "provisioning", Stage: "accepted",
	}}
	handler := NewSessionHandler(sessionServerValidator{principal: principal}, orchestrator)
	requestBody := `{"operation_id":"` + operationID.String() + `","kind":"party","external_key":"party-1","display_name":"Raid","roster_revision":1,"roster_complete":true,"members":[]}`

	created := sessionRequest(handler, http.MethodPost, "/api/v1/sessions", requestBody)
	require.Equal(t, http.StatusAccepted, created.Code, created.Body.String())
	response := sessionResponse(t, created)
	require.Equal(t, operationID.String(), response["operation_id"])
	require.Equal(t, sessionID.String(), response["session_id"])
	require.Equal(t, appID, orchestrator.principal.ApplicationID)
	require.Equal(t, envID, orchestrator.principal.EnvironmentID)
	require.Equal(t, []string{"game.sessions.manage"}, orchestrator.principal.Scopes)
	require.Equal(t, operationID, orchestrator.created.OperationID)
	for _, field := range []string{
		"chat_id", "voice_room_id", "chat_owner_session_id", "chat_create_receipt_id",
		"chat_roster_receipt_id", "voice_provision_receipt_id", "role_grant_receipt_id",
	} {
		require.Contains(t, response, field, "operation response must keep the stable resource/receipt projection")
		require.Nil(t, response[field], "unprovisioned resources/receipts must be explicit nulls")
	}

	status := sessionRequest(handler, http.MethodGet, "/api/v1/operations/"+operationID.String(), "")
	require.Equal(t, http.StatusOK, status.Code, status.Body.String())
	require.Equal(t, appID, orchestrator.principal.ApplicationID)
	require.Equal(t, envID, orchestrator.principal.EnvironmentID)
	profileRosterBody := `{"operation_id":"` + uuid.NewString() + `","kind":"party","external_key":"party-roster","display_name":"Roster","roster_revision":1,"roster_complete":true,"members":["a7c5a70d-e025-41e2-afdc-0090ad5e38d1","b579650d-0534-4f33-8a80-4ac0fd7bd77b"]}`
	rosterCreated := sessionRequest(handler, http.MethodPost, "/api/v1/sessions", profileRosterBody)
	require.Equal(t, http.StatusAccepted, rosterCreated.Code, rosterCreated.Body.String())

	for _, body := range []string{
		`{"operation_id":"` + uuid.NewString() + `","kind":"party","external_key":"scope","display_name":"Raid","roster_revision":1,"roster_complete":true,"members":[],"application_id":"` + uuid.NewString() + `"}`,
		`{"operation_id":"` + uuid.NewString() + `","kind":"party","external_key":"scope","display_name":"Raid","roster_revision":1,"roster_complete":true,"members":[],"environment_id":"` + uuid.NewString() + `"}`,
	} {
		badScope := sessionRequest(handler, http.MethodPost, "/api/v1/sessions", body)
		require.Equal(t, http.StatusBadRequest, badScope.Code, badScope.Body.String())
	}
	require.Equal(t, 2, orchestrator.createN, "only the two valid requests reach orchestration")
}

func TestSessionRoutesRequireManageScope(t *testing.T) {
	principal := registry.SessionPrincipal{ApplicationID: uuid.New(), EnvironmentID: uuid.New(), Scopes: []string{"game.events.write"}}
	orchestrator := &sessionOrchestratorStub{}
	handler := NewSessionHandler(sessionServerValidator{principal: principal}, orchestrator)
	body := `{"operation_id":"` + uuid.NewString() + `","kind":"party","external_key":"party-denied","display_name":"Raid","roster_revision":1,"roster_complete":true,"members":[]}`
	response := sessionRequest(handler, http.MethodPost, "/api/v1/sessions", body)
	require.Equal(t, http.StatusForbidden, response.Code, response.Body.String())
	require.Zero(t, orchestrator.createN, "insufficient scope must be denied before durable acceptance")
}

func TestSessionRoutesRejectNonCanonicalOrAmbiguousCreateJSON(t *testing.T) {
	principal := registry.SessionPrincipal{ApplicationID: uuid.New(), EnvironmentID: uuid.New(), Scopes: []string{"game.sessions.manage"}}
	orchestrator := &sessionOrchestratorStub{}
	handler := NewSessionHandler(sessionServerValidator{principal: principal}, orchestrator)
	operationID := uuid.NewString()
	base := `"operation_id":"` + operationID + `","kind":"party","external_key":"party-1","display_name":"Raid","roster_revision":1,"roster_complete":true,"members":[]`
	bodies := []string{
		`{` + base + `,"operation_id":"` + uuid.NewString() + `"}`,
		`{` + base + `,"unexpected":true}`,
		`{"operation_id":"` + "00000000-0000-4000-8000-00000000000A" + `","kind":"party","external_key":"party-1","display_name":"Raid","roster_revision":1,"roster_complete":true,"members":[]}`,
		`{"operation_id":"` + operationID + `","kind":"party","external_key":"party-1","display_name":"Raid","roster_revision":1,"roster_complete":true,"members":null}`,
	}
	for _, body := range bodies {
		response := sessionRequest(handler, http.MethodPost, "/api/v1/sessions", body)
		require.Equal(t, http.StatusBadRequest, response.Code, body)
	}
	require.Zero(t, orchestrator.createN, "invalid canonical input must be rejected before orchestration")
}

func TestRegistrySessionCredentialVerifierExtractsBearerAndScope(t *testing.T) {
	store := &sessionCredentialStoreStub{principal: registry.ServicePrincipal{
		ApplicationID: uuid.New(), EnvironmentID: uuid.New(), Scopes: []string{"game.sessions.manage"},
	}}
	verifier := RegistrySessionCredentialVerifier{Store: store, Key: []byte("0123456789abcdef0123456789abcdef")}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", nil)
	request.Header.Set("Authorization", "Bearer vgi1_credential_secret")
	principal, err := verifier.VerifyGameServer(request)
	require.NoError(t, err)
	require.Equal(t, store.principal.ApplicationID, principal.ApplicationID)
	require.Equal(t, store.principal.EnvironmentID, principal.EnvironmentID)
	require.Equal(t, []string{"game.sessions.manage"}, principal.Scopes)
	require.Equal(t, "vgi1_credential_secret", store.bearer)
	require.Equal(t, "game.sessions.manage", store.scope)

	request.Header.Set("Authorization", "Basic invalid")
	_, err = verifier.VerifyGameServer(request)
	require.Error(t, err)
}

type sessionCredentialStoreStub struct {
	principal registry.ServicePrincipal
	bearer    string
	scope     string
}

func (s *sessionCredentialStoreStub) VerifyCredential(_ context.Context, bearer, scope string, _ []byte) (registry.ServicePrincipal, error) {
	s.bearer, s.scope = bearer, scope
	return s.principal, nil
}
