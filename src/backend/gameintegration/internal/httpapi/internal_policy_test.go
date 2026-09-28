package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
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
	calls  int
}

func (s *internalPolicyStore) LoadAuthorizationPolicy(context.Context, uuid.UUID) (registry.AuthorizationPolicy, error) {
	s.calls++
	return s.policy, nil
}

type failingNonceStore struct{}

func (failingNonceStore) Use(context.Context, string, time.Duration) (bool, error) {
	return false, errors.New("nonce store unavailable")
}

func TestInternalPolicyRequiresFreshAuthWorkloadProof(t *testing.T) {
	now := time.Unix(1790435000, 0)
	key := []byte("0123456789abcdef0123456789abcdef")
	envID := uuid.New()
	path := "/internal/v1/authorizations/environments/" + envID.String()
	store := &internalPolicyStore{policy: registry.AuthorizationPolicy{EnvironmentID: envID, ApplicationID: uuid.New(), Revision: 2,
		DisplayName: "Game", RedirectURIs: []string{"https://game.example/callback"}, Providers: []string{"google"},
		PlayerScopes: []string{"game.chat.read"}}}
	h := NewInternalPolicyHandler(WorkloadVerifier{Key: key, Now: func() time.Time { return now }, Nonces: &nonceMemory{}}, store)
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
	require.Equal(t, 1, store.calls)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	require.Equal(t, http.StatusUnauthorized, w.Code)
	require.Equal(t, 1, store.calls, "replayed proof must not read policy")
	r = request(uuid.NewString())
	r.URL.Path = "/internal/v1/authorizations/environments/" + uuid.NewString()
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	require.Equal(t, http.StatusUnauthorized, w.Code)
	require.Equal(t, 1, store.calls, "proof bound to another path must not read policy")
}

func TestInternalPolicyRejectsInvalidAuthWorkloadProofBeforeReadingPolicy(t *testing.T) {
	now := time.Unix(1790435000, 0)
	key := []byte("0123456789abcdef0123456789abcdef")
	envID := uuid.New()
	path := "/internal/v1/authorizations/environments/" + envID.String()
	store := &internalPolicyStore{policy: registry.AuthorizationPolicy{EnvironmentID: envID, ApplicationID: uuid.New(), Revision: 2}}
	h := NewInternalPolicyHandler(WorkloadVerifier{Key: key, Now: func() time.Time { return now }, Nonces: &nonceMemory{}}, store)
	newRequest := func() *http.Request {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		SignWorkloadRequest(r, key, now, uuid.NewString())
		return r
	}
	mutations := []struct {
		name       string
		statusCode int
		mutate     func(*http.Request)
	}{
		{name: "missing workload", statusCode: http.StatusUnauthorized, mutate: func(r *http.Request) { r.Header.Del("X-Voice-Workload") }},
		{name: "wrong workload", statusCode: http.StatusUnauthorized, mutate: func(r *http.Request) { r.Header.Set("X-Voice-Workload", "gis") }},
		{name: "duplicate workload", statusCode: http.StatusUnauthorized, mutate: func(r *http.Request) { r.Header.Add("X-Voice-Workload", "auth") }},
		{name: "missing timestamp", statusCode: http.StatusUnauthorized, mutate: func(r *http.Request) { r.Header.Del("X-Voice-Timestamp") }},
		{name: "duplicate timestamp", statusCode: http.StatusUnauthorized, mutate: func(r *http.Request) { r.Header.Add("X-Voice-Timestamp", strconv.FormatInt(now.Unix(), 10)) }},
		{name: "noncanonical timestamp", statusCode: http.StatusUnauthorized, mutate: func(r *http.Request) { r.Header.Set("X-Voice-Timestamp", "0"+r.Header.Get("X-Voice-Timestamp")) }},
		{name: "timestamp older than 30 seconds", statusCode: http.StatusUnauthorized, mutate: func(r *http.Request) {
			setSignedTimestamp(r, key, now.Add(-31*time.Second))
		}},
		{name: "timestamp newer than 30 seconds", statusCode: http.StatusUnauthorized, mutate: func(r *http.Request) {
			setSignedTimestamp(r, key, now.Add(31*time.Second))
		}},
		{name: "missing nonce", statusCode: http.StatusUnauthorized, mutate: func(r *http.Request) { r.Header.Del("X-Voice-Nonce") }},
		{name: "duplicate nonce", statusCode: http.StatusUnauthorized, mutate: func(r *http.Request) { r.Header.Add("X-Voice-Nonce", uuid.NewString()) }},
		{name: "noncanonical nonce", statusCode: http.StatusUnauthorized, mutate: func(r *http.Request) {
			r.Header.Set("X-Voice-Nonce", "AAAAAAAA-AAAA-4AAA-8AAA-AAAAAAAAAAAA")
		}},
		{name: "missing signature", statusCode: http.StatusUnauthorized, mutate: func(r *http.Request) { r.Header.Del("X-Voice-Signature") }},
		{name: "duplicate signature", statusCode: http.StatusUnauthorized, mutate: func(r *http.Request) { r.Header.Add("X-Voice-Signature", "invalid") }},
		{name: "wrong signature", statusCode: http.StatusUnauthorized, mutate: func(r *http.Request) { r.Header.Set("X-Voice-Signature", strings.Repeat("!", 43)) }},
		{name: "changed path", statusCode: http.StatusUnauthorized, mutate: func(r *http.Request) {
			r.URL.Path = "/internal/v1/authorizations/environments/" + uuid.NewString()
		}},
		{name: "query string", statusCode: http.StatusUnauthorized, mutate: func(r *http.Request) { r.URL.RawQuery = "unexpected=1" }},
		{name: "body on GET", statusCode: http.StatusUnauthorized, mutate: func(r *http.Request) {
			r.Body = io.NopCloser(strings.NewReader("unexpected"))
			r.ContentLength = int64(len("unexpected"))
		}},
		{name: "wrong method", statusCode: http.StatusNotFound, mutate: func(r *http.Request) { r.Method = http.MethodPost }},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			r := newRequest()
			tc.mutate(r)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			require.Equal(t, tc.statusCode, w.Code)
			require.Zero(t, store.calls, "invalid workload proof must be rejected before policy lookup")
		})
	}
}

