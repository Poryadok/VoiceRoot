package sourcecustody

import (
	"context"
	"io"
	"time"
)

// monitorProbeStore is implemented by a dedicated control connection pool.
// Publication must compare exact bytes on an existing key, and reads return
// only the current and last accepted probes' immutable response lanes.
type monitorProbeStore interface {
	PublishProbe(context.Context, supervisionProbe) error
	ReadResponses(context.Context, supervisionProbe, supervisionProbe) ([]MonitorResponse, error)
}

type monitorPoller struct {
	session *supervisionSession
	store   monitorProbeStore
	entropy io.Reader
	batch   *monitorBatch
}

func newMonitorPoller(s *supervisionSession, store monitorProbeStore, entropy io.Reader) *monitorPoller {
	return &monitorPoller{session: s, store: store, entropy: entropy, batch: newMonitorBatch(s)}
}

// step is called by one poller only. Neither session nor controller locks are
// held during I/O. The independent expiry loop remains authoritative while a
// store operation is blocked, and a late response cannot revive the session.
func (p *monitorPoller) step(ctx context.Context) error {
	if p == nil || p.session == nil || p.store == nil || p.entropy == nil || ctx == nil {
		return ErrUnavailable
	}
	fail := func() error { p.session.revoke(); return ErrUnavailable }
	if ctx.Err() != nil {
		return fail()
	}
	s := p.session
	s.mu.Lock()
	now := s.clock()
	valid := s.advanceLocked(now)
	due := s.pending.Sequence == 0 && (s.sequence == 0 || !now.Before(s.issued.Add(supervisionInterval)))
	s.mu.Unlock()
	if !valid {
		return fail()
	}
	if due {
		var nonce [32]byte
		if _, err := io.ReadFull(p.entropy, nonce[:]); err != nil {
			return fail()
		}
		probe, err := s.issue(nonce)
		if err != nil {
			return fail()
		}
		call, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		err = p.store.PublishProbe(call, probe)
		callErr := call.Err()
		cancel()
		if err != nil || callErr != nil || ctx.Err() != nil {
			return fail()
		}
	}
	s.mu.Lock()
	current, last := s.pending, s.lastAccepted
	s.mu.Unlock()
	call, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	rows, err := p.store.ReadResponses(call, current, last)
	callErr := call.Err()
	cancel()
	if err != nil || callErr != nil || ctx.Err() != nil || p.batch.accept(rows) != nil {
		return fail()
	}
	return nil
}

func (p *monitorPoller) run(ctx context.Context, ticks <-chan time.Time) {
	if p == nil || p.session == nil {
		return
	}
	defer p.session.revoke()
	if ctx == nil || ticks == nil || p.step(ctx) != nil {
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-p.session.revoked():
			return
		case _, ok := <-ticks:
			if !ok || p.step(ctx) != nil {
				return
			}
		}
	}
}
