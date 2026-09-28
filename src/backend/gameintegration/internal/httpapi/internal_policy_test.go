package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/gameintegration/internal/registry"
)

type nonceMemory struct {
	mu   sync.Mutex
	seen map[string]bool
}

func (s *nonceMemory) Use(_ context.Context, nonce string, _ time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.seen == nil {
		s.seen = map[string]bool{}
	}
	if s.seen[nonce] {
		return false, nil
	}
	s.seen[nonce] = true
	return true, nil
}

type internalPolicyStore struct {
	policy registry.AuthorizationPolicy
}

func (s internalPolicyStore) LoadAuthorizationPolicy(context.Context, uuid.UUID) (registry.AuthorizationPolicy, error) {
	return s.policy, nil
}

func TestInternalPolicyRequiresFreshAuthWorkloadProof(t *testing.T) {
	now := time.Unix(1790435000, 0)
	key := []byte("0123456789abcdef0123456789abcdef")
	envID := uuid.New()
	path := "/internal/v1/authorizations/environments/" + envID.String()
	h := NewInternalPolicyHandler(WorkloadVerifier{Key: key, Now: func() time.Time { return now }, Nonces: &nonceMemory{}},
		internalPolicyStore{policy: registry.AuthorizationPolicy{EnvironmentID: envID, ApplicationID: uuid.New(), Revision: 2,
			DisplayName: "Game", RedirectURIs: []string{"https://game.example/callback"}, Providers: []string{"google"},
			PlayerScopes: []string{"game.chat.read"}}})
	request := func(nonce string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		SignWorkloadRequest(r, key, now, nonce)
		return r
	}
	r := request(uuid.NewString())
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "game.chat.read")
	require.Equal(t, r.Header.Get("X-Voice-Timestamp"), w.Header().Get("X-Voice-Response-Timestamp"))
	require.Equal(t, r.Header.Get("X-Voice-Nonce"), w.Header().Get("X-Voice-Response-Nonce"))
	require.Equal(t, responseSignature(key, http.StatusOK, r.URL.EscapedPath(), r.Header.Get("X-Voice-Timestamp"), r.Header.Get("X-Voice-Nonce"), w.Body.Bytes()), w.Header().Get("X-Voice-Response-Signature"))
	var decoded registry.AuthorizationPolicy
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &decoded))
	require.Equal(t, envID, decoded.EnvironmentID)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	require.Equal(t, http.StatusUnauthorized, w.Code)
	r = request(uuid.NewString())
	r.URL.Path = "/internal/v1/authorizations/environments/" + uuid.NewString()
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthV2WorkloadProofBindsExactBodyAndAssertionHeader(t *testing.T) {
	now := time.Unix(1790435000, 0)
	key := []byte("0123456789abcdef0123456789abcdef")
	assertion := []byte("assertion bytes A")
	body := []byte(`{"operation_id":"00000000-0000-4000-8000-000000000001"}`)
	verifier := WorkloadVerifier{Key: key, Now: func() time.Time { return now }, Nonces: &nonceMemory{}}
	request := func(nonce string) *http.Request {
		r := httptest.NewRequest(http.MethodPost, "/internal/v1/game-integrations/bindings/00000000-0000-4000-8000-000000000002/execution-permits", bytes.NewReader(body))
		r.Header.Set("X-Voice-Device-Authority", string(assertion))
		SignAssertionBoundWorkloadRequest(r, key, now, nonce, body, assertion)
		return r
	}
	r := request(uuid.NewString())
	r.Header.Set("X-Voice-Device-Authority", "assertion bytes B")
	_, err := verifier.VerifyAssertionBound(r)
	require.ErrorIs(t, err, ErrInvalidWorkloadProof, "a swapped assertion must fail before the service store is reached")

	r = request(uuid.NewString())
	r.Body = io.NopCloser(bytes.NewReader(append(append([]byte(nil), body...), ' ')))
	_, err = verifier.VerifyAssertionBound(r)
	require.ErrorIs(t, err, ErrInvalidWorkloadProof, "exact body bytes are part of the HMAC")

	r = request(uuid.NewString())
	verified, err := verifier.VerifyAssertionBound(r)
	require.NoError(t, err)
	require.Equal(t, assertion, verified)
	r.Body = io.NopCloser(bytes.NewReader(body))
	_, err = verifier.VerifyAssertionBound(r)
	require.ErrorIs(t, err, ErrInvalidWorkloadProof, "nonce replays are rejected")
}
