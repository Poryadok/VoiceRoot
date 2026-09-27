package gameintegrationproof

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/workloadproof"
)

type botAuthorityFixture struct {
	owner  uuid.UUID
	status string
	err    error
	calls  int
}

func (f *botAuthorityFixture) LookupGameIntegrationBotAuthority(context.Context, uuid.UUID) (uuid.UUID, string, bool, error) {
	f.calls++
	return f.owner, f.status, f.err == nil, f.err
}

type replayFixture struct{ seen map[string]bool }

func (f *replayFixture) Use(_ context.Context, nonce string, _ time.Duration) (bool, error) {
	if f.seen == nil {
		f.seen = map[string]bool{}
	}
	if f.seen[nonce] {
		return false, nil
	}
	f.seen[nonce] = true
	return true, nil
}

func TestAuthorityEndpointReturnsSignedProofOnlyForLiveOwnedBot(t *testing.T) {
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	key := bytes.Repeat([]byte{0x2a}, 32)
	botID, ownerID := uuid.New(), uuid.New()
	lookup := &botAuthorityFixture{owner: ownerID, status: "live"}
	handler := NewHandler(lookup, key, &replayFixture{}, func() time.Time { return now })
	path := "/internal/v1/game-integrations/bots/" + botID.String() + "/authority"
	body, err := json.Marshal(authorityRequest{ApplicationOwnerAccountID: ownerID.String()})
	require.NoError(t, err)
	request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	workloadproof.SignRequest(request, key, "gameintegration", "bot", now, uuid.NewString(), body)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	var result authorityResponse
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &result))
	require.Equal(t, botID.String(), result.BotID)
	require.Equal(t, ownerID.String(), result.OwnerAccountID)
	require.Equal(t, "live", result.Status)
	_, err = workloadproof.VerifyResponse(key, response.Code, path,
		request.Header.Get("X-Voice-Timestamp"), request.Header.Get("X-Voice-Nonce"), response.Body.Bytes(), response.Header())
	require.NoError(t, err)
	require.Equal(t, 1, lookup.calls)
}

func TestAuthorityEndpointDeniesForeignOwnerInactiveAndDeletedBotWithoutSuccessProof(t *testing.T) {
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	key := bytes.Repeat([]byte{0x2a}, 32)
	botID, requestedOwner := uuid.New(), uuid.New()
	for _, tc := range []struct {
		name string
		row  botAuthorityFixture
	}{
		{name: "foreign owner", row: botAuthorityFixture{owner: uuid.New(), status: "live"}},
		{name: "disabled bot", row: botAuthorityFixture{owner: requestedOwner, status: "disabled"}},
		{name: "deleted bot", row: botAuthorityFixture{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lookup := &tc.row
			handler := NewHandler(lookup, key, &replayFixture{}, func() time.Time { return now })
			path := "/internal/v1/game-integrations/bots/" + botID.String() + "/authority"
			body, err := json.Marshal(authorityRequest{ApplicationOwnerAccountID: requestedOwner.String()})
			require.NoError(t, err)
			request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
			workloadproof.SignRequest(request, key, "gameintegration", "bot", now, uuid.NewString(), body)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			require.Equal(t, http.StatusForbidden, response.Code)
			_, err = workloadproof.VerifyResponse(key, response.Code, path,
				request.Header.Get("X-Voice-Timestamp"), request.Header.Get("X-Voice-Nonce"), response.Body.Bytes(), response.Header())
			require.Error(t, err, "denials must not carry an owner/live proof")
		})
	}
}

func TestAuthorityEndpointFailsClosedForInvalidProofAndMissingConfiguration(t *testing.T) {
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	botID, ownerID := uuid.New(), uuid.New()
	lookup := &botAuthorityFixture{owner: ownerID, status: "live"}
	path := "/internal/v1/game-integrations/bots/" + botID.String() + "/authority"
	body, err := json.Marshal(authorityRequest{ApplicationOwnerAccountID: ownerID.String()})
	require.NoError(t, err)

	invalid := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	workloadproof.SignRequest(invalid, bytes.Repeat([]byte{0x2a}, 32), "attacker", "bot", now, uuid.NewString(), body)
	handler := NewHandler(lookup, bytes.Repeat([]byte{0x2a}, 32), &replayFixture{}, func() time.Time { return now })
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, invalid)
	require.Equal(t, http.StatusUnauthorized, response.Code)
	require.Zero(t, lookup.calls, "invalid workload proof must be rejected before Bot lookup")

	membershipDisabled := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	workloadproof.SignRequest(membershipDisabled, []byte(""), "gameintegration", "bot", now, uuid.NewString(), body)
	handler = NewHandler(lookup, nil, &replayFixture{}, func() time.Time { return now })
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, membershipDisabled)
	require.Equal(t, http.StatusServiceUnavailable, response.Code)
	require.Zero(t, lookup.calls, "missing runtime key must never use a development fallback")
}

func TestAuthorityEndpointUsesRedisReplayBoundaryAndSignsPrincipal(t *testing.T) {
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	key := bytes.Repeat([]byte{0x2a}, 32)
	botID, ownerID := uuid.New(), uuid.New()
	lookup := &botAuthorityFixture{owner: ownerID, status: "live"}
	redisServer, err := miniredis.Run()
	require.NoError(t, err)
	defer redisServer.Close()
	client := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	defer client.Close()
	handler := NewHandler(lookup, key, RedisNonceStore{Client: client}, func() time.Time { return now })
	path := "/internal/v1/game-integrations/bots/" + botID.String() + "/authority"
	body, err := json.Marshal(authorityRequest{ApplicationOwnerAccountID: ownerID.String()})
	require.NoError(t, err)
	nonce := uuid.NewString()

	request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	workloadproof.SignRequest(request, key, "gameintegration", "bot", now.Add(30*time.Second), nonce, body)
	signedHeaders := request.Header.Clone()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code)

	redisServer.FastForward(60 * time.Second)
	now = now.Add(60 * time.Second)
	replayed := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	replayed.Header = signedHeaders
	replayResponse := httptest.NewRecorder()
	handler.ServeHTTP(replayResponse, replayed)
	require.Equal(t, http.StatusUnauthorized, replayResponse.Code)
	require.Equal(t, 1, lookup.calls, "Redis replay rejection occurs before owner lookup")

	forgedPrincipal := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	workloadproof.SignRequest(forgedPrincipal, key, "forged", "bot", now, uuid.NewString(), body)
	forgedPrincipal.Header.Set("X-Voice-Workload", "gameintegration")
	forgedResponse := httptest.NewRecorder()
	handler.ServeHTTP(forgedResponse, forgedPrincipal)
	require.Equal(t, http.StatusUnauthorized, forgedResponse.Code,
		"matching workload metadata cannot repair a signature made for another principal")
	require.Equal(t, 1, lookup.calls, "principal signature mismatch must precede owner lookup")

	forgedAudience := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	workloadproof.SignRequest(forgedAudience, key, "gameintegration", "messaging", now, uuid.NewString(), body)
	forgedAudience.Header.Set("X-Voice-Audience", "bot")
	wrongAudienceResponse := httptest.NewRecorder()
	handler.ServeHTTP(wrongAudienceResponse, forgedAudience)
	require.Equal(t, http.StatusUnauthorized, wrongAudienceResponse.Code,
		"matching audience metadata cannot repair a signature made for another audience")
	require.Equal(t, 1, lookup.calls, "audience signature mismatch must precede owner lookup")
}
