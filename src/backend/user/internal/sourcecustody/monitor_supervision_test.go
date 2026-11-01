package sourcecustody

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func supervisionNonce(value byte) [32]byte {
	var nonce [32]byte
	for i := range nonce {
		nonce[i] = value
	}
	return nonce
}

func signedProbeResponse(t *testing.T, probe supervisionProbe, incarnation uuid.UUID, kind MonitorResponseKind, private ed25519.PrivateKey) MonitorResponse {
	t.Helper()
	response, err := SignMonitorResponse(MonitorResponse{Challenge: probe.Challenge, DecisionDigest: probe.DecisionDigest, Incarnation: incarnation, Sequence: probe.Sequence, Nonce: probe.Nonce, Kind: kind}, private)
	require.NoError(t, err)
	return response
}

func TestSupervisionProbeDeadlineCannotBeExtendedByReplay(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	start := time.Unix(1000, 0)
	now := start
	session, err := newSupervisionSession(supervisionNonce(1), supervisionNonce(2), public, func() time.Time { return now })
	require.NoError(t, err)
	probe, err := session.issue(supervisionNonce(3))
	require.NoError(t, err)
	incarnation := uuid.New()
	armed := signedProbeResponse(t, probe, incarnation, MonitorArmed, private)
	now = start.Add(100 * time.Millisecond)
	require.NoError(t, session.accept(armed))
	now = start.Add(time.Second)
	next, err := session.issue(supervisionNonce(4))
	require.NoError(t, err)
	now = start.Add(2 * time.Second)
	require.True(t, session.live())
	require.Error(t, session.accept(armed), "re-reading one response must not renew it")
	late := signedProbeResponse(t, next, incarnation, MonitorHeartbeat, private)
	now = start.Add(3 * time.Second)
	require.False(t, session.live(), "the original deadline is strict")
	select {
	case <-session.revoked():
	default:
		t.Fatal("expiry must notify held permits")
	}
	require.Error(t, session.accept(late), "late traffic cannot revive an incarnation")
	now = start.Add(4 * time.Second)
	_, err = session.issue(supervisionNonce(5))
	require.Error(t, err)
}

func TestSupervisionRejectsWrongProbeAndIncarnation(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	start := time.Unix(1000, 0)
	now := start
	session, err := newSupervisionSession(supervisionNonce(1), supervisionNonce(2), public, func() time.Time { return now })
	require.NoError(t, err)
	probe, err := session.issue(supervisionNonce(3))
	require.NoError(t, err)
	incarnation := uuid.New()
	wrong := probe
	wrong.Nonce = supervisionNonce(9)
	now = start.Add(100 * time.Millisecond)
	require.Error(t, session.accept(signedProbeResponse(t, wrong, incarnation, MonitorArmed, private)))
	require.False(t, session.live())
	now = start.Add(200 * time.Millisecond)
	require.NoError(t, session.accept(signedProbeResponse(t, probe, incarnation, MonitorArmed, private)))
	now = start.Add(500 * time.Millisecond)
	_, err = session.issue(supervisionNonce(4))
	require.Error(t, err, "probes must respect the interval")
	now = start.Add(time.Second)
	next, err := session.issue(supervisionNonce(4))
	require.NoError(t, err)
	now = start.Add(1100 * time.Millisecond)
	require.Error(t, session.accept(signedProbeResponse(t, next, uuid.New(), MonitorHeartbeat, private)))
	now = start.Add(1200 * time.Millisecond)
	require.NoError(t, session.accept(signedProbeResponse(t, next, incarnation, MonitorHeartbeat, private)))
	now = start.Add(3999 * time.Millisecond)
	require.True(t, session.live())
	now = start.Add(4 * time.Second)
	require.False(t, session.live())
}

func TestSupervisionSignedStopRevokesImmediately(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	start := time.Unix(1000, 0)
	now := start
	session, err := newSupervisionSession(supervisionNonce(1), supervisionNonce(2), public, func() time.Time { return now })
	require.NoError(t, err)
	probe, err := session.issue(supervisionNonce(3))
	require.NoError(t, err)
	incarnation := uuid.New()
	require.NoError(t, session.accept(signedProbeResponse(t, probe, incarnation, MonitorArmed, private)))
	now = start.Add(time.Second)
	next, err := session.issue(supervisionNonce(4))
	require.NoError(t, err)
	require.NoError(t, session.accept(signedProbeResponse(t, next, incarnation, MonitorStop, private)))
	require.False(t, session.live())
	select {
	case <-session.revoked():
	default:
		t.Fatal("signed stop must notify held permits")
	}
}

