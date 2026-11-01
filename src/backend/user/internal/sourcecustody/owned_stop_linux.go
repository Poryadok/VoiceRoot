//go:build linux

package sourcecustody

import (
	"context"
	"os"
	"sync"
	"time"
)

// ownedSourceStop is private and unwired. Its own future launch path is closed
// permanently before dispatch, but it does not control any external launcher.
// A separately enforced boot-default DENY, source/volume binding, launcher
// isolation and independent watchdog are required. Neither this result nor an
// empty cgroup is a physical-fence receipt or launch authorization.
//
// The journal and fresh child are created together and retained for process
// lifetime. Every cooperating generation writer must take the journal flock;
// trusted host provisioning must prevent directory/lock replacement and bypass.
// No close, journal retry, new generation or directory reuse is safe while an
// asynchronous worker can remain. Both kernel I/O and cgroup stop can hang;
// independent supervision remains mandatory even after a coordinator timeout.
type ownedSourceStop struct {
	journal  *denyJournal
	child    *owner
	sequence *stopSequence
}

func newOwnedSourceStop(dir, root *os.File, record denyRecord) (*ownedSourceStop, error) {
	if !record.valid() {
		return nil, errDenyUnavailable
	}
	j, err := openDenyJournal(dir)
	if err != nil {
		return nil, errDenyUnavailable
	}
	child, err := newOwner(root)
	if err != nil {
		j.close()
		return nil, errDenyUnavailable
	}
	o := &ownedSourceStop{journal: j, child: child}
	o.sequence = &stopSequence{wait: 5 * time.Second,
		persist: func() error { return j.persistDeny(record) },
		stop:    child.stop}
	return o, nil
}

func (o *ownedSourceStop) stop(ctx context.Context) error {
	if o == nil || o.sequence == nil {
		return errDenyUnavailable
	}
	return o.sequence.execute(ctx)
}

// stopSequence is a one-shot coordinator, never an authority. Its function
// fields allow isolated ordering/failure tests; production constructor binds
// them only to its own journal and freshly created cgroup.
type stopSequence struct {
	mu      sync.Mutex
	started bool
	wait    time.Duration
	persist func() error
	stop    func(context.Context) error
}

func (s *stopSequence) execute(ctx context.Context) error {
	if s == nil {
		return errDenyUnavailable
	}
	s.mu.Lock()
	if s.started || s.wait <= 0 || s.persist == nil || s.stop == nil {
		s.mu.Unlock()
		return errDenyUnavailable
	}
	// This is an irreversible close of this coordinator's own future launch
	// path, before either operation is dispatched. External gates are separate.
	s.started = true
	s.mu.Unlock()
	deadline := time.Now().Add(s.wait)
	persistDone, stopDone := make(chan error, 1), make(chan error, 1)
	// Dispatch the DENY attempt first, without waiting for its scheduling or I/O.
	go func() { persistDone <- s.persist() }()
	go func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), s.wait)
		defer cancel()
		stopDone <- s.stop(stopCtx)
	}()
	if ctx == nil {
		return errDenyUnavailable
	}
	timer := time.NewTimer(time.Until(deadline))
	defer timer.Stop()
	var persistErr, stopErr error
	for persistDone != nil || stopDone != nil {
		select {
		case persistErr = <-persistDone:
			persistDone = nil
		case stopErr = <-stopDone:
			stopDone = nil
		case <-ctx.Done():
			return errDenyUnavailable
		case <-timer.C:
			return errDenyUnavailable
		}
	}
	if ctx.Err() != nil || !time.Now().Before(deadline) || persistErr != nil || stopErr != nil {
		return errDenyUnavailable
	}
	return nil
}
