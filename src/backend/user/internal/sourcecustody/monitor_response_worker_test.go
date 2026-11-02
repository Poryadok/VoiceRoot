package sourcecustody

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type responseWorkerFixture struct {
	probes    map[uint64]supervisionProbe
	responses []MonitorResponse
	onRead    func()
	onInsert  func()
}

func (f *responseWorkerFixture) ReadProbe(_ context.Context, challenge, digest [32]byte, sequence uint64) (supervisionProbe, bool, error) {
	if f.onRead != nil {
		f.onRead()
	}
	p, ok := f.probes[sequence]
	if ok && (p.Challenge != challenge || p.DecisionDigest != digest) {
		return supervisionProbe{}, false, ErrUnavailable
	}
	return p, ok, nil
}

func TestMonitorResponseWorkerHealthLossDuringReadIsTerminal(t *testing.T) {
	_, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	challenge, digest := supervisionNonce(1), supervisionNonce(2)
	r, err := newMonitorResponder(challenge, digest, uuid.New(), private)
	require.NoError(t, err)
	healthy := true
	f := &responseWorkerFixture{probes: map[uint64]supervisionProbe{1: {Challenge: challenge, DecisionDigest: digest, Sequence: 1, Nonce: supervisionNonce(3)}}, onRead: func() { healthy = false }}
	w := newMonitorResponseWorker(r, f, func(context.Context) error {
		if !healthy {
			return errors.New("unhealthy")
		}
		return nil
	})
	require.Error(t, w.step(context.Background()))
	require.Empty(t, f.responses)
	healthy = true
	require.Error(t, w.step(context.Background()), "health loss must be terminal")
}

func (f *responseWorkerFixture) InsertExactResponse(_ context.Context, response MonitorResponse) error {
	f.responses = append(f.responses, response)
	if f.onInsert != nil {
		f.onInsert()
	}
	return nil
}

func TestMonitorResponseWorkerHealthLossDuringPublicationIsTerminal(t *testing.T) {
	_, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	challenge, digest := supervisionNonce(1), supervisionNonce(2)
	r, err := newMonitorResponder(challenge, digest, uuid.New(), private)
	require.NoError(t, err)
	healthy := true
	f := &responseWorkerFixture{probes: map[uint64]supervisionProbe{1: {Challenge: challenge, DecisionDigest: digest, Sequence: 1, Nonce: supervisionNonce(3)}}, onInsert: func() { healthy = false }}
	w := newMonitorResponseWorker(r, f, func(context.Context) error {
		if !healthy {
			return errors.New("unhealthy")
		}
		return nil
	})
	require.Error(t, w.step(context.Background()))
	require.Len(t, f.responses, 1, "the earlier ARMED may already have committed")
	require.NoError(t, w.stop())
	require.Len(t, f.responses, 2)
	require.Equal(t, MonitorStop, f.responses[1].Kind)
	require.Equal(t, uint64(1), f.responses[1].Sequence)
}

func TestMonitorResponseWorkerHealthLossDuringEmptyReadIsTerminal(t *testing.T) {
	_, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	r, err := newMonitorResponder(supervisionNonce(1), supervisionNonce(2), uuid.New(), private)
	require.NoError(t, err)
	healthy := true
	f := &responseWorkerFixture{onRead: func() { healthy = false }}
	w := newMonitorResponseWorker(r, f, func(context.Context) error {
		if !healthy {
			return errors.New("unhealthy")
		}
		return nil
	})
	require.Error(t, w.step(context.Background()))
	healthy = true
	require.Error(t, w.step(context.Background()))
}

func TestMonitorResponseWorkerCancellationAndStopRetry(t *testing.T) {
	for _, phase := range []string{"read", "publish"} {
		t.Run(phase, func(t *testing.T) {
			_, private, err := ed25519.GenerateKey(rand.Reader)
			require.NoError(t, err)
			challenge, digest := supervisionNonce(1), supervisionNonce(2)
			r, err := newMonitorResponder(challenge, digest, uuid.New(), private)
			require.NoError(t, err)
			ctx, cancel := context.WithCancel(context.Background())
			f := &responseWorkerFixture{probes: map[uint64]supervisionProbe{1: {Challenge: challenge, DecisionDigest: digest, Sequence: 1, Nonce: supervisionNonce(3)}}}
			if phase == "read" {
				f.onRead = cancel
			} else {
				f.onInsert = cancel
			}
			w := newMonitorResponseWorker(r, f, func(context.Context) error { return nil })
			require.Error(t, w.step(ctx))
			require.Error(t, w.step(context.Background()), "cancellation burns this worker")
			if phase == "read" {
				require.Empty(t, f.responses)
				require.Error(t, w.stop(), "no probe was observed to sign a STOP")
				return
			}
			require.NoError(t, w.stop())
			require.NoError(t, w.stop())
			require.Len(t, f.responses, 3)
			require.Equal(t, MonitorStop, f.responses[1].Kind)
			require.Equal(t, f.responses[1], f.responses[2], "STOP retries must be byte-identical")
		})
	}
}

func TestMonitorResponseWorkerRunPublishesStopOnCancellation(t *testing.T) {
	_, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	challenge, digest := supervisionNonce(1), supervisionNonce(2)
	r, err := newMonitorResponder(challenge, digest, uuid.New(), private)
	require.NoError(t, err)
	armed := make(chan struct{}, 1)
	f := &responseWorkerFixture{probes: map[uint64]supervisionProbe{1: {Challenge: challenge, DecisionDigest: digest, Sequence: 1, Nonce: supervisionNonce(3)}}, onInsert: func() {
		select {
		case armed <- struct{}{}:
		default:
		}
	}}
	w := newMonitorResponseWorker(r, f, func(context.Context) error { return nil })
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- w.run(ctx, make(chan time.Time)) }()
	<-armed
	cancel()
	require.Error(t, <-done)
	require.Len(t, f.responses, 2)
	require.Equal(t, MonitorStop, f.responses[1].Kind)
}

func TestMonitorResponseWorkerSignsOnlyHealthyOrderedProbesAndStops(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	challenge, digest := supervisionNonce(1), supervisionNonce(2)
	r, err := newMonitorResponder(challenge, digest, uuid.New(), private)
	require.NoError(t, err)
	f := &responseWorkerFixture{probes: map[uint64]supervisionProbe{1: {Challenge: challenge, DecisionDigest: digest, Sequence: 1, Nonce: supervisionNonce(3)}, 2: {Challenge: challenge, DecisionDigest: digest, Sequence: 2, Nonce: supervisionNonce(4)}}}
	healthy := true
	w := newMonitorResponseWorker(r, f, func(context.Context) error {
		if !healthy {
			return errors.New("unhealthy")
		}
		return nil
	})
	require.NoError(t, w.step(context.Background()))
	require.Equal(t, MonitorArmed, f.responses[0].Kind)
	require.NoError(t, w.step(context.Background()))
	require.Equal(t, MonitorHeartbeat, f.responses[1].Kind)
	healthy = false
	require.Error(t, w.step(context.Background()))
	healthy = true
	require.Error(t, w.step(context.Background()), "a failed health check must burn this responder")
	require.NoError(t, w.stop())
	require.Equal(t, MonitorStop, f.responses[2].Kind)
	require.Equal(t, uint64(2), f.responses[2].Sequence)
	for _, response := range f.responses {
		require.NoError(t, VerifyMonitorResponse(response, public))
	}
	require.Error(t, w.step(context.Background()))
}
