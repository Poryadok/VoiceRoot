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

type testCredentialStore struct {
	issued  registry.IssueCredentialInput
	revoked [4]uuid.UUID
}

func (s *testCredentialStore) CreateApplication(context.Context, registry.CreateApplicationInput) (registry.Application, error) {
	return registry.Application{}, nil
}

func (s *testCredentialStore) IssueCredential(_ context.Context, in registry.IssueCredentialInput) (registry.Credential, error) {
	s.issued = in
	return registry.Credential{ID: uuid.New(), EnvironmentID: in.EnvironmentID, Scopes: in.Scopes, Secret: "secret", Generation: 1}, nil
}

func (s *testCredentialStore) RevokeCredential(_ context.Context, owner, app, env, credential uuid.UUID) error {
	s.revoked = [4]uuid.UUID{owner, app, env, credential}
	return nil
}

func TestCredentialIssueDerivesOwnerAndRequiresDeploymentKey(t *testing.T) {
	owner, appID, envID := uuid.New(), uuid.New(), uuid.New()
	store := &testCredentialStore{}
	h := NewHandler(testValidator{claims: voicejwt.Claims{UserID: owner.String(), AccountType: "regular"}}, store)
	request := func() *http.Request {
		r := httptest.NewRequest(http.MethodPost, "/api/v1/game-integrations/applications/"+appID.String()+"/environments/"+envID.String()+"/credentials",
			bytes.NewBufferString(`{"scopes":["game.events.write"]}`))
		r.Header.Set("Authorization", "Bearer player-token")
		r.Header.Set("Idempotency-Key", "issue-1")
		return r
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, request())
	require.Equal(t, http.StatusServiceUnavailable, w.Code)
	h.CredentialKey = []byte("0123456789abcdef0123456789abcdef")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, request())
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	require.Equal(t, owner, store.issued.OwnerAccountID)
	require.Equal(t, appID, store.issued.ApplicationID)
	require.Equal(t, envID, store.issued.EnvironmentID)
	require.Equal(t, []string{"game.events.write"}, store.issued.Scopes)
	require.NotContains(t, w.Body.String(), owner.String())
}

func TestCredentialRevokeRequiresVerifiedOwner(t *testing.T) {
	owner, appID, envID, credentialID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	store := &testCredentialStore{}
	h := NewHandler(testValidator{claims: voicejwt.Claims{UserID: owner.String(), AccountType: "regular"}}, store)
	r := httptest.NewRequest(http.MethodDelete, "/api/v1/game-integrations/applications/"+appID.String()+
		"/environments/"+envID.String()+"/credentials/"+credentialID.String(), nil)
	r.Header.Set("Authorization", "Bearer player-token")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	require.Equal(t, http.StatusNoContent, w.Code)
	require.Equal(t, [4]uuid.UUID{owner, appID, envID, credentialID}, store.revoked)
}