func TestInternalPolicyAcceptsExactThirtySecondWorkloadProofBoundary(t *testing.T) {
	now := time.Unix(1790435000, 0)
	key := []byte("0123456789abcdef0123456789abcdef")
	envID := uuid.New()
	path := "/internal/v1/authorizations/environments/" + envID.String()
	for _, offset := range []time.Duration{-30 * time.Second, 30 * time.Second} {
		t.Run(offset.String(), func(t *testing.T) {
			store := &internalPolicyStore{policy: registry.AuthorizationPolicy{EnvironmentID: envID, ApplicationID: uuid.New(), Revision: 2}}
			h := NewInternalPolicyHandler(WorkloadVerifier{Key: key, Now: func() time.Time { return now }, Nonces: &nonceMemory{}}, store)
			r := httptest.NewRequest(http.MethodGet, path, nil)
			SignWorkloadRequest(r, key, now.Add(offset), uuid.NewString())
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			require.Equal(t, 1, store.calls)
		})
	}
}

func TestInternalPolicyFailsClosedWhenNonceStoreIsUnavailable(t *testing.T) {
	now := time.Unix(1790435000, 0)
	key := []byte("0123456789abcdef0123456789abcdef")
	envID := uuid.New()
	store := &internalPolicyStore{policy: registry.AuthorizationPolicy{EnvironmentID: envID, ApplicationID: uuid.New(), Revision: 2}}
	h := NewInternalPolicyHandler(WorkloadVerifier{Key: key, Now: func() time.Time { return now }, Nonces: failingNonceStore{}}, store)
	r := httptest.NewRequest(http.MethodGet, "/internal/v1/authorizations/environments/"+envID.String(), nil)
	SignWorkloadRequest(r, key, now, uuid.NewString())
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	require.Equal(t, http.StatusServiceUnavailable, w.Code)
	require.Zero(t, store.calls, "nonce storage failure must not allow policy lookup")
}

func setSignedTimestamp(r *http.Request, key []byte, issuedAt time.Time) {
	timestamp := strconv.FormatInt(issuedAt.Unix(), 10)
	nonce := r.Header.Get("X-Voice-Nonce")
	r.Header.Set("X-Voice-Timestamp", timestamp)
	r.Header.Set("X-Voice-Signature", workloadSignature(key, r.Method, r.URL.EscapedPath(), timestamp, nonce))
}
