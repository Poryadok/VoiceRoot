package sourcecustody

import (
	"bytes"
	"sort"
)

// monitorBatch belongs to a single poller. The store returns only the
// outstanding and previous probes; persisted STOP records must not be erased.
type monitorBatch struct {
	session *supervisionSession
	last    *MonitorResponse
}

func newMonitorBatch(session *supervisionSession) *monitorBatch {
	return &monitorBatch{session: session}
}

func (b *monitorBatch) accept(rows []MonitorResponse) error {
	if b == nil || b.session == nil {
		return ErrUnavailable
	}
	fail := func() error { b.session.revoke(); return ErrUnavailable }
	b.session.mu.Lock()
	valid := b.session.advanceLocked(b.session.clock())
	b.session.mu.Unlock()
	if !valid {
		return ErrUnavailable
	}
	if len(rows) > 4 {
		return fail()
	}
	// STOP takes precedence even if a later heartbeat was returned first.
	for _, row := range rows {
		if row.Kind == MonitorStop {
			if b.session.accept(row) != nil {
				return fail()
			}
			return nil
		}
	}
	ordered := append([]MonitorResponse(nil), rows...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Sequence < ordered[j].Sequence })
	for _, row := range ordered {
		if b.last != nil && sameMonitorResponse(*b.last, row) {
			if !b.session.live() {
				return fail()
			}
			continue
		}
		if b.session.accept(row) != nil {
			return fail()
		}
		row.Signature = append([]byte(nil), row.Signature...)
		b.last = &row
	}
	return nil
}

func sameMonitorResponse(a, b MonitorResponse) bool {
	return a.Challenge == b.Challenge && a.DecisionDigest == b.DecisionDigest && a.Incarnation == b.Incarnation && a.Sequence == b.Sequence && a.Nonce == b.Nonce && a.Kind == b.Kind && bytes.Equal(a.Signature, b.Signature)
}
