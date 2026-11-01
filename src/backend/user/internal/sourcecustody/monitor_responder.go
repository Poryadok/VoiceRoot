package sourcecustody

import (
	"crypto/ed25519"
	"crypto/subtle"
	"math"
	"sync"

	"github.com/google/uuid"
)

// monitorResponder serializes observation, heartbeat signing and shutdown.
// Its caller must independently verify the operator decision and source before
// constructing it; this object does not qualify custody or database access.
type monitorResponder struct {
	mu             sync.Mutex
	challenge      [32]byte
	decisionDigest [32]byte
	incarnation    uuid.UUID
	key            ed25519.PrivateKey
	latest         supervisionProbe
	responded      bool
	stopped        bool
	stopResponse   MonitorResponse
}

func newMonitorResponder(challenge, decision [32]byte, incarnation uuid.UUID, key ed25519.PrivateKey) (*monitorResponder, error) {
	if challenge == [32]byte{} || decision == [32]byte{} || incarnation == uuid.Nil || len(key) != ed25519.PrivateKeySize {
		return nil, ErrUnavailable
	}
	canonical := ed25519.NewKeyFromSeed(key[:ed25519.SeedSize])
	valid := subtle.ConstantTimeCompare(canonical, key) == 1
	clear(canonical)
	if !valid {
		return nil, ErrUnavailable
	}
	return &monitorResponder{challenge: challenge, decisionDigest: decision, incarnation: incarnation, key: append(ed25519.PrivateKey(nil), key...)}, nil
}

func (r *monitorResponder) observe(probe supervisionProbe) error {
	if r == nil {
		return ErrUnavailable
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stopped || probe.Challenge != r.challenge || probe.DecisionDigest != r.decisionDigest || probe.Nonce == [32]byte{} || probe.Sequence == 0 || probe.Sequence > math.MaxInt64 {
		return ErrUnavailable
	}
	if probe == r.latest {
		return nil
	}
	if r.latest.Sequence == 0 {
		if probe.Sequence != 1 {
			return ErrUnavailable
		}
	} else if !r.responded || r.latest.Sequence == math.MaxInt64 || probe.Sequence != r.latest.Sequence+1 {
		return ErrUnavailable
	}
	r.latest = probe
	r.responded = false
	return nil
}

func (r *monitorResponder) respond(expected supervisionProbe) (MonitorResponse, error) {
	if r == nil {
		return MonitorResponse{}, ErrUnavailable
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stopped || r.latest.Sequence == 0 || expected != r.latest {
		return MonitorResponse{}, ErrUnavailable
	}
	kind := MonitorHeartbeat
	if r.latest.Sequence == 1 {
		kind = MonitorArmed
	}
	response, err := r.signLocked(kind)
	if err != nil {
		return MonitorResponse{}, err
	}
	r.responded = true
	return response, nil
}

func (r *monitorResponder) stop() (MonitorResponse, error) {
	if r == nil {
		return MonitorResponse{}, ErrUnavailable
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stopped {
		if len(r.stopResponse.Signature) != ed25519.SignatureSize {
			return MonitorResponse{}, ErrUnavailable
		}
		response := r.stopResponse
		response.Signature = append([]byte(nil), response.Signature...)
		return response, nil
	}
	r.stopped = true
	defer clear(r.key)
	if r.latest.Sequence == 0 {
		return MonitorResponse{}, ErrUnavailable
	}
	response, err := r.signLocked(MonitorStop)
	if err != nil {
		return MonitorResponse{}, err
	}
	r.stopResponse = response
	response.Signature = append([]byte(nil), response.Signature...)
	return response, nil
}

func (r *monitorResponder) signLocked(kind MonitorResponseKind) (MonitorResponse, error) {
	return SignMonitorResponse(MonitorResponse{Challenge: r.challenge, DecisionDigest: r.decisionDigest, Incarnation: r.incarnation, Sequence: r.latest.Sequence, Nonce: r.latest.Nonce, Kind: kind}, r.key)
}
