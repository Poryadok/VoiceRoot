package store

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	rows, err := tx.Query(ctx, `
SELECT ss.id, ss.space_id
FROM space_subscriptions ss
WHERE ss.status = 'pending_cancel' AND ss.current_period_end <= $1`, now.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var candidates []ExpiredSpaceSubscription
	for rows.Next() {
		var item ExpiredSpaceSubscription
		if err := rows.Scan(&item.ID, &item.SpaceID); err != nil {
			return nil, err
		}
		candidates = append(candidates, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].SpaceID == candidates[j].SpaceID {
			return candidates[i].ID.String() < candidates[j].ID.String()
		}
		return candidates[i].SpaceID.String() < candidates[j].SpaceID.String()
	})
	var out []ExpiredSpaceSubscription
	for _, item := range candidates {
		allowed, err := lockReadableSpaceLifecycleTx(ctx, tx, item.SpaceID)
		if err != nil {
			return nil, err
		}
		if !allowed {
			continue
		}
		var lockedID uuid.UUID
		err = tx.QueryRow(ctx, `
SELECT id FROM space_subscriptions
WHERE id=$1 AND space_id=$2 AND status='pending_cancel' AND current_period_end <= $3
FOR SHARE`, item.ID, item.SpaceID, now.UTC()).Scan(&lockedID)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
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
