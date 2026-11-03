package sourcecustody

import (
	"context"
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
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	responder, err := newDecisionResponder(claim, custodian, public, private, uuid.New(), time.Now())
	require.NoError(t, err)
	payload, err := OpenAttestationPayload(claim)
	require.NoError(t, err)
	session, err := newSupervisionSession(claim.Challenge, sha256.Sum256(payload), public, time.Now)
	require.NoError(t, err)
	c.supervision = session
	pulse := func() bool {
		session.mu.Lock()
		due := session.sequence == 0 || !time.Now().Before(session.issued.Add(supervisionInterval))
		session.mu.Unlock()
		if !due {
			return session.live()
		}
		probe, err := randomSupervisionProbe(session)
		if err != nil {
			return false
		}
		if responder.observe(probe) != nil {
			return false
		}
		response, err := responder.respond(probe)
		return err == nil && session.accept(response) == nil
	}
	require.True(t, pulse())
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(20 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-session.revoked():
				return
			case <-ticker.C:
				if !pulse() {
					return
				}
			}
		}
	}()
	t.Cleanup(func() { cancel(); <-done; session.revoke(); _, _ = responder.stop() })
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
			http.Error(w, "unavailable", 503)
			return
		}
		response, err := responder.respond(probe)
		if err != nil {
			http.Error(w, "unavailable", 503)
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
