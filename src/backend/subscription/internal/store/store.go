package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const gracePeriodDays = 7

var ErrDuplicateBillingEvent = errors.New("duplicate billing event")

// SubscriptionStore persists account and space subscriptions.
type SubscriptionStore struct {
	Pool *pgxpool.Pool
}

type SubscriptionRow struct {
	ID                     uuid.UUID
	AccountID              uuid.UUID
	Plan                   string
	BillingPeriod          string
	Status                 string
	Provider               string
	ProviderSubscriptionID string
	CurrentPeriodStart     time.Time
	CurrentPeriodEnd       time.Time
	GracePeriodEnd         *time.Time
	CancelledAt            *time.Time
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

type SpaceSubscriptionRow struct {
	ID                     uuid.UUID
	SpaceID                uuid.UUID
	PurchaserAccountID     uuid.UUID
	Plan                   string
	BillingPeriod          string
	Status                 string
	Provider               string
	ProviderSubscriptionID string
	CurrentPeriodStart     time.Time
	CurrentPeriodEnd       time.Time
	GracePeriodEnd         *time.Time
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

type BillingEventRow struct {
	ID                  uuid.UUID
	SubscriptionID      *uuid.UUID
	SpaceSubscriptionID *uuid.UUID
	Type                string
	Provider            string
	ProviderEventID     string
	Details             json.RawMessage
	CreatedAt           time.Time
}

func (s *SubscriptionStore) GetSubscriptionByAccountID(ctx context.Context, accountID uuid.UUID) (*SubscriptionRow, error) {
	row := s.Pool.QueryRow(ctx, `
SELECT id, account_id, plan, billing_period, status, provider, provider_subscription_id,
	current_period_start, current_period_end, grace_period_end, cancelled_at, created_at, updated_at
FROM subscriptions
WHERE account_id = $1
ORDER BY created_at DESC
LIMIT 1`, accountID)
	return scanSubscription(row)
}

func (s *SubscriptionStore) ActivatePremium(ctx context.Context, accountID uuid.UUID, providerEventID string, details json.RawMessage) (*SubscriptionRow, error) {
	now := time.Now().UTC()
	periodEnd := now.AddDate(0, 1, 0)
	subID := uuid.New()
	providerSubID := "paddle_" + providerEventID

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if inserted, err := insertBillingEventTx(ctx, tx, nil, nil, "subscription.activated", "paddle", providerEventID, details); err != nil {
		return nil, err
	} else if !inserted {
		return nil, ErrDuplicateBillingEvent
	}

	_, err = tx.Exec(ctx, `DELETE FROM subscriptions WHERE account_id = $1`, accountID)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `DELETE FROM subscriptions WHERE account_id = $1`, accountID)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `
INSERT INTO subscriptions (
	id, account_id, plan, billing_period, status, provider, provider_subscription_id,
	current_period_start, current_period_end
) VALUES ($1, $2, 'premium', 'monthly', 'active', 'paddle', $3, $4, $5)`,
		subID, accountID, providerSubID, now, periodEnd)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetSubscriptionByAccountID(ctx, accountID)
}

func (s *SubscriptionStore) MarkPaymentFailed(ctx context.Context, accountID uuid.UUID, providerEventID string, details json.RawMessage) (*SubscriptionRow, error) {
	now := time.Now().UTC()
	graceEnd := now.Add(gracePeriodDays * 24 * time.Hour)

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
	if inserted, err := insertBillingEventTx(ctx, tx, subID, nil, "subscription.payment_failed", "paddle", providerEventID, details); err != nil {
		return nil, err
	} else if !inserted {
		return nil, ErrDuplicateBillingEvent
	}

	_, err = tx.Exec(ctx, `
UPDATE subscriptions
SET status = 'grace_period', grace_period_end = $2, updated_at = now()
WHERE id = $1`, sub.ID, graceEnd)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetSubscriptionByAccountID(ctx, accountID)
}

