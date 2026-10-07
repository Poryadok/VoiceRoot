package spacemedia

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type AdmissionOutboxStore interface {
	ClaimOutbox(context.Context, time.Duration) (*AdmissionOutboxItem, error)
	MarkOutboxDelivered(context.Context, AdmissionOutboxItem) error
	ReleaseOutboxLease(context.Context, AdmissionOutboxItem) error
}

type AdmissionOutboxPublisher interface {
	PublishAdmissionEvent(context.Context, uuid.UUID, string, []byte) error
}

// AdmissionOutboxRelay delivers only journal-approved, post-projection events.
// A lost PubAck may cause a retry, so the event ID and exact payload stay fixed.
type AdmissionOutboxRelay struct {
	Store     AdmissionOutboxStore
	Publisher AdmissionOutboxPublisher
	Every     time.Duration
	Lease     time.Duration
	OnError   func(error)
}

func (r *AdmissionOutboxRelay) DispatchOnce(ctx context.Context) error {
	if r == nil || r.Store == nil || r.Publisher == nil {
		return errors.New("Space media event relay is not configured")
	}
	lease := r.Lease
	if lease <= 0 {
		lease = 30 * time.Second
	}
	item, err := r.Store.ClaimOutbox(ctx, lease)
	if err != nil || item == nil {
		return err
	}
	if err := r.Publisher.PublishAdmissionEvent(ctx, item.ID, item.Subject, item.Payload); err != nil {
		if releaseErr := r.Store.ReleaseOutboxLease(ctx, *item); releaseErr != nil {
			return errors.Join(err, fmt.Errorf("release Space media event lease: %w", releaseErr))
		}
		return err
	}
	return r.Store.MarkOutboxDelivered(ctx, *item)
}

func (r *AdmissionOutboxRelay) Run(ctx context.Context) error {
	if r == nil || r.Store == nil || r.Publisher == nil {
		return errors.New("Space media event relay is not configured")
	}
	every := r.Every
	if every <= 0 {
		every = time.Second
	}
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := r.DispatchOnce(ctx); err != nil && r.OnError != nil {
				r.OnError(err)
			}
		}
	}
}