func TestSupervisionReadsClockAfterMutexContention(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	start := time.Unix(1000, 0)
	var nanos atomic.Int64
	nanos.Store(start.UnixNano())
	clock := func() time.Time { return time.Unix(0, nanos.Load()) }
	session, err := newSupervisionSession(supervisionNonce(1), supervisionNonce(2), public, clock)
	require.NoError(t, err)
	probe, err := session.issue(supervisionNonce(3))
	require.NoError(t, err)
	require.NoError(t, session.accept(signedProbeResponse(t, probe, uuid.New(), MonitorArmed, private)))
	session.mu.Lock()
	started := make(chan struct{})
	done := make(chan bool, 1)
	go func() { close(started); done <- session.live() }()
	<-started
	nanos.Store(start.Add(3 * time.Second).UnixNano())
	session.mu.Unlock()
	require.False(t, <-done, "a waiter must not use the time from before mutex contention")
}

func TestSupervisionSamplesClockWhileLocked(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	var session *supervisionSession
	var unprotected atomic.Bool
	clock := func() time.Time {
		if session != nil && session.mu.TryLock() {
			session.mu.Unlock()
			unprotected.Store(true)
		}
		return time.Unix(1000, 0)
	}
	session, err = newSupervisionSession(supervisionNonce(1), supervisionNonce(2), public, clock)
	require.NoError(t, err)
	probe, err := session.issue(supervisionNonce(3))
	require.NoError(t, err)
	require.NoError(t, session.accept(signedProbeResponse(t, probe, uuid.New(), MonitorArmed, private)))
	require.True(t, session.live())
	require.False(t, unprotected.Load(), "deadline decisions must sample time while serialized")
}

func TestSupervisionLateResponseRevokesWithoutExpiryWorker(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	start := time.Unix(1000, 0)
	now := start
	session, err := newSupervisionSession(supervisionNonce(1), supervisionNonce(2), public, func() time.Time { return now })
	require.NoError(t, err)
	first, err := session.issue(supervisionNonce(3))
	require.NoError(t, err)
	incarnation := uuid.New()
	require.NoError(t, session.accept(signedProbeResponse(t, first, incarnation, MonitorArmed, private)))
	now = start.Add(time.Second)
	next, err := session.issue(supervisionNonce(4))
	require.NoError(t, err)
	now = start.Add(3500 * time.Millisecond)
	err = session.accept(signedProbeResponse(t, next, incarnation, MonitorHeartbeat, private))
	require.Error(t, err, "a new probe must not extend the last accepted deadline")
	select {
	case <-session.revoked():
	default:
		t.Fatal("accept must revoke even if the expiry worker did not run")
	}
}

func TestSupervisionOldExpiryCheckCannotRevokeRenewedSession(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	start := time.Unix(1000, 0)
	now := start
	session, err := newSupervisionSession(supervisionNonce(1), supervisionNonce(2), public, func() time.Time { return now })
	require.NoError(t, err)
	first, err := session.issue(supervisionNonce(3))
	require.NoError(t, err)
	incarnation := uuid.New()
	require.NoError(t, session.accept(signedProbeResponse(t, first, incarnation, MonitorArmed, private)))
	now = start.Add(time.Second)
	next, err := session.issue(supervisionNonce(4))
	require.NoError(t, err)
	require.NoError(t, session.accept(signedProbeResponse(t, next, incarnation, MonitorHeartbeat, private)))
	now = start.Add(3 * time.Second)
	require.True(t, session.live(), "an old deadline callback must recheck the renewed deadline")
	now = start.Add(4 * time.Second)
	require.False(t, session.live())
}

func TestSupervisionUnarmedTimeoutAndClockRollbackRevoke(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	start := time.Unix(1000, 0)
	for _, mode := range []string{"unarmed-timeout", "clock-rollback"} {
		t.Run(mode, func(t *testing.T) {
			now := start
			session, err := newSupervisionSession(supervisionNonce(1), supervisionNonce(2), public, func() time.Time { return now })
			require.NoError(t, err)
			probe, err := session.issue(supervisionNonce(3))
			require.NoError(t, err)
			armed := signedProbeResponse(t, probe, uuid.New(), MonitorArmed, private)
			if mode == "unarmed-timeout" {
				now = start.Add(3 * time.Second)
			} else {
				now = start.Add(-time.Nanosecond)
			}
			require.Error(t, session.accept(armed))
			select {
			case <-session.revoked():
			default:
				t.Fatal("deadline or clock rollback must revoke")
			}
			now = start.Add(4 * time.Second)
			require.Error(t, session.accept(armed), "revocation is permanent")
		})
	}
}

