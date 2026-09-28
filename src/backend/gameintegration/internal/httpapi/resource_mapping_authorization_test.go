package httpapi

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type resourceMappingAuthorizerStub struct {
	allowed                 bool
	revision                int64
	app, env, binding, chat uuid.UUID
	calls                   int
}

func (s *resourceMappingAuthorizerStub) AuthorizeAppBindingChat(_ context.Context, app, env, binding, chat uuid.UUID) (bool, int64, error) {
	s.calls++
	s.app, s.env, s.binding, s.chat = app, env, binding, chat
	return s.allowed, s.revision, nil
}

func TestMessagingResourceMappingAuthorizationUsesExactTupleAndMinimalResponse(t *testing.T) {
	now := time.Unix(1790435000, 0)
	key := []byte("0123456789abcdef0123456789abcdef")
	app, env, binding, chat := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	body := []byte(`{"application_id":"` + app.String() + `","environment_id":"` + env.String() + `","binding_id":"` + binding.String() + `","chat_id":"` + chat.String() + `"}`)
	store := &resourceMappingAuthorizerStub{allowed: true, revision: 7}
	verifier := WorkloadVerifier{Key: key, Principal: "messaging", Now: func() time.Time { return now }, Nonces: &nonceMemory{}}
	handler := NewInternalResourceMappingAuthorizationHandler(verifier, store)
	r := messagingMappingRequest("/internal/v1/game-integrations/resource-mappings/authorize-chat", key, now, uuid.NewString(), body)
	r.TLS = messagingVerifiedTLSState()
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, [4]uuid.UUID{app, env, binding, chat}, [4]uuid.UUID{store.app, store.env, store.binding, store.chat})
	require.Equal(t, 1, store.calls)
	require.JSONEq(t, `{"allowed":true,"mapping_revision":7}`, w.Body.String())
	require.NotContains(t, w.Body.String(), "external_key")
	require.Equal(t, responseSignature(key, http.StatusOK, r.URL.EscapedPath(), r.Header.Get("X-Voice-Timestamp"),
		r.Header.Get("X-Voice-Nonce"), w.Body.Bytes()), w.Header().Get("X-Voice-Response-Signature"))

	denied := &resourceMappingAuthorizerStub{allowed: false}
	handler = NewInternalResourceMappingAuthorizationHandler(verifier, denied)
	r = messagingMappingRequest("/internal/v1/game-integrations/resource-mappings/authorize-chat", key, now, uuid.NewString(), body)
	r.TLS = messagingVerifiedTLSState()
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, `{"allowed":false}`, w.Body.String())
}

func TestMessagingResourceMappingAuthorizationFailsClosedOnIdentityOrBody(t *testing.T) {
	now := time.Unix(1790435000, 0)
	key := []byte("0123456789abcdef0123456789abcdef")
	app, env, binding, chat := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	body := []byte(`{"application_id":"` + app.String() + `","environment_id":"` + env.String() + `","binding_id":"` + binding.String() + `","chat_id":"` + chat.String() + `"}`)
	store := &resourceMappingAuthorizerStub{allowed: true, revision: 7}
	handler := NewInternalResourceMappingAuthorizationHandler(WorkloadVerifier{Key: key, Principal: "messaging", Now: func() time.Time { return now }, Nonces: &nonceMemory{}}, store)
	for _, test := range []struct {
		name   string
		body   []byte
		mutate func(*http.Request)
		status int
	}{
		{name: "missing mTLS identity", mutate: func(*http.Request) {}, status: http.StatusUnauthorized},
		{name: "wrong URI SAN", mutate: func(r *http.Request) { r.TLS = verifiedTLSState("spiffe://voice/service/auth") }, status: http.StatusUnauthorized},
		{name: "extra body field", body: []byte(`{"application_id":"` + app.String() + `","environment_id":"` + env.String() + `","binding_id":"` + binding.String() + `","chat_id":"` + chat.String() + `","external_key":"leak"}`), mutate: func(r *http.Request) { r.TLS = messagingVerifiedTLSState() }, status: http.StatusBadRequest},
	} {
		t.Run(test.name, func(t *testing.T) {
			requestBody := body
			if test.body != nil {
				requestBody = test.body
			}
			r := messagingMappingRequest("/internal/v1/game-integrations/resource-mappings/authorize-chat", key, now, uuid.NewString(), requestBody)
			test.mutate(r)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			require.Equal(t, test.status, w.Code)
		})
	}
	require.Zero(t, store.calls)
}

