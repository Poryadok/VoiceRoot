package sourcecustody

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type monitorPollFixture struct {
	private   ed25519.PrivateKey
	id        uuid.UUID
	probes    []supervisionProbe
	responses []MonitorResponse
	err       error
	onRead    func()
}

type blockingMonitorStore struct {
	private ed25519.PrivateKey
	probe   supervisionProbe
	entered chan struct{}
	release chan struct{}
}

func (f *blockingMonitorStore) PublishProbe(_ context.Context, p supervisionProbe) error {
	f.probe = p
	return nil
}

func (f *blockingMonitorStore) ReadResponses(_ context.Context, _, _ supervisionProbe) ([]MonitorResponse, error) {
	close(f.entered)
	<-f.release // Deliberately violates the adapter contract to model a hung call.
	r, err := SignMonitorResponse(MonitorResponse{Challenge: f.probe.Challenge, DecisionDigest: f.probe.DecisionDigest, Incarnation: uuid.New(), Sequence: f.probe.Sequence, Nonce: f.probe.Nonce, Kind: MonitorArmed}, f.private)
	return []MonitorResponse{r}, err
}

func TestMonitorPollerBlockedReadCannotPreventExpiryOrRevive(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	var nanos atomic.Int64
	nanos.Store(time.Unix(1000, 0).UnixNano())
	s, err := newSupervisionSession(supervisionNonce(1), supervisionNonce(2), public, func() time.Time { return time.Unix(0, nanos.Load()) })
	require.NoError(t, err)
	f := &blockingMonitorStore{private: private, entered: make(chan struct{}), release: make(chan struct{})}
	p := newMonitorPoller(s, f, rand.Reader)
	done := make(chan error, 1)
	go func() { done <- p.step(context.Background()) }()
	<-f.entered
	ticks := make(chan time.Time, 1)
	exit := make(chan struct{})
	go func() { supervisionExpiryLoop(context.Background(), s, ticks); close(exit) }()
	nanos.Add(int64(3 * time.Second))
	ticks <- time.Time{}
	select {
	case <-s.revoked():
	case <-time.After(time.Second):
		t.Fatal("expiry waited for store I/O")
	}
	close(f.release)
	require.Error(t, <-done)
	<-exit
	require.False(t, s.live())
}

func (f *monitorPollFixture) PublishProbe(_ context.Context, p supervisionProbe) error {
	if f.err != nil {
		return f.err
	}
	f.probes = append(f.probes, p)
	return nil
}

func (f *monitorPollFixture) ReadResponses(_ context.Context, _, _ supervisionProbe) ([]MonitorResponse, error) {
	if f.onRead != nil {
		f.onRead()
	}
	return f.responses, f.err
}

func TestMonitorPollerProbeAndResponseLifecycle(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	now := time.Unix(1000, 0)
	s, err := newSupervisionSession(supervisionNonce(1), supervisionNonce(2), public, func() time.Time { return now })
	require.NoError(t, err)
	f := &monitorPollFixture{private: private, id: uuid.New()}
	p := newMonitorPoller(s, f, rand.Reader)
	require.NoError(t, p.step(context.Background()))
	require.Len(t, f.probes, 1)
	f.responses = []MonitorResponse{signedProbeResponse(t, f.probes[0], f.id, MonitorArmed, private)}
	require.NoError(t, p.step(context.Background()))
	require.True(t, s.live())
	now = now.Add(time.Second)
	require.NoError(t, p.step(context.Background()))
	require.Len(t, f.probes, 2)
	f.responses = append(f.responses, signedProbeResponse(t, f.probes[1], f.id, MonitorHeartbeat, private))
	require.NoError(t, p.step(context.Background()))
	require.True(t, s.live())
	now = now.Add(3 * time.Second)
	require.Error(t, p.step(context.Background()))
	require.False(t, s.live())
}

func TestMonitorPollerStoreFailureRevokes(t *testing.T) {
	public, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	s, err := newSupervisionSession(supervisionNonce(1), supervisionNonce(2), public, time.Now)
	require.NoError(t, err)
	f := &monitorPollFixture{err: errors.New("storage unavailable")}
	p := newMonitorPoller(s, f, rand.Reader)
	require.Error(t, p.step(context.Background()))
	select {
	case <-s.revoked():
	default:
		t.Fatal("store failure must revoke")
	}
}

func TestMonitorPollerEmptyReadCrossingDeadlineRevokes(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	now := time.Unix(1000, 0)
	s, err := newSupervisionSession(supervisionNonce(1), supervisionNonce(2), public, func() time.Time { return now })
	require.NoError(t, err)
	probe, err := s.issue(supervisionNonce(3))
	require.NoError(t, err)
	require.NoError(t, s.accept(signedProbeResponse(t, probe, uuid.New(), MonitorArmed, private)))
	now = now.Add(2900 * time.Millisecond)
	f := &monitorPollFixture{onRead: func() { now = now.Add(100 * time.Millisecond) }}
	p := newMonitorPoller(s, f, rand.Reader)
	require.Error(t, p.step(context.Background()))
	require.False(t, s.live())
}

func TestMonitorPollerRunCancellationRevokes(t *testing.T) {
	public, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	s, err := newSupervisionSession(supervisionNonce(1), supervisionNonce(2), public, time.Now)
	require.NoError(t, err)
	p := newMonitorPoller(s, &monitorPollFixture{}, rand.Reader)
	ctx, cancel := context.WithCancel(context.Background())
	ticks := make(chan time.Time)
	done := make(chan struct{})
	go func() { p.run(ctx, ticks); close(done) }()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("poller did not stop")
	}
	select {
	case <-s.revoked():
	default:
		t.Fatal("stopped poller must revoke")
	}
}
