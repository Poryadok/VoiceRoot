package sourcecustody

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestMonitorBatchDuplicateDoesNotRenewAndStopWins(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	now := time.Unix(1000, 0)
	s, err := newSupervisionSession(supervisionNonce(1), supervisionNonce(2), public, func() time.Time { return now })
	require.NoError(t, err)
	p, err := s.issue(supervisionNonce(3))
	require.NoError(t, err)
	id := uuid.New()
	armed := signedProbeResponse(t, p, id, MonitorArmed, private)
	b := newMonitorBatch(s)
	require.NoError(t, b.accept([]MonitorResponse{armed}))
	now = now.Add(time.Second)
	next, err := s.issue(supervisionNonce(4))
	require.NoError(t, err)
	heartbeat := signedProbeResponse(t, next, id, MonitorHeartbeat, private)
	stop := signedProbeResponse(t, p, id, MonitorStop, private)
	require.NoError(t, b.accept([]MonitorResponse{heartbeat, armed, stop}))
	require.False(t, s.live())
	require.Error(t, b.accept([]MonitorResponse{heartbeat}))
}

func TestMonitorBatchRejectsTamperingAndOversizedReads(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	s, err := newSupervisionSession(supervisionNonce(1), supervisionNonce(2), public, time.Now)
	require.NoError(t, err)
	p, err := s.issue(supervisionNonce(3))
	require.NoError(t, err)
	response := signedProbeResponse(t, p, uuid.New(), MonitorArmed, private)
	b := newMonitorBatch(s)
	require.NoError(t, b.accept([]MonitorResponse{response}))
	response.Signature = append([]byte(nil), response.Signature...)
	response.Signature[0] ^= 1
	require.Error(t, b.accept([]MonitorResponse{response}))
	require.False(t, s.live())

	s, err = newSupervisionSession(supervisionNonce(1), supervisionNonce(2), public, time.Now)
	require.NoError(t, err)
	b = newMonitorBatch(s)
	require.Error(t, b.accept(make([]MonitorResponse, 5)))
	select {
	case <-s.revoked():
	default:
		t.Fatal("oversized read must revoke the session")
	}
}

func TestMonitorBatchRepeatedRowDoesNotExtendDeadline(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	now := time.Unix(1000, 0)
	s, err := newSupervisionSession(supervisionNonce(1), supervisionNonce(2), public, func() time.Time { return now })
	require.NoError(t, err)
	p, err := s.issue(supervisionNonce(3))
	require.NoError(t, err)
	row := signedProbeResponse(t, p, uuid.New(), MonitorArmed, private)
	b := newMonitorBatch(s)
	require.NoError(t, b.accept([]MonitorResponse{row}))
	now = now.Add(2 * time.Second)
	require.NoError(t, b.accept([]MonitorResponse{row}))
	now = now.Add(time.Second)
	require.Error(t, b.accept([]MonitorResponse{row}))
	require.False(t, s.live())
}

func TestMonitorBatchEmptyReadChecksDeadline(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	now := time.Unix(1000, 0)
	s, err := newSupervisionSession(supervisionNonce(1), supervisionNonce(2), public, func() time.Time { return now })
	require.NoError(t, err)
	p, err := s.issue(supervisionNonce(3))
	require.NoError(t, err)
	b := newMonitorBatch(s)
	require.NoError(t, b.accept(nil), "unarmed pending probe can wait")
	require.NoError(t, b.accept([]MonitorResponse{signedProbeResponse(t, p, uuid.New(), MonitorArmed, private)}))
	now = now.Add(3 * time.Second)
	require.Error(t, b.accept(nil))
	require.False(t, s.live())
}
