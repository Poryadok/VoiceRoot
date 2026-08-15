package store

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ExpiredSubscription holds a subscription row eligible for lifecycle finalization.
type ExpiredSubscription struct {
	ID        uuid.UUID
	AccountID uuid.UUID
	Plan      string
}

// ExpiredSpaceSubscription holds a space subscription row eligible for expiry.
type ExpiredSpaceSubscription struct {
	ID      uuid.UUID
	SpaceID uuid.UUID
}

// ListPendingSpaceProCancellations returns space subs marked pending_cancel past period end.
func (s *SubscriptionStore) ListPendingSpaceProCancellations(ctx context.Context, now time.Time) ([]ExpiredSpaceSubscription, error) {
	rows, err := s.Pool.Query(ctx, `
SELECT id, space_id
FROM space_subscriptions
WHERE status = 'pending_cancel' AND current_period_end <= $1`, now.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ExpiredSpaceSubscription
	for rows.Next() {
		var item ExpiredSpaceSubscription
		if err := rows.Scan(&item.ID, &item.SpaceID); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// ListGracePeriodExpired returns grace_period subscriptions past grace_period_end.
func (s *SubscriptionStore) ListGracePeriodExpired(ctx context.Context, now time.Time) ([]ExpiredSubscription, error) {
	rows, err := s.Pool.Query(ctx, `
SELECT id, account_id, plan
FROM subscriptions
WHERE status = 'grace_period' AND grace_period_end IS NOT NULL AND grace_period_end <= $1`, now.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ExpiredSubscription
	for rows.Next() {
		var item ExpiredSubscription
		if err := rows.Scan(&item.ID, &item.AccountID, &item.Plan); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// ListPeriodEndedCancellations returns active subscriptions with cancelled_at set past current_period_end.
func (s *SubscriptionStore) ListPeriodEndedCancellations(ctx context.Context, now time.Time) ([]ExpiredSubscription, error) {
	rows, err := s.Pool.Query(ctx, `
SELECT id, account_id, plan
FROM subscriptions
WHERE status = 'active' AND cancelled_at IS NOT NULL AND current_period_end <= $1`, now.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ExpiredSubscription
	for rows.Next() {
		var item ExpiredSubscription
		if err := rows.Scan(&item.ID, &item.AccountID, &item.Plan); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
