package ordinarycall

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"time"
)

type OutboxStore interface {
	ClaimOutbox(context.Context, time.Duration) (*OutboxItem, error)
	MarkDelivered(context.Context, OutboxItem) error
	ReleaseOutbox(context.Context, OutboxItem, time.Time) error
}
type OutboxPublisher interface {
	PublishOrdinaryEvent(context.Context, uuid.UUID, string, []byte) error
}
type OutboxRelay struct {
	Store     OutboxStore
	Publisher OutboxPublisher
	OnError   func(error)
}

func (r *OutboxRelay) DispatchOnce(ctx context.Context) (bool, error) {
	if r == nil || r.Store == nil || r.Publisher == nil {
		return false, ErrUnavailable
	}
	item, err := r.Store.ClaimOutbox(ctx, 15*time.Second)
	if err != nil || item == nil {
		return false, err
	}
	send, cancel := context.WithTimeout(ctx, 5*time.Second)
	err = r.Publisher.PublishOrdinaryEvent(send, item.ID, item.Subject, item.Payload)
	cancel()
	// An unknown PubAck is a retry of immutable saved bytes. Scheduling/ack must
	// survive cancellation of the dispatch caller; a stale lease cannot acknowledge.
	finish, done := context.WithTimeout(context.Background(), 2*time.Second)
	defer done()
	if err != nil {
		return true, errors.Join(err, r.Store.ReleaseOutbox(finish, *item, time.Now().UTC().Add(5*time.Second)))
	}
	return true, r.Store.MarkDelivered(finish, *item)
}

func (r *OutboxRelay) Run(ctx context.Context) error {
	if r == nil || r.Store == nil || r.Publisher == nil {
		return ErrUnavailable
	}
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
			if _, err := r.DispatchOnce(ctx); err != nil && r.OnError != nil {
				r.OnError(err)
			}
		}
	}
}

func (c *Coordinator) RunRecovery(ctx context.Context, queue RecoveryQueue, onError func(error)) error {
	if !c.ready() || queue == nil {
		return ErrUnavailable
	}
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
			if _, err := c.RecoverOne(ctx, queue); err != nil && onError != nil {
				onError(err)
			}
		}
	}
}
