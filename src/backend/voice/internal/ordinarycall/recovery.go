package ordinarycall

import (
	"context"
	"errors"
	"time"
)

type RecoveryQueue interface {
	ClaimRecovery(context.Context, time.Duration) (*RecoveryItem, error)
	ReleaseRecovery(context.Context, RecoveryItem, time.Duration, bool) error
}

// RecoverOne performs one bounded, durably scheduled repair. The caller controls
// worker lifetime/tick cadence and records returned errors in service telemetry.
// A nil item means the queue is currently empty; it is not a lifecycle success.
func (c *Coordinator) RecoverOne(ctx context.Context, queue RecoveryQueue) (bool, error) {
	if !c.ready() || queue == nil {
		return false, ErrUnavailable
	}
	lease := 3 * c.timeout()
	if lease > time.Minute {
		lease = time.Minute
	}
	item, err := queue.ClaimRecovery(ctx, lease)
	if err != nil || item == nil {
		return false, err
	}
	workCtx, cancel := context.WithTimeout(ctx, 2*c.timeout())
	err = c.Reconcile(workCtx, item.RoomID)
	cancel()
	delay := 5 * time.Second
	if err != nil {
		shift := item.Attempts
		if shift > 5 {
			shift = 5
		}
		if shift < 1 {
			shift = 1
		}
		delay = time.Second * time.Duration(1<<shift)
	}
	// Scheduling repair must survive caller cancellation. Unknown lease expiry
	// is harmless: another worker repeats exact-generation idempotent effects.
	releaseCtx, releaseCancel := context.WithTimeout(context.Background(), c.timeout())
	defer releaseCancel()
	return true, errors.Join(err, queue.ReleaseRecovery(releaseCtx, *item, delay, err == nil))
}
