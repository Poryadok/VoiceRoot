package workloadproof

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type nonceMemory struct{ seen map[string]time.Time }

func (m *nonceMemory) Use(_ context.Context, nonce string, ttl time.Duration) (bool, error) {
	if m.seen == nil {
		m.seen = make(map[string]time.Time)
	}
	if _, exists := m.seen[nonce]; exists {
		return false, nil
	}
	m.seen[nonce] = time.Now().Add(ttl)
	return true, nil
}

func TestRequestProofBindsWorkloadMethodPathAndExactBody(t *testing.T) {
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	key := bytes.Repeat([]byte{0x2a}, 32)
	body := []byte(`{"application_owner_account_id":"00000000-0000-4000-8000-000000000001"}`)
	nonce := uuid.NewString()
	request := httptest.NewRequest(http.MethodPost, "/internal/v1/game-integrations/bots/00000000-0000-4000-8000-000000000002/authority", bytes.NewReader(body))
	SignRequest(request, key, "gameintegration", "bot", now, nonce, body)

	verifier := Verifier{Key: key, Principal: "gameintegration", Audience: "bot", Now: func() time.Time { return now }, Nonces: &nonceMemory{}}
	proof, err := verifier.VerifyRequest(request, 1024)
	require.NoError(t, err)
	require.Equal(t, "gameintegration", proof.Principal)
	require.Equal(t, "bot", proof.Audience)
	require.Equal(t, nonce, proof.Nonce)
	require.Equal(t, now.Unix(), proof.Timestamp)
	require.Equal(t, body, proof.Body)

	mutated := httptest.NewRequest(http.MethodPost, request.URL.Path, bytes.NewReader(append(body, ' ')))
	for name, values := range request.Header {
		for _, value := range values {
			mutated.Header.Add(name, value)
		}
	}
	_, err = verifier.VerifyRequest(mutated, 1024)
	require.Error(t, err, "signature must bind the received body bytes")
}

func TestRequestProofRejectsWrongPrincipalStaleTimestampAndReplay(t *testing.T) {
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	key := bytes.Repeat([]byte{0x2a}, 32)
	body := []byte(`{"application_owner_account_id":"00000000-0000-4000-8000-000000000001"}`)
	path := "/internal/v1/game-integrations/bots/00000000-0000-4000-8000-000000000002/authority"
	verifier := Verifier{Key: key, Principal: "gameintegration", Audience: "bot", Now: func() time.Time { return now }, Nonces: &nonceMemory{}}

	wrongPrincipal := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	SignRequest(wrongPrincipal, key, "other-service", "bot", now, uuid.NewString(), body)
	_, err := verifier.VerifyRequest(wrongPrincipal, 1024)
	require.Error(t, err)

	stale := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	SignRequest(stale, key, "gameintegration", "bot", now.Add(-31*time.Second), uuid.NewString(), body)
	_, err = verifier.VerifyRequest(stale, 1024)
	require.Error(t, err)

	replayedNonce := uuid.NewString()
	first := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	SignRequest(first, key, "gameintegration", "bot", now, replayedNonce, body)
	_, err = verifier.VerifyRequest(first, 1024)
	require.NoError(t, err)
	replay := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	SignRequest(replay, key, "gameintegration", "bot", now, replayedNonce, body)
	_, err = verifier.VerifyRequest(replay, 1024)
	require.Error(t, err)
}

func TestRequestProofRejectsAudienceMismatchEvenWhenWorkloadIsCorrect(t *testing.T) {
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	key := bytes.Repeat([]byte{0x2a}, 32)
	body := []byte(`{"application_owner_account_id":"00000000-0000-4000-8000-000000000001"}`)
	path := "/internal/v1/game-integrations/bots/00000000-0000-4000-8000-000000000002/authority"
	request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	SignRequest(request, key, "gameintegration", "messaging", now, uuid.NewString(), body)
	verifier := Verifier{Key: key, Principal: "gameintegration", Audience: "bot", Now: func() time.Time { return now }, Nonces: &nonceMemory{}}
	_, err := verifier.VerifyRequest(request, 1024)
	require.Error(t, err)
}

func TestResponseProofBindsStatusPathAndBody(t *testing.T) {
	key := bytes.Repeat([]byte{0x2a}, 32)
	path := "/internal/v1/game-integrations/bots/00000000-0000-4000-8000-000000000002/authority"
	timestamp := "1790510400"
	nonce := uuid.NewString()
	body := []byte(`{"bot_id":"00000000-0000-4000-8000-000000000002","owner_account_id":"00000000-0000-4000-8000-000000000001","status":"live"}`)
	response := httptest.NewRecorder()
	SignResponse(response, key, http.StatusOK, path, timestamp, nonce, body)
	proof, err := VerifyResponse(key, http.StatusOK, path, timestamp, nonce, body, response.Header())
	require.NoError(t, err)
	require.Equal(t, body, proof.Body)

	_, err = VerifyResponse(key, http.StatusOK, path, timestamp, nonce, append(body, ' '), response.Header())
	require.Error(t, err, "the response authenticator binds exact bytes")
	_, err = VerifyResponse(key, http.StatusAccepted, path, timestamp, nonce, body, response.Header())
	require.Error(t, err, "the response authenticator binds HTTP status")
	_, err = VerifyResponse(key, http.StatusOK, path, timestamp, "not-a-uuid", body, response.Header())
	require.Error(t, err, "response proof must use the canonical request nonce")
}
