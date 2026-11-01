package sourcecustody

import (
	"context"
	"math"
	"time"
)

type monitorResponseTransport interface {
	ReadProbe(context.Context, [32]byte, [32]byte, uint64) (supervisionProbe, bool, error)
	InsertExactResponse(context.Context, MonitorResponse) error
}

// monitorResponseWorker has one owning goroutine. The supplied health check
// is not custody evidence: production must independently qualify and supervise
// its implementation, credentials, source identity and recovery authority.
// Health loss between samples must be latched by that independent authority.
// The owner must call stop on every exit and arrange independent fencing when
// publication or health fails; step and stop must never run concurrently.
type monitorResponseWorker struct {
	responder *monitorResponder
	store     monitorResponseTransport
	health    func(context.Context) error
	next      uint64
	failed    bool
}

func newMonitorResponseWorker(responder *monitorResponder, store monitorResponseTransport, health func(context.Context) error) *monitorResponseWorker {
	return &monitorResponseWorker{responder: responder, store: store, health: health, next: 1}
}

func (w *monitorResponseWorker) step(ctx context.Context) error {
	if w == nil || w.responder == nil || w.store == nil || w.health == nil || w.failed {
		return ErrUnavailable
	}
	fail := func() error { w.failed = true; return ErrUnavailable }
	if ctx == nil || ctx.Err() != nil {
		return fail()
	}
	checkHealth := func() bool {
		call, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		err := w.health(call)
		callErr := call.Err()
		cancel()
		return err == nil && callErr == nil && ctx.Err() == nil
	}
	if !checkHealth() {
		return fail()
	}
	call, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	probe, found, err := w.store.ReadProbe(call, w.responder.challenge, w.responder.decisionDigest, w.next)
	callErr := call.Err()
	cancel()
	if err != nil || callErr != nil || ctx.Err() != nil || !checkHealth() {
		return fail()
	}
	if !found {
		return nil
	}
	if w.responder.observe(probe) != nil {
		return fail()
	}
	response, err := w.responder.respond(probe)
	if err != nil || ctx.Err() != nil {
		return fail()
	}
	call, cancel = context.WithTimeout(ctx, 500*time.Millisecond)
	err = w.store.InsertExactResponse(call, response)
	callErr = call.Err()
	cancel()
	if err != nil || callErr != nil || ctx.Err() != nil || !checkHealth() {
		return fail()
	}
	if w.next == math.MaxInt64 {
		return fail()
	}
	w.next++
	return nil
}

// stop serializes with signing through the responder and publishes the last
// observed probe's STOP using a fresh bounded context even after cancellation.
func (w *monitorResponseWorker) stop() error {
	if w == nil || w.responder == nil || w.store == nil {
		return ErrUnavailable
	}
	w.failed = true
	response, err := w.responder.stop()
	if err != nil {
		return ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if w.store.InsertExactResponse(ctx, response) != nil || ctx.Err() != nil {
		return ErrUnavailable
	}
	return nil
}