func (s *SubscriptionStore) ActivateSpacePro(ctx context.Context, spaceID, purchaserID uuid.UUID, providerEventID string, details json.RawMessage) (*SpaceSubscriptionRow, error) {
	now := time.Now().UTC()
	periodEnd := now.AddDate(0, 1, 0)
	subID := uuid.New()
	providerSubID := "paddle_space_" + providerEventID

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if inserted, err := insertBillingEventTx(ctx, tx, nil, nil, "subscription.activated", "paddle", providerEventID, details); err != nil {
		return nil, err
	} else if !inserted {
		return nil, ErrDuplicateBillingEvent
	}

	_, err = tx.Exec(ctx, `DELETE FROM space_subscriptions WHERE space_id = $1`, spaceID)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `
INSERT INTO space_subscriptions (
	id, space_id, purchaser_account_id, plan, billing_period, status, provider, provider_subscription_id,
	current_period_start, current_period_end
) VALUES ($1, $2, $3, 'space_pro', 'monthly', 'active', 'paddle', $4, $5, $6)`,
		subID, spaceID, purchaserID, providerSubID, now, periodEnd)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetSpaceSubscriptionBySpaceID(ctx, spaceID)
}

func (s *SubscriptionStore) GetSpaceSubscriptionBySpaceID(ctx context.Context, spaceID uuid.UUID) (*SpaceSubscriptionRow, error) {
	row := s.Pool.QueryRow(ctx, `
SELECT id, space_id, purchaser_account_id, plan, billing_period, status, provider, provider_subscription_id,
	current_period_start, current_period_end, grace_period_end, created_at, updated_at
FROM space_subscriptions
WHERE space_id = $1
ORDER BY created_at DESC
LIMIT 1`, spaceID)
	return scanSpaceSubscription(row)
}

func (s *SubscriptionStore) HasActiveSpaceProForPurchaser(ctx context.Context, accountID uuid.UUID) (bool, error) {
	var exists bool
	err := s.Pool.QueryRow(ctx, `
SELECT EXISTS (
	SELECT 1 FROM space_subscriptions
	WHERE purchaser_account_id = $1 AND status IN ('active', 'grace_period')
)`, accountID).Scan(&exists)
	return exists, err
}

func (s *SubscriptionStore) EffectiveAccountTier(ctx context.Context, accountID uuid.UUID) (string, error) {
	sub, err := s.GetSubscriptionByAccountID(ctx, accountID)
	if err != nil {
		return "", err
	}
	if sub == nil {
		return "free", nil
	}
	switch sub.Status {
	case "active":
		return "premium", nil
	case "grace_period":
		return "grace_period", nil
	default:
		return "free", nil
	}
}

func insertBillingEventTx(ctx context.Context, tx pgx.Tx, subID, spaceSubID *uuid.UUID, eventType, provider, providerEventID string, details json.RawMessage) (bool, error) {
	if len(details) == 0 {
		details = json.RawMessage(`{}`)
	}
	tag, err := tx.Exec(ctx, `
INSERT INTO billing_events (subscription_id, space_subscription_id, type, provider, provider_event_id, details)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (provider, provider_event_id) DO NOTHING`,
		subID, spaceSubID, eventType, provider, providerEventID, details)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func scanSubscription(row pgx.Row) (*SubscriptionRow, error) {
	var r SubscriptionRow
	err := row.Scan(
		&r.ID, &r.AccountID, &r.Plan, &r.BillingPeriod, &r.Status, &r.Provider, &r.ProviderSubscriptionID,
		&r.CurrentPeriodStart, &r.CurrentPeriodEnd, &r.GracePeriodEnd, &r.CancelledAt, &r.CreatedAt, &r.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func scanSpaceSubscription(row pgx.Row) (*SpaceSubscriptionRow, error) {
	var r SpaceSubscriptionRow
	err := row.Scan(
		&r.ID, &r.SpaceID, &r.PurchaserAccountID, &r.Plan, &r.BillingPeriod, &r.Status, &r.Provider, &r.ProviderSubscriptionID,
		&r.CurrentPeriodStart, &r.CurrentPeriodEnd, &r.GracePeriodEnd, &r.CreatedAt, &r.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}
