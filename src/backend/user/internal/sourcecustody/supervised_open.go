package sourcecustody

import (
	"bytes"
	"context"
	"crypto/sha256"
	"time"
)

// runningSupervision owns its poller, expiry worker and one SQL recovery attempt.
// A cancellation is not proof of recovery; done carries the actual outcome.
type runningSupervision struct {
	session *supervisionSession
	cancel  context.CancelFunc
	done    chan struct{}
	result  error
}

func (client *supervisorClient) renew(ctx context.Context, session *supervisionSession) bool {
	if ctx == nil || ctx.Err() != nil || session == nil {
		return false
	}
	session.mu.Lock()
	now := session.clock()
	valid := session.advanceLocked(now)
	due := session.pending.Sequence == 0 && (session.sequence == 0 || !now.Before(session.issued.Add(supervisionInterval)))
	session.mu.Unlock()
	if !valid {
		return false
	}
	if !due {
		return true
	}
	challenge, err := randomSupervisionProbe(session)
	if err != nil {
		return false
	}
	call, stop := context.WithTimeout(ctx, time.Second)
	response, err := client.probe(call, challenge)
	callErr := call.Err()
	stop()
	return err == nil && callErr == nil && session.accept(response) == nil
}

func (h *activationHandler) beginSupervision(ctx context.Context, claim OpenAttestation) error {
	if h.supervisor == nil || h.supervised != nil || bytes.Equal(h.supervisor.key, h.signer) {
		return ErrUnavailable
	}
	c := h.controller
	c.mu.Lock()
	if c.stopped || c.parent.Err() != nil || c.supervision != nil || c.activationUsed || c.verifyOpenAttestation(claim, h.signer, time.Now()) != nil {
		c.mu.Unlock()
		return ErrUnavailable
	}
	payload, err := OpenAttestationPayload(claim)
	if err != nil {
		c.mu.Unlock()
		return ErrUnavailable
	}
	session, err := newSupervisionSession(claim.Challenge, sha256.Sum256(payload), h.supervisor.key, time.Now)
	if err != nil {
		c.mu.Unlock()
		return ErrUnavailable
	}
	c.supervision = session
	c.mu.Unlock()
	lifetime, cancel := context.WithCancel(c.parent)
	run := &runningSupervision{session: session, cancel: cancel, done: make(chan struct{})}
	h.supervised = run
	go func() {
		defer close(run.done)
		expiryTicks := time.NewTicker(50 * time.Millisecond)
		expiryDone := make(chan struct{})
		go func() { defer close(expiryDone); supervisionExpiryLoop(lifetime, session, expiryTicks.C) }()
		ticks := time.NewTicker(50 * time.Millisecond)
		defer ticks.Stop()
		running := h.supervisor.renew(lifetime, session)
		for running {
			select {
			case <-lifetime.Done():
				running = false
			case <-session.revoked():
				running = false
			case <-ticks.C:
				running = h.supervisor.renew(lifetime, session)
			}
		}
		session.revoke()
		cancel()
		expiryTicks.Stop()
		<-expiryDone
		// Local revocation is already authoritative even if a controller mutex or a
		// PostgreSQL operation prevents the following durable close from settling.
		closed, stopClose := context.WithTimeout(context.Background(), 5*time.Second)
		_ = c.CloseAdmission(closed)
		_ = c.Shutdown(closed)
		stopClose()
		run.result = recoverSupervisedSource(c.pool, h.admin, claim)
	}()
	return session.waitFor(ctx, claim)
}

func (r *runningSupervision) stop() error {
	if r == nil {
		return nil
	}
	r.session.revoke()
	r.cancel()
	// The owner joins before its database pools can be closed or reused. A stuck
	// operation is HOLD, never successful cleanup inferred from a timer.
	<-r.done
	return r.result
}
