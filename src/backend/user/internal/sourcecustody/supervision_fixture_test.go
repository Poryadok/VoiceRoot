package sourcecustody

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// These synthetic fixtures preserve existing local source/open test coverage.
// They do not exercise the production supervisor's SQL health or custody.
func armSyntheticSupervision(t *testing.T, c *Controller, claim OpenAttestation, custodian ed25519.PublicKey) {
	t.Helper()
	// SQL/recovery fixtures use one deterministic signed session. Deadline and
	// scheduler-loss guarantees belong to the dedicated real-clock tests.
	now := time.Now()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	responder, err := newDecisionResponder(claim, custodian, public, private, uuid.New(), now)
	require.NoError(t, err)
	payload, err := OpenAttestationPayload(claim)
	require.NoError(t, err)
	session, err := newSupervisionSession(claim.Challenge, sha256.Sum256(payload), public, func() time.Time { return now })
	require.NoError(t, err)
	probe, err := randomSupervisionProbe(session)
	require.NoError(t, err)
	require.NoError(t, responder.observe(probe))
	response, err := responder.respond(probe)
	require.NoError(t, err)
	require.NoError(t, session.accept(response))
	require.True(t, session.liveFor(claim))
	c.mu.Lock()
	c.supervision = session
	c.mu.Unlock()
	t.Cleanup(func() { session.revoke(); _, _ = responder.stop() })
}

func attachSyntheticOperatorSupervisor(t *testing.T, h *activationHandler, claim OpenAttestation) {
	t.Helper()
	h.controller.mu.Lock()
	if h.controller.supervision != nil {
		h.controller.supervision.revoke()
	}
	h.controller.supervision = nil
	h.controller.mu.Unlock()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	responder, err := newDecisionResponder(claim, h.signer, public, private, uuid.New(), time.Now())
	require.NoError(t, err)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	t.Cleanup(server.Close)
	h.supervisor = &supervisorClient{client: server.Client(), key: public, endpoint: server.URL + "/probe"}
	t.Cleanup(func() {
		if h.supervised != nil {
			_ = h.supervised.stop()
		}
		_, _ = responder.stop()
	})
}
