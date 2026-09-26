package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/gameintegration/internal/registry"
	voicejwt "voice/backend/pkg/jwt"
)

type testValidator struct {
	claims voicejwt.Claims
}

func (v testValidator) Validate(r *http.Request) (voicejwt.Claims, string) {
	if r.Header.Get("Authorization") != "Bearer player-token" {
		return voicejwt.Claims{}, "invalid_token"
	}
	return v.claims, ""
}

type testApplicationStore struct {
	created registry.CreateApplicationInput
	result  registry.Application
}

func (s *testApplicationStore) CreateApplication(_ context.Context, in registry.CreateApplicationInput) (registry.Application, error) {
	s.created = in
	return s.result, nil
}

func TestCreateApplicationRequiresVerifiedRegularOwner(t *testing.T) {
	owner := uuid.New()
	appID := uuid.New()
	store := &testApplicationStore{result: registry.Application{
		ID: appID, OwnerAccountID: owner, Name: "HerdTrip", Status: "draft", Revision: 1,
	}}
	handler := NewHandler(testValidator{claims: voicejwt.Claims{UserID: owner.String(), AccountType: "regular"}}, store)
	request := func(token string) *http.Request {
		r := httptest.NewRequest(http.MethodPost, "/api/v1/game-integrations/applications", bytes.NewBufferString(`{"name":"HerdTrip"}`))
		r.Header.Set("X-Voice-User-Id", uuid.NewString()) // untrusted direct-call header
		r.Header.Set("Idempotency-Key", "app-1")
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		return r
	}

	missing := httptest.NewRecorder()
	handler.ServeHTTP(missing, request(""))
	require.Equal(t, http.StatusUnauthorized, missing.Code)
	require.Equal(t, uuid.Nil, store.created.OwnerAccountID)

	valid := httptest.NewRecorder()
	handler.ServeHTTP(valid, request("player-token"))
	require.Equal(t, http.StatusCreated, valid.Code, valid.Body.String())
	require.Equal(t, owner, store.created.OwnerAccountID)
	require.Equal(t, "app-1", store.created.IdempotencyKey)
	var response map[string]any
	require.NoError(t, json.Unmarshal(valid.Body.Bytes(), &response))
	require.Equal(t, appID.String(), response["application_id"])
}

func TestCreateApplicationRejectsGuestAndUnknownFields(t *testing.T) {
	owner := uuid.New()
	store := &testApplicationStore{}
	handler := NewHandler(testValidator{claims: voicejwt.Claims{UserID: owner.String(), AccountType: "guest"}}, store)
	r := httptest.NewRequest(http.MethodPost, "/api/v1/game-integrations/applications", bytes.NewBufferString(`{"name":"Game"}`))
	r.Header.Set("Authorization", "Bearer player-token")
	r.Header.Set("Idempotency-Key", "app-1")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	require.Equal(t, http.StatusForbidden, w.Code)
	require.Equal(t, uuid.Nil, store.created.OwnerAccountID)

	handler = NewHandler(testValidator{claims: voicejwt.Claims{UserID: owner.String(), AccountType: "regular"}}, store)
	r = httptest.NewRequest(http.MethodPost, "/api/v1/game-integrations/applications", bytes.NewBufferString(`{"name":"Game","owner_account_id":"`+uuid.NewString()+`"}`))
	r.Header.Set("Authorization", "Bearer player-token")
	r.Header.Set("Idempotency-Key", "app-1")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Equal(t, uuid.Nil, store.created.OwnerAccountID)
}

type unavailableValidator struct{}

func (unavailableValidator) Validate(*http.Request) (voicejwt.Claims, string) {
	return voicejwt.Claims{}, "auth_unavailable"
}

func TestCreateApplicationFailsClosedWhenAuthorityUnavailable(t *testing.T) {
	store := &testApplicationStore{}
	handler := NewHandler(unavailableValidator{}, store)
	r := httptest.NewRequest(http.MethodPost, "/api/v1/game-integrations/applications", bytes.NewBufferString(`{"name":"Game"}`))
	r.Header.Set("Authorization", "Bearer player-token")
	r.Header.Set("Idempotency-Key", "app-1")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	require.Equal(t, http.StatusServiceUnavailable, w.Code)
	require.Equal(t, uuid.Nil, store.created.OwnerAccountID)
}
