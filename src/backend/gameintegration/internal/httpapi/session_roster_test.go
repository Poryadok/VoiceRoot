package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/gameintegration/internal/registry"
)

type rosterUpdateOrchestratorStub struct {
	sessionOrchestratorStub
	updatedSessionID uuid.UUID
	updated          registry.SessionRosterUpdateInput
	updateN          int
}

func (s *rosterUpdateOrchestratorStub) UpdateSessionRoster(_ context.Context, principal registry.SessionPrincipal, sessionID uuid.UUID, in registry.SessionRosterUpdateInput) (registry.SessionOperation, error) {
	s.principal = principal
	s.updatedSessionID = sessionID
	s.updated = in
	s.updateN++
	return s.operation, nil
}

func TestSessionRosterRouteForwardsVerifiedScopeAndCanonicalHost(t *testing.T) {
	appID, envID := uuid.New(), uuid.New()
	operationID, sessionID := uuid.New(), uuid.New()
	member, host := uuid.MustParse("11111111-1111-4111-8111-111111111111"), uuid.MustParse("22222222-2222-4222-8222-222222222222")
	principal := registry.SessionPrincipal{ApplicationID: appID, EnvironmentID: envID, Scopes: []string{"game.sessions.manage"}}
	orchestrator := &rosterUpdateOrchestratorStub{sessionOrchestratorStub: sessionOrchestratorStub{operation: registry.SessionOperation{
		OperationID: operationID, SessionID: sessionID, Status: "pending", SessionStatus: "active", Stage: "roster_accepted",
	}}}
	handler := NewSessionHandler(sessionServerValidator{principal: principal}, orchestrator)
	body := `{"operation_id":"` + operationID.String() + `","roster_revision":2,"roster_complete":true,"members":["` + member.String() + `","` + host.String() + `"],"host_profile_id":"` + host.String() + `"}`
	response := sessionRequest(handler, http.MethodPut, "/api/v1/sessions/"+sessionID.String()+"/roster", body)
	require.Equal(t, http.StatusAccepted, response.Code, response.Body.String())
	require.Equal(t, appID, orchestrator.principal.ApplicationID)
	require.Equal(t, envID, orchestrator.principal.EnvironmentID)
	require.Equal(t, sessionID, orchestrator.updatedSessionID)
	require.Equal(t, operationID, orchestrator.updated.OperationID)
	require.EqualValues(t, 2, orchestrator.updated.RosterRevision)
	require.True(t, orchestrator.updated.RosterComplete)
	require.Equal(t, []uuid.UUID{member, host}, orchestrator.updated.Members)
	require.Equal(t, &host, orchestrator.updated.HostProfileID)
	require.Equal(t, 1, orchestrator.updateN)
}

func TestSessionRosterRouteRequiresManageScope(t *testing.T) {
	principal := registry.SessionPrincipal{ApplicationID: uuid.New(), EnvironmentID: uuid.New(), Scopes: []string{"game.events.write"}}
	orchestrator := &rosterUpdateOrchestratorStub{}
	handler := NewSessionHandler(sessionServerValidator{principal: principal}, orchestrator)
	response := sessionRequest(handler, http.MethodPut, "/api/v1/sessions/"+uuid.NewString()+"/roster", `{}`)
	require.Equal(t, http.StatusForbidden, response.Code, response.Body.String())
	require.Zero(t, orchestrator.updateN, "the unauthorized app scope must not reach registry")
}

func TestSessionRosterRouteRejectsMalformedBodyAndIdentifiersBeforeRegistry(t *testing.T) {
	principal := registry.SessionPrincipal{ApplicationID: uuid.New(), EnvironmentID: uuid.New(), Scopes: []string{"game.sessions.manage"}}
	orchestrator := &rosterUpdateOrchestratorStub{}
	handler := NewSessionHandler(sessionServerValidator{principal: principal}, orchestrator)
	sessionID := uuid.NewString()
	base := `"operation_id":"` + uuid.NewString() + `","roster_revision":2,"roster_complete":true,"members":[]`
	for _, test := range []struct {
		path, body string
		wantStatus int
	}{
		{"/api/v1/sessions/00000000-0000-4000-8000-00000000000A/roster", `{` + base + `}`, http.StatusNotFound},
		{"/api/v1/sessions/" + sessionID + "/roster", `{` + base + `,"host_profile_id":"00000000-0000-4000-8000-00000000000A"}`, http.StatusBadRequest},
		{"/api/v1/sessions/" + sessionID + "/roster", `{` + base + `,"roster_complete":false}`, http.StatusBadRequest},
		{"/api/v1/sessions/" + sessionID + "/roster", `{` + base + `,"members":null}`, http.StatusBadRequest},
		{"/api/v1/sessions/" + sessionID + "/roster", `{` + base + `,"unexpected":true}`, http.StatusBadRequest},
		{"/api/v1/sessions/" + sessionID + "/roster", `{` + base + `,"operation_id":"` + uuid.NewString() + `"}`, http.StatusBadRequest},
	} {
		response := sessionRequest(handler, http.MethodPut, test.path, test.body)
		require.Equal(t, test.wantStatus, response.Code, test.body)
	}
	require.Zero(t, orchestrator.updateN)
}

func TestSessionRosterRouteRejectsUnsortedMembersWithFrozenErrorEnvelope(t *testing.T) {
	principal := registry.SessionPrincipal{ApplicationID: uuid.New(), EnvironmentID: uuid.New(), Scopes: []string{"game.sessions.manage"}}
	orchestrator := &rosterUpdateOrchestratorStub{}
	handler := NewSessionHandler(sessionServerValidator{principal: principal}, orchestrator)
	body := `{"operation_id":"` + uuid.NewString() + `","roster_revision":2,"roster_complete":true,"members":["22222222-2222-4222-8222-222222222222","11111111-1111-4111-8111-111111111111"]}`
	response := sessionRequest(handler, http.MethodPut, "/api/v1/sessions/"+uuid.NewString()+"/roster", body)
	require.Equal(t, http.StatusBadRequest, response.Code)
	var envelope map[string]any
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &envelope))
	require.Equal(t, map[string]any{"error_code": "INVALID_ARGUMENT", "retryable": false}, envelope)
	require.Zero(t, orchestrator.updateN, "noncanonical member ordering never reaches the registry")
}

func TestSessionRosterRouteWrongMethodAndPathDoNotCallRegistry(t *testing.T) {
	principal := registry.SessionPrincipal{ApplicationID: uuid.New(), EnvironmentID: uuid.New(), Scopes: []string{"game.sessions.manage"}}
	orchestrator := &rosterUpdateOrchestratorStub{}
	handler := NewSessionHandler(sessionServerValidator{principal: principal}, orchestrator)
	sessionID := uuid.NewString()
	body := `{}`
	for _, test := range []struct{ method, path string }{
		{http.MethodPost, "/api/v1/sessions/" + sessionID + "/roster"},
		{http.MethodPut, "/api/v1/sessions/" + sessionID + "/close"},
		{http.MethodPut, "/api/v1/sessions/not-a-uuid/roster"},
	} {
		response := sessionRequest(handler, test.method, test.path, body)
		require.Equal(t, http.StatusNotFound, response.Code, test.method+" "+test.path)
	}
	require.Zero(t, orchestrator.updateN)
}
