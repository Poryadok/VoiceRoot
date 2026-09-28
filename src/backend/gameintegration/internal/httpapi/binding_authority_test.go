package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/gameintegration/internal/registry"
)

type bindingAuthorityStoreStub struct {
	authority registry.PlayerBindingAuthority
	err       error
	readID    uuid.UUID
}

func (s *bindingAuthorityStoreStub) LoadPlayerBindingAuthority(_ context.Context, id uuid.UUID) (registry.PlayerBindingAuthority, error) {
	s.readID = id
	return s.authority, s.err
}

func TestInternalBindingAuthorityRequiresAuthenticatedAuthWorkload(t *testing.T) {
	now := time.Unix(1790435000, 0)
	key := []byte("0123456789abcdef0123456789abcdef")
	bindingID := uuid.New()
	store := &bindingAuthorityStoreStub{authority: registry.PlayerBindingAuthority{
		ApplicationID: uuid.New(), EnvironmentID: uuid.New(), BindingID: bindingID,
		Status: "active", BindingRevision: 4, CharacterContext: json.RawMessage(`{"character_id":"private-game-context"}`),
	}}
	handler := NewInternalBindingAuthorityHandler(WorkloadVerifier{Key: key, Now: func() time.Time { return now }, Nonces: &nonceMemory{}}, store)
	path := "/internal/v1/bindings/" + bindingID.String() + "/authority"
	request := func(nonce string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		SignWorkloadRequest(r, key, now, nonce)
		return r
	}

	unauthenticated := httptest.NewRecorder()
	handler.ServeHTTP(unauthenticated, httptest.NewRequest(http.MethodGet, path, nil))
	require.Equal(t, http.StatusUnauthorized, unauthenticated.Code)

	r := request(uuid.NewString())
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, bindingID, store.readID)
	require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	require.Equal(t, r.Header.Get("X-Voice-Nonce"), w.Header().Get("X-Voice-Response-Nonce"))
	require.Equal(t, responseSignature(key, http.StatusOK, r.URL.EscapedPath(), r.Header.Get("X-Voice-Timestamp"),
		r.Header.Get("X-Voice-Nonce"), w.Body.Bytes()), w.Header().Get("X-Voice-Response-Signature"))
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, map[string]any{
		"application_id": store.authority.ApplicationID.String(),
		"environment_id": store.authority.EnvironmentID.String(),
		"binding_id":     bindingID.String(), "status": "active", "binding_revision": float64(4),
		"character_context": map[string]any{"character_id": "private-game-context"},
	}, body)
	require.NotContains(t, body, "account_id")
	require.NotContains(t, body, "profile_id")
	require.NotContains(t, body, "actor_id")

	replay := httptest.NewRecorder()
	handler.ServeHTTP(replay, r)
	require.Equal(t, http.StatusUnauthorized, replay.Code)
}

func TestInternalBindingAuthorityRejectsCallerAuthorityAndUnknownRoutes(t *testing.T) {
	now := time.Unix(1790435000, 0)
	key := []byte("0123456789abcdef0123456789abcdef")
	bindingID := uuid.New()
	store := &bindingAuthorityStoreStub{authority: registry.PlayerBindingAuthority{
		ApplicationID: uuid.New(), EnvironmentID: uuid.New(), BindingID: bindingID,
		Status: "revoked", BindingRevision: 5, CharacterContext: json.RawMessage(`null`),
	}}
	handler := NewInternalBindingAuthorityHandler(WorkloadVerifier{Key: key, Now: func() time.Time { return now }, Nonces: &nonceMemory{}}, store)
	r := httptest.NewRequest(http.MethodGet, "/internal/v1/bindings/"+bindingID.String()+"/authority?profile_id="+uuid.NewString(), nil)
	SignWorkloadRequest(r, key, now, uuid.NewString())
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	require.Equal(t, http.StatusUnauthorized, w.Code)
	require.Equal(t, uuid.Nil, store.readID)

	unknown := httptest.NewRequest(http.MethodPost, "/internal/v1/bindings/"+bindingID.String()+"/authority", nil)
	SignWorkloadRequest(unknown, key, now, uuid.NewString())
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, unknown)
	require.Equal(t, http.StatusNotFound, w.Code)
	require.Equal(t, uuid.Nil, store.readID)
}
