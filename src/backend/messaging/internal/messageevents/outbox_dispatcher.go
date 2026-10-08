package messageevents

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"time"

	"github.com/google/uuid"
)

type outboxLeaseStore interface {
	ClaimMessageEventOutbox(context.Context, int, time.Duration) ([]OutboxEvent, error)
	RecordMessageEventPubAck(context.Context, uuid.UUID, uuid.UUID, uint64) (bool, error)
	RetryMessageEventOutbox(context.Context, uuid.UUID, uuid.UUID, time.Time) (bool, error)
}

type outboxEventPublisher interface {
	PublishOutboxEvent(context.Context, OutboxEvent) (uint64, error)
}

// OutboxDispatcher retries persisted event bytes until JetStream positively
// acknowledges them. PubAck is broker acceptance, not consumer application.
type OutboxDispatcher struct {
	Store     outboxLeaseStore
	Publisher outboxEventPublisher
	Logger    *slog.Logger
	BatchSize int
	Lease     time.Duration
	Interval  time.Duration
	Now       func() time.Time
}

func (d *OutboxDispatcher) defaults() (int, time.Duration, time.Duration, func() time.Time) {
	batchSize, lease, interval, now := d.BatchSize, d.Lease, d.Interval, d.Now
	if batchSize <= 0 {
		batchSize = 32
	}
	if lease <= 0 {
		lease = 30 * time.Second
	}
	if interval <= 0 {
		interval = time.Second
	}
	if now == nil {
		now = time.Now
	}
	return batchSize, lease, interval, now
}

func (d *OutboxDispatcher) Run(ctx context.Context) error {
	if d == nil || d.Store == nil || d.Publisher == nil {
		return errors.New("message outbox dispatcher dependencies unavailable")
	}
	_, _, interval, _ := d.defaults()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if err := d.DispatchBatch(ctx); err != nil && d.Logger != nil {
			d.Logger.Error("message outbox batch failed", slog.String("error", err.Error()))
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (d *OutboxDispatcher) DispatchBatch(ctx context.Context) error {
	if d == nil || d.Store == nil || d.Publisher == nil {
		return errors.New("message outbox dispatcher dependencies unavailable")
	}
	batchSize, lease, _, now := d.defaults()
	events, err := d.Store.ClaimMessageEventOutbox(ctx, batchSize, lease)
	if err != nil {
		return err
	}
	var failures []error
	for _, event := range events {
		if ctx.Err() != nil {
			return errors.Join(append(failures, ctx.Err())...)
		}
		sequence, publishErr := d.Publisher.PublishOutboxEvent(ctx, event)
		if publishErr == nil && sequence > 0 {
			if _, err := d.Store.RecordMessageEventPubAck(ctx, event.EventID, event.LeaseToken, sequence); err != nil {
				failures = append(failures, err)
				continue
			}
			continue
		}
		if publishErr == nil {
			publishErr = errors.New("JetStream publish returned no positive PubAck sequence")
		}
		delay := outboxRetryDelay(event.Attempts)
		if _, err := d.Store.RetryMessageEventOutbox(ctx, event.EventID, event.LeaseToken, now().Add(delay)); err != nil {
			failures = append(failures, errors.Join(publishErr, err))
			continue
		}
		if d.Logger != nil {
			d.Logger.Warn("message outbox publish will retry", slog.String("event_id", event.EventID.String()), slog.String("subject", event.Subject), slog.String("error", publishErr.Error()))
		}
	}
	return errors.Join(failures...)
}

func outboxRetryDelay(attempt int64) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	shift := min(attempt-1, 6)
	seconds := math.Pow(2, float64(shift))
	if seconds > 60 {
		seconds = 60
	}
	return time.Duration(seconds) * time.Second
}
