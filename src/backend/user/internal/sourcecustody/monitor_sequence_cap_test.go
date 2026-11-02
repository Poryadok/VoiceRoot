package sourcecustody

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestMonitorSequenceCapRequiresRecoveryAndRetainsStop(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	now := time.Unix(1000, 0)
	challenge, decision, nonce := supervisionNonce(1), supervisionNonce(2), supervisionNonce(3)
	session, err := newSupervisionSession(challenge, decision, public, func() time.Time { return now })
	require.NoError(t, err)
	session.sequence = maxMonitorProbeSequence
	session.issued = now.Add(-supervisionInterval)
	session.armed = true
	session.validUntil = now.Add(supervisionDeadline)
	_, err = session.issue(nonce)
	require.Error(t, err)
	select {
	case <-session.revoked():
	default:
		t.Fatal("exhaustion must revoke the incarnation")
	}
	probe := supervisionProbe{Challenge: challenge, DecisionDigest: decision, Sequence: maxMonitorProbeSequence, Nonce: nonce}
	require.True(t, validProbe(probe))
	finalSession, err := newSupervisionSession(challenge, decision, public, func() time.Time { return now })
	require.NoError(t, err)
	finalID := uuid.New()
	finalSession.armed = true
	finalSession.incarnation = finalID
	finalSession.sequence = maxMonitorProbeSequence
	finalSession.pending = probe
	finalSession.pendingUntil = now.Add(supervisionDeadline)
	finalSession.validUntil = now.Add(supervisionDeadline)
	finalHeartbeat := signedProbeResponse(t, probe, finalID, MonitorHeartbeat, private)
	require.NoError(t, finalSession.accept(finalHeartbeat))
	select {
	case <-finalSession.revoked():
	default:
		t.Fatal("accepting the final heartbeat must immediately revoke")
	}
	invalid := probe
	invalid.Sequence++
	require.False(t, validProbe(invalid))
	responder, err := newMonitorResponder(challenge, decision, uuid.New(), private)
	require.NoError(t, err)
	responder.latest = probe
	responder.responded = true
	require.Error(t, responder.observe(invalid))
	stop, err := responder.stop()
	require.NoError(t, err)
	require.EqualValues(t, maxMonitorProbeSequence, stop.Sequence)
	require.Equal(t, MonitorStop, stop.Kind)
	require.NoError(t, VerifyMonitorResponse(stop, public))
	stop.Sequence++
	_, err = MonitorResponsePayload(stop)
	require.Error(t, err)
	workerResponder, err := newMonitorResponder(challenge, decision, uuid.New(), private)
	require.NoError(t, err)
	previous := probe
	previous.Sequence--
	workerResponder.latest, workerResponder.responded = previous, true
	transport := &responseWorkerFixture{probes: map[uint64]supervisionProbe{maxMonitorProbeSequence: probe}}
	worker := newMonitorResponseWorker(workerResponder, transport, func(context.Context) error { return nil })
	worker.next = maxMonitorProbeSequence
	require.Error(t, worker.step(context.Background()), "the last response exhausts the incarnation")
	require.NoError(t, worker.stop())
	require.Len(t, transport.responses, 2)
	require.Equal(t, MonitorHeartbeat, transport.responses[0].Kind)
	require.Equal(t, MonitorStop, transport.responses[1].Kind)
	require.EqualValues(t, maxMonitorProbeSequence, transport.responses[1].Sequence)
}
