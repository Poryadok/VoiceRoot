package httpapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/gameintegration/internal/registry"
)

type challengeCreateMemory struct {
	byOperation map[uuid.UUID]registry.BindingChallenge
	hashes      map[uuid.UUID]string
}

func (s *challengeCreateMemory) CreateBindingChallengeForAuth(_ context.Context, request registry.BindingChallengeCreate,
	requestHash string, id uuid.UUID, nonce string) (registry.BindingChallenge, error) {
	if s.byOperation == nil {
		s.byOperation = map[uuid.UUID]registry.BindingChallenge{}
		s.hashes = map[uuid.UUID]string{}
	}
	if saved, ok := s.byOperation[request.OperationID]; ok {
		if s.hashes[request.OperationID] != requestHash {
			return registry.BindingChallenge{}, registry.ErrInvalidBindingChallenge
		}
		return saved, nil
	}
	saved := registry.BindingChallenge{ChallengeID: id, Nonce: nonce, ApplicationID: request.ApplicationID,
		EnvironmentID: request.EnvironmentID, Provider: request.Provider, RedirectURIHash: request.RedirectURIHash,
		PKCEChallenge: request.PKCEChallenge, DeviceKeyID: request.DeviceKeyID, DeviceKeyThumbprint: request.DeviceKeyThumbprint,
		OperationID: request.OperationID, ExpiresAt: request.ExpiresAt, Status: "pending", SourceAccountID: request.SourceAccountID,
		SourceActorID: request.SourceActorID, SourceDeviceID: request.SourceDeviceID, SourceGeneration: request.SourceGeneration,
		TargetAccountID: request.TargetAccountID, TargetProfileID: request.TargetProfileID, ProfileRevision: request.ProfileRevision,
		ConsentRevision: request.ConsentRevision, PolicyRevision: request.PolicyRevision, Scopes: request.Scopes}
	s.byOperation[request.OperationID] = saved
	s.hashes[request.OperationID] = requestHash
	return saved, nil
}

func TestInternalBindingChallengeCreateRequiresAuthProofAndReplaysDurableFacts(t *testing.T) {
	now := time.Unix(1790435000, 0)
	key := []byte("0123456789abcdef0123456789abcdef")
	store := &challengeCreateMemory{}
	handler := &InternalBindingChallengeCreateHandler{Verifier: WorkloadVerifier{Key: key, Now: func() time.Time { return now }, Nonces: &nonceMemory{}}, Store: store}
	requestValue := registry.BindingChallengeCreate{ApplicationID: uuid.New(), EnvironmentID: uuid.New(), Provider: "google",
		RedirectURIHash: strings.Repeat("a", 64), PKCEChallenge: strings.Repeat("p", 43), DeviceKeyID: uuid.New(),
		DeviceKeyThumbprint: strings.Repeat("t", 43), OperationID: uuid.New(), ExpiresAt: now.Add(4 * time.Minute),
		SourceAccountID: uuid.New(), SourceActorID: uuid.New(), SourceDeviceID: uuid.New(), SourceGeneration: 2,
		TargetAccountID: uuid.New(), TargetProfileID: uuid.New(), ProfileRevision: 3, ConsentRevision: 4, PolicyRevision: 5,
		Scopes: []string{"game.chat.read", "game.chat.send"}}
	requestValue.SourceDeviceID = requestValue.DeviceKeyID
	body, err := json.Marshal(requestValue)
	require.NoError(t, err)
	makeRequest := func(nonce string) *http.Request {
		r := httptest.NewRequest(http.MethodPost, "/internal/v1/bindings/challenges", bytes.NewReader(body))
		SignBodyWorkloadRequest(r, key, now, nonce, body)
		return r
	}
	r := makeRequest(uuid.NewString())
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, r)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	var first registry.BindingChallenge
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &first))
	require.NotEqual(t, uuid.Nil, first.ChallengeID)
	require.Len(t, first.Nonce, 43)
	require.Equal(t, requestValue.OperationID, first.OperationID)
	digest := sha256.Sum256(body)
	require.Equal(t, hex.EncodeToString(digest[:]), store.hashes[requestValue.OperationID])

	r = makeRequest(uuid.NewString())
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, r)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	var replay registry.BindingChallenge
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &replay))
	require.Equal(t, first, replay, "exact retry returns the original GIS-generated id and nonce")

	r = httptest.NewRequest(http.MethodPost, "/internal/v1/bindings/challenges", bytes.NewReader(body))
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, r)
	require.Equal(t, http.StatusUnauthorized, response.Code, "unsigned challenge creation is denied")
}

type bindingChallengeStore struct{ challenge registry.BindingChallenge }

func (s bindingChallengeStore) LoadBindingChallenge(_ context.Context, id uuid.UUID) (registry.BindingChallenge, error) {
	if id != s.challenge.ChallengeID {
		return registry.BindingChallenge{}, registry.ErrBindingChallengeNotFound
	}
	return s.challenge, nil
}

func TestInternalBindingChallengeRequiresAuthProofAndSignsExactPersistedFacts(t *testing.T) {
	now := time.Unix(1790435000, 0)
	key := []byte("0123456789abcdef0123456789abcdef")
	challenge := registry.BindingChallenge{ChallengeID: uuid.New(), Nonce: strings.Repeat("n", 43),
		ApplicationID: uuid.New(), EnvironmentID: uuid.New(), Provider: "google", RedirectURIHash: strings.Repeat("a", 64),
		PKCEChallenge: strings.Repeat("p", 43), DeviceKeyID: uuid.New(), DeviceKeyThumbprint: strings.Repeat("t", 43),
		OperationID: uuid.New(), ExpiresAt: now.Add(2 * time.Minute), Status: "pending"}
	handler := NewInternalBindingChallengeHandler(WorkloadVerifier{Key: key, Now: func() time.Time { return now },
		Nonces: &nonceMemory{}}, bindingChallengeStore{challenge: challenge})
	request := httptest.NewRequest(http.MethodGet, "/internal/v1/bindings/challenges/"+challenge.ChallengeID.String(), nil)
	SignWorkloadRequest(request, key, now, uuid.NewString())
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	require.Equal(t, "no-store", response.Header().Get("Cache-Control"))
	require.Equal(t, responseSignature(key, http.StatusOK, request.URL.EscapedPath(), request.Header.Get("X-Voice-Timestamp"),
		request.Header.Get("X-Voice-Nonce"), response.Body.Bytes()), response.Header().Get("X-Voice-Response-Signature"))
	var decoded registry.BindingChallenge
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &decoded))
	require.Equal(t, challenge, decoded)

	bad := httptest.NewRequest(http.MethodGet, request.URL.String(), nil)
	denied := httptest.NewRecorder()
	handler.ServeHTTP(denied, bad)
	require.Equal(t, http.StatusUnauthorized, denied.Code)
}
