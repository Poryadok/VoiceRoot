package sourcecustody

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestMonitorResponderStopUsesHighestObservedProbe(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	incarnation := uuid.New()
	responder, err := newMonitorResponder(supervisionNonce(1), supervisionNonce(2), incarnation, private)
	require.NoError(t, err)
	first := supervisionProbe{Challenge: supervisionNonce(1), DecisionDigest: supervisionNonce(2), Sequence: 1, Nonce: supervisionNonce(3)}
	require.NoError(t, responder.observe(first))
	armed, err := responder.respond(first)
	require.NoError(t, err)
	require.Equal(t, MonitorArmed, armed.Kind)
	require.NoError(t, VerifyMonitorResponse(armed, public))
	second := first
	second.Sequence = 2
	second.Nonce = supervisionNonce(4)
	require.NoError(t, responder.observe(second))
	stop, err := responder.stop()
	require.NoError(t, err)
	require.Equal(t, MonitorStop, stop.Kind)
	require.Equal(t, second.Sequence, stop.Sequence)
	require.Equal(t, second.Nonce, stop.Nonce)
	require.Equal(t, incarnation, stop.Incarnation)
	require.NoError(t, VerifyMonitorResponse(stop, public))
	again, err := responder.stop()
	require.NoError(t, err, "an uncertain STOP acknowledgement must be retryable")
	require.Equal(t, stop, again)
	stop.Signature[0] ^= 1
	require.NoError(t, VerifyMonitorResponse(again, public), "a returned signature must not alias retained retry state")
	_, err = responder.respond(second)
	require.Error(t, err, "shutdown must prevent later heartbeat signing")
	third := second
	third.Sequence = 3
	third.Nonce = supervisionNonce(5)
	require.Error(t, responder.observe(third))
}

func TestMonitorResponderRejectsChangedAndOutOfOrderProbes(t *testing.T) {
	_, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	responder, err := newMonitorResponder(supervisionNonce(1), supervisionNonce(2), uuid.New(), private)
	require.NoError(t, err)
	probe := supervisionProbe{Challenge: supervisionNonce(1), DecisionDigest: supervisionNonce(2), Sequence: 1, Nonce: supervisionNonce(3)}
	for name, mutate := range map[string]func(*supervisionProbe){
		"challenge":  func(p *supervisionProbe) { p.Challenge = supervisionNonce(7) },
		"decision":   func(p *supervisionProbe) { p.DecisionDigest = supervisionNonce(7) },
		"zero nonce": func(p *supervisionProbe) { p.Nonce = [32]byte{} },
		"skip first": func(p *supervisionProbe) { p.Sequence = 2 },
	} {
		t.Run(name, func(t *testing.T) {
			wrong := probe
			mutate(&wrong)
			require.Error(t, responder.observe(wrong))
		})
	}
	require.NoError(t, responder.observe(probe))
	changed := probe
	changed.Nonce = supervisionNonce(8)
	require.Error(t, responder.observe(changed), "one sequence cannot change its nonce")
	next := probe
	next.Sequence = 2
	next.Nonce = supervisionNonce(9)
	require.Error(t, responder.observe(next), "the prior probe must be answered before advancing")
	first, err := responder.respond(probe)
	require.NoError(t, err)
	retry, err := responder.respond(probe)
	require.NoError(t, err)
	require.Equal(t, first, retry, "repeating an immutable response must be deterministic")
	require.NoError(t, responder.observe(next))
	require.Error(t, responder.observe(probe), "an older probe cannot replace the latest")
	next.Sequence = 3
	next.Nonce = supervisionNonce(10)
	require.Error(t, responder.observe(next), "an unanswered probe cannot be skipped")
}

