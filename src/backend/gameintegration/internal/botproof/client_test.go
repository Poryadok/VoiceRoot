package botproof

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/workloadproof"
)

func TestClientAuthenticatesBotOwnerProofAndValidatesSignedResponse(t *testing.T) {
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	key := bytes.Repeat([]byte{0x2a}, 32)
	botID, ownerID := uuid.New(), uuid.New()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proof, err := (workloadproof.Verifier{Key: key, Principal: "gameintegration", Audience: "bot", Now: func() time.Time { return now }, Nonces: &oneNonce{}}).VerifyRequest(r, 1024)
		require.NoError(t, err)
		var request ownerProofRequest
		require.NoError(t, json.Unmarshal(proof.Body, &request))
		require.Equal(t, ownerID.String(), request.ApplicationOwnerAccountID)
		response, err := json.Marshal(ownerProofResponse{BotID: botID.String(), OwnerAccountID: ownerID.String(), Status: "live"})
		require.NoError(t, err)
		w.Header().Set("Content-Type", "application/json")
		workloadproof.SignResponse(w, key, http.StatusOK, r.URL.EscapedPath(), r.Header.Get("X-Voice-Timestamp"), r.Header.Get("X-Voice-Nonce"), response)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(response)
	}))
	defer server.Close()

	client := newTestClient(server.URL, key, now)
	require.NoError(t, client.VerifyGameIntegrationBot(context.Background(), botID, ownerID))
}

func TestClientRejectsResponseTamperForeignOwnerAndInactiveBot(t *testing.T) {
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	key := bytes.Repeat([]byte{0x2a}, 32)
	botID, ownerID := uuid.New(), uuid.New()
	for _, tc := range []struct {
		name   string
		owner  uuid.UUID
		status string
		mutate bool
		want   error
	}{
		{name: "foreign owner", owner: uuid.New(), status: "live", want: ErrDenied},
		{name: "disabled bot", owner: ownerID, status: "disabled", want: ErrDenied},
		{name: "tampered signed response", owner: ownerID, status: "live", mutate: true, want: ErrInvalidProof},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				proof, err := (workloadproof.Verifier{Key: key, Principal: "gameintegration", Audience: "bot", Now: func() time.Time { return now }, Nonces: &oneNonce{}}).VerifyRequest(r, 1024)
				require.NoError(t, err)
				var request ownerProofRequest
				require.NoError(t, json.Unmarshal(proof.Body, &request))
				response, err := json.Marshal(ownerProofResponse{BotID: botID.String(), OwnerAccountID: tc.owner.String(), Status: tc.status})
				require.NoError(t, err)
				w.Header().Set("Content-Type", "application/json")
				workloadproof.SignResponse(w, key, http.StatusOK, r.URL.EscapedPath(), r.Header.Get("X-Voice-Timestamp"), r.Header.Get("X-Voice-Nonce"), response)
				w.WriteHeader(http.StatusOK)
				if tc.mutate {
					response = bytes.Replace(response, []byte("live"), []byte("dead"), 1)
				}
				_, _ = w.Write(response)
			}))
			defer server.Close()
			err := newTestClient(server.URL, key, now).VerifyGameIntegrationBot(context.Background(), botID, ownerID)
			require.ErrorIs(t, err, tc.want)
		})
	}
}

func TestClientFailsClosedWhenKeyOrEndpointMissing(t *testing.T) {
	client := &Client{BaseURL: "", Key: nil}
	require.ErrorIs(t, client.VerifyGameIntegrationBot(context.Background(), uuid.New(), uuid.New()), ErrUnavailable)
	client = &Client{BaseURL: "http://bot:8080"}
	require.ErrorIs(t, client.VerifyGameIntegrationBot(context.Background(), uuid.New(), uuid.New()), ErrUnavailable)
}

func TestClientDoesNotForwardWorkloadProofAcrossRedirect(t *testing.T) {
	var targetCalls atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		targetCalls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Redirect(w, httptest.NewRequest(http.MethodPost, "/", nil), target.URL, http.StatusTemporaryRedirect)
	}))
	defer source.Close()
	client := newTestClient(source.URL, bytes.Repeat([]byte{0x2a}, 32), time.Now().UTC())
	require.ErrorIs(t, client.VerifyGameIntegrationBot(context.Background(), uuid.New(), uuid.New()), ErrUnavailable)
	require.Zero(t, targetCalls.Load(), "signed workload headers must never be forwarded to a redirect target")
}

type oneNonce struct{ used bool }

func (n *oneNonce) Use(context.Context, string, time.Duration) (bool, error) {
	if n.used {
		return false, nil
	}
	n.used = true
	return true, nil
}

func newTestClient(endpoint string, key []byte, now time.Time) *Client {
	return &Client{BaseURL: endpoint, Key: key, HTTP: http.DefaultClient, Now: func() time.Time { return now }, NewNonce: uuid.NewString}
}
