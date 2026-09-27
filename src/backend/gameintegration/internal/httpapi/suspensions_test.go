package httpapi

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/gameintegration/internal/registry"
	"voice/backend/pkg/integrationtest"
	voicejwt "voice/backend/pkg/jwt"
)

type testSuspensionStore struct {
	input registry.SetApplicationSuspensionInput
	app   registry.Application
	err   error
	calls int
}

func TestSuspendedApplicationStopsSignedAuthPolicyResponse(t *testing.T) {
	ctx := context.Background()
	migrations := filepath.Join("..", "..", "..", "migrations", "game_integration_db")
	pool := integrationtest.StartPostgres(t, ctx, "game_integration_test", filepath.Join(migrations, "000001_init.up.sql"))
	securityMigration, err := os.ReadFile(filepath.Join(migrations, "000002_t12_registry_security.up.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(securityMigration))
	require.NoError(t, err)
	store := &registry.Store{Pool: pool}
	ownerID, operatorID := uuid.New(), uuid.New()
	app, err := store.CreateApplication(ctx, registry.CreateApplicationInput{
		OwnerAccountID: ownerID, Name: "Auth suspension", IdempotencyKey: "auth-suspension-app",
	})
	require.NoError(t, err)
	env, err := store.ApproveSandbox(ctx, registry.ApproveSandboxInput{
		ApplicationID: app.ID, OperatorAccountID: operatorID, IdempotencyKey: "auth-suspension-env",
	})
	require.NoError(t, err)
	_, err = store.UpdateSandboxPolicy(ctx, registry.UpdateSandboxPolicyInput{
		OwnerAccountID: ownerID, ApplicationID: app.ID, EnvironmentID: env.ID,
		ExpectedRevision: env.Revision, RedirectURIs: []string{"https://game.example/callback"},
		AllowedOrigins: []string{"https://game.example"}, Providers: []string{"google"},
		PlayerScopes: []string{"game.chat.read"}, IdempotencyKey: "auth-suspension-policy",
	})
	require.NoError(t, err)
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	key := []byte("0123456789abcdef0123456789abcdef")
	handler := NewInternalPolicyHandler(WorkloadVerifier{Key: key, Now: func() time.Time { return now }, Nonces: &nonceMemory{}}, store)
	path := "/internal/v1/authorizations/environments/" + env.ID.String()
	request := func() *http.Request {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		SignWorkloadRequest(r, key, now, uuid.NewString())
		return r
	}
	w := httptest.NewRecorder()
	r := request()
	handler.ServeHTTP(w, r)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, responseSignature(key, http.StatusOK, r.URL.EscapedPath(),
		r.Header.Get("X-Voice-Timestamp"), r.Header.Get("X-Voice-Nonce"), w.Body.Bytes()),
		w.Header().Get("X-Voice-Response-Signature"), "Auth's active policy response must remain correctly signed")

	_, err = store.SetApplicationSuspension(ctx, registry.SetApplicationSuspensionInput{
		ApplicationID: app.ID, OperatorAccountID: operatorID, Suspended: true, IdempotencyKey: "auth-suspension-on",
	})
	require.NoError(t, err)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, request())
	require.Equal(t, http.StatusServiceUnavailable, w.Code)
	require.Contains(t, w.Body.String(), "APP_SUSPENDED")
	require.Empty(t, w.Header().Get("X-Voice-Response-Signature"), "suspended policy must never return a signed authorization result")
}

func (s *testSuspensionStore) CreateApplication(context.Context, registry.CreateApplicationInput) (registry.Application, error) {
	return registry.Application{}, nil
}

func (s *testSuspensionStore) SetApplicationSuspension(_ context.Context, in registry.SetApplicationSuspensionInput) (registry.Application, error) {
	s.calls++
	s.input = in
	return s.app, s.err
}

func TestApplicationSuspensionRequiresAllowlistedOperatorAndMapsTransitionConflict(t *testing.T) {
	operatorID, appID := uuid.New(), uuid.New()
	store := &testSuspensionStore{app: registry.Application{ID: appID, Status: "suspended", Revision: 3}}
	handler := NewHandler(testValidator{claims: voicejwt.Claims{UserID: operatorID.String(), AccountType: "regular"}}, store)
	request := func() *http.Request {
		r := httptest.NewRequest(http.MethodPut,
			"/api/v1/game-integrations/applications/"+appID.String()+"/suspension",
			bytes.NewBufferString(`{"suspended":true}`))
		r.Header.Set("Authorization", "Bearer player-token")
		r.Header.Set("Idempotency-Key", "suspend-1")
		return r
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, request())
	require.Equal(t, http.StatusForbidden, w.Code)
	require.Contains(t, w.Body.String(), "OPERATOR_REQUIRED")
	require.Zero(t, store.calls, "an unconfigured operator never reaches the store")

	handler.OperatorAccounts = map[uuid.UUID]struct{}{operatorID: {}}
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, request())
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, 1, store.calls)
	require.Equal(t, operatorID, store.input.OperatorAccountID)
	require.Equal(t, appID, store.input.ApplicationID)
	require.True(t, store.input.Suspended)
	require.Equal(t, "suspend-1", store.input.IdempotencyKey)
	require.NotContains(t, w.Body.String(), "secret")
	require.NotContains(t, w.Body.String(), "proof")

	store.err = registry.ErrApplicationStateConflict
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, request())
	require.Equal(t, http.StatusConflict, w.Code)
	require.Contains(t, w.Body.String(), "APPLICATION_STATE_CONFLICT")
}