func TestSupervisionConcurrentDuplicateResponseAcceptedOnce(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	start := time.Unix(1000, 0)
	session, err := newSupervisionSession(supervisionNonce(1), supervisionNonce(2), public, func() time.Time { return start })
	require.NoError(t, err)
	probe, err := session.issue(supervisionNonce(3))
	require.NoError(t, err)
	response := signedProbeResponse(t, probe, uuid.New(), MonitorArmed, private)
	startCalls := make(chan struct{})
	results := make(chan error, 8)
	for i := 0; i < cap(results); i++ {
		go func() { <-startCalls; results <- session.accept(response) }()
	}
	close(startCalls)
	accepted := 0
	for i := 0; i < cap(results); i++ {
		if <-results == nil {
			accepted++
		}
	}
	require.Equal(t, 1, accepted)
	require.True(t, session.live())
}

func TestSupervisionStopAndHeartbeatOrderAlwaysRevokes(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	start := time.Unix(1000, 0)
	for _, order := range []string{"heartbeat-first", "stop-first", "concurrent"} {
		t.Run(order, func(t *testing.T) {
			now := start
			session, err := newSupervisionSession(supervisionNonce(1), supervisionNonce(2), public, func() time.Time { return now })
			require.NoError(t, err)
			first, err := session.issue(supervisionNonce(3))
			require.NoError(t, err)
			incarnation := uuid.New()
			require.NoError(t, session.accept(signedProbeResponse(t, first, incarnation, MonitorArmed, private)))
			now = start.Add(time.Second)
			next, err := session.issue(supervisionNonce(4))
			require.NoError(t, err)
			heartbeat := signedProbeResponse(t, next, incarnation, MonitorHeartbeat, private)
			stop := signedProbeResponse(t, next, incarnation, MonitorStop, private)
			switch order {
			case "heartbeat-first":
				require.NoError(t, session.accept(heartbeat))
				require.NoError(t, session.accept(stop))
			case "stop-first":
				require.NoError(t, session.accept(stop))
				require.Error(t, session.accept(heartbeat))
			default:
				startCalls := make(chan struct{})
				results := make(chan error, 2)
				go func() { <-startCalls; results <- session.accept(heartbeat) }()
				go func() { <-startCalls; results <- session.accept(stop) }()
				close(startCalls)
				<-results
				<-results
			}
			require.False(t, session.live())
			select {
			case <-session.revoked():
			default:
				t.Fatal("a valid stop must revoke regardless of ordering")
			}
		})
	}
}

func TestSupervisionExternalRevocationCannotBeReversed(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	start := time.Unix(1000, 0)
	now := start
	session, err := newSupervisionSession(supervisionNonce(1), supervisionNonce(2), public, func() time.Time { return now })
	require.NoError(t, err)
	first, err := session.issue(supervisionNonce(3))
	require.NoError(t, err)
	incarnation := uuid.New()
	require.NoError(t, session.accept(signedProbeResponse(t, first, incarnation, MonitorArmed, private)))
	now = start.Add(time.Second)
	next, err := session.issue(supervisionNonce(4))
	require.NoError(t, err)
	heartbeat := signedProbeResponse(t, next, incarnation, MonitorHeartbeat, private)
	startCalls := make(chan struct{})
	done := make(chan struct{}, 2)
	go func() { <-startCalls; _ = session.accept(heartbeat); done <- struct{}{} }()
	go func() { <-startCalls; session.revoke(); done <- struct{}{} }()
	close(startCalls)
	<-done
	<-done
	require.False(t, session.live())
	require.Error(t, session.accept(heartbeat))
	select {
	case <-session.revoked():
	default:
		t.Fatal("external revocation must notify held permits")
	}
}

func TestSupervisionExpiryWorkerRevokesWithoutTraffic(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	start := time.Unix(1000, 0)
	var nanos atomic.Int64
	nanos.Store(start.UnixNano())
	session, err := newSupervisionSession(supervisionNonce(1), supervisionNonce(2), public, func() time.Time { return time.Unix(0, nanos.Load()) })
	require.NoError(t, err)
	probe, err := session.issue(supervisionNonce(3))
	require.NoError(t, err)
	require.NoError(t, session.accept(signedProbeResponse(t, probe, uuid.New(), MonitorArmed, private)))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ticks := make(chan time.Time, 1)
	done := make(chan struct{})
	go func() { supervisionExpiryLoop(ctx, session, ticks); close(done) }()
	nanos.Store(start.Add(3 * time.Second).UnixNano())
	ticks <- start.Add(3 * time.Second)
	select {
	case <-session.revoked():
	case <-time.After(time.Second):
		t.Fatal("expiry must revoke without an incoming request")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("expiry worker did not exit")
	}
}

