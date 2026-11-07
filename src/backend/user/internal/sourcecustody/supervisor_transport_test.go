package sourcecustody

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestSupervisorTransportEarlyPollDoesNotRevokeOrIssueAnotherProbe(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	session, err := newSupervisionSession([32]byte{1}, [32]byte{2}, public, time.Now)
	require.NoError(t, err)
	responder, err := newMonitorResponder([32]byte{1}, [32]byte{2}, uuid.New(), private)
	require.NoError(t, err)
	var requests atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		var probe supervisionProbe
		if json.NewDecoder(r.Body).Decode(&probe) != nil || responder.observe(probe) != nil {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		response, err := responder.respond(probe)
		if err != nil {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()
	client := &supervisorClient{client: server.Client(), key: public, endpoint: server.URL + "/probe"}
	require.True(t, client.renew(context.Background(), session))
	require.True(t, client.renew(context.Background(), session), "an early scheduler tick must keep the existing valid lease")
	require.EqualValues(t, 1, requests.Load(), "only a due fresh sequence may send another probe")
	require.True(t, session.live())
	time.Sleep(supervisionInterval + 20*time.Millisecond)
	require.True(t, client.renew(context.Background(), session), "a due fresh sequence must renew through the real transport")
	require.EqualValues(t, 2, requests.Load())
	session.revoke()
	require.False(t, client.renew(context.Background(), session), "a revoked incarnation must not renew")
}
