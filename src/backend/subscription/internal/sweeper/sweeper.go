package sweeper

import (
	"context"
	"log/slog"
	"time"

	"voice/backend/subscription/internal/store"
)

// DomainEvents emits subscription lifecycle domain events after sweeper transitions.
type DomainEvents interface {
	PublishPlanExpired(ctx context.Context, accountID, plan string) error
	PublishDowngrade(ctx context.Context, accountID, plan string) error
	PublishSpaceProExpired(ctx context.Context, spaceID string) error
}

// Runner finalizes expired grace periods and ended billing periods.
type Runner struct {
	Store        *store.SubscriptionStore
	DomainEvents DomainEvents
	Logger       *slog.Logger
	Now          func() time.Time
}

// RunOnce processes all due subscription lifecycle transitions.
func (r *Runner) RunOnce(ctx context.Context) error {
	if r == nil || r.Store == nil {
		return nil
	}
	now := time.Now().UTC()
	if r.Now != nil {
		now = r.Now().UTC()
	}

	graceExpired, err := r.Store.ListGracePeriodExpired(ctx, now)
	if err != nil {
		return err
	}
	for _, item := range graceExpired {
		if _, err := r.Store.FinalizeSubscriptionCancellation(ctx, item.ID); err != nil {
			return err
		}
		r.emitExpired(ctx, item.AccountID.String(), item.Plan)
	}

	periodEnded, err := r.Store.ListPeriodEndedCancellations(ctx, now)
	if err != nil {
		return err
	}
	for _, item := range periodEnded {
		if _, err := r.Store.FinalizeSubscriptionCancellation(ctx, item.ID); err != nil {
			return err
		}
		r.emitExpired(ctx, item.AccountID.String(), item.Plan)
	}

	spaceEnded, err := r.Store.ListPendingSpaceProCancellations(ctx, now)
	if err != nil {
		return err
	}
	for _, item := range spaceEnded {
		if _, err := r.Store.FinalizeSpaceProCancellation(ctx, item.ID); err != nil {
			return err
		}
		if r.DomainEvents != nil {
			_ = r.DomainEvents.PublishSpaceProExpired(ctx, item.SpaceID.String())
		}
	}
	return nil
}

func (r *Runner) emitExpired(ctx context.Context, accountID, plan string) {
	if r == nil || r.DomainEvents == nil {
		return
	}
	_ = r.DomainEvents.PublishPlanExpired(ctx, accountID, plan)
	_ = r.DomainEvents.PublishDowngrade(ctx, accountID, plan)
}