func TestMonitorResponderObserveAndStopAreSerialized(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	responder, err := newMonitorResponder(supervisionNonce(1), supervisionNonce(2), uuid.New(), private)
	require.NoError(t, err)
	first := supervisionProbe{Challenge: supervisionNonce(1), DecisionDigest: supervisionNonce(2), Sequence: 1, Nonce: supervisionNonce(3)}
	require.NoError(t, responder.observe(first))
	_, err = responder.respond(first)
	require.NoError(t, err)
	second := first
	second.Sequence = 2
	second.Nonce = supervisionNonce(4)
	start := make(chan struct{})
	observed := make(chan error, 1)
	type result struct {
		response MonitorResponse
		err      error
	}
	stopped := make(chan result, 1)
	go func() { <-start; observed <- responder.observe(second) }()
	go func() { <-start; response, err := responder.stop(); stopped <- result{response, err} }()
	close(start)
	observeErr, stop := <-observed, <-stopped
	require.NoError(t, stop.err)
	require.NoError(t, VerifyMonitorResponse(stop.response, public))
	if observeErr == nil {
		require.Equal(t, second.Sequence, stop.response.Sequence)
		require.Equal(t, second.Nonce, stop.response.Nonce)
	} else {
		require.Equal(t, first.Sequence, stop.response.Sequence)
		require.Equal(t, first.Nonce, stop.response.Nonce)
	}
}

func TestMonitorResponderStopBeforeProbeClearsKey(t *testing.T) {
	_, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	responder, err := newMonitorResponder(supervisionNonce(1), supervisionNonce(2), uuid.New(), private)
	require.NoError(t, err)
	_, err = responder.stop()
	require.Error(t, err)
	for _, value := range responder.key {
		require.Zero(t, value)
	}
	require.Error(t, responder.observe(supervisionProbe{Challenge: supervisionNonce(1), DecisionDigest: supervisionNonce(2), Sequence: 1, Nonce: supervisionNonce(3)}))
}

func TestMonitorResponderConcurrentHeartbeatAndStop(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	responder, err := newMonitorResponder(supervisionNonce(1), supervisionNonce(2), uuid.New(), private)
	require.NoError(t, err)
	first := supervisionProbe{Challenge: supervisionNonce(1), DecisionDigest: supervisionNonce(2), Sequence: 1, Nonce: supervisionNonce(3)}
	require.NoError(t, responder.observe(first))
	_, err = responder.respond(first)
	require.NoError(t, err)
	second := first
	second.Sequence = 2
	second.Nonce = supervisionNonce(4)
	require.NoError(t, responder.observe(second))
	start := make(chan struct{})
	type result struct {
		response MonitorResponse
		err      error
	}
	heartbeat := make(chan result, 1)
	stopped := make(chan result, 1)
	go func() { <-start; response, err := responder.respond(second); heartbeat <- result{response, err} }()
	go func() { <-start; response, err := responder.stop(); stopped <- result{response, err} }()
	close(start)
	h, s := <-heartbeat, <-stopped
	require.NoError(t, s.err)
	require.Equal(t, second.Sequence, s.response.Sequence)
	require.Equal(t, MonitorStop, s.response.Kind)
	require.NoError(t, VerifyMonitorResponse(s.response, public))
	if h.err == nil {
		require.Equal(t, second.Sequence, h.response.Sequence)
		require.Equal(t, MonitorHeartbeat, h.response.Kind)
		require.NoError(t, VerifyMonitorResponse(h.response, public))
	}
	_, err = responder.respond(second)
	require.Error(t, err)
}

func TestMonitorResponderRejectsStaleProducerAndMalformedKey(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	bad := append(ed25519.PrivateKey(nil), private...)
	bad[len(bad)-1] ^= 1
	_, err = newMonitorResponder(supervisionNonce(1), supervisionNonce(2), uuid.New(), bad)
	require.Error(t, err, "private key material must be canonical")
	responder, err := newMonitorResponder(supervisionNonce(1), supervisionNonce(2), uuid.New(), private)
	require.NoError(t, err)
	for i := range private {
		private[i] = 0
	}
	first := supervisionProbe{Challenge: supervisionNonce(1), DecisionDigest: supervisionNonce(2), Sequence: 1, Nonce: supervisionNonce(3)}
	require.NoError(t, responder.observe(first))
	armed, err := responder.respond(first)
	require.NoError(t, err)
	require.NoError(t, VerifyMonitorResponse(armed, public), "the responder must own a key copy")
	second := first
	second.Sequence = 2
	second.Nonce = supervisionNonce(4)
	require.NoError(t, responder.observe(second))
	_, err = responder.respond(first)
	require.Error(t, err, "a delayed producer must not sign a different probe")
	_, err = responder.stop()
	require.NoError(t, err)
	for _, value := range responder.key {
		require.Zero(t, value, "owned key bytes must be cleared after STOP")
	}
}
