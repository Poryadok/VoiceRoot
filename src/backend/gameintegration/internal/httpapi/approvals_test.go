package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/gameintegration/internal/registry"
	voicejwt "voice/backend/pkg/jwt"
)

type testApprovalStore struct {
	approved registry.ApproveSandboxInput
}

func (s *testApprovalStore) CreateApplication(context.Context, registry.CreateApplicationInput) (registry.Application, error) {
	return registry.Application{}, nil
}

func (s *testApprovalStore) ApproveSandbox(_ context.Context, in registry.ApproveSandboxInput) (registry.Environment, error) {
	s.approved = in
	return registry.Environment{ID: uuid.New(), ApplicationID: in.ApplicationID, Kind: "sandbox", Status: "active"}, nil
}

func TestSandboxApprovalRequiresConfiguredOperator(t *testing.T) {
	operatorID, appID := uuid.New(), uuid.New()
	store := &testApprovalStore{}
	handler := NewHandler(testValidator{claims: voicejwt.Claims{UserID: operatorID.String(), AccountType: "regular"}}, store)
	request := func() *http.Request {
		r := httptest.NewRequest(http.MethodPost, "/api/v1/game-integrations/applications/"+appID.String()+"/admissions/sandbox", nil)
		r.Header.Set("Authorization", "Bearer player-token")
		r.Header.Set("Idempotency-Key", "approve-1")
		return r
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, request())
	require.Equal(t, http.StatusForbidden, w.Code)
	require.Equal(t, uuid.Nil, store.approved.ApplicationID)
	handler.OperatorAccounts = map[uuid.UUID]struct{}{operatorID: {}}
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, request())
	require.Equal(t, http.StatusCreated, w.Code)
	require.Equal(t, appID, store.approved.ApplicationID)
	require.Equal(t, operatorID, store.approved.OperatorAccountID)
}
