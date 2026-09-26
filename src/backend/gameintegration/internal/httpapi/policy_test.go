package httpapi

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/gameintegration/internal/registry"
	voicejwt "voice/backend/pkg/jwt"
)

type testPolicyStore struct {
	input registry.UpdateSandboxPolicyInput
}

func (s *testPolicyStore) CreateApplication(context.Context, registry.CreateApplicationInput) (registry.Application, error) {
	return registry.Application{}, nil
}

func (s *testPolicyStore) UpdateSandboxPolicy(_ context.Context, in registry.UpdateSandboxPolicyInput) (registry.Environment, error) {
	s.input = in
	return registry.Environment{ID: in.EnvironmentID, ApplicationID: in.ApplicationID, Kind: "sandbox", Status: "active", Revision: in.ExpectedRevision + 1}, nil
}

func TestUpdateSandboxPolicyUsesVerifiedOwnerAndRevision(t *testing.T) {
	owner, appID, envID := uuid.New(), uuid.New(), uuid.New()
	store := &testPolicyStore{}
	h := NewHandler(testValidator{claims: voicejwt.Claims{UserID: owner.String(), AccountType: "regular"}}, store)
	r := httptest.NewRequest(http.MethodPut, "/api/v1/game-integrations/applications/"+appID.String()+
		"/environments/"+envID.String()+"/policy", bytes.NewBufferString(`{"expected_revision":1,"redirect_uris":["https://game.example/callback"],"allowed_origins":["https://game.example"],"providers":["google"],"player_scopes":["game.chat.read"]}`))
	r.Header.Set("Authorization", "Bearer player-token")
	r.Header.Set("Idempotency-Key", "policy-1")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, owner, store.input.OwnerAccountID)
	require.Equal(t, appID, store.input.ApplicationID)
	require.Equal(t, envID, store.input.EnvironmentID)
	require.Equal(t, int64(1), store.input.ExpectedRevision)
}
