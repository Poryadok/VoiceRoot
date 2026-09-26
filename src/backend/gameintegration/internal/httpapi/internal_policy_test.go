package httpapi

import (
	"context"
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
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	require.Equal(t, http.StatusUnauthorized, w.Code)
	r = request(uuid.NewString())
	r.URL.Path = "/internal/v1/authorizations/environments/" + uuid.NewString()
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	require.Equal(t, http.StatusUnauthorized, w.Code)
}