func TestSupervisionExpiryWorkerCancellationRevokesUnarmedSession(t *testing.T) {
	public, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	session, err := newSupervisionSession(supervisionNonce(1), supervisionNonce(2), public, time.Now)
	require.NoError(t, err)
	_, err = session.issue(supervisionNonce(3))
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { supervisionExpiryLoop(ctx, session, make(chan time.Time)); close(done) }()
	cancel()
	select {
	case <-session.revoked():
	case <-time.After(time.Second):
		t.Fatal("worker shutdown must revoke even before arming")
	}
	<-done
}

func TestSupervisionIsLiveOnlyForExactOpeningDecision(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	start := time.Unix(1000, 0)
	now := start
	claim := OpenAttestation{
		Challenge: supervisionNonce(1), ControlSystem: "123", ControlDatabase: "custody_db", ControlDatabaseOID: 9,
		Binding:  Binding{Epoch: 1, SourceID: "source", HistoryGeneration: uuid.New(), UserSourceGeneration: uuid.New(), KeysetDigest: supervisionNonce(2)},
		Physical: PhysicalSource{SystemIdentifier: "456", Database: "user_db", DatabaseOID: 42}, WriterRole: "writer",
		SourceEvidenceDigest: supervisionNonce(3), WriterManifestDigest: supervisionNonce(4), AuthEvidenceDigest: supervisionNonce(5),
		IssuedAtUnix: 900, ExpiresAtUnix: 1100,
	}
	payload, err := OpenAttestationPayload(claim)
	require.NoError(t, err)
	digest := sha256.Sum256(payload)
	session, err := newSupervisionSession(claim.Challenge, digest, public, func() time.Time { return now })
	require.NoError(t, err)
	probe, err := session.issue(supervisionNonce(6))
	require.NoError(t, err)
	require.False(t, session.liveFor(claim), "an unarmed session cannot authorize opening")
	require.NoError(t, session.accept(signedProbeResponse(t, probe, uuid.New(), MonitorArmed, private)))
	require.True(t, session.liveFor(claim))
	for name, mutate := range map[string]func(*OpenAttestation){
		"challenge":       func(c *OpenAttestation) { c.Challenge = supervisionNonce(9) },
		"physical OID":    func(c *OpenAttestation) { c.Physical.DatabaseOID++ },
		"writer":          func(c *OpenAttestation) { c.WriterRole = "other_writer" },
		"key digest":      func(c *OpenAttestation) { c.Binding.KeysetDigest = supervisionNonce(8) },
		"source evidence": func(c *OpenAttestation) { c.SourceEvidenceDigest = supervisionNonce(7) },
	} {
		t.Run(name, func(t *testing.T) {
			other := claim
			mutate(&other)
			require.False(t, session.liveFor(other))
		})
	}
	now = start.Add(3 * time.Second)
	require.False(t, session.liveFor(claim), "the exact decision cannot outlive the monitor deadline")
}

func TestSupervisionStopForLastAcceptedProbeRejectsWrongBinding(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	start := time.Unix(1000, 0)
	now := start
	session, err := newSupervisionSession(supervisionNonce(1), supervisionNonce(2), public, func() time.Time { return now })
	require.NoError(t, err)
	first, err := session.issue(supervisionNonce(3))
	require.NoError(t, err)
	incarnation := uuid.New()
	require.NoError(t, session.accept(signedProbeResponse(t, first, incarnation, MonitorArmed, private)))
	now = start.Add(time.Second)
	next, err := session.issue(supervisionNonce(4))
	require.NoError(t, err)
	require.NoError(t, session.accept(signedProbeResponse(t, next, incarnation, MonitorHeartbeat, private)))
	now = start.Add(2 * time.Second)
	_, err = session.issue(supervisionNonce(5))
	require.NoError(t, err)
	require.Error(t, session.accept(signedProbeResponse(t, next, uuid.New(), MonitorStop, private)))
	wrong := next
	wrong.Challenge = supervisionNonce(9)
	require.Error(t, session.accept(signedProbeResponse(t, wrong, incarnation, MonitorStop, private)))
	require.True(t, session.live())
	require.NoError(t, session.accept(signedProbeResponse(t, next, incarnation, MonitorStop, private)))
	require.False(t, session.live())

	newSession, err := newSupervisionSession(supervisionNonce(8), supervisionNonce(2), public, func() time.Time { return now })
	require.NoError(t, err)
	_, err = newSession.issue(supervisionNonce(6))
	require.NoError(t, err)
	require.Error(t, newSession.accept(signedProbeResponse(t, next, incarnation, MonitorStop, private)), "old stop cannot bind a new leader challenge")
}
