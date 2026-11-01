package sourcecustody

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"sync"
	"time"

	"github.com/google/uuid"
)

const supervisionInterval = time.Second
const supervisionDeadline = 3 * time.Second

type supervisionProbe struct {
	Challenge      [32]byte
	DecisionDigest [32]byte
	Sequence       uint64
	Nonce          [32]byte
}

// supervisionSession holds one leader incarnation's local, monotonic lease.
// Its methods never perform database I/O or acquire the controller mutex.
type supervisionSession struct {
	mu             sync.Mutex
	challenge      [32]byte
	decisionDigest [32]byte
	key            ed25519.PublicKey
	clock          func() time.Time
	incarnation    uuid.UUID
	sequence       uint64
	pending        supervisionProbe
	lastAccepted   supervisionProbe
	issued         time.Time
	pendingUntil   time.Time
	validUntil     time.Time
	lastObserved   time.Time
	armed          bool
	dead           bool
	armedCh        chan struct{}
	revokedCh      chan struct{}
}

func newSupervisionSession(challenge, decision [32]byte, key ed25519.PublicKey, clock func() time.Time) (*supervisionSession, error) {
	if challenge == [32]byte{} || decision == [32]byte{} || len(key) != ed25519.PublicKeySize || clock == nil {
		return nil, ErrUnavailable
	}
	return &supervisionSession{challenge: challenge, decisionDigest: decision, key: append(ed25519.PublicKey(nil), key...), clock: clock, armedCh: make(chan struct{}), revokedCh: make(chan struct{})}, nil
}

func (s *supervisionSession) revokeLocked() {
	if !s.dead {
		s.dead = true
		close(s.revokedCh)
	}
}

// advanceLocked must be called for every time-sensitive operation, even when
// the periodic expiry worker has not run. Replaying a stored response never
// changes either deadline.
func (s *supervisionSession) advanceLocked(now time.Time) bool {
	if s.dead {
		return false
	}
	if now.IsZero() || (!s.lastObserved.IsZero() && now.Before(s.lastObserved)) {
		s.revokeLocked()
		return false
	}
	s.lastObserved = now
	if (s.armed && !now.Before(s.validUntil)) || (!s.armed && s.pending.Sequence != 0 && !now.Before(s.pendingUntil)) {
		s.revokeLocked()
		return false
	}
	return true
}

func (s *supervisionSession) issue(nonce [32]byte) (supervisionProbe, error) {
	var empty supervisionProbe
	if s == nil {
		return empty, ErrUnavailable
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.clock()
	if !s.advanceLocked(now) || nonce == [32]byte{} || s.pending.Sequence != 0 {
		return empty, ErrUnavailable
	}
	if s.sequence >= maxMonitorProbeSequence {
		s.revokeLocked()
		return empty, ErrUnavailable
	}
	if s.sequence != 0 && now.Before(s.issued.Add(supervisionInterval)) {
		return empty, ErrUnavailable
	}
	s.sequence++
	s.issued = now
	s.pendingUntil = now.Add(supervisionDeadline)
	s.pending = supervisionProbe{Challenge: s.challenge, DecisionDigest: s.decisionDigest, Sequence: s.sequence, Nonce: nonce}
	return s.pending, nil
}

func (s *supervisionSession) accept(response MonitorResponse) error {
	if s == nil || VerifyMonitorResponse(response, s.key) != nil {
		return ErrUnavailable
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.clock()
	if !s.advanceLocked(now) || response.Challenge != s.challenge || response.DecisionDigest != s.decisionDigest || (s.armed && response.Incarnation != s.incarnation) {
		return ErrUnavailable
	}
	if response.Kind == MonitorStop {
		pending := s.pending.Sequence != 0 && response.Sequence == s.pending.Sequence && response.Nonce == s.pending.Nonce && now.Before(s.pendingUntil)
		last := s.armed && response.Sequence == s.lastAccepted.Sequence && response.Nonce == s.lastAccepted.Nonce
		if !pending && !last {
			return ErrUnavailable
		}
		s.revokeLocked()
		return nil
	}
	if s.pending.Sequence == 0 || response.Sequence != s.pending.Sequence || response.Nonce != s.pending.Nonce || !now.Before(s.pendingUntil) {
		return ErrUnavailable
	}
	if (!s.armed && response.Kind != MonitorArmed) || (s.armed && response.Kind != MonitorHeartbeat) {
		return ErrUnavailable
	}
	if !s.armed {
		s.incarnation = response.Incarnation
		s.armed = true
		close(s.armedCh)
	}
	// The final valid heartbeat never extends admission. The responder can
	// still publish STOP for this retained probe while recovery closes it.
	if response.Sequence == maxMonitorProbeSequence {
		s.lastAccepted = s.pending
		s.pending = supervisionProbe{}
		s.revokeLocked()
		return nil
	}
	s.validUntil = s.pendingUntil
	s.lastAccepted = s.pending
	s.pending = supervisionProbe{}
	return nil
}

func (s *supervisionSession) live() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.advanceLocked(s.clock()) && s.armed
}

// liveFor binds admission to the full canonical opening decision, rather
// than allowing supervision of one source or writer to authorize another.
func (s *supervisionSession) liveFor(claim OpenAttestation) bool {
	if s == nil {
		return false
	}
	payload, err := OpenAttestationPayload(claim)
	if err != nil {
		return false
	}
	digest := sha256.Sum256(payload)
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.advanceLocked(s.clock()) && s.armed && s.challenge == claim.Challenge && s.decisionDigest == digest
}

// waitFor never holds a controller or session mutex while waiting. Arming is
// a notification only: the exact decision and deadline are checked again
// after wakeup, including when expiry and arming race.
func (s *supervisionSession) waitFor(ctx context.Context, claim OpenAttestation) error {
	if s == nil || ctx == nil || ctx.Err() != nil {
		return ErrUnavailable
	}
	payload, err := OpenAttestationPayload(claim)
	if err != nil || s.challenge != claim.Challenge || s.decisionDigest != sha256.Sum256(payload) {
		return ErrUnavailable
	}
	select {
	case <-ctx.Done():
		return ErrUnavailable
	case <-s.revoked():
		return ErrUnavailable
	case <-s.armedCh:
		if ctx.Err() != nil || !s.liveFor(claim) || ctx.Err() != nil {
			return ErrUnavailable
		}
		return nil
	}
}

func (s *supervisionSession) revoke() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.revokeLocked()
}

func (s *supervisionSession) revoked() <-chan struct{} { return s.revokedCh }

// supervisionExpiryLoop makes expiration visible without request traffic. It
// does no database I/O and takes neither the controller nor operator mutex.
func supervisionExpiryLoop(ctx context.Context, session *supervisionSession, ticks <-chan time.Time) {
	if session == nil {
		return
	}
	defer session.revoke()
	if ctx == nil || ticks == nil {
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-session.revoked():
			return
		case _, ok := <-ticks:
			if !ok {
				return
			}
			_ = session.live()
		}
	}
}
