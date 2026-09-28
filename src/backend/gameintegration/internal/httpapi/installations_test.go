package httpapi

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/gameintegration/internal/registry"
	voicejwt "voice/backend/pkg/jwt"
)

type testInstallationStore struct {
	created registry.CreateInstallationInput
	result  registry.Installation
	err     error
	calls   int
}

func (s *testInstallationStore) CreateApplication(context.Context, registry.CreateApplicationInput) (registry.Application, error) {
	return registry.Application{}, nil
}

func (s *testInstallationStore) CreateInstallation(_ context.Context, in registry.CreateInstallationInput) (registry.Installation, error) {
	s.calls++
	s.created = in
	return s.result, s.err
}

func TestInstallationRegistrationBindsAuthenticatedOwnerAndPathIDs(t *testing.T) {
	owner, appID, envID, installationID, botID := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	store := &testInstallationStore{result: registry.Installation{
		ID: installationID, ApplicationID: appID, EnvironmentID: envID,
		BotID: botID,
		CallbackURL: "https://callback.example/callback-v1", Status: "active",
	}}
	handler := NewHandler(testValidator{claims: voicejwt.Claims{UserID: owner.String(), AccountType: "regular"}}, store)
	request := httptest.NewRequest(http.MethodPost,
		"/api/v1/game-integrations/applications/"+appID.String()+"/environments/"+envID.String()+"/installations",
		bytes.NewBufferString(`{"callback_url":"https://callback.example/callback-v1","bot_id":"`+botID.String()+`"}`))
	request.Header.Set("Authorization", "Bearer player-token")
	request.Header.Set("Idempotency-Key", "installation-1")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, request)

	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	require.Equal(t, 1, store.calls)
	require.Equal(t, owner, store.created.OwnerAccountID)
	require.Equal(t, appID, store.created.ApplicationID)
	require.Equal(t, envID, store.created.EnvironmentID)
	require.Equal(t, botID, store.created.BotID)
	require.Equal(t, "https://callback.example/callback-v1", store.created.CallbackURL)
	require.Equal(t, "installation-1", store.created.IdempotencyKey)
	require.NotContains(t, w.Body.String(), "secret")
	require.NotContains(t, w.Body.String(), "proof")
}

func TestInstallationRegistrationRejectsOwnerAndBodyOverridesBeforeStore(t *testing.T) {
	owner, appID, envID := uuid.New(), uuid.New(), uuid.New()
	store := &testInstallationStore{}
	request := func(body string) *http.Request {
		r := httptest.NewRequest(http.MethodPost,
			"/api/v1/game-integrations/applications/"+appID.String()+"/environments/"+envID.String()+"/installations",
		bytes.NewBufferString(body))
		r.Header.Set("Authorization", "Bearer player-token")
		r.Header.Set("Idempotency-Key", "installation-1")
		return r
	}
	handler := NewHandler(testValidator{claims: voicejwt.Claims{UserID: owner.String(), AccountType: "regular"}}, store)
	validBody := `{"callback_url":"https://callback.example/callback-v1","bot_id":"` + uuid.NewString() + `"}`
	unauthenticated := request(validBody)
	unauthenticated.Header.Set("Authorization", "Bearer invalid-token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, unauthenticated)
	require.Equal(t, http.StatusUnauthorized, w.Code)
	require.Zero(t, store.calls, "invalid bearer must not call any store or quota/audit path")

	handler = NewHandler(testValidator{claims: voicejwt.Claims{UserID: owner.String(), AccountType: "guest"}}, store)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, request(validBody))
	require.Equal(t, http.StatusForbidden, w.Code)
	require.Zero(t, store.calls)

	handler = NewHandler(testValidator{claims: voicejwt.Claims{UserID: owner.String(), AccountType: "regular"}}, store)
	for _, body := range []string{
		`{"callback_url":"https://callback.example/callback-v1","bot_id":"` + uuid.NewString() + `","owner_account_id":"` + uuid.NewString() + `"}`,
		`{"callback_url":"https://callback.example/callback-v1","bot_id":"` + uuid.NewString() + `","application_id":"` + uuid.NewString() + `"}`,
		`{"callback_url":"https://callback.example/callback-v1","bot_id":"` + uuid.NewString() + `","environment_id":"` + uuid.NewString() + `"}`,
	} {
		w = httptest.NewRecorder()
		handler.ServeHTTP(w, request(body))
		require.Equal(t, http.StatusBadRequest, w.Code, body)
		require.Zero(t, store.calls, "owner, application, and environment come from trusted claims and path IDs")
	}
}

func TestInstallationRegistrationMapsSafeDestinationAndScopeErrors(t *testing.T) {
	owner, appID, envID := uuid.New(), uuid.New(), uuid.New()
	store := &testInstallationStore{err: registry.ErrUnsafeCallbackURL}
	handler := NewHandler(testValidator{claims: voicejwt.Claims{UserID: owner.String(), AccountType: "regular"}}, store)
	request := func() *http.Request {
		r := httptest.NewRequest(http.MethodPost,
			"/api/v1/game-integrations/applications/"+appID.String()+"/environments/"+envID.String()+"/installations",
		bytes.NewBufferString(`{"callback_url":"https://127.0.0.1/callback-v1","bot_id":"`+uuid.NewString()+`"}`))
		r.Header.Set("Authorization", "Bearer player-token")
		r.Header.Set("Idempotency-Key", "installation-1")
		return r
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, request())
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "CALLBACK_URL_REJECTED")

	store.err = registry.ErrInstallationConflict
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, request())
	require.Equal(t, http.StatusForbidden, w.Code)
	require.Contains(t, w.Body.String(), "INSTALLATION_DENIED")

	store.err = &registry.RateLimitError{RetryAfter: 55 * time.Second}
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, request())
	require.Equal(t, http.StatusTooManyRequests, w.Code)
	require.Contains(t, w.Body.String(), "RATE_LIMITED")
	require.Equal(t, "55", w.Header().Get("Retry-After"))
}
