package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrSubscriptionNotFound = errors.New("subscription not found")
	ErrSubscriptionOwner    = errors.New("subscription does not belong to account")
	ErrSubscriptionState    = errors.New("subscription state does not allow operation")
)

// GetSubscriptionByID loads a subscription row by primary key.
func (s *SubscriptionStore) GetSubscriptionByID(ctx context.Context, id uuid.UUID) (*SubscriptionRow, error) {
	row := s.Pool.QueryRow(ctx, `
SELECT id, account_id, plan, billing_period, status, provider, provider_subscription_id,
	current_period_start, current_period_end, grace_period_end, cancelled_at, created_at, updated_at
FROM subscriptions
WHERE id = $1`, id)
	return scanSubscription(row)
}

// HasActiveSpaceProForSpace reports whether the given space has active or grace_period Space Pro.
func (s *SubscriptionStore) HasActiveSpaceProForSpace(ctx context.Context, spaceID uuid.UUID) (bool, error) {
	var exists bool
	err := s.Pool.QueryRow(ctx, `
SELECT EXISTS (
	SELECT 1 FROM space_subscriptions
	WHERE space_id = $1 AND status IN ('active', 'grace_period')
)`, spaceID).Scan(&exists)
	return exists, err
}

// CancelSubscriptionByID marks a subscription for cancellation at period end.
func (s *SubscriptionStore) CancelSubscriptionByID(ctx context.Context, subID, accountID uuid.UUID) (*SubscriptionRow, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	sub, err := scanSubscription(tx.QueryRow(ctx, `
SELECT id, account_id, plan, billing_period, status, provider, provider_subscription_id,
	current_period_start, current_period_end, grace_period_end, cancelled_at, created_at, updated_at
FROM subscriptions
WHERE id = $1
FOR UPDATE`, subID))
	if err != nil {
		return nil, err
	}
	if sub == nil {
		return nil, ErrSubscriptionNotFound
	}
	if sub.AccountID != accountID {
		return nil, ErrSubscriptionOwner
	}
	if sub.Status != "active" && sub.Status != "grace_period" {
		return nil, ErrSubscriptionState
	}
	now := time.Now().UTC()
	_, err = tx.Exec(ctx, `
UPDATE subscriptions
SET cancelled_at = $2, updated_at = now()
WHERE id = $1`, sub.ID, now)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetSubscriptionByID(ctx, subID)
}

