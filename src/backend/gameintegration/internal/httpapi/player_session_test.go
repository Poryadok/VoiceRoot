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

type playerSessionSnapshotStub struct {
	accountID, profileID, sessionID uuid.UUID
	calls                           int
	err                             error
}

func (s *playerSessionSnapshotStub) ListPlayerSessionSnapshots(_ context.Context, accountID, profileID uuid.UUID) ([]registry.PlayerSessionSnapshot, error) {
	s.calls++
	s.accountID, s.profileID = accountID, profileID
	if s.err != nil {
		return nil, s.err
	}
	return []registry.PlayerSessionSnapshot{{SessionID: uuid.New(), Kind: "match", RosterRevision: 9}}, nil
}

func (s *playerSessionSnapshotStub) GetPlayerSessionSnapshot(_ context.Context, accountID, profileID, sessionID uuid.UUID) (registry.PlayerSessionSnapshot, error) {
	s.calls++
	s.accountID, s.profileID, s.sessionID = accountID, profileID, sessionID
	if s.err != nil {
		return registry.PlayerSessionSnapshot{}, s.err
	}
	chatID := uuid.New()
	return registry.PlayerSessionSnapshot{SessionID: sessionID, Kind: "match", RosterRevision: 9, ChatID: &chatID}, nil
}

func TestPlayerSessionSnapshotRequiresCurrentBearerAndUsesClaimIdentity(t *testing.T) {
	accountID, profileID, sessionID := uuid.New(), uuid.New(), uuid.New()
	store := &playerSessionSnapshotStub{}
	handler := NewPlayerSessionSnapshotHandler(Authorizer{
		Tokens: sessionConsentValidator{claims: voicejwt.Claims{UserID: accountID.String(), ProfileID: profileID.String(), JTI: "jti", SessionEpoch: 1}},
		State:  sessionConsentState{},
	}, store)
	path := "/api/v1/sessions/" + sessionID.String()

	unauthenticated := httptest.NewRecorder()
	handler.ServeHTTP(unauthenticated, httptest.NewRequest(http.MethodGet, path, nil))
	require.Equal(t, http.StatusUnauthorized, unauthenticated.Code)
	require.Zero(t, store.calls)

	request := httptest.NewRequest(http.MethodGet, path, nil)
	request.Header.Set("Authorization", "Bearer participant")
	request.Header.Set("X-Voice-User-Id", uuid.NewString())
	request.Header.Set("X-Voice-Profile-Id", uuid.NewString())
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, "no-store", response.Header().Get("Cache-Control"))
	require.Equal(t, accountID, store.accountID)
	require.Equal(t, profileID, store.profileID)
	require.Equal(t, sessionID, store.sessionID)
}

func TestPlayerSessionSnapshotHidesUnknownForeignAndStaleSessions(t *testing.T) {
	store := &playerSessionSnapshotStub{err: registry.ErrPlayerSessionDenied}
	handler := NewPlayerSessionSnapshotHandler(Authorizer{
		Tokens: sessionConsentValidator{claims: voicejwt.Claims{UserID: uuid.NewString(), ProfileID: uuid.NewString(), JTI: "jti", SessionEpoch: 1}},
		State:  sessionConsentState{},
	}, store)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/11111111-1111-4111-8111-111111111111", nil)
	request.Header.Set("Authorization", "Bearer participant")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	require.Equal(t, http.StatusNotFound, response.Code)
	require.Equal(t, 1, store.calls)
}

func TestPlayerSessionListUsesBearerIdentityAndNoStore(t *testing.T) {
	accountID, profileID := uuid.New(), uuid.New()
	store := &playerSessionSnapshotStub{}
	handler := NewPlayerSessionListHandler(Authorizer{Tokens: sessionConsentValidator{claims: voicejwt.Claims{UserID: accountID.String(), ProfileID: profileID.String(), JTI: "jti", SessionEpoch: 1}}, State: sessionConsentState{}}, store)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/player/sessions/me", nil)
	req.Header.Set("Authorization", "Bearer participant")
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)
	require.Equal(t, "private, no-store", resp.Header().Get("Cache-Control"))
	require.Equal(t, accountID, store.accountID)
	require.Equal(t, profileID, store.profileID)
	require.Contains(t, resp.Body.String(), "sessions")
}
