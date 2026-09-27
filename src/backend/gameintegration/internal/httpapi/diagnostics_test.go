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

type testDiagnosticsStore struct {
	ownerID uuid.UUID
	appID   uuid.UUID
	result  registry.OwnerDiagnostics
	err     error
	calls   int
}

func (s *testDiagnosticsStore) CreateApplication(context.Context, registry.CreateApplicationInput) (registry.Application, error) {
	return registry.Application{}, nil
}

func (s *testDiagnosticsStore) LoadOwnerDiagnostics(_ context.Context, ownerID, appID uuid.UUID) (registry.OwnerDiagnostics, error) {
	s.calls++
	s.ownerID, s.appID = ownerID, appID
	return s.result, s.err
}

func TestOwnerDiagnosticsRouteReturnsOnlySanitizedProviderAndCallbackState(t *testing.T) {
	ownerID, appID := uuid.New(), uuid.New()
	store := &testDiagnosticsStore{result: registry.OwnerDiagnostics{
		ApplicationID: appID, ApplicationStatus: "sandbox", ApplicationAdmission: "operator_approved",
		ProviderAdmission: "not_verified", Quota: registry.QuotaDiagnostics{Used: 7, Limit: 120},
	}}
	handler := NewHandler(testValidator{claims: voicejwt.Claims{UserID: ownerID.String(), AccountType: "regular"}}, store)
	request := httptest.NewRequest(http.MethodGet,
		"/api/v1/game-integrations/applications/"+appID.String()+"/diagnostics", nil)
	request.Header.Set("Authorization", "Bearer player-token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, request)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, 1, store.calls)
	require.Equal(t, ownerID, store.ownerID)
	require.Equal(t, appID, store.appID)
	require.Contains(t, w.Body.String(), "operator_approved")
	require.Contains(t, w.Body.String(), "not_verified")
	require.NotContains(t, w.Body.String(), "callback_url")
	require.NotContains(t, w.Body.String(), "secret")
	require.NotContains(t, w.Body.String(), "provider_proof")
	require.NotContains(t, w.Body.String(), "oauth_subject")
}

func TestOwnerDiagnosticsRejectsInvalidBearerAndForeignOwner(t *testing.T) {
	ownerID, appID := uuid.New(), uuid.New()
	store := &testDiagnosticsStore{err: registry.ErrDiagnosticsDenied}
	handler := NewHandler(testValidator{claims: voicejwt.Claims{UserID: ownerID.String(), AccountType: "regular"}}, store)
	request := func() *http.Request {
		r := httptest.NewRequest(http.MethodGet,
			"/api/v1/game-integrations/applications/"+appID.String()+"/diagnostics", nil)
		r.Header.Set("Authorization", "Bearer player-token")
		return r
	}
	unauthenticated := request()
	unauthenticated.Header.Set("Authorization", "Bearer invalid-token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, unauthenticated)
	require.Equal(t, http.StatusUnauthorized, w.Code)
	require.Zero(t, store.calls, "invalid bearer never reaches the diagnostics store")

	w = httptest.NewRecorder()
	handler.ServeHTTP(w, request())
	require.Equal(t, http.StatusForbidden, w.Code)
	require.Contains(t, w.Body.String(), "DIAGNOSTICS_DENIED")
	require.Equal(t, 1, store.calls)
	require.NotContains(t, w.Body.String(), appID.String(), "foreign owner must not learn whether the app exists")
}