func TestMessagingWorkloadVerifierCurrentPreviousRotationAndExpiry(t *testing.T) {
	now := time.Unix(1790435000, 0)
	current := []byte("0123456789abcdef0123456789abcdef")
	previous := []byte("abcdef0123456789abcdef0123456789")
	body := []byte(`{"application_id":"` + uuid.NewString() + `"}`)
	request := func(key []byte, now time.Time) *http.Request {
		return messagingMappingRequest("/internal/v1/game-integrations/resource-mappings/authorize-chat", key, now, uuid.NewString(), body)
	}
	verifier := WorkloadVerifier{Key: current, PreviousKey: previous, PreviousKeyUntil: now.Add(5 * time.Minute), Principal: "messaging",
		Now: func() time.Time { return now }, Nonces: &nonceMemory{}}
	got, verifiedKey, err := verifier.VerifyBodyWithKey(request(current, now))
	require.NoError(t, err)
	require.Equal(t, body, got)
	require.Equal(t, current, verifiedKey)
	got, verifiedKey, err = verifier.VerifyBodyWithKey(request(previous, now))
	require.NoError(t, err)
	require.Equal(t, body, got)
	require.Equal(t, previous, verifiedKey, "response must use the key that verified the request")
	verifier.Now = func() time.Time { return now.Add(5*time.Minute + time.Second) }
	_, _, err = verifier.VerifyBodyWithKey(request(previous, now.Add(5*time.Minute+time.Second)))
	require.ErrorIs(t, err, ErrInvalidWorkloadProof)
}

func TestMessagingAuthorizationSignsPreviousKeyResponseDuringRotation(t *testing.T) {
	now := time.Unix(1790435000, 0)
	current := []byte("0123456789abcdef0123456789abcdef")
	previous := []byte("abcdef0123456789abcdef0123456789")
	app, env, binding, chat := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	body := []byte(`{"application_id":"` + app.String() + `","environment_id":"` + env.String() + `","binding_id":"` + binding.String() + `","chat_id":"` + chat.String() + `"}`)
	store := &resourceMappingAuthorizerStub{allowed: true, revision: 2}
	verifier := WorkloadVerifier{Key: current, PreviousKey: previous, PreviousKeyUntil: now.Add(5 * time.Minute),
		Now: func() time.Time { return now }, Nonces: &nonceMemory{}}
	handler := NewInternalResourceMappingAuthorizationHandler(verifier, store)
	r := messagingMappingRequest("/internal/v1/game-integrations/resource-mappings/authorize-chat", previous, now, uuid.NewString(), body)
	r.TLS = messagingVerifiedTLSState()
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, responseSignature(previous, http.StatusOK, r.URL.EscapedPath(), r.Header.Get("X-Voice-Timestamp"),
		r.Header.Get("X-Voice-Nonce"), w.Body.Bytes()), w.Header().Get("X-Voice-Response-Signature"))
}

func TestMessagingMTLSConfigRequiresVerifiedClientCertificate(t *testing.T) {
	ca := x509.NewCertPool()
	config := MessagingMTLSConfig(ca)
	require.Equal(t, uint16(tls.VersionTLS12), config.MinVersion)
	require.Equal(t, tls.RequireAndVerifyClientCert, config.ClientAuth)
	require.Same(t, ca, config.ClientCAs)
}

func messagingMappingRequest(path string, key []byte, now time.Time, nonce string, body []byte) *http.Request {
	r := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	r.Header.Set("X-Voice-Workload", "messaging")
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Voice-Timestamp", "1790435000")
	r.Header.Set("X-Voice-Nonce", nonce)
	r.Header.Set("X-Voice-Signature", bodyWorkloadSignature(key, r.Method, r.URL.EscapedPath(), "1790435000", nonce, body))
	return r
}

func messagingVerifiedTLSState() *tls.ConnectionState {
	return verifiedTLSState("spiffe://voice/service/messaging")
}

func verifiedTLSState(uri string) *tls.ConnectionState {
	parsed, _ := url.Parse(uri)
	cert := &x509.Certificate{URIs: []*url.URL{parsed}}
	return &tls.ConnectionState{PeerCertificates: []*x509.Certificate{cert}, VerifiedChains: [][]*x509.Certificate{{cert}}}
}