// ResumeSubscriptionByID clears a pending cancellation when still entitled.
func (s *SubscriptionStore) ResumeSubscriptionByID(ctx context.Context, subID, accountID uuid.UUID) (*SubscriptionRow, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	sub, err := scanSubscription(tx.QueryRow(ctx, `
SELECT id, account_id, plan, billing_period, status, provider, provider_subscription_id,
	current_period_start, current_period_end, grace_period_end, cancelled_at, created_at, updated_at
FROM subscriptions
WHERE id = $1
FOR UPDATE`, subID))
	if err != nil {
		return nil, err
	}
	if sub == nil {
		return nil, ErrSubscriptionNotFound
	}
	if sub.AccountID != accountID {
		return nil, ErrSubscriptionOwner
	}
	if sub.CancelledAt == nil {
		return nil, ErrSubscriptionState
	}
	if sub.Status != "active" && sub.Status != "grace_period" {
		return nil, ErrSubscriptionState
	}
	_, err = tx.Exec(ctx, `
UPDATE subscriptions
SET cancelled_at = NULL, updated_at = now()
WHERE id = $1`, sub.ID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetSubscriptionByID(ctx, subID)
}

// RenewPremium extends the billing period after a successful renewal webhook.
func (s *SubscriptionStore) RenewPremium(ctx context.Context, accountID uuid.UUID, providerEventID string, details json.RawMessage, periodEnd time.Time) (*SubscriptionRow, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	sub, err := scanSubscription(tx.QueryRow(ctx, `
SELECT id, account_id, plan, billing_period, status, provider, provider_subscription_id,
	current_period_start, current_period_end, grace_period_end, cancelled_at, created_at, updated_at
FROM subscriptions
WHERE account_id = $1
ORDER BY created_at DESC
LIMIT 1
FOR UPDATE`, accountID))
	if err != nil {
		return nil, err
	}
	if sub == nil {
		return nil, pgx.ErrNoRows
	}

	subID := &sub.ID
	if inserted, err := insertBillingEventTx(ctx, tx, subID, nil, "subscription.renewed", "paddle", providerEventID, details); err != nil {
		return nil, err
	} else if !inserted {
		return nil, ErrDuplicateBillingEvent
	}

	_, err = tx.Exec(ctx, `
UPDATE subscriptions
SET status = 'active', grace_period_end = NULL, current_period_end = $2, updated_at = now()
WHERE id = $1`, sub.ID, periodEnd.UTC())
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetSubscriptionByAccountID(ctx, accountID)
}

// MarkSubscriptionCancelled marks a subscription cancelled at period end via provider webhook.
func (s *SubscriptionStore) MarkSubscriptionCancelled(ctx context.Context, accountID uuid.UUID, providerEventID string, details json.RawMessage) (*SubscriptionRow, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	sub, err := scanSubscription(tx.QueryRow(ctx, `
SELECT id, account_id, plan, billing_period, status, provider, provider_subscription_id,
	current_period_start, current_period_end, grace_period_end, cancelled_at, created_at, updated_at
FROM subscriptions
WHERE account_id = $1
ORDER BY created_at DESC
LIMIT 1
FOR UPDATE`, accountID))
	if err != nil {
		return nil, err
	}
	if sub == nil {
		return nil, pgx.ErrNoRows
	}

	subID := &sub.ID
	if inserted, err := insertBillingEventTx(ctx, tx, subID, nil, "subscription.cancelled", "paddle", providerEventID, details); err != nil {
		return nil, err
	} else if !inserted {
		return nil, ErrDuplicateBillingEvent
	}

	now := time.Now().UTC()
	_, err = tx.Exec(ctx, `
UPDATE subscriptions
SET cancelled_at = COALESCE(cancelled_at, $2), updated_at = now()
WHERE id = $1`, sub.ID, now)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetSubscriptionByAccountID(ctx, accountID)
}

// PauseSubscription marks a subscription paused by the provider.
func (s *SubscriptionStore) PauseSubscription(ctx context.Context, accountID uuid.UUID, providerEventID string, details json.RawMessage) (*SubscriptionRow, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	sub, err := scanSubscription(tx.QueryRow(ctx, `
SELECT id, account_id, plan, billing_period, status, provider, provider_subscription_id,
	current_period_start, current_period_end, grace_period_end, cancelled_at, created_at, updated_at
FROM subscriptions
WHERE account_id = $1
ORDER BY created_at DESC
LIMIT 1
FOR UPDATE`, accountID))
	if err != nil {
		return nil, err
	}
	if sub == nil {
		return nil, pgx.ErrNoRows
	}

	subID := &sub.ID
	if inserted, err := insertBillingEventTx(ctx, tx, subID, nil, "subscription.paused", "paddle", providerEventID, details); err != nil {
		return nil, err
	} else if !inserted {
		return nil, ErrDuplicateBillingEvent
	}

	_, err = tx.Exec(ctx, `
UPDATE subscriptions
SET status = 'paused', updated_at = now()
WHERE id = $1`, sub.ID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetSubscriptionByAccountID(ctx, accountID)
}

// FinalizeSubscriptionCancellation sets status cancelled for an account subscription.
func (s *SubscriptionStore) FinalizeSubscriptionCancellation(ctx context.Context, subID uuid.UUID) (*SubscriptionRow, error) {
	_, err := s.Pool.Exec(ctx, `
UPDATE subscriptions
SET status = 'cancelled', grace_period_end = NULL, updated_at = now()
WHERE id = $1`, subID)
	if err != nil {
		return nil, err
	}
	return s.GetSubscriptionByID(ctx, subID)
}

// MarkSpaceProCancelled marks a space subscription for cancellation at period end.
func (s *SubscriptionStore) MarkSpaceProCancelled(ctx context.Context, spaceID uuid.UUID, providerEventID string, details json.RawMessage) (*SpaceSubscriptionRow, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	row, err := scanSpaceSubscription(tx.QueryRow(ctx, `
SELECT id, space_id, purchaser_account_id, plan, billing_period, status, provider, provider_subscription_id,
	current_period_start, current_period_end, grace_period_end, created_at, updated_at
FROM space_subscriptions
WHERE space_id = $1
ORDER BY created_at DESC
LIMIT 1
FOR UPDATE`, spaceID))
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, pgx.ErrNoRows
	}

	spaceSubID := &row.ID
	if inserted, err := insertBillingEventTx(ctx, tx, nil, spaceSubID, "subscription.cancelled", "paddle", providerEventID, details); err != nil {
		return nil, err
	} else if !inserted {
		return nil, ErrDuplicateBillingEvent
	}

	now := time.Now().UTC()
	newStatus := "pending_cancel"
	if !row.CurrentPeriodEnd.After(now) {
		newStatus = "cancelled"
	}
	_, err = tx.Exec(ctx, `
UPDATE space_subscriptions
SET status = $2, updated_at = now()
WHERE id = $1`, row.ID, newStatus)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetSpaceSubscriptionBySpaceID(ctx, spaceID)
}

// FinalizeSpaceProCancellation sets status cancelled for a space subscription.
func (s *SubscriptionStore) FinalizeSpaceProCancellation(ctx context.Context, spaceSubID uuid.UUID) (*SpaceSubscriptionRow, error) {
	_, err := s.Pool.Exec(ctx, `
UPDATE space_subscriptions
SET status = 'cancelled', grace_period_end = NULL, updated_at = now()
WHERE id = $1`, spaceSubID)
	if err != nil {
		return nil, err
	}
	row := s.Pool.QueryRow(ctx, `
SELECT id, space_id, purchaser_account_id, plan, billing_period, status, provider, provider_subscription_id,
	current_period_start, current_period_end, grace_period_end, created_at, updated_at
FROM space_subscriptions
WHERE id = $1`, spaceSubID)
	return scanSpaceSubscription(row)
}
